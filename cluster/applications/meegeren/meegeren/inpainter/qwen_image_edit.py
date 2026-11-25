import threading
import typing

import diffusers
import diffusers.hooks
import numpy
import PIL.Image
import torch
import transformers

import meegeren.imaging
import meegeren.inpainter
import meegeren.interruption
import meegeren.telemetry
import meegeren.upscaler

# https://huggingface.co/prithivMLmods/QIE-2511-Object-Remover-v2
QWEN_IMAGE_EDIT_PROMPT = "Remove the red highlighted object from the scene."
QWEN_IMAGE_EDIT_HIGHLIGHT = (255, 0, 0)
# https://huggingface.co/lightx2v/Qwen-Image-Edit-2511-Lightning
QWEN_IMAGE_EDIT_TRUE_CFG_SCALE = 1.0
QWEN_IMAGE_EDIT_PAINT_PROMPTS = {
    "replace": "Replace the red highlighted area with {prompt}, matching the perspective and lighting of the scene.",
    "restyle": "Change the {prompt}, keeping its shape, structure, position, lighting and shadows. Keep everything else unchanged.",
    "relight": "Relight this room as lit by {prompt}. Keep every object, its shape, position and material, and the view outside the windows exactly unchanged.",
    "insert": "Add {prompt} inside the red outline, keeping what is already there and matching the perspective and lighting of the scene, then remove the red outline.",
    "background": "Replace the red highlighted area with {prompt} as the background behind the subjects, matching their perspective and lighting, together with the shadows they cast on it.",
}
QWEN_IMAGE_EDIT_OUTLINE_WIDTH = 4


