import enum

import pydantic
import pydantic.alias_generators

MANIFEST = "manifest.jsonl"
REGIONS = "regions.jsonl"
REJECTED = "rejected.txt"
JUDGMENTS = "judgments.jsonl"
NHR = "nhr"
PAIRS = "pairs.jsonl"


# Values of the pixels in each mask
class Region(enum.IntEnum):
    OTHER = 0
    WALL = 1
    FLOOR = 2
    EXCLUDED = 3


# https://schema.org/ImageObject
class ImageObject(pydantic.BaseModel):
    model_config = pydantic.ConfigDict(
        alias_generator=pydantic.alias_generators.to_camel, populate_by_name=True
    )

    url: str
    content_url: str
    creator: str | None
    license: str
    is_part_of: str
    sha256: str
    width: int | None
    height: int | None


# Fractions of the photo each region covers in its mask
class Regions(pydantic.BaseModel):
    sha256: str
    wall: float
    floor: float
    excluded: float


# An edit of NHR-Edit adding or removing one object, ordered as the image with it and the one without
class Pair(pydantic.BaseModel):
    sample_id: int
    repository: str
    file: str
    group: int
    category: str
    kind: str
    style: str
    edit_instruction: str
    original_prompt: str
    difference_threshold: int
    # Fraction of the image where the two differ by more than difference_threshold, which the mask covers
    mask_fraction: float


# What the Decisions API answered for a pair
class PairJudgment(pydantic.BaseModel):
    sample_id: int
    # Changes with the questions, the model, the mask and how the images are drawn
    key: str
    # Probabilities by question name, None where the model refused
    answers: dict[str, float | None]
    response: dict


# What the Decisions API answered for a photo, or for one tile of it given as fractions of its width and height
class Judgment(pydantic.BaseModel):
    sha256: str
    tile: tuple[float, float, float, float] | None
    # Changes with the questions, the model, the mask and how the images are drawn
    key: str
    # Probabilities by question name, None where the model refused
    answers: dict[str, float | None]
    response: dict
