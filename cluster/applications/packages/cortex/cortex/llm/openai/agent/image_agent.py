import base64
import io
import os.path
import tempfile

import PIL.Image
import httpx
import openai
import tiktoken

import cortex.exceptions
import cortex.llm.openai.model


class ImageAgent(cortex.llm.openai.agent.Agent):
    image_model: cortex.llm.openai.model.ImageModel

    def __init__(
        self,
        model: cortex.llm.openai.model.CompletionModel,
        encoder: tiktoken.Encoding,
        image_model: cortex.llm.openai.model.ImageModel,
    ):
        self.model = model
        self.encoder = encoder
        self.image_model = image_model
        self.functions = {
            "explain_image": cortex.llm.openai.agent.FunctionDefinition(
                func=self.explain_image,
                cache=cortex.llm.openai.agent.FunctionCache(enabled=True, similarity_threshold=1.0),
                direct_return=True,
                description="Explains an image given a content.",
                parameters={
                    "type": "object",
                    "properties": {
                        "content": {
                            "type": "string",
                            "description": "What image you need to explain. e.g. 'https://files.slack.com/files-pri/T12345-F0S43P1CZ/image.png content', 'screenshotted image'",
                        },
                    },
                    "required": ["content"],
                },
            ),
            "generate_image": cortex.llm.openai.agent.FunctionDefinition(
                func=self.generate_image,
                direct_return=True,
                description="Creates an image given a prompt.",
                parameters={
                    "type": "object",
                    "properties": {
                        "prompt": {
                            "type": "string",
                            "description": "A text description of the desired image(s). The maximum length is 1000 characters.",
                        },
                    },
                    "required": ["prompt"],
                },
            ),
        }

        if self.image_model.model_name == "dall-e-2":
            self.functions["generate_image_variation"] = cortex.llm.openai.agent.FunctionDefinition(
                func=self.generate_image_variation,
                direct_return=True,
                description="Creates a other variation given a content.",
                parameters={
                    "type": "object",
                    "properties": {
                        "content": {
                            "type": "string",
                            "description": "What image you need to generate variation. e.g. 'https://files.slack.com/files-pri/T12345-F0S43P1CZ/image.png content', 'generated image'",
                        },
                    },
                    "required": ["content"],
                },
            )

    async def explain_image(
        self,
        content: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        histories = await context.restore_histories(query=content, similarity_threshold=0.4, namespace="content")
        if len(histories) == 0:
            return cortex.llm.openai.agent.FunctionResult(response="Content not found")

        latest_history = sorted(histories, key=lambda x: x.created_at)[-1]

        if len(latest_history.body) > cortex.llm.openai.model.CompletionModel.GPT4O.limit_size_of_image:
            return cortex.llm.openai.agent.FunctionResult(response="Content is too large")

        try:
            response = await cortex.llm.openai.AsyncOpenAI(
                http_client=httpx.AsyncClient(proxies={
                    "http://": os.getenv("HTTP_PROXY"),
                    "https://": os.getenv("HTTPS_PROXY"),
                }, verify=os.getenv("SSL_CERT_FILE")),
            ).chat.completions.create(
                model=cortex.llm.openai.model.CompletionModel.GPT4O,
                messages=[
                    {
                        "role": "user",
                        "content": [
                            {
                                "type": "text",
                                "text": "Explain this image.",
                            },
                            {
                                "type": "image_url",
                                "image_url": {
                                    "url": f"data:image/jpeg;base64,{base64.b64encode(latest_history.body).decode()}",
                                },
                            },
                        ],
                    },
                ],
                max_tokens=cortex.llm.openai.model.CompletionModel.GPT4O.max_completion_tokens,
            )
        except openai.APIConnectionError as e:
            raise cortex.exceptions.RetryableError(e) from e
        except openai.APIStatusError as e:
            match e.status_code:
                case 400:
                    return cortex.llm.openai.agent.FunctionResult(response="Content is not an image")
                case 409 | 429 | 502 | 503 | 504:
                    raise cortex.exceptions.RetryableError(e) from e
            raise e

        context.increment_prompt_tokens(
            cortex.llm.openai.model.CompletionModel.GPT4O,
            response.usage.prompt_tokens,
        )
        context.increment_completion_tokens(
            cortex.llm.openai.model.CompletionModel.GPT4O,
            response.usage.completion_tokens,
        )
        return cortex.llm.openai.agent.FunctionResult(response=response.choices[0].message.content)

    async def generate_image(
        self,
        prompt: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        try:
            response = await cortex.llm.openai.AsyncOpenAI(
                http_client=httpx.AsyncClient(proxies={
                    "http://": os.getenv("HTTP_PROXY"),
                    "https://": os.getenv("HTTPS_PROXY"),
                }, verify=os.getenv("SSL_CERT_FILE")),
            ).images.generate(
                prompt=prompt,
                n=1,
                model=self.image_model.model_name,
                size=self.image_model.resolution,
                response_format="b64_json",
            )
        except openai.APIConnectionError as e:
            raise cortex.exceptions.RetryableError(e) from e
        except openai.APIStatusError as e:
            match e.status_code:
                case 409 | 429 | 502 | 503 | 504:
                    raise cortex.exceptions.RetryableError(e) from e
            raise e

        await context.save_history(
            f"Generated image",
            self.functions["generate_image"].cache.expire,
            cortex.llm.openai.agent.History(body=base64.b64decode(response.data[0].b64_json)),
            namespace="content",
        )

        context.increment_generated_images(self.image_model, 1)
        return cortex.llm.openai.agent.FunctionResult(
            instruction="Then you need to upload `Generated image`.",
            response=(
                "Generated image is successfully saved.\n"
                f"description:\n```\n{response.data[0].revised_prompt}\n```"
            ),
        )

    async def generate_image_variation(
        self,
        content: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        histories = await context.restore_histories(query=content, similarity_threshold=0.4, namespace="content")
        if len(histories) == 0:
            return cortex.llm.openai.agent.FunctionResult(response="Content not found")

        latest_history = sorted(histories, key=lambda x: x.created_at)[-1]

        with tempfile.NamedTemporaryFile(suffix=".png") as f:
            try:
                PIL.Image.open(io.BytesIO(latest_history.body)).convert("RGBA").save(f.name)
            except Exception:
                return cortex.llm.openai.agent.FunctionResult(response="Content not found")

            # https://platform.openai.com/docs/api-reference/images/create-edit
            if os.path.getsize(f.name) > 4 * 1024 * 1024:
                return cortex.llm.openai.agent.FunctionResult(response="Content is too large")

            try:
                response = await cortex.llm.openai.AsyncOpenAI(
                    http_client=httpx.AsyncClient(proxies={
                        "http://": os.getenv("HTTP_PROXY"),
                        "https://": os.getenv("HTTPS_PROXY"),
                    }, verify=os.getenv("SSL_CERT_FILE")),
                ).images.create_variation(
                    image=f,
                    n=1,
                    model=self.image_model.model_name,
                    size=self.image_model.resolution,
                    response_format="b64_json",
                )
            except openai.APIConnectionError as e:
                raise cortex.exceptions.RetryableError(e) from e
            except openai.APIStatusError as e:
                match e.status_code:
                    case 409 | 429 | 502 | 503 | 504:
                        raise cortex.exceptions.RetryableError(e) from e
                raise e

        await context.save_history(
            f"Generated image",
            self.functions["generate_image_variation"].cache.expire,
            cortex.llm.openai.agent.History(body=base64.b64decode(response.data[0].b64_json)),
            namespace="content",
        )

        context.increment_generated_images(self.image_model, 1)
        return cortex.llm.openai.agent.FunctionResult(
            instruction="Then you need to upload `Generated image`.",
            response=(
                "Generated image is successfully saved.\n"
                f"description:\n```\n{response.data[0].revised_prompt}\n```"
            ),
        )
