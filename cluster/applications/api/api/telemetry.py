import opentelemetry.metrics
import opentelemetry.trace

import api.context_logging

tracer = opentelemetry.trace.get_tracer("api")
meter = opentelemetry.metrics.get_meter("api")
logger = api.context_logging.getLogger("api")
