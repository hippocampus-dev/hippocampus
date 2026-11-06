import datetime
import logging
import math
import os
import random
import re
import typing

import diffusers
import diffusers.utils
import dotenv
import meegeren.model
import meegeren.refiner.sdxl_inpainting
import numpy
import peft
import peft.utils
import PIL.Image
import PIL.ImageDraw
import PIL.ImageFilter
import PIL.ImageOps
import pythonjsonlogger.jsonlogger
import torch

import lora.model
import lora.settings

s = lora.settings.Settings()

if s.is_debug():
    dotenv.load_dotenv(override=True)


class JsonFormatter(pythonjsonlogger.jsonlogger.JsonFormatter):
    def add_fields(
        self,
        log_record: dict[str, typing.Any],
        record: logging.LogRecord,
        message_dict: dict[str, typing.Any],
    ):
        now = datetime.datetime.now()
        log_record["name"] = record.name
        log_record["time"] = now.isoformat()
        log_record["severitytext"] = record.levelname

        super().add_fields(log_record, record, message_dict)


handler = logging.StreamHandler()
handler.setFormatter(JsonFormatter())
if s.is_debug():
    handler.setFormatter(
        logging.Formatter(
            "time=%(asctime)s name=%(name)s severitytext=%(levelname)s body=%(message)s"
        )
    )
logging.basicConfig(level=s.convert_log_level(), handlers=[handler])
logger = logging.getLogger(__name__)

# The refiner the LoRA is trained for
REFINER = meegeren.model.SdxlInpaintingConfiguration(type="sdxl-inpainting")
MINIMUM_MASK_FRACTION = 0.005
MASK_ATTEMPTS = 16


