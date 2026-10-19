import threading

import diffusers
import numpy
import PIL.Image
import safetensors.torch
import torch
import torch.nn.functional
import transformers

import meegeren.harmonizer
import meegeren.interruption
import meegeren.telemetry

# https://github.com/lllyasviel/IC-Light/blob/main/gradio_demo_bg.py
IC_LIGHT_RESOLUTION = 768
IC_LIGHT_SIZE_MODULO = 64
IC_LIGHT_HIGHRES_SCALE = 1.5
IC_LIGHT_HIGHRES_STRENGTH = 0.5
IC_LIGHT_GUIDANCE_SCALE = 7.0
IC_LIGHT_ADDED_PROMPT = "best quality"
IC_LIGHT_NEGATIVE_PROMPT = "lowres, bad anatomy, bad hands, cropped, worst quality"
# Transparent foreground pixels are given to the model as this gray, which it takes as 0
IC_LIGHT_GRAY = 127
# The model redraws the foreground in detail of its own, so only light smoother than this fraction of the longer side is taken from it
IC_LIGHT_SHADING_RADIUS = 1 / 160
# https://www.itu.int/rec/R-REC-BT.709
LUMA = (0.2126, 0.7152, 0.0722)


class IcLightHarmonizer(meegeren.harmonizer.Harmonizer):
    def __init__(
        self,
        text2image: diffusers.StableDiffusionPipeline,
        image2image: diffusers.StableDiffusionImg2ImgPipeline,
        steps: int,
        keep_on_device: bool,
        device: str,
    ):
        for pipeline in (text2image, image2image):
            if not keep_on_device and device.startswith("cuda"):
                pipeline.enable_model_cpu_offload(device=device)
            else:
                pipeline.to(device)
            pipeline.set_progress_bar_config(disable=True)
        self.text2image = text2image
        self.image2image = image2image
        self.steps = steps
        self.device = device

    @classmethod
    def from_pretrained(
        cls,
        base_model: str,
        weights_path: str,
        steps: int,
        dtype: torch.dtype,
        keep_on_device: bool,
        device: str,
    ) -> IcLightHarmonizer:
        unet = diffusers.UNet2DConditionModel.from_pretrained(
            base_model, subfolder="unet", torch_dtype=dtype
        )
        # The model takes the foreground's and the background's latents beside the noisy ones
        conv_in = torch.nn.Conv2d(
            12,
            unet.conv_in.out_channels,
            unet.conv_in.kernel_size,
            unet.conv_in.stride,
            unet.conv_in.padding,
            dtype=dtype,
        )
        with torch.no_grad():
            conv_in.weight.zero_()
            conv_in.weight[:, :4].copy_(unet.conv_in.weight)
            conv_in.bias.copy_(unet.conv_in.bias)
        # The config keeps 4 input channels, from which the pipelines shape the noisy latents
        unet.conv_in = conv_in
        # The published weights are offsets from the base model's
        offsets = safetensors.torch.load_file(weights_path)
        with torch.no_grad():
            for name, weight in unet.state_dict().items():
                weight.add_(offsets[name])
        forward = unet.forward

        def concatenated(sample, timestep, encoder_hidden_states, **kwargs):
            conditions = kwargs.pop("cross_attention_kwargs")["concat_conds"]
            conditions = conditions.to(sample).repeat(
                sample.shape[0] // conditions.shape[0], 1, 1, 1
            )
            return forward(
                torch.cat([sample, conditions], dim=1),
                timestep,
                encoder_hidden_states,
                **kwargs,
            )

        # The pipelines pass cross_attention_kwargs through to the UNet unchanged, which carries the conditions as the demo does
        unet.forward = concatenated
        components = {
            "vae": diffusers.AutoencoderKL.from_pretrained(
                base_model, subfolder="vae", torch_dtype=dtype
            ),
            "text_encoder": transformers.CLIPTextModel.from_pretrained(
                base_model, subfolder="text_encoder", dtype=dtype
            ),
            "tokenizer": transformers.CLIPTokenizer.from_pretrained(
                base_model, subfolder="tokenizer"
            ),
            "unet": unet,
            "scheduler": diffusers.DPMSolverMultistepScheduler(
                num_train_timesteps=1000,
                beta_start=0.00085,
                beta_end=0.012,
                algorithm_type="sde-dpmsolver++",
                use_karras_sigmas=True,
                steps_offset=1,
            ),
            "safety_checker": None,
            "feature_extractor": None,
            "requires_safety_checker": False,
        }
        return cls(
            text2image=diffusers.StableDiffusionPipeline(**components),
            image2image=diffusers.StableDiffusionImg2ImgPipeline(**components),
            steps=steps,
            keep_on_device=keep_on_device,
            device=device,
        )

    def encode(
        self,
        foreground: numpy.ndarray,
        background: numpy.ndarray,
        size: tuple[int, int],
    ) -> torch.Tensor:
        vae = self.text2image.vae
        x = (
            torch.from_numpy(
                numpy.stack(
                    [
                        numpy.asarray(
                            PIL.Image.fromarray(image).resize(
                                size, PIL.Image.Resampling.LANCZOS
                            )
                        )
                        for image in (foreground, background)
                    ]
                )
            )
            .permute(0, 3, 1, 2)
            .float()
            / IC_LIGHT_GRAY
            - 1
        )
        latents = (
            vae.encode(
                x.to(self.text2image._execution_device, vae.dtype)
            ).latent_dist.mode()
            * vae.config.scaling_factor
        )
        return torch.cat([latents[0:1], latents[1:2]], dim=1)

    @torch.inference_mode()
    def harmonize(
        self,
        foreground: PIL.Image.Image,
        alpha: numpy.ndarray,
        background: PIL.Image.Image,
        prompt: str,
        cancelled: threading.Event,
    ) -> PIL.Image.Image:
        with meegeren.telemetry.tracer.start_as_current_span("ic_light.harmonize"):
            source = numpy.asarray(foreground).astype(numpy.float32)
            weight = alpha[..., None]
            gray = (
                (source * weight + IC_LIGHT_GRAY * (1 - weight))
                .round()
                .astype(numpy.uint8)
            )
            scale = IC_LIGHT_RESOLUTION / max(foreground.width, foreground.height)
            size = (
                max(1, round(foreground.width * scale / IC_LIGHT_SIZE_MODULO))
                * IC_LIGHT_SIZE_MODULO,
                max(1, round(foreground.height * scale / IC_LIGHT_SIZE_MODULO))
                * IC_LIGHT_SIZE_MODULO,
            )
            highres = (
                round(size[0] * IC_LIGHT_HIGHRES_SCALE / IC_LIGHT_SIZE_MODULO)
                * IC_LIGHT_SIZE_MODULO,
                round(size[1] * IC_LIGHT_HIGHRES_SCALE / IC_LIGHT_SIZE_MODULO)
                * IC_LIGHT_SIZE_MODULO,
            )
            prompt = f"{prompt}, {IC_LIGHT_ADDED_PROMPT}"
            pipeline = self.text2image
            with meegeren.interruption.interruptible(
                cancelled, pipeline.text_encoder, pipeline.unet, pipeline.vae
            ):
                latents = pipeline(
                    prompt=prompt,
                    negative_prompt=IC_LIGHT_NEGATIVE_PROMPT,
                    width=size[0],
                    height=size[1],
                    num_inference_steps=self.steps,
                    guidance_scale=IC_LIGHT_GUIDANCE_SCALE,
                    output_type="latent",
                    cross_attention_kwargs={
                        "concat_conds": self.encode(
                            gray, numpy.asarray(background), size
                        )
                    },
                ).images
                # The demo redraws its first result enlarged, which sharpens the light it draws
                pixels = pipeline.vae.decode(
                    latents / pipeline.vae.config.scaling_factor
                ).sample
                # The demo takes the result as an 8-bit image, which clamps it, before enlarging it
                pixels = torch.nn.functional.interpolate(
                    pixels.float().clamp(-1, 1), size=highres[::-1], mode="bicubic"
                ).to(pipeline.vae.dtype)
                latents = (
                    pipeline.vae.encode(pixels).latent_dist.mode()
                    * pipeline.vae.config.scaling_factor
                )
                relit = self.image2image(
                    prompt=prompt,
                    negative_prompt=IC_LIGHT_NEGATIVE_PROMPT,
                    image=latents,
                    strength=IC_LIGHT_HIGHRES_STRENGTH,
                    num_inference_steps=round(self.steps / IC_LIGHT_HIGHRES_STRENGTH),
                    guidance_scale=IC_LIGHT_GUIDANCE_SCALE,
                    cross_attention_kwargs={
                        "concat_conds": self.encode(
                            gray, numpy.asarray(background), highres
                        )
                    },
                ).images[0]
            if cancelled.is_set():
                # The pipelines offload their models only when they return
                self.text2image.maybe_free_model_hooks()
                return foreground

            # The light it draws is taken onto the original pixels as a change in luminance, so the foreground keeps its detail and colors
            original = foreground.resize(highres, PIL.Image.Resampling.LANCZOS)
            radius = max(
                1,
                round(
                    max(foreground.width, foreground.height) * IC_LIGHT_SHADING_RADIUS
                ),
            )
            mask = torch.from_numpy(alpha).to(self.device)[None, None]
            luma = torch.tensor(LUMA, device=self.device)

            def luminance(image: PIL.Image.Image) -> torch.Tensor:
                # sRGB is decoded with its approximate gamma
                x = (
                    torch.from_numpy(
                        numpy.array(
                            image.resize(foreground.size, PIL.Image.Resampling.LANCZOS)
                        )
                    )
                    .to(self.device)
                    .float()
                    / 255
                ) ** 2.2
                y = (x @ luma)[None, None]
                # Pixels outside the foreground are left out of the blur, and so are those past the image
                for _ in range(2):
                    y, blurred = y * mask, mask
                    for kernel, padding in (
                        ((2 * radius + 1, 1), (radius, 0)),
                        ((1, 2 * radius + 1), (0, radius)),
                    ):
                        y = torch.nn.functional.avg_pool2d(
                            y,
                            kernel,
                            stride=1,
                            padding=padding,
                            count_include_pad=False,
                        )
                        blurred = torch.nn.functional.avg_pool2d(
                            blurred,
                            kernel,
                            stride=1,
                            padding=padding,
                            count_include_pad=False,
                        )
                    y = y / blurred.clamp(min=1e-6)
                return y[0, 0]

            gain = (luminance(relit) + 1e-3) / (luminance(original) + 1e-3)
            x = (torch.from_numpy(source).to(self.device) / 255) ** 2.2
            return PIL.Image.fromarray(
                (((x * gain[..., None]).clamp(0, 1) ** (1 / 2.2)) * 255)
                .round()
                .to(torch.uint8)
                .cpu()
                .numpy()
            )
