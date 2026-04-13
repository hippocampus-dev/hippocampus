import datetime
import logging
import sys
import typing
import urllib.parse

import dotenv
import numpy
import pandas
import prometheus_client
import prometheus_client.exposition
import pythonjsonlogger.jsonlogger
import requests
import statsforecast
import statsforecast.models
import yaml

import promphet.settings

s = promphet.settings.Settings()

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
logging.getLogger("statsforecast").setLevel(logging.WARNING)

logger = logging.getLogger(__name__)

FREQUENCY_STEP: dict[str, tuple[str, str]] = {
    "1minutes": ("60s", "1min"),
    "5minutes": ("300s", "5min"),
    "10minutes": ("600s", "10min"),
    "hours": ("3600s", "h"),
    "days": ("86400s", "D"),
}

LOOKBACK_SECONDS: dict[str, int] = {
    "1h": 3600,
    "6h": 21600,
    "1d": 86400,
    "3d": 259200,
    "7d": 604800,
    "14d": 1209600,
    "30d": 2592000,
}


def fetch_series(query: str, start: float, end: float, step: str) -> list:
    url = s.prometheus_host + "/api/v1/query_range"
    params = {
        "query": query,
        "start": start,
        "end": end,
        "step": step,
    }
    response = requests.get(url, params=params, timeout=30)
    response.raise_for_status()
    body = response.json()
    if body["status"] == "error":
        raise RuntimeError(body["error"])
    return body["data"]["result"]


def run_job(job: dict) -> None:
    name: str = job["name"]
    query: str = job["query"]
    lookback: str = job.get("lookback", "7d")
    frequency: str = job.get("frequency", "hours")
    periods: int = job.get("periods", 24)

    if frequency not in FREQUENCY_STEP:
        raise ValueError(
            f"Unknown frequency {frequency!r}, valid: {list(FREQUENCY_STEP)}"
        )
    if lookback not in LOOKBACK_SECONDS:
        raise ValueError(
            f"Unknown lookback {lookback!r}, valid: {list(LOOKBACK_SECONDS)}"
        )

    step, freq = FREQUENCY_STEP[frequency]
    lookback_seconds = LOOKBACK_SECONDS[lookback]

    now = datetime.datetime.now(datetime.UTC).timestamp()
    start = now - lookback_seconds

    logger.info(
        "Fetching query=%s lookback=%s frequency=%s", query, lookback, frequency
    )
    results = fetch_series(query, start, now, step)

    registry = prometheus_client.CollectorRegistry()
    gauge_map: dict[str, prometheus_client.Gauge] = {}

    frames = []
    series_labels: dict[str, dict[str, str]] = {}

    for result in results:
        labels = {k: v for k, v in result["metric"].items() if k != "__name__"}
        values = result["values"]

        if len(values) < 2:
            continue

        series_id = ",".join(f"{k}={v}" for k, v in sorted(labels.items()))

        df = pandas.DataFrame(
            [
                [
                    datetime.datetime.fromtimestamp(float(ts), tz=datetime.UTC),
                    float(val),
                ]
                for ts, val in values
            ],
            columns=["ds", "y"],
        )
        df["unique_id"] = series_id
        frames.append(df)
        series_labels[series_id] = labels

    if not frames:
        logger.warning("No data to predict for %s", name)
        return

    combined = pandas.concat(frames, ignore_index=True)

    models = [
        statsforecast.models.AutoARIMA(),
        statsforecast.models.AutoETS(),
        statsforecast.models.AutoTheta(),
    ]
    sf = statsforecast.StatsForecast(models=models, freq=freq, n_jobs=1)
    forecast = sf.forecast(h=periods, df=combined)

    for series_id, labels in series_labels.items():
        series_forecast = forecast.loc[series_id]
        predictions = numpy.median(
            series_forecast[["AutoARIMA", "AutoETS", "AutoTheta"]].values, axis=1
        )

        label_keys = list(labels.keys())
        label_values = list(labels.values())

        gauge_key = name + ":" + ",".join(label_keys)
        if gauge_key not in gauge_map:
            gauge_map[gauge_key] = prometheus_client.Gauge(
                name + "_prediction",
                "",
                label_keys + ["predicted_period", "frequency"],
                registry=registry,
            )

        for i, value in enumerate(predictions):
            gauge_map[gauge_key].labels(
                *label_values,
                str(i + 1),
                frequency,
            ).set(value)

    prometheus_client.push_to_gateway(
        s.pushgateway_host,
        job="promphet",
        grouping_key={"query": urllib.parse.quote(query, safe="") + ":" + frequency},
        registry=registry,
    )
    logger.info("Pushed predictions for %s", name)


def main() -> None:
    with open(s.config_file) as f:
        configuration = yaml.safe_load(f)

    jobs: list[dict] = configuration.get("jobs", [])
    if not jobs:
        logger.warning("No jobs defined in %s", s.config_file)
        return

    errors = []
    for job in jobs:
        try:
            run_job(job)
        except Exception as e:
            logger.exception("Job %s failed: %s", job.get("name"), e)
            errors.append(e)

    if errors:
        sys.exit(1)


if __name__ == "__main__":
    main()