def main():
    directory = os.path.expanduser(s.dataset_directory)
    device = s.device
    dtype = getattr(torch, s.training_dtype)
    rng = random.Random(s.seed)
    torch.manual_seed(s.seed)

    rejected = set()
    if os.path.exists(os.path.join(directory, lora.model.REJECTED)):
        with open(os.path.join(directory, lora.model.REJECTED)) as f:
            rejected = {line.strip() for line in f}
    judgments = {}
    with open(os.path.join(directory, lora.model.JUDGMENTS)) as f:
        for line in f:
            judgment = lora.model.Judgment.model_validate_json(line)
            judgments[judgment.sha256, judgment.tile] = judgment.answers
    photos = [
        sha256
        for (sha256, tile), answers in judgments.items()
        if tile is None
        and sha256 not in rejected
        and answers["interior"] is not None
        and answers["interior"] >= s.interior_threshold
        and answers["misplaced"] is not None
        and answers["misplaced"] < s.misplaced_threshold
    ]
    # As lora.judge splits photos, leaving out tiles judged on another grid
    tiles = {
        (
            column / s.tile_grid,
            row / s.tile_grid,
            (column + 1) / s.tile_grid,
            (row + 1) / s.tile_grid,
        )
        for column in range(s.tile_grid)
        for row in range(s.tile_grid)
    }
    clean_tiles = {sha256: [] for sha256 in photos}
    for (sha256, tile), answers in judgments.items():
        if (
            sha256 in clean_tiles
            and tile in tiles
            and answers["object"] is not None
            and answers["object"] < s.object_threshold
        ):
            clean_tiles[sha256].append(tile)
    photos = [sha256 for sha256 in photos if len(clean_tiles[sha256]) > 0]
    pairs = []
    if os.path.exists(os.path.join(directory, lora.model.NHR, lora.model.JUDGMENTS)):
        with open(os.path.join(directory, lora.model.NHR, lora.model.PAIRS)) as f:
            fetched = {
                pair.sample_id: pair
                for pair in (lora.model.Pair.model_validate_json(line) for line in f)
            }
        pattern = re.compile(s.nhr_room_pattern)
        answers_by_pair = {}
        with open(os.path.join(directory, lora.model.NHR, lora.model.JUDGMENTS)) as f:
            for line in f:
                judgment = lora.model.PairJudgment.model_validate_json(line)
                answers_by_pair[judgment.sample_id] = judgment.answers
        pairs = [
            sample_id
            for sample_id, answers in answers_by_pair.items()
            if s.nhr_minimum_mask_fraction
            <= fetched[sample_id].mask_fraction
            <= s.nhr_maximum_mask_fraction
            and fetched[sample_id].difference_threshold == s.nhr_difference_threshold
            and fetched[sample_id].style in s.nhr_styles
            and pattern.search(fetched[sample_id].original_prompt.lower())
            and all(
                answers[name] is not None and answers[name] >= s.pair_threshold
                for name in ("room", "surface", "clean")
            )
        ]
    logger.info(f"training on {len(photos)} photos and {len(pairs)} pairs")

    pipeline = meegeren.refiner.sdxl_inpainting.SdxlInpaintingRefiner.from_pretrained(
        base_model=os.path.join(os.path.expanduser(s.model_directory), s.base_model),
        dtype=dtype,
        keep_on_device=True,
        device=device,
    ).pipeline
    with torch.no_grad():
        prompt_embeds, _, pooled_prompt_embeds, _ = pipeline.encode_prompt(
            prompt=REFINER.prompt,
            device=device,
            num_images_per_prompt=1,
            do_classifier_free_guidance=False,
        )
    unet = pipeline.unet
    vae = pipeline.vae
    vae.requires_grad_(False)
    unet.requires_grad_(False)
    unet.add_adapter(
        peft.LoraConfig(
            r=s.rank,
            lora_alpha=s.rank,
            init_lora_weights="gaussian",
            target_modules=["to_k", "to_q", "to_v", "to_out.0"],
        )
    )
    parameters = [p for p in unet.parameters() if p.requires_grad]
    # As cast_training_params in https://github.com/huggingface/diffusers/blob/main/examples/text_to_image/train_text_to_image_lora_sdxl.py
    for p in parameters:
        p.data = p.data.float()
    unet.enable_gradient_checkpointing()
    unet.train()
    optimizer = torch.optim.AdamW(parameters, lr=s.learning_rate)

    alphas_cumprod = diffusers.DDPMScheduler.from_config(
        pipeline.scheduler.config
    ).alphas_cumprod.to(device)
    # The timestep the refiner starts from, as StableDiffusionXLInpaintPipeline.get_timesteps picks it
    pipeline.scheduler.set_timesteps(REFINER.steps)
    maximum_timestep = int(
        pipeline.scheduler.timesteps[
            REFINER.steps - int(REFINER.steps * REFINER.strength)
        ]
    )
    latent_size = s.resolution // pipeline.vae_scale_factor
    del pipeline

    def encode(pixels: numpy.ndarray) -> torch.Tensor:
        image = (
            torch.from_numpy(pixels).permute(2, 0, 1)[None].to(device, dtype) / 127.5
            - 1
        )
        return vae.encode(image).latent_dist.sample() * vae.config.scaling_factor

    for step in range(1, s.training_steps + 1):
        total = 0.0
        for _ in range(s.gradient_accumulation_steps):
            mask = None
            with_object = None
            if len(pairs) > 0 and rng.random() < s.nhr_fraction:
                for _ in range(MASK_ATTEMPTS):
                    pair_directory = os.path.join(
                        directory, lora.model.NHR, "pairs", str(rng.choice(pairs))
                    )
                    photo = PIL.Image.open(
                        os.path.join(pair_directory, "background.png")
                    ).convert("RGB")
                    scale = s.resolution / min(photo.size)
                    size = (
                        max(round(photo.width * scale), s.resolution),
                        max(round(photo.height * scale), s.resolution),
                    )
                    left = rng.randint(0, size[0] - s.resolution)
                    top = rng.randint(0, size[1] - s.resolution)
                    box = (left, top, left + s.resolution, top + s.resolution)
                    candidate = (
                        numpy.asarray(
                            PIL.Image.open(os.path.join(pair_directory, "mask.png"))
                            .resize(size, PIL.Image.Resampling.NEAREST)
                            .crop(box)
                        )
                        > 0
                    )
                    if candidate.mean() >= MINIMUM_MASK_FRACTION:
                        mask = candidate
                        photo = photo.resize(size, PIL.Image.Resampling.BICUBIC).crop(
                            box
                        )
                        with_object = (
                            PIL.Image.open(os.path.join(pair_directory, "object.png"))
                            .convert("RGB")
                            .resize(size, PIL.Image.Resampling.BICUBIC)
                            .crop(box)
                        )
                        break
            while mask is None:
                sha256 = rng.choice(photos)
                photo = PIL.Image.open(
                    os.path.join(directory, "images", f"{sha256}.jpg")
                )
                photo.draft("RGB", (s.resolution, s.resolution))
                photo = PIL.ImageOps.exif_transpose(photo).convert("RGB")
                regions = PIL.Image.open(
                    os.path.join(directory, "masks", f"{sha256}.png")
                )
                scale = s.resolution / min(photo.size)
                size = (
                    max(round(photo.width * scale), s.resolution),
                    max(round(photo.height * scale), s.resolution),
                )
                left = rng.randint(0, size[0] - s.resolution)
                top = rng.randint(0, size[1] - s.resolution)
                box = (left, top, left + s.resolution, top + s.resolution)
                photo = photo.resize(size, PIL.Image.Resampling.BICUBIC).crop(box)
                clean = numpy.zeros((size[1], size[0]), bool)
                for tile_left, tile_top, tile_right, tile_bottom in clean_tiles[sha256]:
                    clean[
                        round(tile_top * size[1]) : round(tile_bottom * size[1]),
                        round(tile_left * size[0]) : round(tile_right * size[0]),
                    ] = True
                allowed = (
                    numpy.isin(
                        numpy.asarray(
                            regions.resize(size, PIL.Image.Resampling.NEAREST).crop(box)
                        ),
                        [lora.model.Region.WALL, lora.model.Region.FLOOR],
                    )
                    & clean[top : top + s.resolution, left : left + s.resolution]
                )
                for _ in range(MASK_ATTEMPTS):
                    canvas = PIL.Image.new("L", (s.resolution, s.resolution))
                    for _ in range(rng.randint(1, 4)):
                        area = rng.uniform(0.01, 0.3) * s.resolution**2
                        aspect = math.exp(rng.uniform(-1.5, 1.5))
                        width = math.sqrt(area * aspect)
                        height = area / width
                        x = rng.uniform(0, s.resolution)
                        y = rng.uniform(0, s.resolution)
                        angle = rng.uniform(0, math.pi)
                        corners = (
                            [
                                (math.cos(a) / 2, math.sin(a) / 2)
                                for a in numpy.linspace(0, 2 * math.pi, 32)
                            ]
                            if rng.random() < 0.5
                            else [(-0.5, -0.5), (0.5, -0.5), (0.5, 0.5), (-0.5, 0.5)]
                        )
                        PIL.ImageDraw.Draw(canvas).polygon(
                            [
                                (
                                    x
                                    + u * width * math.cos(angle)
                                    - v * height * math.sin(angle),
                                    y
                                    + u * width * math.sin(angle)
                                    + v * height * math.cos(angle),
                                )
                                for u, v in corners
                            ],
                            fill=255,
                        )
                    candidate = (numpy.asarray(canvas) > 0) & allowed
                    if candidate.mean() >= MINIMUM_MASK_FRACTION:
                        mask = candidate
                        break

            pixels = numpy.asarray(photo)
            degraded = rng.random() < s.degraded_probability
            with torch.no_grad():
                latents = encode(pixels)
                # StableDiffusionXLInpaintPipeline zeroes the masked pixels after scaling them to [-1, 1]
                masked_latents = encode(numpy.where(mask[..., None], 127.5, pixels))
                start = latents
                if degraded and with_object is not None:
                    # NHR-Edit's editor redraws the whole image, where the refiner keeps what lies outside the mask
                    start = encode(
                        numpy.where(mask[..., None], numpy.asarray(with_object), pixels)
                    )
                elif degraded:
                    blurred = numpy.asarray(
                        photo.filter(PIL.ImageFilter.GaussianBlur(rng.uniform(2, 8)))
                    ).astype(numpy.float32) + numpy.array(
                        [rng.uniform(-20, 20) for _ in range(3)], numpy.float32
                    )
                    start = encode(
                        numpy.where(
                            mask[..., None], blurred.clip(0, 255), pixels
                        ).astype(numpy.uint8)
                    )
            # StableDiffusionXLInpaintPipeline.prepare_mask_latents shrinks the mask by nearest neighbors
            mask_latents = torch.nn.functional.interpolate(
                torch.from_numpy(mask)[None, None].to(device, dtype),
                size=(latent_size, latent_size),
            )
            timestep = rng.randint(
                maximum_timestep // 4 if degraded else 1, maximum_timestep
            )
            alpha = alphas_cumprod[timestep].sqrt()
            sigma = (1 - alphas_cumprod[timestep]).sqrt()
            noise = torch.randn_like(latents, dtype=torch.float32)
            noisy = alpha * start.float() + sigma * noise
            # The noise that leads back to the clean crop rather than to where it started
            target = (noisy - alpha * latents.float()) / sigma
            with torch.autocast(device_type=device.split(":")[0], dtype=dtype):
                prediction = unet(
                    torch.cat([noisy.to(dtype), mask_latents, masked_latents], dim=1),
                    timestep,
                    encoder_hidden_states=prompt_embeds,
                    added_cond_kwargs={
                        "text_embeds": pooled_prompt_embeds,
                        # As StableDiffusionXLInpaintPipeline passes them when the refiner gives no original_size or crops_coords_top_left
                        "time_ids": torch.tensor(
                            [
                                [
                                    s.resolution,
                                    s.resolution,
                                    0,
                                    0,
                                    s.resolution,
                                    s.resolution,
                                ]
                            ],
                            device=device,
                            dtype=dtype,
                        ),
                    },
                ).sample
            error = (
                (prediction.float() - target.float()).pow(2).mean(dim=1, keepdim=True)
            )
            inside = mask_latents.float()
            loss = (error * inside).sum() / inside.sum().clamp(min=1) + (
                s.background_weight
                * (error * (1 - inside)).sum()
                / (1 - inside).sum().clamp(min=1)
            )
            (loss / s.gradient_accumulation_steps).backward()
            total += loss.item()
        optimizer.step()
        optimizer.zero_grad()
        logger.info(f"step {step} loss {total / s.gradient_accumulation_steps:.4f}")

        if step % s.checkpoint_steps == 0 or step == s.training_steps:
            checkpoint = os.path.join(directory, "checkpoints", f"{step:06d}")
            diffusers.StableDiffusionXLInpaintPipeline.save_lora_weights(
                checkpoint,
                unet_lora_layers=diffusers.utils.convert_state_dict_to_diffusers(
                    peft.utils.get_peft_model_state_dict(unet)
                ),
            )
            with open(os.path.join(checkpoint, "settings.json"), "w") as f:
                f.write(s.model_dump_json(indent=2))
            logger.info(f"saved {checkpoint}")


if __name__ == "__main__":
    main()
