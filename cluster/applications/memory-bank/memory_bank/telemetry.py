import memory_bank.context_logging
import opentelemetry.metrics
import opentelemetry.trace

tracer = opentelemetry.trace.get_tracer("memory-bank")
meter = opentelemetry.metrics.get_meter("memory-bank")
logger = memory_bank.context_logging.getLogger("memory-bank")
