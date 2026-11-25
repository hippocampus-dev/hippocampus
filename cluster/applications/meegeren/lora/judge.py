import base64
import concurrent.futures
import datetime
import hashlib
import io
import json
import logging
import os
import typing

import dotenv
import numpy
import openai
import PIL.Image
import PIL.ImageDraw
import PIL.ImageOps
import pythonjsonlogger.jsonlogger

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

PHOTO_SIZE = 1024
TILE_SOURCE_SIZE = 2048
TILE_SIZE = 768
JPEG_QUALITY = 90
TINT = 0.3
DARKENING = 0.35
OUTLINE_WIDTH = 3
REGIONS = """The first image is a photo. The second is the same photo where the wall regions are tinted blue, the floor regions tinted green and everything else darkened.
The tinted regions will be used to teach an image model what a plain wall and a plain floor look like."""
PHOTO_QUESTIONS = [
    {
        "type": "predicate",
        "name": "interior",
        "instructions": "The first image is a real color photograph of the inside of a room, not an outdoor scene, a model, a drawing, a rendering, a screenshot or a black-and-white or sepia photograph.",
    },
    {
        "type": "predicate",
        "name": "misplaced",
        "instructions": "Any tinted region covers something that is not a wall or a floor, such as a window, glass, an outdoor view, a ceiling, furniture, stairs or a doorway into another room, over more than a sliver along its edge.",
    },
]
TILE_QUESTIONS = [
    {
        "type": "predicate",
        "name": "object",
        "instructions": "Inside the yellow box, the photo shows on the tinted wall or floor any object or marking: a light switch, power outlet, wall plate, thermostat, intercom, smoke detector, air conditioner, vent or grille, pipe, cable, lamp, sign, paper, text, picture, poster, sticker, hook, shelf, plant, rug or mat, a stain, crack, peeling paint or graffiti, or the shadow or reflection of an object. Judge from the first image and use the second only to locate the tinted regions. Paint, wallpaper patterns, tiles and their joints, wood grain, carpet, baseboards and soft light and shade are not objects.",
    },
]


def ask(
    client: openai.OpenAI,
    text: str,
    images: list[PIL.Image.Image],
    questions: list[dict[str, str]],
) -> tuple[dict[str, float | None], dict]:
    content = [{"type": "input_text", "text": text}]
    for image in images:
        buffer = io.BytesIO()
        image.save(buffer, format="JPEG", quality=JPEG_QUALITY)
        content.append(
            {
                "type": "input_image",
                "image_url": f"data:image/jpeg;base64,{base64.b64encode(buffer.getvalue()).decode('ascii')}",
            }
        )
    decision = client.decisions.create(
        model=s.judge_model,
        input=[{"role": "user", "content": content}],
        questions=questions,
    )
    return {
        answer.name: None if answer.type == "refusal" else answer.probability
        for answer in decision.answers
    }, decision.model_dump(mode="json")


