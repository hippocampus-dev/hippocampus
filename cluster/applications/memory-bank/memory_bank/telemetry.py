import opentelemetry.metrics
import opentelemetry.trace

import memory_bank.context_logging

tracer = opentelemetry.trace.get_tracer("memory-bank")
meter = opentelemetry.metrics.get_meter("memory-bank")
logger = memory_bank.context_logging.getLogger("memory-bank")
