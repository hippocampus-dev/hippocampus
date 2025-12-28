import abc
import asyncio
import base64
import collections.abc
import copy
import enum
import json
import os
import re
import time
import typing

import aredis_om
import aredis_om.model
import httpx
import numpy
import openai
import openai.types.chat
import openai.types.shared_params
import pydantic
import tiktoken

import cortex.exceptions
import cortex.llm.openai.agent.memory
import cortex.llm.openai.model
import cortex.llm.openai.summarizer.map_reduce
import cortex.llm.openai.summarizer.refine


class MemoryType(enum.StrEnum):
    Redis = "redis"


class ProgressStage(enum.StrEnum):
    FunctionCalling = "function_calling"
    ResponseStarting = "response_starting"


class History(pydantic.BaseModel):
    body: bytes
    created_at: float | None = None


DEFAULT_EXPIRE = 60 * 60 * 24 * 7
OPENAI_FUNCTION_CALLING_NAME_REGEXP = re.compile(r"^[a-zA-Z0-9_-]{1,64}$")


class FunctionResult(pydantic.BaseModel):
    instruction: str | None = None
    response: str

    def __str__(self):
        if self.instruction is not None:
            return (
                f"### Instruction\n"
                f"{self.instruction}\n"
                f"\n"
                f"### Response\n"
                f"{self.response}\n"
            )
        return self.response


class FunctionCache(pydantic.BaseModel):
    enabled: bool = False
    similarity_threshold: float = 0.9
    expire: int = DEFAULT_EXPIRE


class Capability(enum.IntEnum):
    DEFAULT = 1 << 0
    INTERNAL = 1 << 1
    ALL = (1 << 31) - 1


class FunctionDefinition(pydantic.BaseModel):
    func: typing.Callable[..., typing.Awaitable[FunctionResult]]
    cache: FunctionCache = pydantic.Field(default_factory=FunctionCache)
    direct_return: bool = False
    consumption_budget: int = 1
    description: str
    strict: bool = False
    parameters: collections.abc.Mapping[str, typing.Any]
    capability: int = Capability.DEFAULT
    dependencies: set["FunctionDefinition"] = pydantic.Field(default_factory=set)
    escalation: typing.Callable[["Context"], str | None] | None = None

    def __hash__(self):
        return hash(self.func)


