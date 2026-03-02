import os

import httpx
import openai

import cortex.llm
from .model import ModerationModel


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


class OpenAIModerator(cortex.llm.Moderator):
    model: ModerationModel

    def __init__(self, model: ModerationModel):
        self.model = model

    async def moderate(self, content: str) -> bool:
        response = await cortex.llm.openai.AsyncOpenAI(
            http_client=httpx.AsyncClient(timeout=None, mounts={
                "http://": httpx.AsyncHTTPTransport(proxy=os.getenv("HTTP_PROXY")),
                "https://": httpx.AsyncHTTPTransport(proxy=os.getenv("HTTPS_PROXY")),
            }, verify=os.getenv("SSL_CERT_FILE")),
        ).moderations.create(
            model=self.model,
            input=content,
        )
        return response.results[0].flagged
