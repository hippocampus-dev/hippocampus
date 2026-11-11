# PCT-Net's ViT variant, laid out to load the weights published with https://github.com/rakutentech/pct-net-image-harmonization (MPL-2.0)
import threading

import numpy
import PIL.Image
import torch
import torch.nn
import torch.nn.functional

import meegeren.harmonizer
import meegeren.imaging
import meegeren.offload
import meegeren.telemetry

PCT_NET_RESOLUTION = 256
PCT_NET_PATCH_SIZE = 4
PCT_NET_DIMENSION = 256
# A 3x3 matrix and an offset per pixel
PCT_NET_PARAMETERS = 12
PCT_NET_BAND_HEIGHT = 512


class Encoder(torch.nn.Module):
    def __init__(self):
        super().__init__()
        # The first layer of the published model rearranges pixels into patches, which forward does
        self.patch_to_embedding = torch.nn.Sequential(
            torch.nn.Identity(),
            torch.nn.Linear(PCT_NET_PATCH_SIZE**2 * 4, PCT_NET_DIMENSION),
        )
        self.transformer_enc = torch.nn.TransformerEncoder(
            torch.nn.TransformerEncoderLayer(
                PCT_NET_DIMENSION,
                nhead=2,
                dim_feedforward=PCT_NET_DIMENSION * 2,
                activation="gelu",
            ),
            num_layers=9,
            enable_nested_tensor=False,
        )
        self.dec = torch.nn.Sequential(
            torch.nn.ConvTranspose2d(
                PCT_NET_DIMENSION,
                PCT_NET_PARAMETERS,
                kernel_size=PCT_NET_PATCH_SIZE,
                stride=PCT_NET_PATCH_SIZE,
            ),
            torch.nn.Tanh(),
        )

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        n, c, h, w = x.shape
        size = PCT_NET_PATCH_SIZE
        patches = (
            x.reshape(n, c, h // size, size, w // size, size)
            .permute(0, 2, 4, 3, 5, 1)
            .reshape(n, (h // size) * (w // size), size * size * c)
        )
        # The published model takes the sequence first
        content = self.transformer_enc(
            self.patch_to_embedding(patches).permute(1, 0, 2)
        )
        return self.dec(
            content.permute(1, 2, 0).reshape(n, PCT_NET_DIMENSION, h // size, w // size)
        )


class PctNet(torch.nn.Module):
    def __init__(self):
        super().__init__()
        self.encoder = Encoder()
        self.get_params = torch.nn.Conv2d(
            PCT_NET_PARAMETERS, PCT_NET_PARAMETERS, kernel_size=1
        )

    def forward(self, image: torch.Tensor, mask: torch.Tensor) -> torch.Tensor:
        return self.get_params(self.encoder(torch.cat((image, mask), dim=1)))


class PctNetHarmonizer(meegeren.harmonizer.Harmonizer):
    def __init__(
        self, model: PctNet, dtype: torch.dtype, keep_on_device: bool, device: str
    ):
        self.model = model.to(device if keep_on_device else "cpu", dtype).eval()
        self.dtype = dtype
        self.keep_on_device = keep_on_device
        self.device = device

    @classmethod
    def from_file(
        cls, path: str, dtype: torch.dtype, keep_on_device: bool, device: str
    ) -> PctNetHarmonizer:
        model = PctNet()
        model.load_state_dict(torch.load(path, map_location="cpu", weights_only=True))
        return cls(
            model=model, dtype=dtype, keep_on_device=keep_on_device, device=device
        )

    @torch.inference_mode()
    def harmonize(
        self,
        foreground: PIL.Image.Image,
        alpha: numpy.ndarray,
        background: PIL.Image.Image,
        prompt: str,
        cancelled: threading.Event,
    ) -> PIL.Image.Image:
        with meegeren.telemetry.tracer.start_as_current_span("pct_net.harmonize"):
            weight = alpha[..., None]
            composite = PIL.Image.fromarray(
                (
                    numpy.asarray(foreground) * weight
                    + numpy.asarray(background) * (1 - weight)
                )
                .round()
                .astype(numpy.uint8)
            )
            size = (PCT_NET_RESOLUTION, PCT_NET_RESOLUTION)
            x = (
                torch.from_numpy(
                    numpy.array(composite.resize(size, PIL.Image.Resampling.BILINEAR))
                )
                .permute(2, 0, 1)[None]
                .float()
                / 255
            )
            m = (
                torch.from_numpy(
                    numpy.array(
                        PIL.Image.fromarray(
                            (alpha * 255).round().astype(numpy.uint8)
                        ).resize(size, PIL.Image.Resampling.BILINEAR)
                    )
                )[None, None].float()
                / 255
            )
            with meegeren.offload.placed(self.device, self.keep_on_device, self.model):
                parameters = self.model(
                    x.to(self.device, self.dtype), m.to(self.device, self.dtype)
                ).float()

            # Only what alpha shows is transformed, so the parameters are upsampled within its box alone, a band of rows at a time
            left, top, right, bottom = meegeren.imaging.bounding_box(alpha > 0)
            height, width = alpha.shape
            columns = (
                torch.arange(left, right, device=self.device) + 0.5
            ) / width * 2 - 1
            result = numpy.array(foreground)
            for start in range(top, bottom, PCT_NET_BAND_HEIGHT):
                end = min(bottom, start + PCT_NET_BAND_HEIGHT)
                rows = (
                    torch.arange(start, end, device=self.device) + 0.5
                ) / height * 2 - 1
                grid = torch.stack(torch.meshgrid(columns, rows, indexing="xy"), dim=-1)
                # Bicubic as the published model upsamples them, which grid_sample matches with border padding
                upsampled = torch.nn.functional.grid_sample(
                    parameters,
                    grid[None],
                    mode="bicubic",
                    padding_mode="border",
                    align_corners=False,
                )[0]
                pixels = (
                    torch.from_numpy(result[start:end, left:right])
                    .to(self.device)
                    .permute(2, 0, 1)
                    .float()
                    / 255
                )
                transformed = (
                    torch.einsum(
                        "ijhw,jhw->ihw",
                        upsampled[:9].reshape(3, 3, *pixels.shape[1:]),
                        pixels,
                    )
                    + upsampled[9:]
                ).clamp(0, 1)
                result[start:end, left:right] = (
                    (transformed * 255)
                    .round()
                    .to(torch.uint8)
                    .permute(1, 2, 0)
                    .cpu()
                    .numpy()
                )
            return PIL.Image.fromarray(result)
