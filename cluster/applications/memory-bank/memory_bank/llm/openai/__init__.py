import os

import openai


def AsyncOpenAI(*args, **kwargs) -> openai.AsyncOpenAI:
    if os.getenv("OPENAI_API_TYPE") == "azure":
        return openai.AsyncAzureOpenAI(
            *args,
            **kwargs,
        )
    return openai.AsyncOpenAI(
        *args,
        **kwargs,
    )
