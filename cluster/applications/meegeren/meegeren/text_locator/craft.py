# CRAFT text detector, laid out to load the weights published with https://github.com/clovaai/CRAFT-pytorch (MIT) and redistributed by EasyOCR
import collections
import zipfile

import numpy
import PIL.Image
import torch
import torch.nn
import torch.nn.functional
import torchvision.models

import meegeren.offload
import meegeren.telemetry
import meegeren.text_locator

# VGG16 downsamples five times
CRAFT_PAD_MODULO = 32
# Upscaling small images makes small glyphs detectable, as CRAFT's own demo does
CRAFT_MAGNIFICATION = 1.5
# Longer sides are detected downscaled to bound inference time
CRAFT_MAX_SIZE = 1280
CRAFT_MEAN = torch.tensor([0.485, 0.456, 0.406]).view(1, 3, 1, 1)
CRAFT_STD = torch.tensor([0.229, 0.224, 0.225]).view(1, 3, 1, 1)


class Vgg16Bn(torch.nn.Module):
    def __init__(self):
        super().__init__()
        features = torchvision.models.vgg16_bn(weights=None).features

        def layers(start: int, stop: int) -> torch.nn.Sequential:
            # Layers keep their index in vgg16_bn, as the published weights name them
            return torch.nn.Sequential(
                collections.OrderedDict(
                    (str(i), features[i]) for i in range(start, stop)
                )
            )

        self.slice1 = layers(0, 12)  # conv2_2
        self.slice2 = layers(12, 19)  # conv3_3
        self.slice3 = layers(19, 29)  # conv4_3
        self.slice4 = layers(29, 39)  # conv5_3
        # fc6 and fc7 as convolutions
        self.slice5 = torch.nn.Sequential(
            torch.nn.MaxPool2d(kernel_size=3, stride=1, padding=1),
            torch.nn.Conv2d(512, 1024, kernel_size=3, padding=6, dilation=6),
            torch.nn.Conv2d(1024, 1024, kernel_size=1),
        )

    def forward(self, x: torch.Tensor) -> list[torch.Tensor]:
        relu2_2 = self.slice1(x)
        relu3_2 = self.slice2(relu2_2)
        relu4_3 = self.slice3(relu3_2)
        relu5_3 = self.slice4(relu4_3)
        fc7 = self.slice5(relu5_3)
        return [fc7, relu5_3, relu4_3, relu3_2, relu2_2]


class DoubleConv(torch.nn.Module):
    def __init__(self, in_channels: int, mid_channels: int, out_channels: int):
        super().__init__()
        self.conv = torch.nn.Sequential(
            torch.nn.Conv2d(in_channels + mid_channels, mid_channels, kernel_size=1),
            torch.nn.BatchNorm2d(mid_channels),
            torch.nn.ReLU(inplace=True),
            torch.nn.Conv2d(mid_channels, out_channels, kernel_size=3, padding=1),
            torch.nn.BatchNorm2d(out_channels),
            torch.nn.ReLU(inplace=True),
        )

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        return self.conv(x)


class Craft(torch.nn.Module):
    def __init__(self):
        super().__init__()
        self.basenet = Vgg16Bn()
        self.upconv1 = DoubleConv(1024, 512, 256)
        self.upconv2 = DoubleConv(512, 256, 128)
        self.upconv3 = DoubleConv(256, 128, 64)
        self.upconv4 = DoubleConv(128, 64, 32)
        self.conv_cls = torch.nn.Sequential(
            torch.nn.Conv2d(32, 32, kernel_size=3, padding=1),
            torch.nn.ReLU(inplace=True),
            torch.nn.Conv2d(32, 32, kernel_size=3, padding=1),
            torch.nn.ReLU(inplace=True),
            torch.nn.Conv2d(32, 16, kernel_size=3, padding=1),
            torch.nn.ReLU(inplace=True),
            torch.nn.Conv2d(16, 16, kernel_size=1),
            torch.nn.ReLU(inplace=True),
            torch.nn.Conv2d(16, 2, kernel_size=1),
        )

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        sources = self.basenet(x)
        y = self.upconv1(torch.cat([sources[0], sources[1]], dim=1))
        for upconv, source in zip(
            [self.upconv2, self.upconv3, self.upconv4], sources[2:], strict=True
        ):
            y = torch.nn.functional.interpolate(
                y, size=source.shape[2:], mode="bilinear", align_corners=False
            )
            y = upconv(torch.cat([y, source], dim=1))
        # (batch, 2, height / 2, width / 2) of region and affinity scores
        return self.conv_cls(y)


class CraftTextLocator(meegeren.text_locator.TextLocator):
    def __init__(
        self,
        model: torch.nn.Module,
        region_threshold: float,
        affinity_threshold: float,
        dtype: torch.dtype,
        keep_on_device: bool,
        device: str,
    ):
        self.model = model.to(device if keep_on_device else "cpu", dtype).eval()
        self.region_threshold = region_threshold
        self.affinity_threshold = affinity_threshold
        self.dtype = dtype
        self.keep_on_device = keep_on_device
        self.device = device

    @classmethod
    def from_file(
        cls,
        path: str,
        region_threshold: float,
        affinity_threshold: float,
        dtype: torch.dtype,
        keep_on_device: bool,
        device: str,
    ) -> CraftTextLocator:
        # EasyOCR publishes the weights as the only member of a zip archive
        with zipfile.ZipFile(path) as archive:
            (member,) = archive.namelist()
            with archive.open(member) as f:
                state = torch.load(f, map_location="cpu", weights_only=True)
        model = Craft()
        # The weights were saved from torch.nn.DataParallel
        model.load_state_dict({k.removeprefix("module."): v for k, v in state.items()})
        return cls(
            model=model,
            region_threshold=region_threshold,
            affinity_threshold=affinity_threshold,
            dtype=dtype,
            keep_on_device=keep_on_device,
            device=device,
        )

    @torch.inference_mode()
    def locate(self, image: PIL.Image.Image) -> numpy.ndarray:
        with meegeren.telemetry.tracer.start_as_current_span("craft.locate"):
            longest = max(image.width, image.height)
            scale = min(CRAFT_MAGNIFICATION, CRAFT_MAX_SIZE / longest)
            width = max(1, round(image.width * scale))
            height = max(1, round(image.height * scale))

            # numpy.asarray of a PIL image is read-only, which torch.from_numpy warns on
            x = torch.from_numpy(
                numpy.array(
                    image.resize((width, height), PIL.Image.Resampling.BILINEAR)
                )
            )
            x = x.permute(2, 0, 1)[None].float() / 255
            x = (x - CRAFT_MEAN) / CRAFT_STD
            x = torch.nn.functional.pad(
                x, (0, -width % CRAFT_PAD_MODULO, 0, -height % CRAFT_PAD_MODULO)
            )

            with meegeren.offload.placed(self.device, self.keep_on_device, self.model):
                scores = self.model(x.to(self.device, self.dtype)).float().cpu()
            scores = scores[:, :, : (height + 1) // 2, : (width + 1) // 2]
            scores = torch.nn.functional.interpolate(
                scores,
                size=(image.height, image.width),
                mode="bilinear",
                align_corners=False,
            )[0]

            # Affinity links neighboring characters, so words are masked as a whole
            mask = (scores[0] > self.region_threshold) | (
                scores[1] > self.affinity_threshold
            )
            return mask.numpy()
