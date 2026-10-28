import abc
import threading

import PIL.Image


class Upscaler(abc.ABC):
    scale: int

    @abc.abstractmethod
    def upscale(
        self, image: PIL.Image.Image, cancelled: threading.Event
    ) -> PIL.Image.Image:
        raise NotImplementedError