class Context(abc.ABC):
    context_id: str
    memory_type: MemoryType
    embedding_model: cortex.llm.openai.model.EmbeddingModel
    encoder: tiktoken.Encoding
    prompt_tokens: collections.abc.MutableMapping[cortex.llm.openai.model.CompletionModel, int]
    completion_tokens: collections.abc.MutableMapping[cortex.llm.openai.model.CompletionModel, int]
    embedding_tokens: collections.abc.MutableMapping[cortex.llm.openai.model.EmbeddingModel, int]
    generated_images: collections.abc.MutableMapping[cortex.llm.openai.model.ImageModel, int]
    processed_audio_seconds: collections.abc.MutableMapping[cortex.llm.openai.model.AudioModel, float]
    current_messages: collections.abc.Sequence[collections.abc.Mapping[str, typing.Any]]
    call_stack: collections.abc.MutableSet[FunctionDefinition]

    def __init__(
        self,
        context_id: str,
        memory_type: MemoryType,
        embedding_model: cortex.llm.openai.model.EmbeddingModel,
        encoder: tiktoken.Encoding,
    ):
        self.context_id = context_id
        self.memory_type = memory_type
        self.embedding_model = embedding_model
        self.encoder = encoder
        self.prompt_tokens = {}
        self.completion_tokens = {}
        self.embedding_tokens = {}
        self.generated_images = {}
        self.processed_audio_seconds = {}
        self.converted_text_characters = {}
        self.current_messages = list()
        self.call_stack = set()

    @property
    def capability(self) -> Capability:
        return Capability.ALL

    @property
    def limit(self) -> int | None:
        return None

    @abc.abstractmethod
    async def report_progress(self, message: str, stage: ProgressStage):
        raise NotImplementedError

    async def cache_history(
        self,
        query: str,
        expire: int,
        history: History,
        namespace: str | None = None,
    ) -> str:
        context_id = self.context_id if namespace is None else f"{namespace}:{self.context_id}"
        match self.memory_type:
            case MemoryType.Redis:
                result = await cortex.llm.openai.AsyncOpenAI(
                    http_client=httpx.AsyncClient(timeout=None, mounts={
                        "http://": httpx.AsyncHTTPTransport(proxy=os.getenv("HTTP_PROXY")),
                        "https://": httpx.AsyncHTTPTransport(proxy=os.getenv("HTTPS_PROXY")),
                    }, verify=os.getenv("SSL_CERT_FILE")),
                ).embeddings.create(
                    input=[query],
                    model=self.embedding_model,
                    dimensions=cortex.llm.openai.model.OPENAI_VECTOR_SIZE if self.embedding_model in (
                        cortex.llm.openai.model.EmbeddingModel.ADA_V3_SMALL,
                        cortex.llm.openai.model.EmbeddingModel.ADA_V3_LARGE,
                    ) else openai._types.NOT_GIVEN,
                )
                saved_history = await cortex.llm.openai.agent.memory.RedisMemory(
                    context_id=context_id,
                    text=query,
                    embedding=result.data[0].embedding,
                    history=base64.b64encode(history.body),
                    created_at=time.time() if history.created_at is None else history.created_at,
                ).save()
                await saved_history.expire(expire)

                embedding_tokens = len(self.encoder.encode(query, disallowed_special=()))
                self.increment_embedding_tokens(self.embedding_model, embedding_tokens)

                return saved_history.pk
            case _:
                raise NotImplementedError

    async def restore_histories(
        self,
        pk: str | None = None,
        query: str | None = None,
        similarity_threshold: float = 0.9,
        namespace: str | None = None,
    ) -> collections.abc.Sequence[History]:
        context_id = self.context_id if namespace is None else f"{namespace}:{self.context_id}"
        match self.memory_type:
            case MemoryType.Redis:
                try:
                    if pk is not None:
                        history_memory = await cortex.llm.openai.agent.memory.RedisMemory.find(
                            cortex.llm.openai.agent.memory.RedisMemory.pk == pk,
                        ).first()
                        return [history_memory.history]
                    if query is not None:
                        result = await cortex.llm.openai.AsyncOpenAI(
                            http_client=httpx.AsyncClient(timeout=None, mounts={
                                "http://": httpx.AsyncHTTPTransport(proxy=os.getenv("HTTP_PROXY")),
                                "https://": httpx.AsyncHTTPTransport(proxy=os.getenv("HTTPS_PROXY")),
                            }, verify=os.getenv("SSL_CERT_FILE")),
                        ).embeddings.create(
                            input=[query],
                            model=self.embedding_model,
                            dimensions=cortex.llm.openai.model.OPENAI_VECTOR_SIZE if self.embedding_model in (
                                cortex.llm.openai.model.EmbeddingModel.ADA_V3_SMALL,
                                cortex.llm.openai.model.EmbeddingModel.ADA_V3_LARGE,
                            ) else openai._types.NOT_GIVEN,
                        )

                        memories = await cortex.llm.openai.agent.memory.RedisMemory.find(
                            cortex.llm.openai.agent.memory.RedisMemory.context_id == context_id,
                            knn=aredis_om.KNNExpression(
                                k=3,
                                vector_field=cortex.llm.openai.agent.memory.RedisMemory.embedding,
                                score_field=cortex.llm.openai.agent.memory.RedisMemory.embedding_score,
                                reference_vector=numpy.array(
                                    result.data[0].embedding,
                                    dtype=numpy.float64,
                                ).tobytes(),
                            ),
                        ).all()

                        embedding_tokens = len(self.encoder.encode(query, disallowed_special=()))
                        self.increment_embedding_tokens(self.embedding_model, embedding_tokens)

                        return [
                            History(body=base64.b64decode(m.history), created_at=m.created_at)
                            for m in memories
                            if m.similarity >= similarity_threshold
                        ]
                except aredis_om.model.model.NotFoundError:
                    return []
                else:
                    return []

            case _:
                raise NotImplementedError

    async def save_messages(self, messages: collections.abc.Sequence[collections.abc.Mapping[str, typing.Any]]):
        match self.memory_type:
            case MemoryType.Redis:
                length = await cortex.llm.openai.agent.memory.RedisMemory.db().zcard(f"{self.context_id}:messages")
                for i, message in enumerate(messages):
                    await cortex.llm.openai.agent.memory.RedisMemory.db().zadd(
                        f"{self.context_id}:messages",
                        {json.dumps(message, ensure_ascii=False, separators=(",", ":")): length + i},
                    )
                await cortex.llm.openai.agent.memory.RedisMemory.db().expire(
                    f"{self.context_id}:messages",
                    DEFAULT_EXPIRE,
                )
            case _:
                raise NotImplementedError

    async def load_messages(self) -> collections.abc.Sequence[collections.abc.Mapping[str, typing.Any]]:
        match self.memory_type:
            case MemoryType.Redis:
                messages = await cortex.llm.openai.agent.memory.RedisMemory.db().zrange(
                    f"{self.context_id}:messages",
                    0, -1,
                )
                if messages is None:
                    return []
                return [json.loads(message) for message in messages]
            case _:
                raise NotImplementedError

    async def get_budget(self) -> int:
        match self.memory_type:
            case MemoryType.Redis:
                budget = await cortex.llm.openai.agent.memory.RedisMemory.db().get(f"{self.context_id}:budget")
                if budget is None:
                    return 0
                return int(budget)
            case _:
                raise NotImplementedError

    async def acquire_budget(self, budget: int):
        match self.memory_type:
            case MemoryType.Redis:
                await cortex.llm.openai.agent.memory.RedisMemory.db().setex(
                    f"{self.context_id}:budget",
                    DEFAULT_EXPIRE,
                    budget,
                )
            case _:
                raise NotImplementedError

    async def consume_budget(self, budget: int) -> bool:
        match self.memory_type:
            case MemoryType.Redis:
                remaining_budget = await self.get_budget()
                if remaining_budget < budget:
                    return False
                await cortex.llm.openai.agent.memory.RedisMemory.db().decr(f"{self.context_id}:budget", budget)
                return True
            case _:
                raise NotImplementedError

    def increment_prompt_tokens(self, model: cortex.llm.openai.model.CompletionModel, prompt_tokens: int):
        if model not in self.prompt_tokens:
            self.prompt_tokens[model] = 0
        self.prompt_tokens[model] += prompt_tokens

    def increment_completion_tokens(self, model: cortex.llm.openai.model.CompletionModel, completion_tokens: int):
        if model not in self.completion_tokens:
            self.completion_tokens[model] = 0
        self.completion_tokens[model] += completion_tokens

    def increment_embedding_tokens(self, model: cortex.llm.openai.model.EmbeddingModel, embedding_tokens: int):
        if model not in self.embedding_tokens:
            self.embedding_tokens[model] = 0
        self.embedding_tokens[model] += embedding_tokens

    def increment_generated_images(self, model: cortex.llm.openai.model.ImageModel, count: int):
        if model not in self.generated_images:
            self.generated_images[model] = 0
        self.generated_images[model] += count

    def increment_processed_audio_seconds(self, model: cortex.llm.openai.model.AudioModel, duration: float):
        if model not in self.processed_audio_seconds:
            self.processed_audio_seconds[model] = 0
        self.processed_audio_seconds[model] += duration

    def increment_converted_text_characters(self, model: cortex.llm.openai.model.AudioModel, characters: int):
        if model not in self.converted_text_characters:
            self.converted_text_characters[model] = 0
        self.converted_text_characters[model] += characters


