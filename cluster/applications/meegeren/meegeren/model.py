import typing

import pydantic

type Dtype = typing.Literal["float32", "bfloat16", "float16"]


class StrictModel(pydantic.BaseModel):
    model_config = pydantic.ConfigDict(extra="forbid")


class GroundingDinoSam2Configuration(StrictModel):
    type: typing.Literal["grounding-dino-sam2"]
    grounding_model: str = "IDEA-Research/grounding-dino-tiny"
    threshold: float = pydantic.Field(0.35, ge=0, le=1)
    segmentation_model: str = "facebook/sam2.1-hiera-tiny"
    deformable_attention_path: str = "kernels-community/deformable-detr"
    dtype: Dtype = "float32"
    keep_on_device: bool = True


class Sam3Configuration(StrictModel):
    type: typing.Literal["sam3"]
    model: str = "facebook/sam3"
    threshold: float = pydantic.Field(0.5, ge=0, le=1)
    mask_threshold: float = 0.5
    dtype: Dtype = "float32"
    keep_on_device: bool = True


class CraftConfiguration(StrictModel):
    type: typing.Literal["craft"]
    model_path: str = "craft_mlt_25k.zip"
    region_threshold: float = 0.4
    affinity_threshold: float = 0.4
    dtype: Dtype = "float32"
    keep_on_device: bool = True


class BigLamaConfiguration(StrictModel):
    type: typing.Literal["big-lama"]
    model_path: str = "big-lama.pt"
    keep_on_device: bool = True


class OsorConfiguration(StrictModel):
    type: typing.Literal["osor"]
    base_model: str = "diffusers/stable-diffusion-xl-1.0-inpainting-0.1"
    weights_path: str = (
        "QinmingZhou/OSOR/osor-sdxlinpainting/weights/sdxlinpainting_phase2.pth"
    )
    dtype: Dtype = "float32"
    keep_on_device: bool = True


class GptImageConfiguration(StrictModel):
    type: typing.Literal["gpt-image"]
    model: str = "gpt-image-2.5-sunburst"
    quality: str = "high"
    whole_image: bool = False


class QwenImageEditComponentConfiguration(StrictModel):
    keep_on_device: bool = True


class QwenImageEditConfiguration(StrictModel):
    type: typing.Literal["qwen-image-edit"]
    model: str = "Qwen/Qwen-Image-Edit-2511"
    transformer_path: str = (
        "unsloth/Qwen-Image-Edit-2511-GGUF/qwen-image-edit-2511-Q4_K_M.gguf"
    )
    lightning_path: str = "lightx2v/Qwen-Image-Edit-2511-Lightning/Qwen-Image-Edit-2511-Lightning-8steps-V1.0-bf16.safetensors"
    remover_path: str = "prithivMLmods/QIE-2511-Object-Remover-v2/Qwen-Image-Edit-2511-Object-Remover-v2-9200.safetensors"
    steps: int = 8
    whole_image: bool = False
    # ComfyUI leaves float16 out of Qwen-Image's supported_inference_dtypes: https://github.com/comfyanonymous/ComfyUI/blob/master/comfy/supported_models.py
    dtype: typing.Literal["float32", "bfloat16"] = "float32"
    text_encoder: QwenImageEditComponentConfiguration = pydantic.Field(
        default_factory=QwenImageEditComponentConfiguration
    )
    transformer: QwenImageEditComponentConfiguration = pydantic.Field(
        default_factory=QwenImageEditComponentConfiguration
    )
    vae: QwenImageEditComponentConfiguration = pydantic.Field(
        default_factory=QwenImageEditComponentConfiguration
    )


class SdxlInpaintingConfiguration(StrictModel):
    type: typing.Literal["sdxl-inpainting"]
    base_model: str = "diffusers/stable-diffusion-xl-1.0-inpainting-0.1"
    prompt: str = (
        "a photo of an empty room, plain wall, clean floor, sharp, high detail"
    )
    negative_prompt: str = "furniture, object, plant, blurry, smudge"
    guidance_scale: float = pydantic.Field(6.0, ge=0, allow_inf_nan=False)
    strength: float = pydantic.Field(0.4, gt=0, le=1)
    steps: int = pydantic.Field(30, gt=0)
    dtype: Dtype = "float32"
    keep_on_device: bool = True

    @pydantic.model_validator(mode="after")
    def validate_denoising_steps(self) -> typing.Self:
        # https://github.com/huggingface/diffusers/blob/v0.41.0/src/diffusers/pipelines/stable_diffusion_xl/pipeline_stable_diffusion_xl_inpaint.py
        if int(self.steps * self.strength) < 1:
            raise ValueError(
                f"steps {self.steps} times strength {self.strength} must reach 1"
            )
        return self


