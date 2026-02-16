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
    GPT_IMAGE_1_5 = "gpt-image-1.5"
    GPT_IMAGE_1 = "gpt-image-1"
    GPT_52_ALIAS = "gpt-5.2"
    GPT_52 = "gpt-5.2-2025-12-11"
    GPT_51_ALIAS = "gpt-5.1"
    GPT_51 = "gpt-5.1-2025-11-13"
    GPT_5_ALIAS = "gpt-5"
    GPT_5 = "gpt-5-2025-08-07"
    GPT_5_MINI_ALIAS = "gpt-5-mini"
    GPT_5_MINI = "gpt-5-mini-2025-08-07"
    GPT_5_NANO_ALIAS = "gpt-5-nano"
    GPT_5_NANO = "gpt-5-nano-2025-08-07"
    O4_MINI_ALIAS = "o4-mini"
    O4_MINI = "o4-2025-04-16"
    O3_PRO_ALIAS = "o3-pro"
    O3_PRO = "o3-pro-2025-06-10"
    O3_DEEP_RESEARCH_ALIAS = "o3-deep-research"
    O3_DEEP_RESEARCH = "o3-deep-research-2025-06-26"
    O3_ALIAS = "o3"
    O3 = "o3-2025-04-16"
    O3_MINI_ALIAS = "o3-mini"
    O3_MINI = "o3-mini-2025-01-31"
    O1_PREVIEW_ALIAS = "o1-preview"
    O1_PREVIEW = "o1-preview-2024-09-12"
    O1_ALIAS = "o1"
    O1 = "o1-2024-12-17"
    O1_PRO_ALIAS = "o1-pro"
    O1_PRO = "o1-pro-2025-03-19"
    O1_MINI_ALIAS = "o1-mini"
    O1_MINI = "o1-mini-2024-09-12"
    GPT41_ALIAS = "gpt-4.1"
    GPT41 = "gpt-4.1-2025-04-14"
    GPT41_MINI_ALIAS = "gpt-4.1-mini"
    GPT41_MINI = "gpt-4.1-mini-2025-04-14"
    GPT41_NANO_ALIAS = "gpt-4.1-nano"
    GPT41_NANO = "gpt-4.1-nano-2025-04-14"
    GPT4O_LONG_OUTPUT = "gpt-4o-64k-output-alpha"
    CHATGPT_4O = "chatgpt-4o-latest"
    GPT4O_ALIAS = "gpt-4o"
    GPT4O = "gpt-4o-2024-11-20"
    GPT4O_MINI_ALIAS = "gpt-4o-mini"
    GPT4O_MINI = "gpt-4o-mini-2024-07-18"
    GPT4_TURBO_PREVIEW_ALIAS = "gpt-4-turbo-preview"
    GPT4_TURBO_ALIAS = "gpt-4-turbo"
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

    # https://platform.openai.com/docs/pricing
    @property
    def prices(self) -> collections.abc.Mapping[str, float]:
        match self:
            case CompletionModel.GPT_IMAGE_1_5:
                return {
                    "price_per_prompt": 5 / 1000 / 1000,
                    "price_per_completion": 10 / 1000 / 1000,
                }
            case CompletionModel.GPT_IMAGE_1:
                return {
                    "price_per_prompt": 5 / 1000 / 1000,
                    "price_per_completion": 10 / 1000 / 1000,
                }
            case CompletionModel.GPT_52 | CompletionModel.GPT_52_ALIAS:
                return {
                    "price_per_prompt": 1.75 / 1000 / 1000,
                    "price_per_completion": 14 / 1000 / 1000,
                }
            case CompletionModel.GPT_51 | CompletionModel.GPT_51_ALIAS | CompletionModel.GPT_5 | CompletionModel.GPT_5_ALIAS:
                return {
                    "price_per_prompt": 1.25 / 1000 / 1000,
                    "price_per_completion": 10 / 1000 / 1000,
                }
            case CompletionModel.GPT_5_MINI | CompletionModel.GPT_5_MINI_ALIAS:
                return {
                    "price_per_prompt": 0.25 / 1000 / 1000,
                    "price_per_completion": 2 / 1000 / 1000,
                }
            case CompletionModel.GPT_5_NANO | CompletionModel.GPT_5_NANO_ALIAS:
                return {
                    "price_per_prompt": 0.05 / 1000 / 1000,
                    "price_per_completion": 0.4 / 1000 / 1000,
                }
            case CompletionModel.O3_PRO | CompletionModel.O3_PRO_ALIAS:
                return {
                    "price_per_prompt": 20 / 1000 / 1000,
                    "price_per_completion": 80 / 1000 / 1000,
                }
            case CompletionModel.O3_DEEP_RESEARCH | CompletionModel.O3_DEEP_RESEARCH_ALIAS:
                return {
                    "price_per_prompt": 10 / 1000 / 1000,
                    "price_per_completion": 40 / 1000 / 1000,
                }
            case CompletionModel.O3 | CompletionModel.O3_ALIAS:
                return {
                    "price_per_prompt": 2 / 1000 / 1000,
                    "price_per_completion": 8 / 1000 / 1000,
                }
            case CompletionModel.O4_MINI | CompletionModel.O4_MINI_ALIAS | CompletionModel.O3_MINI | CompletionModel.O3_MINI_ALIAS | CompletionModel.O1_MINI | CompletionModel.O1_MINI_ALIAS:
                return {
                    "price_per_prompt": 1.1 / 1000 / 1000,
                    "price_per_completion": 4.4 / 1000 / 1000,
                }
            case CompletionModel.O1_PREVIEW | CompletionModel.O1_PREVIEW_ALIAS | CompletionModel.O1 | CompletionModel.O1_ALIAS:
                return {
                    "price_per_prompt": 15 / 1000 / 1000,
                    "price_per_completion": 60 / 1000 / 1000,
                }
            case CompletionModel.O1_PRO | CompletionModel.O1_PRO_ALIAS:
                return {
                    "price_per_prompt": 150 / 1000 / 1000,
                    "price_per_completion": 600 / 1000 / 1000,
                }
            case CompletionModel.GPT41 | CompletionModel.GPT41_ALIAS:
                return {
                    "price_per_prompt": 2 / 1000 / 1000,
                    "price_per_completion": 8 / 1000 / 1000,
                }
            case CompletionModel.GPT41_MINI | CompletionModel.GPT41_MINI_ALIAS:
                return {
                    "price_per_prompt": 0.4 / 1000 / 1000,
                    "price_per_completion": 1.6 / 1000 / 1000,
                }
            case CompletionModel.GPT41_NANO | CompletionModel.GPT41_NANO_ALIAS:
                return {
                    "price_per_prompt": 0.1 / 1000 / 1000,
                    "price_per_completion": 0.4 / 1000 / 1000,
                }
            case CompletionModel.CHATGPT_4O:
                return {
                    "price_per_prompt": 5 / 1000 / 1000,
                    "price_per_completion": 15 / 1000 / 1000,
                }
            case CompletionModel.GPT4O_LONG_OUTPUT:
                return {
                    "price_per_prompt": 6 / 1000 / 1000,
                    "price_per_completion": 18 / 1000 / 1000,
                }
            case CompletionModel.GPT4O | CompletionModel.GPT4O_ALIAS:
                return {
                    "price_per_prompt": 2.5 / 1000 / 1000,
                    "price_per_completion": 10 / 1000 / 1000,
                }
            case CompletionModel.GPT4O_MINI | CompletionModel.GPT4O_MINI_ALIAS:
                return {
                    "price_per_prompt": 0.15 / 1000 / 1000,
                    "price_per_completion": 0.6 / 1000 / 1000,
                }
            case CompletionModel.GPT4_TURBO_PREVIEW_ALIAS | CompletionModel.GPT4_TURBO | CompletionModel.GPT4_TURBO_ALIAS | CompletionModel.GPT4_TURBO_VISION:
                return {
                    "price_per_prompt": 10 / 1000 / 1000,
                    "price_per_completion": 30 / 1000 / 1000,
                }
            case CompletionModel.GPT4 | CompletionModel.GPT4_ALIAS:
                return {
                    "price_per_prompt": 30 / 1000 / 1000,
                    "price_per_completion": 60 / 1000 / 1000,
                }
            case CompletionModel.GPT4_32K | CompletionModel.GPT4_32K_ALIAS:
                return {
                    "price_per_prompt": 60 / 1000 / 1000,
                    "price_per_completion": 120 / 1000 / 1000,
                }
            case CompletionModel.GPT35_TURBO | CompletionModel.GPT35_TURBO_ALIAS:
                return {
                    "price_per_prompt": 0.5 / 1000 / 1000,
                    "price_per_completion": 1.5 / 1000 / 1000,
                }
            case CompletionModel.GPT35_TURBO_16K | CompletionModel.GPT35_TURBO_16K_ALIAS:
                return {
                    "price_per_prompt": 0.5 / 1000 / 1000,
                    "price_per_completion": 1.5 / 1000 / 1000,
                }
            case _:
                raise ValueError(f"Unknown model: {self}")

    # https://platform.openai.com/docs/models/gpt-4
    # https://platform.openai.com/docs/models/gpt-3-5
    @property
    def max_tokens(self) -> int:
        match self:
            case CompletionModel.GPT41 | CompletionModel.GPT41_ALIAS | CompletionModel.GPT41_MINI | CompletionModel.GPT41_MINI_ALIAS | CompletionModel.GPT41_NANO | CompletionModel.GPT41_NANO_ALIAS:
                return 1_047_576
            case CompletionModel.GPT_52 | CompletionModel.GPT_52_ALIAS | CompletionModel.GPT_51 | CompletionModel.GPT_51_ALIAS | CompletionModel.GPT_5 | CompletionModel.GPT_5_ALIAS | CompletionModel.GPT_5_MINI | CompletionModel.GPT_5_MINI_ALIAS | CompletionModel.GPT_5_NANO | CompletionModel.GPT_5_NANO_ALIAS:
                return 400000
            case CompletionModel.O4_MINI | CompletionModel.O4_MINI_ALIAS | CompletionModel.O3_PRO | CompletionModel.O3_PRO_ALIAS | CompletionModel.O3_DEEP_RESEARCH | CompletionModel.O3_DEEP_RESEARCH_ALIAS | CompletionModel.O3 | CompletionModel.O3_ALIAS | CompletionModel.O3_MINI | CompletionModel.O3_MINI_ALIAS | CompletionModel.O1_PREVIEW | CompletionModel.O1_PREVIEW_ALIAS | CompletionModel.O1 | CompletionModel.O1_ALIAS | CompletionModel.O1_PRO | CompletionModel.O1_PRO_ALIAS:
                return 200000
            case CompletionModel.O1_MINI | CompletionModel.O1_MINI_ALIAS | CompletionModel.GPT4O_LONG_OUTPUT | CompletionModel.CHATGPT_4O | CompletionModel.GPT4O | CompletionModel.GPT4O_ALIAS | CompletionModel.GPT4O_MINI | CompletionModel.GPT4O_MINI_ALIAS | CompletionModel.GPT4_TURBO_PREVIEW_ALIAS | CompletionModel.GPT4_TURBO | CompletionModel.GPT4_TURBO_ALIAS | CompletionModel.GPT4_TURBO_VISION:
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
            case CompletionModel.GPT41 | CompletionModel.GPT41_ALIAS | CompletionModel.GPT41_MINI | CompletionModel.GPT41_MINI_ALIAS | CompletionModel.GPT41_NANO | CompletionModel.GPT41_NANO_ALIAS:
                return 32768
            case CompletionModel.GPT_52 | CompletionModel.GPT_52_ALIAS | CompletionModel.GPT_51 | CompletionModel.GPT_51_ALIAS | CompletionModel.GPT_5 | CompletionModel.GPT_5_ALIAS | CompletionModel.GPT_5_MINI | CompletionModel.GPT_5_MINI_ALIAS | CompletionModel.GPT_5_NANO | CompletionModel.GPT_5_NANO_ALIAS:
                return 128000
            case CompletionModel.O4_MINI | CompletionModel.O4_MINI_ALIAS | CompletionModel.O3_PRO | CompletionModel.O3_PRO_ALIAS | CompletionModel.O3_DEEP_RESEARCH | CompletionModel.O3_DEEP_RESEARCH_ALIAS | CompletionModel.O3 | CompletionModel.O3_ALIAS | CompletionModel.O3_MINI | CompletionModel.O3_MINI_ALIAS | CompletionModel.O1_PREVIEW | CompletionModel.O1_PREVIEW_ALIAS | CompletionModel.O1 | CompletionModel.O1_ALIAS | CompletionModel.O1_PRO | CompletionModel.O1_PRO_ALIAS:
                return 100000
            case CompletionModel.O1_MINI | CompletionModel.O1_MINI_ALIAS | CompletionModel.GPT4O_LONG_OUTPUT:
                return 65536
            case CompletionModel.CHATGPT_4O | CompletionModel.GPT4O | CompletionModel.GPT4O_ALIAS | CompletionModel.GPT4O_MINI | CompletionModel.GPT4O_MINI_ALIAS:
                return 16384
            case CompletionModel.GPT4 | CompletionModel.GPT4_ALIAS:
                return 8192
            case CompletionModel.GPT4_TURBO_PREVIEW_ALIAS | CompletionModel.GPT4_TURBO | CompletionModel.GPT4_TURBO_ALIAS | CompletionModel.GPT4_TURBO_VISION | CompletionModel.GPT35_TURBO | CompletionModel.GPT35_TURBO_ALIAS:
                return 4096
            case CompletionModel.GPT4_32K | CompletionModel.GPT4_32K_ALIAS | CompletionModel.GPT35_TURBO_16K | CompletionModel.GPT35_TURBO_16K_ALIAS:
                return None
            case _:
                return None

    @property
    def reasoning_supported(self):
        match self:
            case CompletionModel.GPT_52 | CompletionModel.GPT_52_ALIAS | CompletionModel.GPT_51 | CompletionModel.GPT_51_ALIAS | CompletionModel.GPT_5 | CompletionModel.GPT_5_ALIAS | CompletionModel.GPT_5_MINI | CompletionModel.GPT_5_MINI_ALIAS | CompletionModel.GPT_5_NANO | CompletionModel.GPT_5_NANO_ALIAS | CompletionModel.O4_MINI | CompletionModel.O4_MINI_ALIAS | CompletionModel.O3_PRO | CompletionModel.O3_PRO_ALIAS | CompletionModel.O3_DEEP_RESEARCH | CompletionModel.O3_DEEP_RESEARCH_ALIAS | CompletionModel.O3 | CompletionModel.O3_ALIAS | CompletionModel.O3_MINI | CompletionModel.O3_MINI_ALIAS | CompletionModel.O1_PREVIEW | CompletionModel.O1_PREVIEW_ALIAS | CompletionModel.O1 | CompletionModel.O1_ALIAS | CompletionModel.O1_PRO | CompletionModel.O1_PRO_ALIAS:
                return True
            case _:
                return False

    @property
    def verbosity_supported(self):
        match self:
            case CompletionModel.GPT_52 | CompletionModel.GPT_52_ALIAS | CompletionModel.GPT_51 | CompletionModel.GPT_51_ALIAS | CompletionModel.GPT_5 | CompletionModel.GPT_5_ALIAS | CompletionModel.GPT_5_MINI | CompletionModel.GPT_5_MINI_ALIAS | CompletionModel.GPT_5_NANO | CompletionModel.GPT_5_NANO_ALIAS:
                return True
            case _:
                return False

    @property
    def tools_supported(self):
        return self not in [CompletionModel.CHATGPT_4O, CompletionModel.GPT4_TURBO_VISION]

    @property
    def stream_supported(self):
        return self not in [
            CompletionModel.O3_PRO,
            CompletionModel.O3_PRO_ALIAS,
            CompletionModel.O3_DEEP_RESEARCH,
            CompletionModel.O3_DEEP_RESEARCH_ALIAS,
            CompletionModel.O3,
            CompletionModel.O3_ALIAS,
            CompletionModel.O1_PRO,
            CompletionModel.O1_PRO_ALIAS,
        ]

    @property
    def image_url_supported(self):
        match self:
            case CompletionModel.GPT_52 | CompletionModel.GPT_52_ALIAS | CompletionModel.GPT_51 | CompletionModel.GPT_51_ALIAS | CompletionModel.GPT_5 | CompletionModel.GPT_5_ALIAS | CompletionModel.GPT_5_MINI | CompletionModel.GPT_5_MINI_ALIAS | CompletionModel.GPT_5_NANO | CompletionModel.GPT_5_NANO_ALIAS | CompletionModel.GPT41 | CompletionModel.GPT41_ALIAS | CompletionModel.GPT41_MINI | CompletionModel.GPT41_MINI_ALIAS | CompletionModel.GPT41_NANO | CompletionModel.GPT41_NANO_ALIAS | CompletionModel.CHATGPT_4O | CompletionModel.GPT4O | CompletionModel.GPT4O_ALIAS | CompletionModel.GPT4O_MINI | CompletionModel.GPT4O_MINI_ALIAS | CompletionModel.GPT4_TURBO | CompletionModel.GPT4_TURBO_ALIAS | CompletionModel.GPT4_TURBO_VISION:
                return True
            case _:
                return False

    # https://platform.openai.com/docs/guides/vision
    @property
    def limit_size_of_image(self) -> int:
        match self:
            case CompletionModel.GPT_52 | CompletionModel.GPT_52_ALIAS | CompletionModel.GPT_51 | CompletionModel.GPT_51_ALIAS | CompletionModel.GPT_5 | CompletionModel.GPT_5_ALIAS | CompletionModel.GPT_5_MINI | CompletionModel.GPT_5_MINI_ALIAS | CompletionModel.GPT_5_NANO | CompletionModel.GPT_5_NANO_ALIAS | CompletionModel.GPT41 | CompletionModel.GPT41_ALIAS | CompletionModel.GPT41_MINI | CompletionModel.GPT41_MINI_ALIAS | CompletionModel.GPT41_NANO | CompletionModel.GPT41_NANO_ALIAS | CompletionModel.CHATGPT_4O | CompletionModel.GPT4O | CompletionModel.GPT4O_ALIAS | CompletionModel.GPT4O_MINI | CompletionModel.GPT4O_MINI_ALIAS | CompletionModel.GPT4_TURBO | CompletionModel.GPT4_TURBO_ALIAS | CompletionModel.GPT4_TURBO_VISION:
                return 20971520
            case _:
                raise ValueError(f"{self} does not support image")

    @classmethod
    def model_options(cls) -> collections.abc.Sequence[str]:
        return [
            cls.GPT_52_ALIAS,
            cls.GPT_51_ALIAS,
            cls.GPT_5_ALIAS,
            cls.GPT_5_MINI_ALIAS,
            cls.GPT_5_NANO_ALIAS,
            cls.O4_MINI_ALIAS,
            cls.O3_ALIAS,
            cls.O3_MINI_ALIAS,
            cls.O1_ALIAS,
            cls.GPT41_ALIAS,
            cls.GPT41_MINI_ALIAS,
            cls.GPT41_NANO_ALIAS,
            cls.GPT4O_ALIAS,
        ]


