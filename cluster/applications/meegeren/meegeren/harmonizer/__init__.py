import abc
import threading

import numpy
import PIL.Image


class Harmonizer(abc.ABC):
    @abc.abstractmethod
    def harmonize(
        self,
        foreground: PIL.Image.Image,
        alpha: numpy.ndarray,
        background: PIL.Image.Image,
        prompt: str,
        cancelled: threading.Event,
    ) -> PIL.Image.Image:
        # foreground is laid over background by alpha, and comes back with its colors or light brought to those of background
        # prompt describes background
        raise NotImplementedError
