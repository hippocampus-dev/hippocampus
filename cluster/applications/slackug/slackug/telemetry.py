import opentelemetry.metrics
import opentelemetry.trace

import slackug.context_logging

tracer = opentelemetry.trace.get_tracer("slackug")
meter = opentelemetry.metrics.get_meter("slackug")
logger = slackug.context_logging.getLogger("slackug")
