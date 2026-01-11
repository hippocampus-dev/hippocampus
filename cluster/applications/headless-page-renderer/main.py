import argparse
import asyncio
import os
import time
import urllib.parse

import playwright.async_api


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
    args = parser.parse_args()

    async with playwright.async_api.async_playwright() as pw:
        browser = await pw.chromium.launch(
            proxy={"server": os.getenv("HTTP_PROXY"), "bypass": "*"} if os.getenv("HTTP_PROXY") else None,
        )
        for url in args.urls:
            page = await browser.new_page()
            await page.set_viewport_size({"width": 1920, "height": 1080})

            start_time = time.time()
            await page.goto(url, wait_until="networkidle")
            latency = time.time() - start_time
            if args.use_statsd:
                import statsd

                statsd_client = statsd.StatsClient(host=args.statsd_host, port=args.statsd_port)
                # https://github.com/prometheus/statsd_exporter?tab=readme-ov-file#tagging-extensions
                statsd_client.timing(f"headless-page-renderer.latency,url={urllib.parse.quote(url)}", latency)

            await page.close()
            await asyncio.sleep(args.interval)


if __name__ == "__main__":
    asyncio.run(main())
