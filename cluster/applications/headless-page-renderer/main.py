import argparse
import asyncio
import datetime
import logging
import os
import time
import typing
import urllib.parse

import dotenv
import playwright.async_api
import pythonjsonlogger.jsonlogger

import headless_page_renderer.settings

s = headless_page_renderer.settings.Settings()

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


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("urls", nargs="+")
    parser.add_argument(
        "--interval",
        type=int,
        default=60,
    )
    parser.add_argument(
        "--use-statsd",
        action="store_true",
    )
    parser.add_argument(
        "--statsd-host",
        type=str,
        default="localhost",
    )
    parser.add_argument(
        "--statsd-port",
        type=int,
        default=8125,
    )
    parser.add_argument(
        "--timeout",
        type=int,
        default=30000,
        help="Page navigation timeout in milliseconds",
    )
    args = parser.parse_args()

    async with playwright.async_api.async_playwright() as playwright_driver:
        browser = await playwright_driver.chromium.launch(
            proxy={"server": os.getenv("HTTP_PROXY"), "bypass": "*"}
            if os.getenv("HTTP_PROXY")
            else None,
        )
        for url in args.urls:
            page = await browser.new_page()
            await page.set_viewport_size({"width": 1920, "height": 1080})

            failed_responses = []
            page.on(
                "response",
                lambda response: (
                    failed_responses.append((response.status, response.url))
                    if response.status >= 400
                    else None
                ),
            )

            start_time = time.time()
            try:
                await page.goto(url, wait_until="networkidle", timeout=args.timeout)
                latency = time.time() - start_time
                if args.use_statsd:
                    import statsd

                    statsd_client = statsd.StatsClient(
                        host=args.statsd_host, port=args.statsd_port
                    )
                    # https://github.com/prometheus/statsd_exporter?tab=readme-ov-file#tagging-extensions
                    statsd_client.timing(
                        f"headless-page-renderer.latency,url={urllib.parse.quote(url)}",
                        latency,
                    )
                    statsd_client.gauge(
                        f"headless-page-renderer.failed,url={urllib.parse.quote(url)}",
                        len(failed_responses),
                    )
            except playwright.async_api.TimeoutError:
                logger.warning(f"Timeout after {args.timeout}ms loading {url}")

            await page.close()
            await asyncio.sleep(args.interval)


if __name__ == "__main__":
    asyncio.run(main())
