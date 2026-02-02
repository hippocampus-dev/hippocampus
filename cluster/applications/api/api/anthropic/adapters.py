import collections.abc
import json
import typing
import uuid

import openai._types
import openai.types.chat

import api.anthropic.model


def generate_message_id() -> str:
    return f"msg_{uuid.uuid4().hex[:24]}"


def convert_anthropic_image_to_openai(
    image: api.anthropic.model.AnthropicImageContent,
) -> dict[str, typing.Any]:
    if image.source.type == "base64":
        media_type = image.source.media_type or "image/png"
        data_uri = f"data:{media_type};base64,{image.source.data}"
        return {"type": "image_url", "image_url": {"url": data_uri}}
    elif image.source.type == "url":
        return {"type": "image_url", "image_url": {"url": image.source.url}}
    else:
        raise ValueError(f"Unsupported image source type: {image.source.type}")


def convert_anthropic_content_to_openai(
    content: str | collections.abc.Sequence[api.anthropic.model.AnthropicContentBlock],
) -> str | list[dict[str, typing.Any]]:
    if isinstance(content, str):
        return content

    result: list[dict[str, typing.Any]] = []
    for block in content:
        if isinstance(block, api.anthropic.model.AnthropicTextContent):
            result.append({"type": "text", "text": block.text})
        elif isinstance(block, api.anthropic.model.AnthropicImageContent):
            result.append(convert_anthropic_image_to_openai(block))
        elif isinstance(block, api.anthropic.model.AnthropicToolUseContent):
            pass
        elif isinstance(block, api.anthropic.model.AnthropicToolResultContent):
            pass

    return (
        result
        if len(result) > 1
        else (
            result[0]["text"] if result and result[0].get("type") == "text" else result
        )
    )


def convert_anthropic_messages_to_openai(
    body: api.anthropic.model.V1Messages,
) -> list[dict[str, typing.Any]]:
    messages: list[dict[str, typing.Any]] = []

    if body.system is not None:
        if isinstance(body.system, str):
            messages.append({"role": "system", "content": body.system})
        else:
            system_text = " ".join(block.text for block in body.system)
            messages.append({"role": "system", "content": system_text})

    previous_role: str | None = None
    for message in body.messages:
        if isinstance(message.content, str):
            if previous_role == message.role:
                messages[-1]["content"] += "\n\n" + message.content
            else:
                messages.append({"role": message.role, "content": message.content})
                previous_role = message.role
        else:
            tool_results: list[dict[str, typing.Any]] = []
            other_content: list[dict[str, typing.Any]] = []
            tool_calls: list[dict[str, typing.Any]] = []

            for block in message.content:
                if isinstance(block, api.anthropic.model.AnthropicToolResultContent):
                    content_str = (
                        block.content if isinstance(block.content, str) else ""
                    )
                    if isinstance(block.content, list):
                        texts = [
                            b.text
                            for b in block.content
                            if isinstance(b, api.anthropic.model.AnthropicTextContent)
                        ]
                        content_str = "\n".join(texts)
                    tool_results.append(
                        {
                            "role": "tool",
                            "tool_call_id": block.tool_use_id,
                            "content": content_str,
                        }
                    )
                elif isinstance(block, api.anthropic.model.AnthropicToolUseContent):
                    tool_calls.append(
                        {
                            "id": block.id,
                            "type": "function",
                            "function": {
                                "name": block.name,
                                "arguments": json.dumps(block.input),
                            },
                        }
                    )
                elif isinstance(block, api.anthropic.model.AnthropicTextContent):
                    other_content.append({"type": "text", "text": block.text})
                elif isinstance(block, api.anthropic.model.AnthropicImageContent):
                    other_content.append(convert_anthropic_image_to_openai(block))

            if tool_calls:
                assistant_msg: dict[str, typing.Any] = {
                    "role": "assistant",
                    "content": None,
                    "tool_calls": tool_calls,
                }
                if other_content:
                    if (
                        len(other_content) == 1
                        and other_content[0].get("type") == "text"
                    ):
                        assistant_msg["content"] = other_content[0]["text"]
                    else:
                        assistant_msg["content"] = other_content
                messages.append(assistant_msg)
                previous_role = "assistant"

            for tool_result in tool_results:
                messages.append(tool_result)
                previous_role = "tool"

            if other_content and not tool_calls:
                content: str | list[dict[str, typing.Any]]
                if len(other_content) == 1 and other_content[0].get("type") == "text":
                    content = other_content[0]["text"]
                else:
                    content = other_content

                if previous_role == message.role:
                    if isinstance(messages[-1]["content"], str) and isinstance(
                        content, str
                    ):
                        messages[-1]["content"] += "\n\n" + content
                    elif isinstance(messages[-1]["content"], list) and isinstance(
                        content, list
                    ):
                        messages[-1]["content"].extend(content)
                    else:
                        messages.append({"role": message.role, "content": content})
                        previous_role = message.role
                else:
                    messages.append({"role": message.role, "content": content})
                    previous_role = message.role

    return messages


