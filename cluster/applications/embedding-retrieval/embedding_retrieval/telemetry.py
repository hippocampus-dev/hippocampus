import opentelemetry.metrics
import opentelemetry.trace

import embedding_retrieval.context_logging

tracer = opentelemetry.trace.get_tracer("embedding-retrieval")
meter = opentelemetry.metrics.get_meter("embedding-retrieval")
logger = embedding_retrieval.context_logging.getLogger("embedding-retrieval")