async def deconstruct_function_call_from_response(
    response: openai.types.chat.ChatCompletion | openai.AsyncStream[openai.types.chat.ChatCompletionChunk],
) -> collections.abc.Sequence[openai.types.chat.ChatCompletionMessageToolCall]:
    function_calls = []
    if isinstance(response, openai.AsyncStream):
        async for r in response:
            if len(r.choices) == 0:
                continue

            choice = r.choices[0]

            # Skip the function calling when the response is not a function call
            if os.getenv("OPENAI_API_TYPE") == "azure":
                if choice.delta.tool_calls is None or choice.delta.refusal is not None:
                    return function_calls
            else:
                if choice.delta.content is not None or choice.delta.refusal is not None:
                    return function_calls

            if choice.delta.tool_calls is not None:
                for tool_call in choice.delta.tool_calls:
                    if tool_call.id is not None and tool_call.type is not None and tool_call.function is not None:
                        function_calls.append(openai.types.chat.ChatCompletionMessageToolCall(
                            id=tool_call.id,
                            type=tool_call.type,
                            function=tool_call.function.model_dump(),
                        ))
                    function_calls[tool_call.index].function.arguments += tool_call.function.arguments
        return function_calls

    if len(response.choices) > 0:
        choice = response.choices[0]
        if choice.message.tool_calls is not None:
            return choice.message.tool_calls

    return function_calls


