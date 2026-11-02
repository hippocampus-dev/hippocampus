import argparse
import os
import time
import urllib.parse

import opentelemetry.exporter.prometheus
import opentelemetry.metrics
import opentelemetry.sdk.metrics
import playwright.sync_api
import prometheus_client

COUNT_RECENT_APPEARANCES = """() => {
  const regexp = /^[0-9]+(?:秒前|分前)/;

  let count = 0;
  for (const tweet of document.querySelectorAll("div#sr>div")) {
    const bodies = tweet.querySelectorAll(":scope div>div>div");
    if (bodies.length === 0) {
      continue;
    }
    const timeElement = bodies[bodies.length - 1].querySelector("time");
    if (timeElement === null) {
      continue;
    }
    if (!regexp.test(timeElement.textContent)) {
      break;
    }
    count += 1;
  }
  return count;
}"""


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

    opentelemetry.metrics.set_meter_provider(
        opentelemetry.sdk.metrics.MeterProvider(
            metric_readers=[opentelemetry.exporter.prometheus.PrometheusMetricReader()],
        )
    )

    m = {}

    meter = opentelemetry.metrics.get_meter("realtime-search-exporter")
    meter.create_observable_gauge(
        "keyword_appears_per_hour",
        description="The number of times the keyword appears in an hour",
        callbacks=[
            lambda options: (
                opentelemetry.metrics.Observation(value=v, attributes={"keyword": k})
                for k, v in m.items()
            )
        ],
    )

    with playwright.sync_api.sync_playwright() as pw:
        browser = pw.chromium.launch(
            proxy={"server": os.getenv("HTTP_PROXY"), "bypass": "*"}
            if os.getenv("HTTP_PROXY")
            else None,
        )
        while True:
            for keyword in args.keywords:
                page = browser.new_page()
                try:
                    page.set_viewport_size({"width": 1920, "height": 1080})
                    page.goto(
                        f"https://search.yahoo.co.jp/realtime/search?p={urllib.parse.quote(keyword)}&ei=UTF-8&ifr=tp_sc",
                        wait_until="networkidle",
                    )

                    m[keyword] = page.evaluate(COUNT_RECENT_APPEARANCES)
                except playwright.sync_api.TimeoutError:
                    # Drop the series rather than keep reporting the previous count
                    m.pop(keyword, None)
                    print(f"Timeout loading {keyword}")
                finally:
                    page.close()

                time.sleep(args.interval)


if __name__ == "__main__":
    main()
