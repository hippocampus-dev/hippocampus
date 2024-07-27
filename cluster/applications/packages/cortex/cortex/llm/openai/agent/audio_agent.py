import base64
import io
import os
import shutil
import tempfile

import httpx
import openai
import tiktoken

import cortex.exceptions
import cortex.llm.openai.model


class AudioAgent(cortex.llm.openai.agent.Agent):
    audio_model: cortex.llm.openai.model.AudioModel

    def __init__(
        self,
        model: cortex.llm.openai.model.CompletionModel,
        encoder: tiktoken.Encoding,
        audio_model: cortex.llm.openai.model.AudioModel,
    ):
        self.model = model
        self.encoder = encoder
        self.audio_model = audio_model
        self.functions = {
            "transcribe": cortex.llm.openai.agent.FunctionDefinition(
                func=self.transcribe,
                cache=cortex.llm.openai.agent.FunctionCache(enabled=True, similarity_threshold=1.0),
                direct_return=True,
                description="Transcribe an audio file.",
                parameters={
                    "type": "object",
                    "properties": {
                        "content": {
                            "type": "string",
                            "description": "What file you need to transcribe. e.g. 'https://files.slack.com/files-pri/T12345-F0S43P1CZ/audio.mp4 content', 'downloaded audio file'",
                        },
                    },
                    "required": ["content"],
                },
            ),
            "text_to_speech": cortex.llm.openai.agent.FunctionDefinition(
                func=self.text_to_speech,
                cache=cortex.llm.openai.agent.FunctionCache(enabled=True, similarity_threshold=1.0),
                direct_return=True,
                description="Convert a text to speech.",
                parameters={
                    "type": "object",
                    "properties": {
                        "content": {
                            "type": "string",
                            "description": "What text you need to convert. e.g. `Hi, I'm cortex. Nice to meet you.`"
                        },
                    },
                    "required": ["content"],
                },
            ),
        }

    async def transcribe(
        self,
        content: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        histories = await context.restore_histories(query=content, similarity_threshold=0.4, namespace="content")
        if len(histories) == 0:
            return cortex.llm.openai.agent.FunctionResult(response="Content not found")

        latest_history = sorted(histories, key=lambda x: x.created_at)[-1]

        with tempfile.NamedTemporaryFile(suffix=".mp4") as f:
            shutil.copyfileobj(io.BytesIO(latest_history.body), f)
            f.seek(0)

            try:
                response = await cortex.llm.openai.AsyncOpenAI(
                    http_client=httpx.AsyncClient(proxies={
                        "http://": os.getenv("HTTP_PROXY"),
                        "https://": os.getenv("HTTPS_PROXY"),
                    }, verify=os.getenv("SSL_CERT_FILE")),
                ).audio.transcriptions.create(
                    model=self.audio_model,
                    file=f.file,
                    response_format="verbose_json",
                )
            except openai.APIConnectionError as e:
                raise cortex.exceptions.RetryableError(e) from e
            except openai.APIStatusError as e:
                match e.status_code:
                    case 400:
                        return cortex.llm.openai.agent.FunctionResult(response="Content is not an audio file")
                    case 409 | 429 | 502 | 503 | 504:
                        raise cortex.exceptions.RetryableError(e) from e
                raise e

        d = response.dict()
        context.increment_processed_audio_seconds(self.audio_model, d.get("audio_seconds", 0))
        return cortex.llm.openai.agent.FunctionResult(response=d["text"])

    async def text_to_speech(
        self,
        content: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        try:
            response = await cortex.llm.openai.AsyncOpenAI(
                http_client=httpx.AsyncClient(proxies={
                    "http://": os.getenv("HTTP_PROXY"),
                    "https://": os.getenv("HTTPS_PROXY"),
                }, verify=os.getenv("SSL_CERT_FILE")),
            ).audio.speech.create(
                model=cortex.llm.openai.model.AudioModel.TTS1,
                voice="alloy",
                input=content,
            )
        except openai.APIConnectionError as e:
            raise cortex.exceptions.RetryableError(e) from e
        except openai.APIStatusError as e:
            match e.status_code:
                case 409 | 429 | 502 | 503 | 504:
                    raise cortex.exceptions.RetryableError(e) from e
            raise e

        await context.save_history(
            f"Generated audio file",
            self.functions["text_to_speech"].cache.expire,
            cortex.llm.openai.agent.History(body=await response.aread()),
            namespace="content",
        )

        context.increment_converted_text_characters(cortex.llm.openai.model.AudioModel.TTS1, len(content))
        return cortex.llm.openai.agent.FunctionResult(
            instruction="Then you need to upload `Generated audio file`.",
            response="Generated audio file is successfully saved.",
        )