def main():
    directory = os.path.expanduser(s.dataset_directory)
    with open(os.path.join(directory, lora.model.REGIONS)) as f:
        regions = [lora.model.Regions.model_validate_json(line) for line in f]
    judged = set()
    if os.path.exists(os.path.join(directory, lora.model.JUDGMENTS)):
        with open(os.path.join(directory, lora.model.JUDGMENTS)) as f:
            for line in f:
                judgment = lora.model.Judgment.model_validate_json(line)
                judged.add((judgment.sha256, judgment.tile, judgment.key))

    client = openai.OpenAI()
    executor = concurrent.futures.ThreadPoolExecutor(s.judge_concurrency)
    futures = {}

    def write(done: set[concurrent.futures.Future]):
        with open(os.path.join(directory, lora.model.JUDGMENTS), "a") as f:
            for future in done:
                sha256, tile, key = futures.pop(future)
                answers, response = future.result()
                f.write(
                    lora.model.Judgment(
                        sha256=sha256,
                        tile=tile,
                        key=key,
                        answers=answers,
                        response=response,
                    ).model_dump_json()
                    + "\n"
                )

    try:
        for region in regions:
            if region.wall + region.floor < s.minimum_region_fraction:
                continue
            photo = PIL.Image.open(
                os.path.join(directory, "images", f"{region.sha256}.jpg")
            )
            if min(photo.size) < s.minimum_photo_side:
                continue
            with open(
                os.path.join(directory, "masks", f"{region.sha256}.png"), "rb"
            ) as f:
                mask_hash = hashlib.sha256(f.read()).hexdigest()
            common = [
                s.judge_model,
                REGIONS,
                TILE_SOURCE_SIZE,
                JPEG_QUALITY,
                TINT,
                DARKENING,
                mask_hash,
            ]
            photo_key = hashlib.sha256(
                json.dumps(common + [PHOTO_QUESTIONS, PHOTO_SIZE]).encode()
            ).hexdigest()
            tile_key = hashlib.sha256(
                json.dumps(common + [TILE_QUESTIONS, TILE_SIZE, OUTLINE_WIDTH]).encode()
            ).hexdigest()

            photo.draft("RGB", (TILE_SOURCE_SIZE, TILE_SOURCE_SIZE))
            photo = PIL.ImageOps.exif_transpose(photo).convert("RGB")
            photo.thumbnail((TILE_SOURCE_SIZE, TILE_SOURCE_SIZE))
            mask = numpy.asarray(
                PIL.Image.open(
                    os.path.join(directory, "masks", f"{region.sha256}.png")
                ).resize(photo.size, PIL.Image.Resampling.NEAREST)
            )
            allowed = numpy.isin(
                mask, [lora.model.Region.WALL, lora.model.Region.FLOOR]
            )
            pixels = numpy.asarray(photo).astype(numpy.float32)
            for value, color in [
                (lora.model.Region.WALL, (0, 90, 255)),
                (lora.model.Region.FLOOR, (0, 200, 0)),
            ]:
                pixels[mask == value] = pixels[mask == value] * (1 - TINT) + (
                    numpy.array(color, numpy.float32) * TINT
                )
            pixels[~allowed] *= DARKENING
            tinted = PIL.Image.fromarray(pixels.astype(numpy.uint8))

            requests = {}
            if (region.sha256, None, photo_key) not in judged:
                images = []
                for image in (photo, tinted):
                    image = image.copy()
                    image.thumbnail(
                        (PHOTO_SIZE, PHOTO_SIZE), PIL.Image.Resampling.LANCZOS
                    )
                    images.append(image)
                requests[None] = images, PHOTO_QUESTIONS, photo_key
            width = photo.width / s.tile_grid
            height = photo.height / s.tile_grid
            for column in range(s.tile_grid):
                for row in range(s.tile_grid):
                    tile = (
                        column / s.tile_grid,
                        row / s.tile_grid,
                        (column + 1) / s.tile_grid,
                        (row + 1) / s.tile_grid,
                    )
                    box = (
                        round(column * width),
                        round(row * height),
                        round((column + 1) * width),
                        round((row + 1) * height),
                    )
                    if (region.sha256, tile, tile_key) in judged or allowed[
                        box[1] : box[3], box[0] : box[2]
                    ].mean() < s.minimum_tile_fraction:
                        continue
                    context = (
                        max(round(box[0] - width / 2), 0),
                        max(round(box[1] - height / 2), 0),
                        min(round(box[2] + width / 2), photo.width),
                        min(round(box[3] + height / 2), photo.height),
                    )
                    crops = []
                    for image in (photo, tinted):
                        crop = image.crop(context)
                        PIL.ImageDraw.Draw(crop).rectangle(
                            (
                                box[0] - context[0],
                                box[1] - context[1],
                                box[2] - context[0] - 1,
                                box[3] - context[1] - 1,
                            ),
                            outline=(255, 255, 0),
                            width=OUTLINE_WIDTH,
                        )
                        crop.thumbnail(
                            (TILE_SIZE, TILE_SIZE), PIL.Image.Resampling.LANCZOS
                        )
                        crops.append(crop)
                    requests[tile] = crops, TILE_QUESTIONS, tile_key

            for tile, (images, questions, key) in requests.items():
                futures[executor.submit(ask, client, REGIONS, images, questions)] = (
                    region.sha256,
                    tile,
                    key,
                )
            if len(requests) > 0:
                logger.info(f"sent {len(requests)} requests on {region.sha256}")
            while len(futures) > 2 * s.judge_concurrency:
                write(
                    concurrent.futures.wait(
                        futures, return_when=concurrent.futures.FIRST_COMPLETED
                    ).done
                )
        write(concurrent.futures.wait(futures).done)
    finally:
        executor.shutdown(cancel_futures=True)


if __name__ == "__main__":
    main()
