import opentelemetry.metrics
import opentelemetry.trace

import translator.context_logging

tracer = opentelemetry.trace.get_tracer("translator")
meter = opentelemetry.metrics.get_meter("translator")
logger = translator.context_logging.getLogger("translator")
