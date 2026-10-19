import abc
import threading
import typing

import numpy
import PIL.Image

import meegeren.imaging
import meegeren.upscaler

CONTEXT_MARGIN = 0.5
# Below this ratio, Lanczos kept more detail than Real-ESRGAN in the photos e2e.sh sends
UPSCALER_MIN_RATIO = 1.5


class Inpainter(abc.ABC):
    # inpaint pastes the filling back by masks, rather than by a mask of its own or over the whole image
    pastes_back: bool

    @abc.abstractmethod
    def inpaint(
        self,
        image: PIL.Image.Image,
        masks: list[numpy.ndarray],
        feathering: int,
        upscaler: meegeren.upscaler.Upscaler | None,
        cancelled: threading.Event,
    ) -> PIL.Image.Image:
        # One mask per target
        # upscaler enlarges the filling where it comes out UPSCALER_MIN_RATIO times smaller than the box it fills or more
        # The filling blends into image over up to feathering pixels inside the edge of masks, if pastes_back
        raise NotImplementedError


class Painter(abc.ABC):
    @abc.abstractmethod
    def paint(
        self,
        image: PIL.Image.Image,
        mask: numpy.ndarray,
        prompt: str,
        mode: typing.Literal["replace", "insert", "background"],
        feathering: int,
        upscaler: meegeren.upscaler.Upscaler | None,
        cancelled: threading.Event,
    ) -> PIL.Image.Image:
        # replace draws prompt in place of what mask covers
        # insert draws prompt among what mask covers
        # background draws prompt as the scene behind what mask leaves out
        # upscaler enlarges the drawing where it comes out UPSCALER_MIN_RATIO times smaller than the box it fills or more
        # The drawing blends into image over up to feathering pixels inside the edge of mask
        raise NotImplementedError


def crop_box(mask: numpy.ndarray) -> tuple[int, int, int, int]:
    left, top, right, bottom = meegeren.imaging.bounding_box(mask)
    margin = round(max(bottom - top, right - left) * CONTEXT_MARGIN)
    return (
        max(0, left - margin),
        max(0, top - margin),
        min(mask.shape[1], right + margin),
        min(mask.shape[0], bottom + margin),
    )


def crop(
    image: PIL.Image.Image, mask: numpy.ndarray
) -> tuple[tuple[int, int, int, int], PIL.Image.Image, numpy.ndarray]:
    box = crop_box(mask)
    return box, image.crop(box), mask[box[1] : box[3], box[0] : box[2]]


def composite(
    image: PIL.Image.Image,
    box: tuple[int, int, int, int],
    filled: numpy.ndarray,
    alpha: numpy.ndarray,
) -> PIL.Image.Image:
    left, top, right, bottom = box
    result = numpy.array(image)
    weight = alpha.astype(numpy.float32)[..., None]
    region = result[top:bottom, left:right]
    result[top:bottom, left:right] = (
        (filled * weight + region * (1 - weight)).round().astype(numpy.uint8)
    )
    return PIL.Image.fromarray(result)


def enlarge(
    filled: PIL.Image.Image,
    box: tuple[int, int, int, int],
    upscaler: meegeren.upscaler.Upscaler | None,
    cancelled: threading.Event,
) -> PIL.Image.Image:
    left, top, right, bottom = box
    if (
        upscaler is None
        or max((right - left) / filled.width, (bottom - top) / filled.height)
        < UPSCALER_MIN_RATIO
    ):
        return filled
    return upscaler.upscale(filled, cancelled)


def paste_back(
    image: PIL.Image.Image,
    box: tuple[int, int, int, int],
    filled: PIL.Image.Image,
    mask: numpy.ndarray,
    feathering: int,
) -> PIL.Image.Image:
    left, top, right, bottom = box
    return composite(
        image,
        box,
        numpy.asarray(
            filled.resize((right - left, bottom - top), PIL.Image.Resampling.LANCZOS)
        ),
        meegeren.imaging.feather(mask, feathering),
    )
