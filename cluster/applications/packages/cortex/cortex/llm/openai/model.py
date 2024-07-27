import collections.abc
import enum

import typing_extensions

OPENAI_VECTOR_SIZE = 1536


class EmbeddingModel(enum.StrEnum):
    ADA_V2 = "text-embedding-ada-002"
    ADA_V3_SMALL = "text-embedding-3-small"
    ADA_V3_LARGE = "text-embedding-3-large"

    # https://openai.com/pricing
    @property
    def price(self) -> float:
        match self:
            case EmbeddingModel.ADA_V2:
                return 0.0001 / 1000
            case EmbeddingModel.ADA_V3_SMALL:
                return 0.00002 / 1000
            case EmbeddingModel.ADA_V3_LARGE:
                return 0.00013 / 1000
            case _:
                raise ValueError(f"Unknown model: {self}")

    # https://platform.openai.com/docs/guides/embeddings/what-are-embeddings
    @property
    def max_tokens(self) -> int:
        match self:
            case EmbeddingModel.ADA_V2 | EmbeddingModel.ADA_V3_SMALL | EmbeddingModel.ADA_V3_LARGE:
                return 8191
            case _:
                raise ValueError(f"Unknown model: {self}")


class CompletionModel(enum.StrEnum):
    GPT4O_ALIAS = "gpt-4o"
    GPT4O = "gpt-4o-2024-05-13"
    GPT4O_MINI_ALIAS = "gpt-4o-mini"
    GPT4O_MINI = "gpt-4o-mini-2024-07-18"
    GPT4_TURBO_ALIAS = "gpt-4-turbo"
    GPT4_TURBO_PREVIEW_ALIAS = "gpt-4-turbo-preview"
    GPT4_TURBO = "gpt-4-turbo-2024-04-09"
    GPT4_TURBO_VISION = "gpt-4-1106-vision-preview"
    GPT4 = "gpt-4-0613"
    GPT4_ALIAS = "gpt-4"
    GPT4_32K = "gpt-4-32k-0613"
    GPT4_32K_ALIAS = "gpt-4-32k"
    GPT35_TURBO = "gpt-3.5-turbo-0125"
    GPT35_TURBO_ALIAS = "gpt-3.5-turbo"
    GPT35_TURBO_16K = "gpt-3.5-turbo-0125"
    GPT35_TURBO_16K_ALIAS = "gpt-3.5-turbo-16k"

    # https://openai.com/pricing
    @property
    def prices(self) -> collections.abc.Mapping[str, float]:
        match self:
            case CompletionModel.GPT4O | CompletionModel.GPT4O_ALIAS:
                return {
                    "price_per_prompt": 0.005 / 1000,
                    "price_per_completion": 0.015 / 1000,
                }
            case CompletionModel.GPT4O_MINI | CompletionModel.GPT4O_MINI_ALIAS:
                return {
                    "price_per_prompt": 0.00015 / 1000,
                    "price_per_completion": 0.000075 / 1000,
                }
            case CompletionModel.GPT4_TURBO | CompletionModel.GPT4_TURBO_ALIAS | CompletionModel.GPT4_TURBO_PREVIEW_ALIAS | CompletionModel.GPT4_TURBO_VISION:
                return {
                    "price_per_prompt": 0.01 / 1000,
                    "price_per_completion": 0.03 / 1000,
                }
            case CompletionModel.GPT4 | CompletionModel.GPT4_ALIAS:
                return {
                    "price_per_prompt": 0.03 / 1000,
                    "price_per_completion": 0.06 / 1000,
                }
            case CompletionModel.GPT4_32K | CompletionModel.GPT4_32K_ALIAS:
                return {
                    "price_per_prompt": 0.06 / 1000,
                    "price_per_completion": 0.12 / 1000,
                }
            case CompletionModel.GPT35_TURBO | CompletionModel.GPT35_TURBO_ALIAS:
                return {
                    "price_per_prompt": 0.0005 / 1000,
                    "price_per_completion": 0.0015 / 1000,
                }
            case CompletionModel.GPT35_TURBO_16K | CompletionModel.GPT35_TURBO_16K_ALIAS:
                return {
                    "price_per_prompt": 0.0005 / 1000,
                    "price_per_completion": 0.0015 / 1000,
                }
            case _:
                raise ValueError(f"Unknown model: {self}")

    # https://platform.openai.com/docs/models/gpt-4
    # https://platform.openai.com/docs/models/gpt-3-5
    @property
    def max_tokens(self) -> int:
        match self:
            case CompletionModel.GPT4O | CompletionModel.GPT4O_ALIAS | CompletionModel.GPT4O_MINI | CompletionModel.GPT4O_MINI_ALIAS | CompletionModel.GPT4_TURBO | CompletionModel.GPT4_TURBO_ALIAS | CompletionModel.GPT4_TURBO_PREVIEW_ALIAS | CompletionModel.GPT4_TURBO_VISION:
                return 128000
            case CompletionModel.GPT4 | CompletionModel.GPT4_ALIAS:
                return 8192
            case CompletionModel.GPT4_32K | CompletionModel.GPT4_32K_ALIAS:
                return 32768
            case CompletionModel.GPT35_TURBO | CompletionModel.GPT35_TURBO_ALIAS:
                return 16385
            case CompletionModel.GPT35_TURBO_16K | CompletionModel.GPT35_TURBO_16K_ALIAS:
                return 16385
            case _:
                raise ValueError(f"Unknown model: {self}")

    @property
    def max_completion_tokens(self) -> int | None:
        match self:
            case CompletionModel.GPT4O | CompletionModel.GPT4O_ALIAS | CompletionModel.GPT4O_MINI | CompletionModel.GPT4O_MINI_ALIAS | CompletionModel.GPT4_TURBO | CompletionModel.GPT4_TURBO_ALIAS | CompletionModel.GPT4_TURBO_VISION:
                return 4096
            case _:
                return None

    # https://platform.openai.com/docs/guides/vision
    @property
    def limit_size_of_image(self) -> int:
        match self:
            case CompletionModel.GPT4O | CompletionModel.GPT4O_ALIAS | CompletionModel.GPT4O_MINI | CompletionModel.GPT4O_MINI_ALIAS | CompletionModel.GPT4_TURBO | CompletionModel.GPT4_TURBO_ALIAS | CompletionModel.GPT4_TURBO_VISION:
                return 20971520
            case _:
                raise ValueError(f"Unknown model: {self}")


