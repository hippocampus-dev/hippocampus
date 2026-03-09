import argparse
import os
import re
import time
import urllib.parse

import opentelemetry.exporter.prometheus
import opentelemetry.metrics
import opentelemetry.sdk.metrics
import playwright.sync_api
import prometheus_client


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("keywords", nargs="+")
    parser.add_argument(
        "--interval",
        type=int,
        default=60,
    )
    parser.add_argument(
        "--port",
        type=int,
        default=8080,
    )
    args = parser.parse_args()

    prometheus_client.start_http_server(args.port)

    opentelemetry.metrics.set_meter_provider(opentelemetry.sdk.metrics.MeterProvider(
        metric_readers=[opentelemetry.exporter.prometheus.PrometheusMetricReader()],
    ))

    m = {}

    meter = opentelemetry.metrics.get_meter("realtime-search-exporter")
    meter.create_observable_gauge(
        "keyword_appears_per_hour",
        description="The number of times the keyword appears in an hour",
        callbacks=[
            lambda options: (
                opentelemetry.metrics.Observation(value=v, attributes={"keyword": k}) for k, v in m.items()
            )
        ],
    )

    regexp = re.compile("[0-9]+(?:秒前|分前)")

    with playwright.sync_api.sync_playwright() as pw:
        browser = pw.chromium.launch(
            proxy={"server": os.getenv("HTTP_PROXY"), "bypass": "*"} if os.getenv("HTTP_PROXY") else None,
        )
        while True:
            for keyword in args.keywords:
                page = browser.new_page()
                page.set_viewport_size({"width": 1920, "height": 1080})
                page.goto(
                    f"https://search.yahoo.co.jp/realtime/search?p={urllib.parse.quote(keyword)}&ei=UTF-8&ifr=tp_sc",
                    wait_until="networkidle",
                )

                i = 0
                tweets = page.query_selector_all("div#sr>div")
                for tweet in tweets:
                    bodies = tweet.query_selector_all("div>div>div")
                    if len(bodies) == 0:
                        continue
                    time_element = bodies[len(bodies) - 1].query_selector("time")
                    if time_element is None:
                        continue
                    content = time_element.text_content()
                    if not regexp.match(content):
                        break
                    i += 1

                m[keyword] = i

                page.close()
                time.sleep(args.interval)


if __name__ == "__main__":
    main()
