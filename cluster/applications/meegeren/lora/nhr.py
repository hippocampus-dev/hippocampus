import concurrent.futures
import datetime
import hashlib
import io
import json
import logging
import os
import re
import typing

import dotenv
import huggingface_hub
import meegeren.imaging
import numpy
import openai
import PIL.Image
import PIL.ImageFilter
import pyarrow.parquet
import pythonjsonlogger.jsonlogger

import lora.judge
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

COLUMNS = [
    "sample_id",
    "category",
    "kind",
    "style",
    "edit_instruction",
    "original_prompt",
]
CATEGORIES = ("Add object", "Remove object")
# An inverse edit swaps the images of a forward one, as NHR-Edit's metadata_only/metadata.jsonl pairs them by file name
KIND = "forward"
BLUR_RADIUS = 2
MASK_DILATION = 8
PAIR_SIZE = 768
OUTLINE_WIDTH = 3
PAIR_TEXT = "The first image shows a scene with one object outlined in red. The second is the same scene after an image editor removed that object."
PAIR_QUESTIONS = [
    {
        "type": "predicate",
        "name": "room",
        "instructions": "The images are photorealistic and show the inside of a room, not an outdoor scene, a close-up of a tabletop, a drawing, a painting or a cartoon.",
    },
    {
        "type": "predicate",
        "name": "surface",
        "instructions": "In the second image, the area inside the red outline shows only the floor or a wall of the room, not a table, counter, shelf, bed, sofa or other furniture.",
    },
    {
        "type": "predicate",
        "name": "clean",
        "instructions": "In the second image, nothing of the removed object, its shadow or its reflection is left inside or around the red outline, and the area looks like an untouched photo.",
    },
]


def difference(with_object: bytes, without_object: bytes) -> numpy.ndarray:
    grays = [
        numpy.asarray(
            PIL.Image.open(io.BytesIO(image))
            .convert("L")
            .filter(PIL.ImageFilter.GaussianBlur(BLUR_RADIUS))
        ).astype(numpy.int16)
        for image in (with_object, without_object)
    ]
    return meegeren.imaging.dilate(
        numpy.abs(grays[0] - grays[1]) > s.nhr_difference_threshold, MASK_DILATION
    )


def fetch(
    filesystem: huggingface_hub.HfFileSystem,
    repository: str,
    path: str,
    group: int,
    examples: dict[int, dict],
) -> list[lora.model.Pair]:
    with filesystem.open(path, cache_type="none") as f:
        table = pyarrow.parquet.ParquetFile(f).read_row_group(
            group, columns=["sample_id", "source_image", "edited_image"]
        )
    pairs = []
    for row in table.to_pylist():
        example = examples.get(row["sample_id"])
        if example is None:
            continue
        source = row["source_image"]["bytes"]
        edited = row["edited_image"]["bytes"]
        with_object, without_object = (
            (edited, source)
            if example["category"] == "Add object"
            else (source, edited)
        )
        mask = difference(with_object, without_object)
        directory = os.path.join(
            os.path.expanduser(s.dataset_directory),
            lora.model.NHR,
            "pairs",
            str(example["sample_id"]),
        )
        os.makedirs(directory, exist_ok=True)
        with open(os.path.join(directory, "object.png"), "wb") as f:
            f.write(with_object)
        with open(os.path.join(directory, "background.png"), "wb") as f:
            f.write(without_object)
        PIL.Image.fromarray(mask).save(os.path.join(directory, "mask.png"))
        pairs.append(
            lora.model.Pair(
                repository=repository,
                file=os.path.basename(path),
                group=group,
                difference_threshold=s.nhr_difference_threshold,
                mask_fraction=float(mask.mean()),
                **example,
            )
        )
    return pairs