class QwenImageEditInpainter(meegeren.inpainter.Inpainter, meegeren.inpainter.Painter):
    relights = True

    def __init__(
        self,
        pipeline: diffusers.QwenImageEditPlusPipeline,
        steps: int,
        whole_image: bool,
        keep_text_encoder_on_device: bool,
        keep_transformer_on_device: bool,
        keep_vae_on_device: bool,
        device: str,
    ):
        for component, keep_on_device in (
            (pipeline.text_encoder, keep_text_encoder_on_device),
            (pipeline.transformer, keep_transformer_on_device),
            (pipeline.vae, keep_vae_on_device),
        ):
            if not keep_on_device and device.startswith("cuda"):
                # enable_model_cpu_offload took 5.4s per request moving the text encoder back to the CPU
                diffusers.hooks.apply_group_offloading(
                    component,
                    onload_device=torch.device(device),
                    offload_type="leaf_level",
                    # The pipeline never calls the VAE's forward, where diffusers stops recording the layer order for streams
                    use_stream=component is not pipeline.vae,
                )
            else:
                component.to(device)
        pipeline.set_progress_bar_config(disable=True)
        self.pipeline = pipeline
        self.steps = steps
        self.whole_image = whole_image
        self.pastes_back = not whole_image

    @classmethod
    def from_pretrained(
        cls,
        model: str,
        transformer_path: str,
        lightning_path: str,
        remover_path: str,
        steps: int,
        whole_image: bool,
        dtype: torch.dtype,
        keep_text_encoder_on_device: bool,
        keep_transformer_on_device: bool,
        keep_vae_on_device: bool,
        device: str,
    ) -> QwenImageEditInpainter:
        pipeline = diffusers.QwenImageEditPlusPipeline(
            scheduler=diffusers.FlowMatchEulerDiscreteScheduler.from_pretrained(
                model, subfolder="scheduler"
            ),
            vae=diffusers.AutoencoderKLQwenImage.from_pretrained(
                model, subfolder="vae", torch_dtype=dtype
            ),
            text_encoder=transformers.Qwen2_5_VLForConditionalGeneration.from_pretrained(
                model, subfolder="text_encoder", dtype=dtype
            ),
            tokenizer=transformers.Qwen2Tokenizer.from_pretrained(
                model, subfolder="tokenizer"
            ),
            processor=transformers.Qwen2VLProcessor.from_pretrained(
                model, subfolder="processor"
            ),
            transformer=diffusers.QwenImageTransformer2DModel.from_single_file(
                transformer_path,
                quantization_config=diffusers.GGUFQuantizationConfig(
                    compute_dtype=dtype
                ),
                config=model,
                subfolder="transformer",
                torch_dtype=dtype,
            ),
        )
        # The VAE runs while the text encoder is still on the GPU, which leaves too little memory for it untiled
        pipeline.vae.enable_tiling()
        pipeline.load_lora_weights(lightning_path, adapter_name="lightning")
        pipeline.load_lora_weights(remover_path, adapter_name="remover")
        pipeline.set_adapters(["lightning", "remover"], [1.0, 1.0])
        return cls(
            pipeline=pipeline,
            steps=steps,
            whole_image=whole_image,
            keep_text_encoder_on_device=keep_text_encoder_on_device,
            keep_transformer_on_device=keep_transformer_on_device,
            keep_vae_on_device=keep_vae_on_device,
            device=device,
        )

    @torch.inference_mode()
    def inpaint(
        self,
        image: PIL.Image.Image,
        masks: list[numpy.ndarray],
        feathering: int,
        upscaler: meegeren.upscaler.Upscaler | None,
        cancelled: threading.Event,
    ) -> PIL.Image.Image:
        with meegeren.telemetry.tracer.start_as_current_span("qwen_image_edit.inpaint"):
            # The model removes whatever is highlighted, so every target is removed at once
            return self.edit(
                image,
                numpy.logical_or.reduce(masks),
                QWEN_IMAGE_EDIT_PROMPT,
                "fill",
                self.whole_image,
                feathering,
                upscaler,
                cancelled,
            )

    @torch.inference_mode()
    def paint(
        self,
        image: PIL.Image.Image,
        mask: numpy.ndarray,
        prompt: str,
        mode: typing.Literal["replace", "restyle", "relight", "insert", "background"],
        feathering: int,
        upscaler: meegeren.upscaler.Upscaler | None,
        cancelled: threading.Event,
    ) -> PIL.Image.Image:
        with meegeren.telemetry.tracer.start_as_current_span("qwen_image_edit.paint"):
            # The object remover LoRA would remove what the prompt asks to draw
            self.pipeline.set_adapters(["lightning"], [1.0])
            try:
                return self.edit(
                    image,
                    mask,
                    QWEN_IMAGE_EDIT_PAINT_PROMPTS[mode].format(prompt=prompt),
                    # Filling the area would hide what the object is placed among
                    "outline"
                    if mode == "insert"
                    # Outlined, the model recolored only the outline, leaving the sofa inside unchanged
                    else "none"
                    if mode in ("restyle", "relight")
                    else "fill",
                    # The whole image redrawn would lose what keep names, which only /v1/remove pastes back
                    False,
                    feathering,
                    upscaler,
                    cancelled,
                )
            finally:
                self.pipeline.set_adapters(["lightning", "remover"], [1.0, 1.0])

    def edit(
        self,
        image: PIL.Image.Image,
        mask: numpy.ndarray,
        prompt: str,
        highlight: typing.Literal["fill", "outline", "none"],
        whole_image: bool,
        feathering: int,
        upscaler: meegeren.upscaler.Upscaler | None,
        cancelled: threading.Event,
    ) -> PIL.Image.Image:
        if whole_image:
            box, crop, crop_mask = (0, 0, image.width, image.height), image, mask
        else:
            box, crop, crop_mask = meegeren.inpainter.crop(image, mask)

        highlighted = numpy.array(crop)
        if highlight == "outline":
            highlighted[
                meegeren.imaging.dilate(crop_mask, QWEN_IMAGE_EDIT_OUTLINE_WIDTH)
                & ~crop_mask
            ] = QWEN_IMAGE_EDIT_HIGHLIGHT
        elif highlight == "fill":
            highlighted[crop_mask] = QWEN_IMAGE_EDIT_HIGHLIGHT
        with meegeren.interruption.interruptible(
            cancelled,
            self.pipeline.text_encoder,
            self.pipeline.transformer,
            self.pipeline.vae,
        ):
            filled = self.pipeline(
                image=[PIL.Image.fromarray(highlighted)],
                prompt=prompt,
                true_cfg_scale=QWEN_IMAGE_EDIT_TRUE_CFG_SCALE,
                num_inference_steps=self.steps,
            ).images[0]
        if cancelled.is_set():
            # The VAE clears its cached feature maps only when an encode or decode completes
            self.pipeline.vae.clear_cache()
            return image
        filled = meegeren.inpainter.enlarge(filled, box, upscaler, cancelled)
        if whole_image:
            return filled.resize(image.size, PIL.Image.Resampling.LANCZOS)
        # The model redraws the whole crop, so only the masked pixels are composited back
        return meegeren.inpainter.paste_back(image, box, filled, crop_mask, feathering)
