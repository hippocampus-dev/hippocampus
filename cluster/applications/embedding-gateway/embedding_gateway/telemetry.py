import opentelemetry.metrics
import opentelemetry.trace

import embedding_gateway.context_logging

tracer = opentelemetry.trace.get_tracer("embedding-gateway")
meter = opentelemetry.metrics.get_meter("embedding-gateway")
logger = embedding_gateway.context_logging.getLogger("embedding-gateway")
