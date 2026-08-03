import opentelemetry.metrics
import opentelemetry.trace

import lancer.context_logging

tracer = opentelemetry.trace.get_tracer("lancer")
meter = opentelemetry.metrics.get_meter("lancer")
logger = lancer.context_logging.getLogger("lancer")
