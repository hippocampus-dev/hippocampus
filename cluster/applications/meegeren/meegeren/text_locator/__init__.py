import abc

import numpy
import PIL.Image


class TextLocator(abc.ABC):
    @abc.abstractmethod
    def locate(self, image: PIL.Image.Image) -> numpy.ndarray:
        raise NotImplementedError
