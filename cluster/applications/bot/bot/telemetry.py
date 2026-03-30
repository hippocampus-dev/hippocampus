import opentelemetry.metrics
import opentelemetry.trace

import bot.context_logging

tracer = opentelemetry.trace.get_tracer("bot")
meter = opentelemetry.metrics.get_meter("bot")
logger = bot.context_logging.getLogger("bot")
