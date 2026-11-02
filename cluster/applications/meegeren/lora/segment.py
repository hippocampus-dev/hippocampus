import datetime
import logging
import os
import typing

import dotenv
import meegeren.object_locator.grounding_dino_sam2
import numpy
import PIL.Image
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

# Above the 1333 pixels Grounding DINO base and the 1024 SAM 2.1 resize photos to
SEGMENTATION_SIZE = 2048


def main():
    directory = os.path.expanduser(s.dataset_directory)
    model_directory = os.path.expanduser(s.model_directory)
    os.makedirs(os.path.join(directory, "masks"), exist_ok=True)
    segmented = set()
    if os.path.exists(os.path.join(directory, lora.model.REGIONS)):
        with open(os.path.join(directory, lora.model.REGIONS)) as f:
            segmented = {
                lora.model.Regions.model_validate_json(line).sha256 for line in f
            }

    locator = meegeren.object_locator.grounding_dino_sam2.GroundingDinoSam2Locator.from_pretrained(
        grounding_model=os.path.join(model_directory, s.grounding_model),
        segmentation_model=os.path.join(model_directory, s.segmentation_model),
        deformable_attention_path=os.path.join(
            model_directory, s.deformable_attention_path
        ),
        dtype=torch.float32,
        keep_on_device=True,
        device=s.device,
    )
    labels = ["wall", "floor"] + s.excluded_labels
    with open(os.path.join(directory, lora.model.MANIFEST)) as f:
        images = [lora.model.ImageObject.model_validate_json(line) for line in f]
    for image in images:
        if image.sha256 in segmented:
            continue
        photo = PIL.ImageOps.exif_transpose(
            PIL.Image.open(os.path.join(directory, "images", f"{image.sha256}.jpg"))
        ).convert("RGB")
        size = photo.size
        # transformers' Sam2ImageProcessor.post_process_masks upsamples every box's mask to the size of what it is given, and the photos reach 14200x9467
        photo.thumbnail((SEGMENTATION_SIZE, SEGMENTATION_SIZE))
        located = locator.locate(photo, labels, s.threshold)
        mask = numpy.full(
            (photo.height, photo.width), lora.model.Region.OTHER, numpy.uint8
        )
        for region, indices in [
            (lora.model.Region.WALL, {0}),
            (lora.model.Region.FLOOR, {1}),
            (lora.model.Region.EXCLUDED, set(range(2, len(labels)))),
        ]:
            for index, m in located:
                if index in indices:
                    mask[m] = region
        PIL.Image.fromarray(mask).resize(size, PIL.Image.Resampling.NEAREST).save(
            os.path.join(directory, "masks", f"{image.sha256}.png")
        )
        with open(os.path.join(directory, lora.model.REGIONS), "a") as f:
            f.write(
                lora.model.Regions(
                    sha256=image.sha256,
                    wall=float((mask == lora.model.Region.WALL).mean()),
                    floor=float((mask == lora.model.Region.FLOOR).mean()),
                    excluded=float((mask == lora.model.Region.EXCLUDED).mean()),
                ).model_dump_json()
                + "\n"
            )
        logger.info(f"segmented {image.url}")


if __name__ == "__main__":
    main()