class ImageModel(enum.StrEnum):
    Low15_1024x1024 = "low15-1024x1024"
    Low15_1024x1536 = "low15-1024x1536"
    Low15_1536x1024 = "low15-1536x1024"
    Medium15_1024x1024 = "medium15-1024x1024"
    Medium15_1024x1536 = "medium15-1024x1536"
    Medium15_1536x1024 = "medium15-1536x1024"
    High15_1024x1024 = "high15-1024x1024"
    High15_1024x1536 = "high15-1024x1536"
    High15_1536x1024 = "high15-1536x1024"
    Low1024x1024 = "low-1024x1024"
    Low1024x1536 = "low-1024x1536"
    Low1536x1024 = "low-1536x1024"
    Medium1024x1024 = "medium-1024x1024"
    Medium1024x1536 = "medium-1024x1536"
    Medium1536x1024 = "medium-1536x1024"
    High1024x1024 = "high-1024x1024"
    High1024x1536 = "high-1024x1536"
    High1536x1024 = "high-1536x1024"
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
            case ImageModel.Low15_1024x1024 | ImageModel.Low15_1024x1536 | ImageModel.Low15_1536x1024 | ImageModel.Medium15_1024x1024 | ImageModel.Medium15_1024x1536 | ImageModel.Medium15_1536x1024 | ImageModel.High15_1024x1024 | ImageModel.High15_1024x1536 | ImageModel.High15_1536x1024:
                return 0
            case ImageModel.Low1024x1024:
                return 0.011
            case ImageModel.Low1024x1536 | ImageModel.Low1536x1024:
                return 0.016
            case ImageModel.Medium1024x1024:
                return 0.042
            case ImageModel.Medium1024x1536 | ImageModel.Medium1536x1024:
                return 0.063
            case ImageModel.High1024x1024:
                return 0.167
            case ImageModel.High1024x1536 | ImageModel.High1536x1024:
                return 0.25
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
    def model_name(self) -> typing_extensions.Literal["gpt-image-1.5", "gpt-image-1", "dall-e-3", "dall-e-2"]:
        match self:
            case ImageModel.Low15_1024x1024 | ImageModel.Low15_1024x1536 | ImageModel.Low15_1536x1024 | ImageModel.Medium15_1024x1024 | ImageModel.Medium15_1024x1536 | ImageModel.Medium15_1536x1024 | ImageModel.High15_1024x1024 | ImageModel.High15_1024x1536 | ImageModel.High15_1536x1024:
                return "gpt-image-1.5"
            case ImageModel.Low1024x1024 | ImageModel.Low1024x1536 | ImageModel.Low1536x1024 | ImageModel.Medium1024x1024 | ImageModel.Medium1024x1536 | ImageModel.Medium1536x1024 | ImageModel.High1024x1024 | ImageModel.High1024x1536 | ImageModel.High1536x1024:
                return "gpt-image-1"
            case ImageModel.Standard1024x1024 | ImageModel.Standard1024x1792 | ImageModel.Standard1792x1024 | ImageModel.HD1024x1024 | ImageModel.HD1024x1792 | ImageModel.HD1792x1024:
                return "dall-e-3"
            case ImageModel.x1024 | ImageModel.x512 | ImageModel.x256:
                return "dall-e-2"
        raise ValueError(f"Unknown model: {self}")

    @property
    def quality(self) -> typing_extensions.Literal["standard", "hd", "low", "medium", "high"]:
        match self:
            case ImageModel.Standard1024x1024 | ImageModel.Standard1024x1792 | ImageModel.Standard1792x1024 | ImageModel.x1024 | ImageModel.x512 | ImageModel.x256:
                return "standard"
            case ImageModel.HD1024x1024 | ImageModel.HD1024x1792 | ImageModel.HD1792x1024:
                return "hd"
            case ImageModel.Low15_1024x1024 | ImageModel.Low15_1024x1536 | ImageModel.Low15_1536x1024 | ImageModel.Low1024x1024 | ImageModel.Low1024x1536 | ImageModel.Low1536x1024:
                return "low"
            case ImageModel.Medium15_1024x1024 | ImageModel.Medium15_1024x1536 | ImageModel.Medium15_1536x1024 | ImageModel.Medium1024x1024 | ImageModel.Medium1024x1536 | ImageModel.Medium1536x1024:
                return "medium"
            case ImageModel.High15_1024x1024 | ImageModel.High15_1024x1536 | ImageModel.High15_1536x1024 | ImageModel.High1024x1024 | ImageModel.High1024x1536 | ImageModel.High1536x1024:
                return "high"
        raise ValueError(f"Unknown model: {self}")

    @property
    def resolution(self) -> typing_extensions.Literal[
        "1024x1024", "1024x1536", "1536x1024", "1024x1792", "1792x1024", "512x512", "256x256"
    ]:
        match self:
            case ImageModel.Low15_1024x1024 | ImageModel.Medium15_1024x1024 | ImageModel.High15_1024x1024 | ImageModel.Low1024x1024 | ImageModel.Medium1024x1024 | ImageModel.High1024x1024 | ImageModel.Standard1024x1024 | ImageModel.HD1024x1024 | ImageModel.x1024:
                return "1024x1024"
            case ImageModel.Low15_1024x1536 | ImageModel.Medium15_1024x1536 | ImageModel.High15_1024x1536 | ImageModel.Low1024x1536 | ImageModel.Medium1024x1536 | ImageModel.High1024x1536:
                return "1024x1536"
            case ImageModel.Low15_1536x1024 | ImageModel.Medium15_1536x1024 | ImageModel.High15_1536x1024 | ImageModel.Low1536x1024 | ImageModel.Medium1536x1024 | ImageModel.High1536x1024:
                return "1536x1024"
            case ImageModel.Standard1024x1792 | ImageModel.HD1024x1792:
                return "1024x1792"
            case ImageModel.Standard1792x1024 | ImageModel.HD1792x1024:
                return "1792x1024"
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