class VitMatteConfiguration(StrictModel):
    type: typing.Literal["vitmatte"]
    model: str = "hustvl/vitmatte-base-distinctions-646"
    # Pixels on each side of the located edge left for the model to decide
    trimap_width: int = pydantic.Field(20, gt=0)
    dtype: Dtype = "float32"
    keep_on_device: bool = True


class PctNetConfiguration(StrictModel):
    type: typing.Literal["pct-net"]
    model_path: str = "PCTNet_ViT.pth"
    dtype: Dtype = "float32"
    keep_on_device: bool = True


class IcLightConfiguration(StrictModel):
    type: typing.Literal["ic-light"]
    base_model: str = "stablediffusionapi/realistic-vision-v51"
    weights_path: str = "lllyasviel/ic-light/iclight_sd15_fbc.safetensors"
    steps: int = pydantic.Field(25, gt=0)
    dtype: Dtype = "float32"
    keep_on_device: bool = True


class SwinirConfiguration(StrictModel):
    name: str
    type: typing.Literal["swinir"]
    model_path: str = "003_realSR_BSRGAN_DFOWMFC_s64w8_SwinIR-L_x4_GAN.pth"
    keep_on_device: bool = True


class RealEsrganConfiguration(StrictModel):
    name: str
    type: typing.Literal["real-esrgan"]
    model_path: str = "realesr-general-x4v3.pth"
    weak_denoise_model_path: str = "realesr-general-wdn-x4v3.pth"
    # Weighs model_path against weak_denoise_model_path, as Real-ESRGAN's -dn does, since model_path alone paints fine textures such as rugs flat
    denoise_strength: float = pydantic.Field(0.2, ge=0, le=1)
    dtype: Dtype = "float32"
    keep_on_device: bool = True


type ObjectLocatorConfiguration = typing.Annotated[
    GroundingDinoSam2Configuration | Sam3Configuration,
    pydantic.Field(discriminator="type"),
]


type InpainterConfiguration = typing.Annotated[
    BigLamaConfiguration
    | OsorConfiguration
    | GptImageConfiguration
    | QwenImageEditConfiguration,
    pydantic.Field(discriminator="type"),
]


type HarmonizerConfiguration = typing.Annotated[
    PctNetConfiguration | IcLightConfiguration,
    pydantic.Field(discriminator="type"),
]


type UpscalerConfiguration = typing.Annotated[
    SwinirConfiguration | RealEsrganConfiguration,
    pydantic.Field(discriminator="type"),
]


class Pipeline(StrictModel):
    name: str
    object_locator: ObjectLocatorConfiguration
    text_locator: CraftConfiguration
    inpainter: InpainterConfiguration
    refiner: SdxlInpaintingConfiguration | None = None
    matting: VitMatteConfiguration | None = None
    # Applied in order to what /v1/background keeps
    harmonizers: list[HarmonizerConfiguration] = []
    # Names the upscaler in upscalers that enlarges what the inpainter or the refiner draws meegeren.inpainter.UPSCALER_MIN_RATIO times smaller than the area it fills or more
    upscaler: str | None = None
    # Removes the prompt's objects one at a time, locating them again after each removal
    redetect: bool = False
    # Bounds redetect when it keeps finding remnants of removed objects
    max_removals: int = pydantic.Field(12, gt=0)

    @pydantic.model_validator(mode="after")
    def validate_max_removals(self) -> typing.Self:
        if "max_removals" in self.model_fields_set and not self.redetect:
            raise ValueError("max_removals applies only with redetect")
        return self


class Configuration(StrictModel):
    pipelines: list[Pipeline] = []
    upscalers: list[UpscalerConfiguration] = []

    @pydantic.model_validator(mode="after")
    def validate_not_empty(self) -> typing.Self:
        if len(self.pipelines) == 0 and len(self.upscalers) == 0:
            raise ValueError("pipelines or upscalers must not be empty")
        return self

    @pydantic.model_validator(mode="after")
    def validate_unique_names(self) -> typing.Self:
        # e2e.sh writes what each pipeline and upscaler returns to a directory named after it
        names = [pipeline.name for pipeline in self.pipelines] + [
            upscaler.name for upscaler in self.upscalers
        ]
        duplicates = sorted({name for name in names if names.count(name) > 1})
        if len(duplicates) > 0:
            raise ValueError(
                f"pipeline and upscaler names must be unique: {duplicates}"
            )
        return self
