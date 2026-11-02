import threading

import diffusers
import numpy
import PIL.Image
import torch
import torch.nn.functional
import transformers

import meegeren.imaging
import meegeren.inpainter
import meegeren.interruption
import meegeren.offload
import meegeren.telemetry
import meegeren.upscaler

# https://github.com/Zhouqm-Git/osor/blob/main/osor-sdxlinpainting/configs/phase2.yaml
OSOR_PROMPT = "Remove the instance of object"
OSOR_TIMESTEP = 400
OSOR_LORA_SUFFIX = ".lora_A.default.weight"
# https://github.com/Zhouqm-Git/osor/blob/main/osor-sdxlinpainting/scripts/inference_enhance.py
OSOR_RESOLUTION = 512
OSOR_SIZE_MODULO = 16
# The SDXL VAE downsamples three times
OSOR_LATENT_SCALE = 8
# Removed the potted plant SDXL-Inpainting drew in a photo e2e.sh sends, which CONTEXT_MARGIN left
OSOR_CONTEXT_MARGIN = 1.0


# https://github.com/Zhouqm-Git/osor
class OsorInpainter(meegeren.inpainter.Inpainter):
    pastes_back = False

    def __init__(
        self,
        unet: torch.nn.Module,
        vae: torch.nn.Module,
        scheduler: diffusers.DDPMScheduler,
        prompt_embeds: torch.Tensor,
        pooled_prompt_embeds: torch.Tensor,
        keep_on_device: bool,
        device: str,
    ):
        self.unet = unet.to(device if keep_on_device else "cpu").eval()
        self.vae = vae.to(device if keep_on_device else "cpu").eval()
        self.scheduler = scheduler
        self.prompt_embeds = prompt_embeds.to(device)
        self.pooled_prompt_embeds = pooled_prompt_embeds.to(device)
        self.keep_on_device = keep_on_device
        self.device = device

    @classmethod
    @torch.inference_mode()
    def from_pretrained(
        cls,
        base_model: str,
        weights_path: str,
        dtype: torch.dtype,
        keep_on_device: bool,
        device: str,
    ) -> OsorInpainter:
        unet = diffusers.UNet2DConditionModel.from_pretrained(
            base_model, subfolder="unet", variant="fp16", torch_dtype=dtype
        )
        state = torch.load(
            weights_path,
            map_location="cpu",
            weights_only=True,
        )
        # The fifth output channel predicts the pixels the removal changes, shadows and reflections included
        conv_out = unet.conv_out
        unet.conv_out = torch.nn.Conv2d(
            conv_out.in_channels,
            5,
            kernel_size=conv_out.kernel_size,
            padding=conv_out.padding,
            dtype=dtype,
        )
        unet.conv_out.weight.copy_(state.pop("unet.conv_out.weight"))
        unet.conv_out.bias.copy_(state.pop("unet.conv_out.bias"))
        # The LoRA is merged into the weights it adapts, unscaled since its alpha equals its rank
        for key in [key for key in state if key.endswith(OSOR_LORA_SUFFIX)]:
            module = key.removeprefix("unet.").removesuffix(OSOR_LORA_SUFFIX)
            a = state[key]
            b = state[f"unet.{module}.lora_B.default.weight"]
            # A convolution's A keeps the kernel while its B is 1x1
            unet.get_submodule(module).weight.add_(
                torch.einsum("or,ri...->oi...", b.flatten(1), a)
            )

        tokenizer = transformers.CLIPTokenizer.from_pretrained(
            base_model, subfolder="tokenizer"
        )
        text_encoder = transformers.CLIPTextModel.from_pretrained(
            base_model, subfolder="text_encoder", variant="fp16", dtype=dtype
        )
        tokenizer_2 = transformers.CLIPTokenizer.from_pretrained(
            base_model, subfolder="tokenizer_2"
        )
        text_encoder_2 = transformers.CLIPTextModelWithProjection.from_pretrained(
            base_model, subfolder="text_encoder_2", variant="fp16", dtype=dtype
        )
        hidden_states = text_encoder(
            tokenizer(
                OSOR_PROMPT,
                padding="max_length",
                max_length=tokenizer.model_max_length,
                truncation=True,
                return_tensors="pt",
            ).input_ids,
            output_hidden_states=True,
        ).hidden_states
        output_2 = text_encoder_2(
            tokenizer_2(
                OSOR_PROMPT,
                padding="max_length",
                max_length=tokenizer_2.model_max_length,
                truncation=True,
                return_tensors="pt",
            ).input_ids,
            output_hidden_states=True,
        )

        return cls(
            unet=unet,
            vae=diffusers.AutoencoderKL.from_pretrained(
                base_model, subfolder="vae", variant="fp16", torch_dtype=dtype
            ),
            scheduler=diffusers.DDPMScheduler.from_pretrained(
                base_model, subfolder="scheduler"
            ),
            prompt_embeds=torch.cat(
                [hidden_states[-2], output_2.hidden_states[-2]], dim=-1
            ),
            pooled_prompt_embeds=output_2.text_embeds,
            keep_on_device=keep_on_device,
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
        with (
            meegeren.telemetry.tracer.start_as_current_span("osor.inpaint"),
            meegeren.offload.placed(
                self.device, self.keep_on_device, self.unet, self.vae
            ),
            meegeren.interruption.interruptible(cancelled, self.unet, self.vae),
        ):
            # OSOR is trained to remove one instance per step, and redraws overlapping objects masked together.
            # Nearer objects occlude the ones behind them, so they go first
            for mask in sorted(masks, key=meegeren.imaging.bottom, reverse=True):
                if cancelled.is_set():
                    break
                box, crop, crop_mask = meegeren.inpainter.crop(
                    image, mask, OSOR_CONTEXT_MARGIN
                )

                scale = min(1.0, OSOR_RESOLUTION / min(crop.width, crop.height))
                width = (
                    -(-round(crop.width * scale) // OSOR_SIZE_MODULO) * OSOR_SIZE_MODULO
                )
                height = (
                    -(-round(crop.height * scale) // OSOR_SIZE_MODULO)
                    * OSOR_SIZE_MODULO
                )
                x = torch.from_numpy(
                    numpy.array(
                        crop.resize((width, height), PIL.Image.Resampling.BICUBIC)
                    )
                )
                x = (x.permute(2, 0, 1)[None].float() / 127.5 - 1).to(
                    self.device, self.unet.dtype
                )
                m = torch.nn.functional.interpolate(
                    torch.from_numpy(crop_mask)[None, None].float(),
                    size=(height, width),
                    mode="nearest",
                ).to(self.device, self.unet.dtype)

                latents = (
                    self.vae.encode(x).latent_dist.sample()
                    * self.vae.config.scaling_factor
                )
                timesteps = torch.tensor([OSOR_TIMESTEP], device=self.device)
                noisy = self.scheduler.add_noise(
                    latents, torch.randn_like(latents), timesteps
                )
                output = self.unet(
                    torch.cat(
                        [
                            noisy,
                            torch.nn.functional.max_pool2d(
                                m, kernel_size=OSOR_LATENT_SCALE
                            ),
                            latents,
                        ],
                        dim=1,
                    ),
                    timesteps,
                    encoder_hidden_states=self.prompt_embeds,
                    added_cond_kwargs={
                        "text_embeds": self.pooled_prompt_embeds,
                        "time_ids": torch.tensor(
                            [[height, width, 0, 0, height, width]],
                            dtype=torch.float32,
                            device=self.device,
                        ),
                    },
                    return_dict=False,
                )[0]
                alpha = torch.sigmoid(output[:, 4:])
                denoised = self.scheduler.step(
                    output[:, :4], OSOR_TIMESTEP, noisy
                ).pred_original_sample
                y = self.vae.decode(
                    (denoised * alpha + latents * (1 - alpha))
                    / self.vae.config.scaling_factor
                ).sample

                filled = numpy.asarray(
                    meegeren.inpainter.enlarge(
                        PIL.Image.fromarray(
                            ((y[0].float().clamp(-1, 1) + 1) * 127.5)
                            .round()
                            .to(torch.uint8)
                            .permute(1, 2, 0)
                            .cpu()
                            .numpy()
                        ),
                        box,
                        upscaler,
                        cancelled,
                    ).resize((crop.width, crop.height), PIL.Image.Resampling.BICUBIC)
                )
                # Pixels outside alpha keep full resolution, as upstream's --paste_back composites them
                alpha = torch.nn.functional.interpolate(
                    alpha, size=(crop.height, crop.width), mode="bilinear"
                )[0, 0]
                image = meegeren.inpainter.composite(
                    image, box, filled, alpha.float().cpu().numpy()
                )
        return image