def convert_anthropic_tools_to_openai(
    tools: collections.abc.Sequence[api.anthropic.model.AnthropicTool] | None,
) -> list[openai.types.chat.ChatCompletionToolParam] | openai._types.NotGiven:
    if tools is None:
        return openai._types.NOT_GIVEN

    result: list[openai.types.chat.ChatCompletionToolParam] = []
    for tool in tools:
        parameters: dict[str, typing.Any] = {"type": "object"}
        if tool.input_schema.properties is not None:
            parameters["properties"] = tool.input_schema.properties
        if tool.input_schema.required is not None:
            parameters["required"] = list(tool.input_schema.required)

        result.append(
            {
                "type": "function",
                "function": {
                    "name": tool.name,
                    "description": tool.description or "",
                    "parameters": parameters,
                },
            }
        )
    return result


def convert_anthropic_tool_choice_to_openai(
    tool_choice: typing.Literal["auto", "any", "none"] | dict[str, typing.Any] | None,
) -> (
    typing.Literal["none", "auto", "required"]
    | openai.types.chat.ChatCompletionNamedToolChoiceParam
    | openai._types.NotGiven
):
    if tool_choice is None:
        return openai._types.NOT_GIVEN
    if tool_choice == "auto":
        return "auto"
    if tool_choice == "none":
        return "none"
    if tool_choice == "any":
        return "required"
    if isinstance(tool_choice, dict):
        if tool_choice.get("type") == "tool" and "name" in tool_choice:
            return {"type": "function", "function": {"name": tool_choice["name"]}}
    return openai._types.NOT_GIVEN


def convert_openai_finish_reason_to_anthropic(
    finish_reason: str | None,
) -> typing.Literal["end_turn", "max_tokens", "stop_sequence", "tool_use"] | None:
    if finish_reason is None:
        return None
    if finish_reason == "stop":
        return "end_turn"
    if finish_reason == "length":
        return "max_tokens"
    if finish_reason == "tool_calls":
        return "tool_use"
    if finish_reason == "content_filter":
        return "end_turn"
    return "end_turn"


def convert_openai_response_to_anthropic(
    response: openai.types.chat.ChatCompletion,
    model: str,
) -> api.anthropic.model.AnthropicResponse:
    choice = response.choices[0]
    content: list[api.anthropic.model.AnthropicResponseContentBlock] = []

    if choice.message.content:
        content.append(
            api.anthropic.model.AnthropicResponseTextBlock(text=choice.message.content)
        )

    if choice.message.tool_calls:
        for tool_call in choice.message.tool_calls:
            try:
                input_data = json.loads(tool_call.function.arguments)
            except json.JSONDecodeError:
                input_data = {}
            content.append(
                api.anthropic.model.AnthropicResponseToolUseBlock(
                    id=tool_call.id,
                    name=tool_call.function.name,
                    input=input_data,
                )
            )

    stop_reason = convert_openai_finish_reason_to_anthropic(choice.finish_reason)

    return api.anthropic.model.AnthropicResponse(
        id=generate_message_id(),
        content=content,
        model=model,
        stop_reason=stop_reason,
        usage=api.anthropic.model.AnthropicUsage(
            input_tokens=response.usage.prompt_tokens if response.usage else 0,
            output_tokens=response.usage.completion_tokens if response.usage else 0,
        ),
    )


