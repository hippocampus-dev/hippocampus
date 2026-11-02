import abc

import numpy
import PIL.Image


class Matting(abc.ABC):
    @abc.abstractmethod
    def matte(
        self, image: PIL.Image.Image, mask: numpy.ndarray
    ) -> tuple[numpy.ndarray, PIL.Image.Image]:
        # The (height, width) opacity in [0, 1] of what mask covers, decided past its edge
        # image with the partly opaque pixels' colors freed of what was behind them
        raise NotImplementedError
