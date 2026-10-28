import threading

import numpy
import PIL.Image
import spandrel
import torch

import meegeren.interruption
import meegeren.offload
import meegeren.telemetry
import meegeren.upscaler

# Tiles bound GPU memory on large images, overlapping so that their seams do not show
SWINIR_TILE = 256
SWINIR_TILE_OVERLAP = 16


class SwinirUpscaler(meegeren.upscaler.Upscaler):
    def __init__(
        self, model: spandrel.ImageModelDescriptor, keep_on_device: bool, device: str
    ):
        self.model = model.to(device if keep_on_device else "cpu").eval()
        self.scale = model.scale
        self.keep_on_device = keep_on_device
        self.device = device

    @classmethod
    def from_file(cls, path: str, keep_on_device: bool, device: str) -> SwinirUpscaler:
        return cls(
            model=spandrel.ModelLoader().load_from_state_dict(
                torch.load(path, map_location="cpu", weights_only=True)
            ),
            keep_on_device=keep_on_device,
            device=device,
        )

    @torch.inference_mode()
    def upscale(
        self, image: PIL.Image.Image, cancelled: threading.Event
    ) -> PIL.Image.Image:
        with (
            meegeren.telemetry.tracer.start_as_current_span("swinir.upscale"),
            meegeren.offload.placed(self.device, self.keep_on_device, self.model.model),
            meegeren.interruption.interruptible(cancelled, self.model.model),
        ):
            # numpy.asarray of a PIL image is read-only, which torch.from_numpy warns on
            x = (
                torch.from_numpy(numpy.array(image)).permute(2, 0, 1)[None].float()
                / 255
            )
            upscaled = numpy.zeros(
                (image.height * self.scale, image.width * self.scale, 3),
                dtype=numpy.uint8,
            )
            for top in range(0, image.height, SWINIR_TILE):
                for left in range(0, image.width, SWINIR_TILE):
                    if cancelled.is_set():
                        return image
                    bottom = min(image.height, top + SWINIR_TILE)
                    right = min(image.width, left + SWINIR_TILE)
                    padded_top = max(0, top - SWINIR_TILE_OVERLAP)
                    padded_left = max(0, left - SWINIR_TILE_OVERLAP)
                    padded_bottom = min(image.height, bottom + SWINIR_TILE_OVERLAP)
                    padded_right = min(image.width, right + SWINIR_TILE_OVERLAP)
                    y = self.model(
                        x[:, :, padded_top:padded_bottom, padded_left:padded_right].to(
                            self.device
                        )
                    )[
                        0,
                        :,
                        (top - padded_top) * self.scale : (bottom - padded_top)
                        * self.scale,
                        (left - padded_left) * self.scale : (right - padded_left)
                        * self.scale,
                    ]
                    upscaled[
                        top * self.scale : bottom * self.scale,
                        left * self.scale : right * self.scale,
                    ] = (
                        (y.clamp(0, 1) * 255)
                        .round()
                        .to(torch.uint8)
                        .permute(1, 2, 0)
                        .cpu()
                        .numpy()
                    )
        if cancelled.is_set():
            return image
        return PIL.Image.fromarray(upscaled)
