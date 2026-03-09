import opentelemetry.metrics
import opentelemetry.trace

import slack_bolt_proxy.context_logging

tracer = opentelemetry.trace.get_tracer("slack-bolt-proxy")
meter = opentelemetry.metrics.get_meter("slack-bolt-proxy")
logger = slack_bolt_proxy.context_logging.getLogger("slack-bolt-proxy")
