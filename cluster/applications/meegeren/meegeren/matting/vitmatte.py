import numpy
import PIL.Image
import torch
import torch.nn.functional
import transformers

import meegeren.imaging
import meegeren.matting
import meegeren.offload
import meegeren.telemetry

# The backbone's global attention over a whole 1920x2880 photo took 10.4GiB more than a 20GB GPU had left
VITMATTE_TILE_SIZE = 1024
VITMATTE_TILE_OVERLAP = 128
# https://github.com/Photoroom/fast-foreground-estimation, whose box blurs of 90 and 6 pixels are rounded up to odd sizes to stay centered
FOREGROUND_BLUR_SIZES = (91, 7)


class VitMatteMatting(meegeren.matting.Matting):
    def __init__(
        self,
        processor: transformers.VitMatteImageProcessor,
        model: torch.nn.Module,
        trimap_width: int,
        keep_on_device: bool,
        device: str,
    ):
        self.processor = processor
        self.model = model.to(device if keep_on_device else "cpu").eval()
        self.trimap_width = trimap_width
        self.keep_on_device = keep_on_device
        self.device = device

    @classmethod
    def from_pretrained(
        cls,
        model: str,
        trimap_width: int,
        dtype: torch.dtype,
        keep_on_device: bool,
        device: str,
    ) -> VitMatteMatting:
        return cls(
            processor=transformers.VitMatteImageProcessor.from_pretrained(model),
            model=transformers.VitMatteForImageMatting.from_pretrained(
                model, dtype=dtype
            ),
            trimap_width=trimap_width,
            keep_on_device=keep_on_device,
            device=device,
        )

    @torch.inference_mode()
    def matte(
        self, image: PIL.Image.Image, mask: numpy.ndarray
    ) -> tuple[numpy.ndarray, PIL.Image.Image]:
        with meegeren.telemetry.tracer.start_as_current_span("vitmatte.matte"):
            foreground = ~meegeren.imaging.dilate(~mask, self.trimap_width)
            unknown = meegeren.imaging.dilate(mask, self.trimap_width) & ~foreground
            trimap = PIL.Image.fromarray(
                numpy.where(foreground, 255, numpy.where(unknown, 128, 0)).astype(
                    numpy.uint8
                )
            )
            height, width = mask.shape
            blended = numpy.zeros((height, width), dtype=numpy.float32)
            weights = numpy.zeros((height, width), dtype=numpy.float32)
            step = VITMATTE_TILE_SIZE - VITMATTE_TILE_OVERLAP
            with meegeren.offload.placed(self.device, self.keep_on_device, self.model):
                for start_top in range(0, max(1, height - VITMATTE_TILE_OVERLAP), step):
                    for start_left in range(
                        0, max(1, width - VITMATTE_TILE_OVERLAP), step
                    ):
                        # Tiles at the bottom and right edges are shifted back to full size
                        top = min(start_top, max(0, height - VITMATTE_TILE_SIZE))
                        left = min(start_left, max(0, width - VITMATTE_TILE_SIZE))
                        bottom = min(height, top + VITMATTE_TILE_SIZE)
                        right = min(width, left + VITMATTE_TILE_SIZE)
                        box = (left, top, right, bottom)
                        if not unknown[top:bottom, left:right].any():
                            continue
                        inputs = self.processor(
                            images=image.crop(box),
                            trimaps=trimap.crop(box),
                            return_tensors="pt",
                        ).to(self.device, self.model.dtype)
                        # The processor pads the tile at its bottom and right
                        alphas = (
                            self.model(**inputs)
                            .alphas[0, 0, : bottom - top, : right - left]
                            .float()
                            .cpu()
                            .numpy()
                        )
                        ramps = [
                            numpy.minimum(
                                numpy.arange(size) + 1, numpy.arange(size)[::-1] + 1
                            ).clip(max=VITMATTE_TILE_OVERLAP)
                            for size in (bottom - top, right - left)
                        ]
                        weight = numpy.outer(*ramps).astype(numpy.float32)
                        blended[top:bottom, left:right] += alphas * weight
                        weights[top:bottom, left:right] += weight
            alpha = numpy.where(
                unknown,
                blended / numpy.maximum(weights, numpy.finfo(numpy.float32).tiny),
                foreground,
            ).astype(numpy.float32)

            def blur(x: torch.Tensor, size: int) -> torch.Tensor:
                # A box blur is separable, and pixels past the image are left out
                vertical = torch.nn.functional.avg_pool2d(
                    x,
                    kernel_size=(size, 1),
                    stride=1,
                    padding=(size // 2, 0),
                    count_include_pad=False,
                )
                return torch.nn.functional.avg_pool2d(
                    vertical,
                    kernel_size=(1, size),
                    stride=1,
                    padding=(0, size // 2),
                    count_include_pad=False,
                )

            # Fully opaque pixels come out as they are, so only what the blurs reach from the partly opaque ones is estimated
            margin = sum(size // 2 for size in FOREGROUND_BLUR_SIZES)
            left, top, right, bottom = meegeren.imaging.bounding_box(unknown)
            left, top = max(0, left - margin), max(0, top - margin)
            right, bottom = min(width, right + margin), min(height, bottom + margin)
            result = numpy.array(image)
            x = (
                torch.from_numpy(result[top:bottom, left:right])
                .to(self.device)
                .permute(2, 0, 1)[None]
                .float()
                / 255
            )
            a = torch.from_numpy(alpha[top:bottom, left:right]).to(self.device)[
                None, None
            ]
            estimated, behind = x, x
            for size in FOREGROUND_BLUR_SIZES:
                blurred_alpha = blur(a, size)
                blurred = blur(estimated * a, size) / (blurred_alpha + 1e-5)
                behind = blur(behind * (1 - a), size) / (1 - blurred_alpha + 1e-5)
                estimated = (blurred + a * (x - a * blurred - (1 - a) * behind)).clamp(
                    0, 1
                )
            result[top:bottom, left:right] = (
                (estimated[0] * 255)
                .round()
                .to(torch.uint8)
                .permute(1, 2, 0)
                .cpu()
                .numpy()
            )
            return alpha, PIL.Image.fromarray(result)
