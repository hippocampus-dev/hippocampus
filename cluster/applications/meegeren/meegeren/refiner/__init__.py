import abc
import threading

import PIL.Image

import meegeren.model
import meegeren.upscaler


class Refiner(abc.ABC):
    @abc.abstractmethod
    def refine(
        self,
        source: PIL.Image.Image,
        image: PIL.Image.Image,
        configuration: meegeren.model.SdxlInpaintingConfiguration,
        upscaler: meegeren.upscaler.Upscaler | None,
        cancelled: threading.Event,
    ) -> PIL.Image.Image:
        # image is source after inpainting
        # What image changed from source is redrawn
        # upscaler enlarges the redrawing where it comes out meegeren.inpainter.UPSCALER_MIN_RATIO times smaller than the box it fills or more
        raise NotImplementedError
