import collections.abc
import typing

import pydantic

import cortex.llm.openai.model


class AnthropicTextContent(pydantic.BaseModel):
    type: typing.Literal["text"]
    text: str


class AnthropicImageSource(pydantic.BaseModel):
    type: typing.Literal["base64", "url"]
    media_type: str | None = None
    data: str | None = None
    url: str | None = None


class AnthropicImageContent(pydantic.BaseModel):
    type: typing.Literal["image"]
    source: AnthropicImageSource


class AnthropicToolUseContent(pydantic.BaseModel):
    type: typing.Literal["tool_use"]
    id: str
    name: str
    input: dict[str, typing.Any]


class AnthropicToolResultContent(pydantic.BaseModel):
    type: typing.Literal["tool_result"]
    tool_use_id: str
    content: str | collections.abc.Sequence["AnthropicContentBlock"] | None = None
    is_error: bool | None = None


AnthropicContentBlock = (
    AnthropicTextContent
    | AnthropicImageContent
    | AnthropicToolUseContent
    | AnthropicToolResultContent
)


class AnthropicMessage(pydantic.BaseModel):
    role: typing.Literal["user", "assistant"]
    content: str | collections.abc.Sequence[AnthropicContentBlock]


class AnthropicToolInputSchema(pydantic.BaseModel):
    type: typing.Literal["object"] = "object"
    properties: dict[str, typing.Any] | None = None
    required: collections.abc.Sequence[str] | None = None


class AnthropicTool(pydantic.BaseModel):
    name: str
    description: str | None = None
    input_schema: AnthropicToolInputSchema


class V1Messages(pydantic.BaseModel):
    model: cortex.llm.openai.model.CompletionModel = pydantic.Field(
        ...,
        description="The model to use. Uses OpenAI model names (e.g., gpt-4o, gpt-4o-mini).",
    )
    messages: collections.abc.Sequence[AnthropicMessage] = pydantic.Field(
        ...,
        description="Input messages. Must alternate between user and assistant roles.",
    )
    max_tokens: int = pydantic.Field(
        ...,
        description="The maximum number of tokens to generate.",
    )
    system: str | collections.abc.Sequence[AnthropicTextContent] | None = (
        pydantic.Field(
            None,
            description="System prompt.",
        )
    )
    stream: bool = pydantic.Field(
        False,
        description="Whether to stream the response using server-sent events.",
    )
    stop_sequences: collections.abc.Sequence[str] | None = pydantic.Field(
        None,
        description="Custom text sequences that will cause the model to stop generating.",
    )
    temperature: float | None = pydantic.Field(
        None,
        ge=0.0,
        le=2.0,
        description="Sampling temperature between 0 and 2.",
    )
    top_p: float | None = pydantic.Field(
        None,
        ge=0.0,
        le=1.0,
        description="Nucleus sampling parameter.",
    )
    top_k: int | None = pydantic.Field(
        None,
        ge=0,
        description="Top-k sampling parameter. Not supported by OpenAI backend.",
    )
    tools: collections.abc.Sequence[AnthropicTool] | None = pydantic.Field(
        None,
        description="Tools the model may use.",
    )
    tool_choice: (
        typing.Literal["auto", "any", "none"] | dict[str, typing.Any] | None
    ) = pydantic.Field(
        None,
        description="How the model should use tools.",
    )


class AnthropicResponseTextBlock(pydantic.BaseModel):
    type: typing.Literal["text"] = "text"
    text: str


class AnthropicResponseToolUseBlock(pydantic.BaseModel):
    type: typing.Literal["tool_use"] = "tool_use"
    id: str
    name: str
    input: dict[str, typing.Any]


AnthropicResponseContentBlock = (
    AnthropicResponseTextBlock | AnthropicResponseToolUseBlock
)


class AnthropicUsage(pydantic.BaseModel):
    input_tokens: int
    output_tokens: int


class AnthropicResponse(pydantic.BaseModel):
    id: str
    type: typing.Literal["message"] = "message"
    role: typing.Literal["assistant"] = "assistant"
    content: collections.abc.Sequence[AnthropicResponseContentBlock]
    model: str
    stop_reason: (
        typing.Literal["end_turn", "max_tokens", "stop_sequence", "tool_use"] | None
    )
    stop_sequence: str | None = None
    usage: AnthropicUsage


class MessageStartEventMessage(pydantic.BaseModel):
    id: str
    type: typing.Literal["message"] = "message"
    role: typing.Literal["assistant"] = "assistant"
    content: collections.abc.Sequence[AnthropicResponseContentBlock] = []
    model: str
    stop_reason: str | None = None
    stop_sequence: str | None = None
    usage: AnthropicUsage


class MessageStartEvent(pydantic.BaseModel):
    type: typing.Literal["message_start"] = "message_start"
    message: MessageStartEventMessage


class ContentBlockStartEvent(pydantic.BaseModel):
    type: typing.Literal["content_block_start"] = "content_block_start"
    index: int
    content_block: AnthropicResponseContentBlock


class TextDelta(pydantic.BaseModel):
    type: typing.Literal["text_delta"] = "text_delta"
    text: str


class InputJsonDelta(pydantic.BaseModel):
    type: typing.Literal["input_json_delta"] = "input_json_delta"
    partial_json: str


class ContentBlockDeltaEvent(pydantic.BaseModel):
    type: typing.Literal["content_block_delta"] = "content_block_delta"
    index: int
    delta: TextDelta | InputJsonDelta


class ContentBlockStopEvent(pydantic.BaseModel):
    type: typing.Literal["content_block_stop"] = "content_block_stop"
    index: int


class MessageDeltaUsage(pydantic.BaseModel):
    output_tokens: int


class MessageDeltaDelta(pydantic.BaseModel):
    stop_reason: (
        typing.Literal["end_turn", "max_tokens", "stop_sequence", "tool_use"] | None
    ) = None
    stop_sequence: str | None = None


class MessageDeltaEvent(pydantic.BaseModel):
    type: typing.Literal["message_delta"] = "message_delta"
    delta: MessageDeltaDelta
    usage: MessageDeltaUsage


class MessageStopEvent(pydantic.BaseModel):
    type: typing.Literal["message_stop"] = "message_stop"


class PingEvent(pydantic.BaseModel):
    type: typing.Literal["ping"] = "ping"


class ErrorEvent(pydantic.BaseModel):
    type: typing.Literal["error"] = "error"
    error: dict[str, typing.Any]


AnthropicStreamEvent = (
    MessageStartEvent
    | ContentBlockStartEvent
    | ContentBlockDeltaEvent
    | ContentBlockStopEvent
    | MessageDeltaEvent
    | MessageStopEvent
    | PingEvent
    | ErrorEvent
)
