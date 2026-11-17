import math
import threading

import diffusers
import numpy
import PIL.Image
import torch
import transformers

import meegeren.imaging
import meegeren.inpainter
import meegeren.interruption
import meegeren.model
import meegeren.refiner
import meegeren.telemetry
import meegeren.upscaler

# https://huggingface.co/diffusers/stable-diffusion-xl-1.0-inpainting-0.1
SDXL_INPAINTING_RESOLUTION = 1024
# The SDXL VAE downsamples three times
SDXL_INPAINTING_SIZE_MODULO = 8
SDXL_INPAINTING_CHANGE_THRESHOLD = 8
SDXL_INPAINTING_CHANGE_DILATION = 8
# The inpainter's speckles grow with the image
SDXL_INPAINTING_CHANGE_CLOSING = 24
SDXL_INPAINTING_FEATHERING = 8
# Set the objects SDXL-Inpainting drew apart from the grain it redraws around them in the photos e2e.sh sends, most of them 1280 pixels on the longer side, the window and the area being fractions of that side and its square
SDXL_INPAINTING_ADDITION_WINDOW = 9 / 1280
SDXL_INPAINTING_ADDITION_THRESHOLD = 24
SDXL_INPAINTING_ADDITION_MIN_AREA = 200 / 1280**2


class SdxlInpaintingRefiner(meegeren.refiner.Refiner):
    def __init__(
        self,
        pipeline: diffusers.StableDiffusionXLInpaintPipeline,
        keep_on_device: bool,
        device: str,
    ):
        if not keep_on_device and device.startswith("cuda"):
            pipeline.enable_model_cpu_offload(device=device)
        else:
            pipeline.to(device)
        pipeline.set_progress_bar_config(disable=True)
        self.pipeline = pipeline

    @classmethod
    def from_pretrained(
        cls,
        base_model: str,
        loras: list[tuple[str, float]],
        timestep_spacing: str | None,
        dtype: torch.dtype,
        keep_on_device: bool,
        device: str,
    ) -> SdxlInpaintingRefiner:
        pipeline = diffusers.StableDiffusionXLInpaintPipeline(
            vae=diffusers.AutoencoderKL.from_pretrained(
                base_model,
                subfolder="vae",
                variant="fp16",
                torch_dtype=dtype,
            ),
            text_encoder=transformers.CLIPTextModel.from_pretrained(
                base_model,
                subfolder="text_encoder",
                variant="fp16",
                dtype=dtype,
            ),
            text_encoder_2=transformers.CLIPTextModelWithProjection.from_pretrained(
                base_model,
                subfolder="text_encoder_2",
                variant="fp16",
                dtype=dtype,
            ),
            tokenizer=transformers.CLIPTokenizer.from_pretrained(
                base_model, subfolder="tokenizer"
            ),
            tokenizer_2=transformers.CLIPTokenizer.from_pretrained(
                base_model, subfolder="tokenizer_2"
            ),
            unet=diffusers.UNet2DConditionModel.from_pretrained(
                base_model,
                subfolder="unet",
                variant="fp16",
                torch_dtype=dtype,
            ),
            scheduler=diffusers.EulerDiscreteScheduler.from_pretrained(
                base_model,
                subfolder="scheduler",
                **(
                    {}
                    if timestep_spacing is None
                    else {"timestep_spacing": timestep_spacing}
                ),
            ),
        )
        if len(loras) > 0:
            names = [f"lora{i}" for i in range(len(loras))]
            for name, (path, _) in zip(names, loras):
                pipeline.load_lora_weights(path, adapter_name=name)
            pipeline.set_adapters(names, [weight for _, weight in loras])
            # Merged into the weights they adapt, so each step runs no slower than base_model alone
            pipeline.fuse_lora(adapter_names=names)
            pipeline.unload_lora_weights()
        return cls(
            pipeline=pipeline,
            keep_on_device=keep_on_device,
            device=device,
        )

    @torch.inference_mode()
    def refine(
        self,
        source: PIL.Image.Image,
        image: PIL.Image.Image,
        configuration: meegeren.model.SdxlInpaintingConfiguration,
        upscaler: meegeren.upscaler.Upscaler | None,
        cancelled: threading.Event,
    ) -> PIL.Image.Image:
        with meegeren.telemetry.tracer.start_as_current_span("sdxl_inpainting.refine"):
            changed = (
                numpy.abs(
                    numpy.asarray(image).astype(numpy.int16)
                    - numpy.asarray(source).astype(numpy.int16)
                ).max(axis=2)
                > SDXL_INPAINTING_CHANGE_THRESHOLD
            )
            # OSOR leaves what it does not take for an object as it was
            if not changed.any():
                return image
            left, top, right, bottom = meegeren.inpainter.crop_box(changed)
            closing = max(
                math.ceil(
                    SDXL_INPAINTING_CHANGE_CLOSING
                    * ((right - left) * (bottom - top)) ** 0.5
                    / SDXL_INPAINTING_RESOLUTION
                ),
                SDXL_INPAINTING_CHANGE_DILATION,
            )
            # The changed pixels are speckled, and SDXL-Inpainting drew the holes left between them as gray squares
            mask = ~meegeren.imaging.dilate(
                ~meegeren.imaging.dilate(changed, closing),
                closing - SDXL_INPAINTING_CHANGE_DILATION,
            )
            box, crop, crop_mask = meegeren.inpainter.crop(image, mask)

            scale = (SDXL_INPAINTING_RESOLUTION**2 / (crop.width * crop.height)) ** 0.5
            width = (
                round(crop.width * scale / SDXL_INPAINTING_SIZE_MODULO)
                * SDXL_INPAINTING_SIZE_MODULO
            )
            height = (
                round(crop.height * scale / SDXL_INPAINTING_SIZE_MODULO)
                * SDXL_INPAINTING_SIZE_MODULO
            )
            with meegeren.interruption.interruptible(
                cancelled,
                self.pipeline.text_encoder,
                self.pipeline.text_encoder_2,
                self.pipeline.unet,
                self.pipeline.vae,
            ):
                filled = self.pipeline(
                    prompt=configuration.prompt,
                    negative_prompt=configuration.negative_prompt,
                    image=crop.resize((width, height), PIL.Image.Resampling.BICUBIC),
                    mask_image=PIL.Image.fromarray(
                        crop_mask.astype(numpy.uint8) * 255
                    ).resize((width, height), PIL.Image.Resampling.NEAREST),
                    width=width,
                    height=height,
                    strength=configuration.strength,
                    num_inference_steps=configuration.steps,
                    guidance_scale=configuration.guidance_scale,
                ).images[0]
            if cancelled.is_set():
                # The pipeline offloads its models only when it returns
                self.pipeline.maybe_free_model_hooks()
                return image
            return meegeren.inpainter.paste_back(
                image,
                box,
                meegeren.inpainter.enlarge(filled, box, upscaler, cancelled),
                crop_mask,
                SDXL_INPAINTING_FEATHERING,
            )
