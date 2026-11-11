import cortex.llm


class RetryableError(Exception):
    pass


class InsufficientBudgetError(Exception):
    pass


class GuardrailTripwireError(Exception):
    def __init__(
        self,
        guardrail: cortex.llm.Guardrail,
        result: cortex.llm.GuardrailResult,
    ):
        self.guardrail = guardrail
        self.result = result
        super().__init__(result.reason or "guardrail tripwire triggered")
