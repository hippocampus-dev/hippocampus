import abc

import numpy
import PIL.Image


def parse_labels(prompt: str) -> list[str]:
    # Phrases are separated with periods, e.g. "power line. person.", as Grounding DINO expects
    return [label.strip() for label in prompt.split(".") if label.strip() != ""]


class Locator(abc.ABC):
    @abc.abstractmethod
    def locate(
        self, image: PIL.Image.Image, labels: list[str], threshold: float
    ) -> list[tuple[int, numpy.ndarray]]:
        # Each object scoring above threshold comes with the index of the label naming it and its mask
        raise NotImplementedError