def main():
    directory = os.path.join(os.path.expanduser(s.dataset_directory), lora.model.NHR)
    pattern = re.compile(s.nhr_room_pattern)
    filesystem = huggingface_hub.HfFileSystem()
    groups = {}
    for repository in s.nhr_repositories:
        cache = os.path.join(directory, "metadata", repository.replace("/", "__"))
        os.makedirs(cache, exist_ok=True)
        for path in sorted(filesystem.glob(f"datasets/{repository}/*.parquet")):
            metadata = os.path.join(
                cache,
                f"{os.path.basename(path)}.{hashlib.sha256(json.dumps(COLUMNS).encode()).hexdigest()[:8]}.jsonl",
            )
            sizes = os.path.join(cache, f"{os.path.basename(path)}.groups.json")
            if not os.path.exists(metadata) or not os.path.exists(sizes):
                # The default read-ahead fetches a block of the image columns lying between the text ones
                with filesystem.open(path, cache_type="none") as f:
                    parquet = pyarrow.parquet.ParquetFile(f)
                    if not os.path.exists(metadata):
                        with open(f"{metadata}.tmp", "w") as m:
                            m.writelines(
                                json.dumps(example) + "\n"
                                for example in parquet.read(columns=COLUMNS).to_pylist()
                            )
                        os.rename(f"{metadata}.tmp", metadata)
                    with open(sizes, "w") as m:
                        json.dump(
                            [
                                parquet.metadata.row_group(i).num_rows
                                for i in range(parquet.metadata.num_row_groups)
                            ],
                            m,
                        )
                logger.info(f"read {path}")
            with open(metadata) as f:
                examples = [json.loads(line) for line in f]
            with open(sizes) as f:
                rows = json.load(f)
            start = 0
            for group, count in enumerate(rows):
                selected = {
                    example["sample_id"]: example
                    for example in examples[start : start + count]
                    if example["category"] in CATEGORIES
                    and example["kind"] == KIND
                    and example["style"] in s.nhr_styles
                    and pattern.search(example["original_prompt"].lower())
                }
                if len(selected) > 0:
                    groups[repository, path, group] = selected
                start += count
    logger.info(
        f"found {sum(len(g) for g in groups.values())} candidates in {len(groups)} row groups"
    )

    fetched = {}
    if os.path.exists(os.path.join(directory, lora.model.PAIRS)):
        with open(os.path.join(directory, lora.model.PAIRS)) as f:
            for line in f:
                pair = lora.model.Pair.model_validate_json(line)
                fetched[pair.sample_id] = pair
    # Each row group is read whole, so the ones holding the most candidates come first, in a fixed order
    order = sorted(
        groups,
        key=lambda g: (
            -len(groups[g]),
            hashlib.sha256(f"{g[1]}#{g[2]}".encode()).hexdigest(),
        ),
    )
    remaining = []
    count = 0
    for g in order:
        if count >= s.nhr_pairs:
            break
        count += len(groups[g])
        if any(sample_id not in fetched for sample_id in groups[g]):
            remaining.append(g)
    with (
        concurrent.futures.ThreadPoolExecutor(s.nhr_concurrency) as executor,
        open(os.path.join(directory, lora.model.PAIRS), "a") as f,
    ):
        for pairs in executor.map(
            lambda g: fetch(filesystem, g[0], g[1], g[2], groups[g]), remaining
        ):
            for pair in pairs:
                f.write(pair.model_dump_json() + "\n")
                fetched[pair.sample_id] = pair
            f.flush()
            logger.info(f"fetched {len(pairs)} pairs")
        for pair in list(fetched.values()):
            if pair.difference_threshold == s.nhr_difference_threshold:
                continue
            pair_directory = os.path.join(directory, "pairs", str(pair.sample_id))
            with (
                open(os.path.join(pair_directory, "object.png"), "rb") as with_object,
                open(
                    os.path.join(pair_directory, "background.png"), "rb"
                ) as without_object,
            ):
                mask = difference(with_object.read(), without_object.read())
            PIL.Image.fromarray(mask).save(os.path.join(pair_directory, "mask.png"))
            pair = pair.model_copy(
                update={
                    "difference_threshold": s.nhr_difference_threshold,
                    "mask_fraction": float(mask.mean()),
                }
            )
            f.write(pair.model_dump_json() + "\n")
            fetched[pair.sample_id] = pair
            logger.info(f"masked {pair.sample_id} again")

    judged = set()
    if os.path.exists(os.path.join(directory, lora.model.JUDGMENTS)):
        with open(os.path.join(directory, lora.model.JUDGMENTS)) as f:
            for line in f:
                judgment = lora.model.PairJudgment.model_validate_json(line)
                judged.add((judgment.sample_id, judgment.key))
    pairs = []
    for pair in fetched.values():
        if not (
            s.nhr_minimum_mask_fraction
            <= pair.mask_fraction
            <= s.nhr_maximum_mask_fraction
            and pair.style in s.nhr_styles
            and pattern.search(pair.original_prompt.lower())
        ):
            continue
        with open(
            os.path.join(directory, "pairs", str(pair.sample_id), "mask.png"), "rb"
        ) as f:
            key = hashlib.sha256(
                json.dumps(
                    [
                        s.judge_model,
                        PAIR_TEXT,
                        PAIR_QUESTIONS,
                        PAIR_SIZE,
                        OUTLINE_WIDTH,
                        lora.judge.JPEG_QUALITY,
                        hashlib.sha256(f.read()).hexdigest(),
                    ]
                ).encode()
            ).hexdigest()
        if (pair.sample_id, key) not in judged:
            pairs.append((pair, key))
    client = openai.OpenAI()

    def judge(pair: lora.model.Pair, key: str) -> lora.model.PairJudgment:
        pair_directory = os.path.join(directory, "pairs", str(pair.sample_id))
        mask = numpy.asarray(PIL.Image.open(os.path.join(pair_directory, "mask.png")))
        outline = meegeren.imaging.dilate(mask, OUTLINE_WIDTH) & ~mask
        images = []
        for name in ("object.png", "background.png"):
            pixels = numpy.array(
                PIL.Image.open(os.path.join(pair_directory, name)).convert("RGB")
            )
            pixels[outline] = (255, 0, 0)
            image = PIL.Image.fromarray(pixels)
            image.thumbnail((PAIR_SIZE, PAIR_SIZE), PIL.Image.Resampling.LANCZOS)
            images.append(image)
        answers, response = lora.judge.ask(client, PAIR_TEXT, images, PAIR_QUESTIONS)
        return lora.model.PairJudgment(
            sample_id=pair.sample_id, key=key, answers=answers, response=response
        )

    with (
        concurrent.futures.ThreadPoolExecutor(s.judge_concurrency) as executor,
        open(os.path.join(directory, lora.model.JUDGMENTS), "a") as f,
    ):
        for judgment in executor.map(lambda p: judge(*p), pairs):
            f.write(judgment.model_dump_json() + "\n")
            f.flush()
            logger.info(f"judged {judgment.sample_id}")


if __name__ == "__main__":
    main()
