import threading

import numpy
import PIL.Image
import spandrel
import torch

import meegeren.interruption
import meegeren.offload
import meegeren.telemetry
import meegeren.upscaler


class RealEsrganUpscaler(meegeren.upscaler.Upscaler):
    def __init__(
        self,
        model: spandrel.ImageModelDescriptor,
        dtype: torch.dtype,
        keep_on_device: bool,
        device: str,
    ):
        self.model = model.to(device if keep_on_device else "cpu", dtype).eval()
        self.scale = model.scale
        self.dtype = dtype
        self.keep_on_device = keep_on_device
        self.device = device

    @classmethod
    def from_file(
        cls,
        path: str,
        weak_denoise_path: str,
        denoise_strength: float,
        dtype: torch.dtype,
        keep_on_device: bool,
        device: str,
    ) -> RealEsrganUpscaler:
        strong = torch.load(path, map_location="cpu", weights_only=True)["params"]
        weak = torch.load(weak_denoise_path, map_location="cpu", weights_only=True)[
            "params"
        ]
        # https://github.com/xinntao/Real-ESRGAN/blob/master/realesrgan/utils.py
        interpolated = {
            name: denoise_strength * weight + (1 - denoise_strength) * weak[name]
            for name, weight in strong.items()
        }
        return cls(
            model=spandrel.ModelLoader().load_from_state_dict({"params": interpolated}),
            dtype=dtype,
            keep_on_device=keep_on_device,
            device=device,
        )

    @torch.inference_mode()
    def upscale(
        self, image: PIL.Image.Image, cancelled: threading.Event
    ) -> PIL.Image.Image:
        with meegeren.telemetry.tracer.start_as_current_span("real_esrgan.upscale"):
            # numpy.asarray of a PIL image is read-only, which torch.from_numpy warns on
            x = (
                torch.from_numpy(numpy.array(image)).permute(2, 0, 1)[None].float()
                / 255
            )
            with (
                meegeren.offload.placed(
                    self.device, self.keep_on_device, self.model.model
                ),
                meegeren.interruption.interruptible(cancelled, self.model.model),
            ):
                y = self.model(x.to(self.device, self.dtype))[0].float()
            if cancelled.is_set():
                return image
            return PIL.Image.fromarray(
                (y.clamp(0, 1) * 255)
                .round()
                .to(torch.uint8)
                .permute(1, 2, 0)
                .cpu()
                .numpy()
            )