class StreamingAdapter:
    def __init__(self, model: str, message_id: str | None = None):
        self.model = model
        self.message_id = message_id or generate_message_id()
        self.content_block_index = 0
        self.current_block_started = False
        self.has_sent_message_start = False
        self.tool_calls_in_progress: dict[int, dict[str, typing.Any]] = {}
        self.total_output_tokens = 0

    def create_message_start_event(
        self,
        input_tokens: int = 0,
    ) -> api.anthropic.model.MessageStartEvent:
        self.has_sent_message_start = True
        return api.anthropic.model.MessageStartEvent(
            message=api.anthropic.model.MessageStartEventMessage(
                id=self.message_id,
                model=self.model,
                usage=api.anthropic.model.AnthropicUsage(
                    input_tokens=input_tokens,
                    output_tokens=0,
                ),
            ),
        )

    def process_openai_chunk(
        self,
        chunk: openai.types.chat.ChatCompletionChunk,
    ) -> collections.abc.Sequence[api.anthropic.model.AnthropicStreamEvent]:
        events: list[api.anthropic.model.AnthropicStreamEvent] = []

        if not chunk.choices:
            return events

        choice = chunk.choices[0]
        delta = choice.delta

        if delta.content is not None and delta.content != "":
            if not self.current_block_started:
                events.append(
                    api.anthropic.model.ContentBlockStartEvent(
                        index=self.content_block_index,
                        content_block=api.anthropic.model.AnthropicResponseTextBlock(
                            text=""
                        ),
                    )
                )
                self.current_block_started = True

            events.append(
                api.anthropic.model.ContentBlockDeltaEvent(
                    index=self.content_block_index,
                    delta=api.anthropic.model.TextDelta(text=delta.content),
                )
            )

        if delta.tool_calls:
            for tool_call in delta.tool_calls:
                tool_index = tool_call.index

                if tool_index not in self.tool_calls_in_progress:
                    if self.current_block_started:
                        events.append(
                            api.anthropic.model.ContentBlockStopEvent(
                                index=self.content_block_index,
                            )
                        )
                        self.content_block_index += 1
                        self.current_block_started = False

                    self.tool_calls_in_progress[tool_index] = {
                        "id": tool_call.id or f"call_{uuid.uuid4().hex[:24]}",
                        "name": tool_call.function.name if tool_call.function else "",
                        "arguments": "",
                    }

                    events.append(
                        api.anthropic.model.ContentBlockStartEvent(
                            index=self.content_block_index,
                            content_block=api.anthropic.model.AnthropicResponseToolUseBlock(
                                id=self.tool_calls_in_progress[tool_index]["id"],
                                name=self.tool_calls_in_progress[tool_index]["name"],
                                input={},
                            ),
                        )
                    )
                    self.current_block_started = True

                if tool_call.function and tool_call.function.arguments:
                    self.tool_calls_in_progress[tool_index]["arguments"] += (
                        tool_call.function.arguments
                    )
                    events.append(
                        api.anthropic.model.ContentBlockDeltaEvent(
                            index=self.content_block_index,
                            delta=api.anthropic.model.InputJsonDelta(
                                partial_json=tool_call.function.arguments,
                            ),
                        )
                    )

        if choice.finish_reason is not None:
            if self.current_block_started:
                events.append(
                    api.anthropic.model.ContentBlockStopEvent(
                        index=self.content_block_index,
                    )
                )

            stop_reason = convert_openai_finish_reason_to_anthropic(
                choice.finish_reason
            )

            if hasattr(chunk, "usage") and chunk.usage:
                self.total_output_tokens = chunk.usage.completion_tokens

            events.append(
                api.anthropic.model.MessageDeltaEvent(
                    delta=api.anthropic.model.MessageDeltaDelta(
                        stop_reason=stop_reason
                    ),
                    usage=api.anthropic.model.MessageDeltaUsage(
                        output_tokens=self.total_output_tokens
                    ),
                )
            )
            events.append(api.anthropic.model.MessageStopEvent())

        return events