async def construct_function_result_from_response(
    function_result: FunctionResult,
    response: openai.types.chat.ChatCompletion | openai.AsyncStream[openai.types.chat.ChatCompletionChunk],
) -> openai.types.chat.ChatCompletion | typing.AsyncGenerator[openai.types.chat.ChatCompletionChunk, None]:
    if isinstance(response, openai.AsyncStream):
        async def async_generator():
            yield openai.types.chat.ChatCompletionChunk(
                choices=[
                    openai.types.chat.ChatCompletionMessage(
                        role="assistant",
                        content=str(function_result),
                    ),
                ],
            )

        return async_generator()

    response.choices[0].message.tool_calls = None
    response.choices[0].message.content = str(function_result)

    return response


class Agent(abc.ABC):
    model: cortex.llm.openai.model.CompletionModel
    reasoning_effort: openai.types.chat.ChatCompletionReasoningEffort = "medium"
    verbosity: typing.Literal["low", "medium", "high"] = "medium"
    encoder: tiktoken.Encoding
    system_prompt: str | None = None
    functions: collections.abc.MutableMapping[str, FunctionDefinition]
    summarize_history: bool = True

    async def chat_completion_loop(
        self,
        messages: collections.abc.Sequence[collections.abc.Mapping[str, typing.Any]],
        context: Context,
        response_format: openai.types.shared_params.ResponseFormatText | openai.types.shared_params.ResponseFormatJSONObject | openai.types.shared_params.ResponseFormatJSONSchema | None = None,
        stream: bool = False,
    ) -> openai.types.chat.ChatCompletion | typing.AsyncGenerator[openai.types.chat.ChatCompletionChunk, None]:
        if self.system_prompt is not None:
            messages = [
                {
                    "role": "system",
                    "content": self.system_prompt,
                },
                *messages,
            ]

        functions = [
            {
                "type": "function",
                "function": {
                    "name": key,
                    "description": definition.description,
                    "strict": definition.strict,
                    "parameters": definition.parameters,
                }
            }
            for key, definition in self.functions.items()
            if definition.capability & context.capability and (
                len(definition.dependencies) == 0 or definition.dependencies & context.call_stack
            )
        ]

        # Summarize messages when reaching the max tokens
        prompt_tokens = self._calculate_consumed_prompt_tokens(messages, functions)
        if self.summarize_history and prompt_tokens > self.model.max_tokens:
            summarizer = cortex.llm.openai.summarizer.refine.RefineSummarizer(
                cortex.llm.openai.model.CompletionModel.GPT35_TURBO_ALIAS,
                self.encoder,
            )
            summary = await summarizer.summarize(
                "".join([json.dumps(message, ensure_ascii=False, separators=(",", ":")) for message in messages[0:-1]]),
            )
            context.increment_prompt_tokens(
                cortex.llm.openai.model.CompletionModel.GPT35_TURBO_ALIAS,
                summarizer.prompt_tokens,
            )
            context.increment_completion_tokens(
                cortex.llm.openai.model.CompletionModel.GPT35_TURBO_ALIAS,
                summarizer.completion_tokens,
            )
            messages = [
                {
                    "role": "system",
                    "content": (
                        "Below is a summary of our conversation so far.\n"
                        "Please continue the conversation based on it.\n"
                        "---\n"
                        "{summary}\n"
                        "---"
                    ).format(summary=summary),
                },
                messages[-1],
            ]

        try:
            response = await cortex.llm.openai.AsyncOpenAI(
                http_client=httpx.AsyncClient(timeout=None, mounts={
                    "http://": httpx.AsyncHTTPTransport(proxy=os.getenv("HTTP_PROXY")),
                    "https://": httpx.AsyncHTTPTransport(proxy=os.getenv("HTTPS_PROXY")),
                }, verify=os.getenv("SSL_CERT_FILE")),
            ).chat.completions.create(
                model=self.model.replace(".", "") if os.getenv("OPENAI_API_TYPE") == "azure" else self.model,
                messages=messages,
                reasoning_effort=self.reasoning_effort if self.model.reasoning_supported else openai._types.NOT_GIVEN,
                verbosity=self.verbosity if self.model.verbosity_supported else openai._types.NOT_GIVEN,
                tools=[openai.types.chat.ChatCompletionToolParam(**function) for function in
                       functions] if self.model.tools_supported else openai._types.NOT_GIVEN,
                tool_choice="auto" if self.model.tools_supported else openai._types.NOT_GIVEN,
                max_completion_tokens=self.model.max_completion_tokens,
                response_format=response_format if response_format is not None else openai._types.NOT_GIVEN,
                stream=stream,
            )
        except openai.APIConnectionError as e:
            raise cortex.exceptions.RetryableError(e) from e
        except openai.APIStatusError as e:
            match e.status_code:
                case 404:
                    if e.message == "Engine not found":
                        raise cortex.exceptions.RetryableError(e) from e
                case 409 | 429 | 502 | 503 | 504:
                    raise cortex.exceptions.RetryableError(e) from e
            raise e
        except openai.APIError as e:
            match e.code:
                case "server_error" | "rate_limit_exceeded":
                    raise cortex.exceptions.RetryableError(e) from e
            raise e

        prompt_tokens = self._calculate_consumed_prompt_tokens(messages, functions)
        context.increment_prompt_tokens(self.model, prompt_tokens)

        try:
            function_calls = await deconstruct_function_call_from_response(response)
        except (httpx.RemoteProtocolError, httpx.ReadTimeout, openai.APIConnectionError) as e:
            raise cortex.exceptions.RetryableError(e) from e
        except openai.APIError as e:
            match e.code:
                case "server_error" | "rate_limit_exceeded":
                    raise cortex.exceptions.RetryableError(e) from e
            raise e

        if len(function_calls) == 0:
            # No need to execute any function
            await context.report_progress(
                f"{self.__class__.__name__}",
                stage=ProgressStage.ResponseStarting,
            )

            return response

        assistant_message = {
            "role": "assistant",
            "content": None,
            "tool_calls": [tool_call.model_dump() for tool_call in function_calls],
        }
        # Count the number of tokens in the function call
        completion_tokens = len(self.encoder.encode(
            json.dumps(
                assistant_message,
                ensure_ascii=False,
                separators=(",", ":"),
                default=lambda x: x.model_dump(),
            ),
            disallowed_special=(),
        ))
        context.increment_completion_tokens(self.model, completion_tokens)

        tool_messages = []

        for function_call in function_calls:
            name = function_call.function.name
            arguments = function_call.function.arguments

            # function calling may return an invalid function name
            if not OPENAI_FUNCTION_CALLING_NAME_REGEXP.match(name):
                replaced_function_call = openai.types.chat.ChatCompletionMessageToolCall(
                    **function_call.model_dump(),
                )
                replaced_function_call.function.name = "invalid"

                assistant_message["tool_calls"] = [
                    replaced_function_call.model_dump() if tool_call["id"] == function_call.id else tool_call
                    for tool_call in assistant_message["tool_calls"]
                ]
                tool_messages.append({
                    "tool_call_id": function_call.id,
                    "role": "tool",
                    "name": replaced_function_call.function.name,
                    "content": "name is not match with ^[a-zA-Z0-9_-]{1,64}$",
                })
                continue

            # function calling may return a nonexistent function name
            if name not in self.functions:
                tool_messages.append({
                    "tool_call_id": function_call.id,
                    "role": "tool",
                    "name": name,
                    "content": "name is not found",
                })
                continue

            # Execute a function when the budget is enough
            if not await context.consume_budget(self.functions[name].consumption_budget):
                raise cortex.exceptions.InsufficientBudgetError

            try:
                raw_arguments = json.loads(arguments)
            except json.JSONDecodeError:
                tool_messages.append({
                    "tool_call_id": function_call.id,
                    "role": "tool",
                    "name": name,
                    "content": "arguments is not a valid JSON string",
                })
                continue

            context.call_stack.add(self.functions[name])
            await context.report_progress(
                f"# {self.functions[name].description}\n"
                f"{self.__class__.__name__}.{name}({json.dumps(raw_arguments, ensure_ascii=False)})",
                stage=ProgressStage.FunctionCalling,
            )

            cache_key = json.dumps({"name": name, "arguments": raw_arguments}, ensure_ascii=False)

            function_result = await self._execute_function_or_cache(
                name,
                raw_arguments,
                cache_key,
                context,
            )

            # Directly return function_result as this agent's response
            if len(function_calls) == 1 and self.functions[name].direct_return:
                return await construct_function_result_from_response(function_result, response)

            remaining_tokens = self.model.max_tokens - prompt_tokens - completion_tokens
            function_result = await self._summary_function_result_with_cache(
                name,
                function_result,
                remaining_tokens,
                cache_key,
                context,
            )

            tool_messages.append({
                "tool_call_id": function_call.id,
                "role": "tool",
                "name": name,
                "content": str(function_result),
            })

        _ = asyncio.create_task(context.save_messages([assistant_message, *tool_messages]))

        next_messages = [*messages, assistant_message, *tool_messages]
        return await self.chat_completion_loop(
            next_messages,
            context,
            response_format=response_format,
            stream=stream,
        )

    def _calculate_consumed_prompt_tokens(
        self,
        messages: collections.abc.Sequence[collections.abc.MutableMapping[str, typing.Any]],
        functions: collections.abc.Sequence[collections.abc.Mapping[str, typing.Any]],
    ) -> int:
        consumed_messages = copy.deepcopy(messages)
        for message in consumed_messages:
            content = message.get("content")
            if not isinstance(content, collections.abc.MutableSequence):
                continue
            message["content"] = [
                c for c in content
                if not (isinstance(c, collections.abc.Mapping) and c.get("type") == "image_url")
            ]

        prompts = [
                      json.dumps(message, ensure_ascii=False, separators=(",", ":")) for message in consumed_messages
                  ] + [
                      json.dumps(function, ensure_ascii=False, separators=(",", ":")) for function in functions
                  ]
        prompt_tokens = len(self.encoder.encode("".join(prompts), disallowed_special=()))

        return prompt_tokens

    async def _execute_function_or_cache(
        self,
        name: str,
        raw_arguments: collections.abc.Mapping[str, typing.Any],
        cache_key: str,
        context: Context,
    ) -> FunctionResult:
        if self.functions[name].cache.enabled:
            cache = await context.restore_histories(
                query=cache_key,
                # Absorb shaking of function calling
                similarity_threshold=self.functions[name].cache.similarity_threshold,
            )

            if len(cache) > 0:
                return FunctionResult(
                    # Pick the first cache sorted by score
                    response=cache[0].body.decode("utf-8"),
                )

        actual_arguments = {
            key: value
            for key, value in raw_arguments.items()
            if key in self.functions[name].parameters.get("properties", {})
        }

        if set(self.functions[name].parameters.get("required", [])) - set(actual_arguments.keys()):
            return FunctionResult(
                response="Required parameters are missing",
            )

        return await self.functions[name].func(**actual_arguments, context=context)

    async def _summary_function_result_with_cache(
        self,
        name: str,
        function_result: FunctionResult,
        remaining_tokens: int,
        cache_key: str,
        context: Context,
    ) -> FunctionResult:
        tokens = len(self.encoder.encode(function_result.response, disallowed_special=()))
        if tokens > remaining_tokens:
            summarizer = cortex.llm.openai.summarizer.map_reduce.MapReduceSummarizer(
                cortex.llm.openai.model.CompletionModel.GPT35_TURBO_ALIAS,
                self.encoder,
                max_challenge=1,
            )
            function_result.response = await summarizer.summarize(function_result.response)
            context.increment_prompt_tokens(
                cortex.llm.openai.model.CompletionModel.GPT35_TURBO_ALIAS,
                summarizer.prompt_tokens,
            )
            context.increment_completion_tokens(
                cortex.llm.openai.model.CompletionModel.GPT35_TURBO_ALIAS,
                summarizer.completion_tokens,
            )

        if self.functions[name].cache.enabled:
            await context.cache_history(
                cache_key,
                self.functions[name].cache.expire,
                cortex.llm.openai.agent.History(body=bytes(function_result.response, encoding="utf-8")),
            )

        return function_result
