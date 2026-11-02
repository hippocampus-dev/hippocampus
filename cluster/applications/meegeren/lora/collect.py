import datetime
import hashlib
import html
import json
import logging
import os
import re
import time
import typing
import urllib.error
import urllib.parse
import urllib.request

import dotenv
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

COMMONS_API = "https://commons.wikimedia.org/w/api.php"
COMMONS_LICENSES = ("cc by", "cc0", "public domain", "no restrictions")
OPENVERSE_API = "https://api.openverse.org/v1/images/"
OPENVERSE_PAGE_SIZE = 20


def fetch(url: str) -> bytes:
    max_retries = 3
    retries = 0
    while True:
        try:
            with urllib.request.urlopen(
                urllib.request.Request(url, headers={"User-Agent": s.user_agent})
            ) as response:
                return response.read()
        except urllib.error.HTTPError as e:
            match e.code:
                case 429 | 500 | 502 | 503 | 504 if retries < max_retries:
                    retries += 1
                    time.sleep(int(e.headers.get("Retry-After", 2**retries)))
                    continue
            raise


def commons(query: dict[str, str]) -> dict[str, typing.Any]:
    return json.loads(
        fetch(f"{COMMONS_API}?{urllib.parse.urlencode({**query, 'format': 'json'})}")
    )


def members(category: str, kind: str) -> list[str]:
    titles = []
    continuation = {}
    while True:
        response = commons(
            {
                "action": "query",
                "list": "categorymembers",
                "cmtitle": category,
                "cmtype": kind,
                "cmlimit": "500",
                **continuation,
            }
        )
        titles += [m["title"] for m in response["query"]["categorymembers"]]
        if "continue" not in response:
            return titles
        continuation = response["continue"]


def store(
    directory: str,
    collected: set[str],
    url: str,
    content_url: str,
    creator: str | None,
    license: str,
    is_part_of: str,
    width: int | None,
    height: int | None,
):
    if url in collected or content_url in collected:
        return
    try:
        image = fetch(content_url)
    # Openverse keeps results whose images return 403: https://github.com/WordPress/openverse/blob/main/api/api/utils/check_dead_links/provider_status_mappings.py
    except urllib.error.HTTPError as e:
        logger.warning(f"skipped {url}, as {content_url} returned {e.code}")
        return
    sha256 = hashlib.sha256(image).hexdigest()
    # Openverse indexes Wikimedia Commons as well
    if sha256 in collected:
        logger.info(f"skipped {url}, which duplicates {sha256}")
        return
    with open(os.path.join(directory, "images", f"{sha256}.jpg"), "wb") as f:
        f.write(image)
    with open(os.path.join(directory, lora.model.MANIFEST), "a") as f:
        f.write(
            lora.model.ImageObject(
                url=url,
                content_url=content_url,
                creator=creator,
                license=license,
                is_part_of=is_part_of,
                sha256=sha256,
                width=width,
                height=height,
            ).model_dump_json(by_alias=True)
            + "\n"
        )
    collected |= {url, content_url, sha256}
    logger.info(f"collected {url}")


def main():
    directory = os.path.expanduser(s.dataset_directory)
    os.makedirs(os.path.join(directory, "images"), exist_ok=True)
    collected = set()
    if os.path.exists(os.path.join(directory, lora.model.MANIFEST)):
        with open(os.path.join(directory, lora.model.MANIFEST)) as f:
            for line in f:
                image = lora.model.ImageObject.model_validate_json(line)
                collected |= {image.url, image.content_url, image.sha256}

    categories = {}
    visited = set()
    queue = [(s.commons_category, 0)]
    while len(queue) > 0:
        category, depth = queue.pop()
        if category in visited:
            continue
        visited.add(category)
        for title in members(category, "file"):
            categories.setdefault(title, category)
        if depth < s.commons_depth:
            queue += [(c, depth + 1) for c in members(category, "subcat")]
    titles = sorted(categories)
    # https://www.mediawiki.org/wiki/API:Query#Additional_notes
    for i in range(0, len(titles), 50):
        response = commons(
            {
                "action": "query",
                "titles": "|".join(titles[i : i + 50]),
                "prop": "imageinfo",
                "iiprop": "url|size|mime|extmetadata",
            }
        )
        for page in response["query"]["pages"].values():
            information = page["imageinfo"][0]
            metadata = information["extmetadata"]
            license = metadata.get("LicenseShortName", {}).get("value", "")
            if information["mime"] != "image/jpeg" or not license.lower().startswith(
                COMMONS_LICENSES
            ):
                continue
            artist = metadata.get("Artist", {}).get("value")
            store(
                directory,
                collected,
                url=information["descriptionurl"],
                # Commons appends tracking parameters to the URL Openverse indexes
                content_url=urllib.parse.urlsplit(information["url"])
                ._replace(query="")
                .geturl(),
                # Artist is HTML linking the author's user page
                creator=None
                if artist is None
                else html.unescape(re.sub(r"<[^>]+>", "", artist)).strip(),
                license=metadata.get("LicenseUrl", {}).get("value", license),
                is_part_of=categories[page["title"]],
                width=information["width"],
                height=information["height"],
            )

    for query in s.openverse_queries:
        page = 1
        while True:
            response = json.loads(
                fetch(
                    f"{OPENVERSE_API}?{
                        urllib.parse.urlencode(
                            {
                                'q': query,
                                'license_type': 'commercial,modification',
                                'page_size': OPENVERSE_PAGE_SIZE,
                                'page': page,
                            }
                        )
                    }"
                )
            )
            for result in response["results"]:
                if not result["url"].lower().endswith((".jpg", ".jpeg")):
                    continue
                store(
                    directory,
                    collected,
                    url=result["foreign_landing_url"],
                    content_url=result["url"],
                    creator=result["creator"],
                    license=result["license_url"],
                    is_part_of=result["creator_url"] or result["source"],
                    width=result["width"],
                    height=result["height"],
                )
            if page >= response["page_count"]:
                break
            page += 1


if __name__ == "__main__":
    main()
