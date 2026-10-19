import threading

import numpy
import PIL.Image
import torch

import meegeren.inpainter
import meegeren.offload
import meegeren.telemetry
import meegeren.upscaler

# big-lama downsamples three times
LAMA_PAD_MODULO = 8
# Longer sides are inpainted downscaled to bound inference time
LAMA_MAX_SIZE = 1024


class LamaInpainter(meegeren.inpainter.Inpainter):
    pastes_back = True

    def __init__(self, lama: torch.nn.Module, keep_on_device: bool, device: str):
        self.lama = lama.to(device if keep_on_device else "cpu").eval()
        self.keep_on_device = keep_on_device
        self.device = device

    @classmethod
    def from_file(cls, path: str, keep_on_device: bool, device: str) -> LamaInpainter:
        return cls(
            lama=torch.jit.load(path, map_location="cpu"),
            keep_on_device=keep_on_device,
            device=device,
        )

    @torch.inference_mode()
    def inpaint(
        self,
        image: PIL.Image.Image,
        masks: list[numpy.ndarray],
        feathering: int,
        upscaler: meegeren.upscaler.Upscaler | None,
        cancelled: threading.Event,
    ) -> PIL.Image.Image:
        with meegeren.telemetry.tracer.start_as_current_span("lama.inpaint"):
            # big-lama fills whatever is masked, so every target is filled at once
            mask = numpy.logical_or.reduce(masks)
            box, crop, crop_mask = meegeren.inpainter.crop(image, mask)

            # Unmasked pixels keep full resolution
            scale = min(1.0, LAMA_MAX_SIZE / max(crop.width, crop.height))
            width = max(1, round(crop.width * scale))
            height = max(1, round(crop.height * scale))
            resized_image = numpy.asarray(
                crop.resize((width, height), PIL.Image.Resampling.LANCZOS)
            )
            resized_mask = (
                numpy.asarray(
                    PIL.Image.fromarray(crop_mask.astype(numpy.uint8) * 255).resize(
                        (width, height), PIL.Image.Resampling.BILINEAR
                    )
                )
                > 0
            )

            pad_height = -height % LAMA_PAD_MODULO
            pad_width = -width % LAMA_PAD_MODULO
            padded_image = numpy.pad(
                resized_image,
                ((0, pad_height), (0, pad_width), (0, 0)),
                mode="symmetric",
            )
            padded_mask = numpy.pad(
                resized_mask, ((0, pad_height), (0, pad_width)), mode="symmetric"
            )

            x = (
                torch.from_numpy(padded_image).permute(2, 0, 1)[None].float() / 255
            ).to(self.device)
            m = torch.from_numpy(padded_mask)[None, None].float().to(self.device)
            with meegeren.offload.placed(self.device, self.keep_on_device, self.lama):
                y = self.lama(x, m)[0, :, :height, :width].cpu()
            inpainted = (
                (y.clamp(0, 1) * 255).round().to(torch.uint8).permute(1, 2, 0).numpy()
            )

            return meegeren.inpainter.paste_back(
                image,
                box,
                meegeren.inpainter.enlarge(
                    PIL.Image.fromarray(inpainted), box, upscaler, cancelled
                ),
                crop_mask,
                feathering,
            )
