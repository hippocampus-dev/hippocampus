import opentelemetry.metrics
import opentelemetry.trace

import meegeren.context_logging

tracer = opentelemetry.trace.get_tracer("meegeren")
meter = opentelemetry.metrics.get_meter("meegeren")
logger = meegeren.context_logging.getLogger("meegeren")