class ImageModel(enum.StrEnum):
    Standard1024x1024 = "standard-1024x1024"
    Standard1024x1792 = "standard-1024x1792"
    Standard1792x1024 = "standard-1792x1024"
    HD1024x1024 = "hd-1024x1024"
    HD1024x1792 = "hd-1024x1792"
    HD1792x1024 = "hd-1792x1024"
    x1024 = "1024x1024"
    x512 = "512x512"
    x256 = "256x256"

    # https://openai.com/pricing
    @property
    def price(self) -> float:
        match self:
            case ImageModel.Standard1024x1024:
                return 0.040
            case ImageModel.Standard1024x1792 | ImageModel.Standard1792x1024:
                return 0.080
            case ImageModel.HD1024x1024:
                return 0.080
            case ImageModel.HD1024x1792 | ImageModel.HD1792x1024:
                return 0.120
            case ImageModel.x1024:
                return 0.020
            case ImageModel.x512:
                return 0.018
            case ImageModel.x256:
                return 0.016
        raise ValueError(f"Unknown model: {self}")

    @property
    def model_name(self) -> typing_extensions.Literal["dall-e-3", "dall-e-2"]:
        match self:
            case ImageModel.Standard1024x1024 | ImageModel.Standard1024x1792 | ImageModel.Standard1792x1024 | ImageModel.HD1024x1024 | ImageModel.HD1024x1792 | ImageModel.HD1792x1024:
                return "dall-e-3"
            case ImageModel.x1024 | ImageModel.x512 | ImageModel.x256:
                return "dall-e-2"

    @property
    def resolution(self) -> typing_extensions.Literal["1024x1024", "1024x1792", "1792x1024", "512x512", "256x256"]:
        match self:
            case ImageModel.Standard1024x1024:
                return "1024x1024"
            case ImageModel.Standard1024x1792:
                return "1024x1792"
            case ImageModel.Standard1792x1024:
                return "1792x1024"
            case ImageModel.HD1024x1024:
                return "1024x1024"
            case ImageModel.HD1024x1792:
                return "1024x1792"
            case ImageModel.HD1792x1024:
                return "1792x1024"
            case ImageModel.x1024:
                return "1024x1024"
            case ImageModel.x512:
                return "512x512"
            case ImageModel.x256:
                return "256x256"
        raise ValueError(f"Unknown model: {self}")


class AudioModel(enum.StrEnum):
    Whisper1 = "whisper-1"
    TTS1 = "tts-1"
    TTSHD1 = "tts-hd-1"

    # https://openai.com/pricing
    @property
    def price(self) -> float:
        match self:
            case AudioModel.Whisper1:
                return 0.006 / 60
            case AudioModel.TTS1:
                return 0.015 / 1000
            case AudioModel.TTSHD1:
                return 0.030 / 1000
        raise ValueError(f"Unknown model: {self}")


class ModerationModel(enum.StrEnum):
    STABLE = "text-moderation-stable"
    LATEST = "text-moderation-latest"
