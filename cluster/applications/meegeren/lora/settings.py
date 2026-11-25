import logging
import sys
import typing

import pydantic
import pydantic_settings


def inject(self: pydantic_settings.PydanticBaseSettingsSource):
    original_prepare_field_value = self.prepare_field_value

    def prepare_field_value(
        field_name: str,
        field: pydantic.fields.FieldInfo,
        value: typing.Any,
        value_is_complex: bool,
    ) -> typing.Any:
        match field_name:
            case (
                "openverse_queries"
                | "excluded_labels"
                | "nhr_repositories"
                | "nhr_styles"
            ):
                if value is None:
                    return None
                if not value:
                    return []
                return [x.strip() for x in value.split(",")]
        return original_prepare_field_value(field_name, field, value, value_is_complex)

    self.prepare_field_value = prepare_field_value


class Settings(pydantic_settings.BaseSettings):
    model_config = pydantic_settings.SettingsConfigDict(extra="allow", env_file=".env")

    log_level: str = "info"

    dataset_directory: str = "~/.cache/meegeren-lora"
    # https://foundation.wikimedia.org/wiki/Policy:Wikimedia_Foundation_User-Agent_Policy
    user_agent: str = "meegeren/0.1.0 (https://github.com/hippocampus-dev/hippocampus)"
    commons_category: str = "Category:Empty rooms"
    commons_depth: int = 2
    openverse_queries: list[str] = [
        "empty room",
        "empty apartment",
        "unfurnished room",
        "vacant apartment interior",
    ]

    device: str = "cpu"
    # Model paths are relative to this, where `make download` puts them
    model_directory: str = "~/.cache/meegeren"
    grounding_model: str = "IDEA-Research/grounding-dino-base"
    segmentation_model: str = "facebook/sam2.1-hiera-tiny"
    deformable_attention_path: str = "kernels-community/deformable-detr"
    threshold: float = 0.25
    # Masked for training, these would teach the refiner to draw them
    excluded_labels: list[str] = [
        "outlet",
        "switch",
        "vent",
        "light fixture",
        "radiator",
        "door",
        "window",
        "clock",
        "calendar",
        "picture",
        "poster",
        "sign",
        "curtain",
        "screen",
        "table",
        "chair",
        "shelf",
        "cabinet",
        "furniture",
        "person",
    ]

    base_model: str = "diffusers/stable-diffusion-xl-1.0-inpainting-0.1"
    training_dtype: str = "bfloat16"
    resolution: int = 1024
    rank: int = 16
    learning_rate: float = 5e-5
    training_steps: int = 2000
    gradient_accumulation_steps: int = 4
    checkpoint_steps: int = 500
    # Fraction of samples starting from a blurred and recolored crop, as the refiner starts from what OSOR draws
    degraded_probability: float = 0.5
    background_weight: float = 0.1
    seed: int = 0

    nhr_repositories: list[str] = [
        "iitolstykh/NHR-Edit",
        "iitolstykh/NHR-Edit-part2",
    ]
    nhr_room_pattern: str = r"\b(living room|bedroom|dining room|kitchen|hallway|apartment|loft|office|study|nursery|lounge|corridor)\b"
    nhr_styles: list[str] = [
        "standard",
        "realistic",
        "photo",
        "dslr",
        "snapshot",
        "shot",
        "realistic shot",
        "wide-angle",
    ]
    nhr_pairs: int = 2000
    nhr_concurrency: int = 4
    nhr_difference_threshold: int = 24
    # As small as lora.train lets a mask be in a crop
    nhr_minimum_mask_fraction: float = 0.005
    nhr_maximum_mask_fraction: float = 0.4
    pair_threshold: float = 0.5
    # Fraction of samples drawn from NHR-Edit pairs rather than empty-room photos
    nhr_fraction: float = 0.7

    judge_model: str = "gpt-6-luna"
    judge_concurrency: int = 8
    minimum_region_fraction: float = 0.1
    minimum_photo_side: int = 500
    tile_grid: int = 4
    # Tiles with less wall and floor than this are left out of training without being judged
    minimum_tile_fraction: float = 0.02
    interior_threshold: float = 0.5
    misplaced_threshold: float = 0.5
    object_threshold: float = 0.5

    @staticmethod
    def is_debug() -> bool:
        return sys.prefix != sys.base_prefix

    def convert_log_level(self) -> int:
        return logging.getLevelName(self.log_level.upper())

    @classmethod
    def settings_customise_sources(
        cls,
        settings_cls: type[pydantic_settings.BaseSettings],
        init_settings: pydantic_settings.PydanticBaseSettingsSource,
        env_settings: pydantic_settings.PydanticBaseSettingsSource,
        dotenv_settings: pydantic_settings.PydanticBaseSettingsSource,
        file_secret_settings: pydantic_settings.PydanticBaseSettingsSource,
    ) -> tuple[pydantic_settings.PydanticBaseSettingsSource, ...]:
        inject(env_settings)
        inject(dotenv_settings)
        return init_settings, env_settings, dotenv_settings, file_secret_settings
