import abc
import collections.abc
import os

import httpx
import openai
import tiktoken

import cortex.exceptions
import cortex.llm.openai.model


class Summarizer(abc.ABC):
    model: cortex.llm.openai.model.CompletionModel
    encoder: tiktoken.Encoding

    def __init__(self, model: cortex.llm.openai.model.CompletionModel, encoder: tiktoken.Encoding):
        self.model = model
        self.encoder = encoder
        self.prompt_tokens = 0
        self.completion_tokens = 0

    @abc.abstractmethod
    def summarize(self, text: str) -> str:
        raise NotImplementedError

    async def _summary(self, prompt: str) -> str:
        messages = [{
            "role": "system",
            "content": prompt,
        }]
        try:
            response = await cortex.llm.openai.AsyncOpenAI(
                http_client=httpx.AsyncClient(proxies={
                    "http://": os.getenv("HTTP_PROXY"),
                    "https://": os.getenv("HTTPS_PROXY"),
                }, verify=os.getenv("SSL_CERT_FILE")),
            ).chat.completions.create(
                model=self.model.replace(".", "") if os.getenv("OPENAI_API_TYPE") == "azure" else self.model,
                messages=messages,
                temperature=1,
                top_p=1,
                n=1,
                max_tokens=None,
                presence_penalty=0,
                frequency_penalty=0,
                stream=False,
            )
        except openai.APIConnectionError as e:
            raise cortex.exceptions.RetryableError(e) from e
        except openai.APIStatusError as e:
            match e.status_code:
                case 409 | 429 | 502 | 503 | 504:
                    raise cortex.exceptions.RetryableError(e) from e
            raise e

        self.prompt_tokens += response.usage.prompt_tokens
        self.completion_tokens += response.usage.completion_tokens

        return response.choices[0].message.content

    def _get_text_chunks(self, text: str, chunk_size: int) -> collections.abc.Sequence[str]:
        tokens = self.encoder.encode(text, disallowed_special=())
        if len(tokens) <= chunk_size:
            return [text]

        chunks = []

        while tokens:
            chunk_text = self._punctuate(self.encoder.decode(tokens[:chunk_size]))

            tokens = tokens[len(self.encoder.encode(chunk_text, disallowed_special=())):]

            chunks.append(chunk_text)

        return chunks

    def _punctuate(self, text: str) -> str:
        last_punctuation = max(
            text.rfind("."),
            text.rfind("．"),
            text.rfind("。"),
            text.rfind("?"),
            text.rfind("？"),
            text.rfind("!"),
            text.rfind("！"),
            text.rfind("\n"),
        )

        if last_punctuation != -1:
            return text[:(last_punctuation + 1)]

        return text
