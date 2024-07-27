import argparse
import asyncio
import os

import playwright.async_api


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("targets", nargs="+")
    parser.add_argument(
        "--interval",
        type=int,
        default=60,
    )
    args = parser.parse_args()

    async with playwright.async_api.async_playwright() as pw:
        browser = await pw.chromium.launch(
            proxy={"server": os.getenv("HTTPS_PROXY"), "bypass": "*"} if os.getenv("HTTPS_PROXY") else None,
        )
        for target in args.targets:
            page = await browser.new_page()
            await page.set_viewport_size({"width": 1920, "height": 1080})
            await page.goto(target, wait_until="networkidle")
            await page.close()
            await asyncio.sleep(args.interval)


if __name__ == "__main__":
    asyncio.run(main())
