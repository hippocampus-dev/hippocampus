import collections.abc
import os
import typing

import httpx
import memory_bank.llm.openai
import memory_bank.exceptions
import memory_bank.telemetry
import openai
import pydantic


class MergedMemory(pydantic.BaseModel):
    merged_content: str
    reasoning: str
    preserved_key_information: collections.abc.Sequence[str]
    removed_redundancies: collections.abc.Sequence[str]


class Merger:
    model: str
    service_tier: typing.Literal["auto", "default", "flex", "priority"] | None

    def __init__(
        self,
        model: str = "gpt-5-mini",
        service_tier: typing.Literal["auto", "default", "flex", "priority"]
        | None = None,
    ):
        self.model = model
        self.service_tier = service_tier

    async def merge_memories(
        self,
        existing_memory: str,
        new_memory: str,
        category: str,
    ) -> str:
        system_prompt = f"""You are a memory consolidation assistant specialized in merging related memories while preserving important information and removing redundancies.

Your task is to merge two memory segments from the '{category}' category into a single, coherent memory that:
1. Preserves all unique and important information from both memories
2. Removes duplicate or redundant information
3. Maintains factual accuracy and context
4. Creates a natural, coherent narrative flow
5. Keeps the most recent or detailed version when information conflicts

Format the merged memory using Markdown structure with 5W1H elements:
- Use # for main topic headers
- Use ## for subtopics
- Use ### for detailed sections
- Use bullet points (-) for lists of related items
- Use **bold** for key terms, names, or important facts (especially Who, What, Where)
- Use > for notable quotes or critical information
- Use numbered lists for sequential steps or chronological events
- Add horizontal rules (---) to separate major sections if needed
- Structure information hierarchically from general to specific

When merging, ensure the 5W1H elements are preserved and consolidated:
- **Who**: Consolidate all people, organizations, roles, or entities from both memories
- **What**: Merge actions, events, objects, or concepts, removing duplicates
- **When**: Preserve all dates, times, durations, keeping the most specific temporal references
- **Where**: Combine all locations, places, or geographical references
- **Why**: Merge reasons, motivations, or causes, keeping the most detailed explanations
- **How**: Consolidate processes, methods, or procedures, maintaining step-by-step clarity

Important guidelines:
- The merged memory should be comprehensive but concise
- Preserve specific details, dates, numbers, and names
- Maintain chronological order when applicable
- Avoid repetition while ensuring no important information is lost
- If the memories contain conflicting information, prioritize the new memory as it's likely more recent"""

        user_prompt = f"""Please merge the following two memory segments:

EXISTING MEMORY:
{existing_memory}

NEW MEMORY:
{new_memory}

Create a single merged memory that combines both effectively."""

        messages = [
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": user_prompt},
        ]

        try:
            response = await memory_bank.llm.openai.AsyncOpenAI(
                http_client=httpx.AsyncClient(
                    timeout=None,
                    mounts={
                        "http://": httpx.AsyncHTTPTransport(
                            proxy=os.getenv("HTTP_PROXY")
                        ),
                        "https://": httpx.AsyncHTTPTransport(
                            proxy=os.getenv("HTTPS_PROXY")
                        ),
                    },
                    verify=os.getenv("SSL_CERT_FILE"),
                ),
            ).beta.chat.completions.parse(
                model=self.model.replace(".", "")
                if os.getenv("OPENAI_API_TYPE") == "azure"
                else self.model,
                messages=messages,
                response_format=MergedMemory,
                service_tier=self.service_tier
                if self.service_tier is not None
                else openai._types.NOT_GIVEN,
            )

            if response.choices[0].message.parsed:
                merged = response.choices[0].message.parsed
                memory_bank.telemetry.logger.debug(
                    f"Memory merged for category '{category}': {merged.reasoning}"
                )
                memory_bank.telemetry.logger.debug(
                    f"Preserved information: {', '.join(merged.preserved_key_information[:3])}"
                )
                return merged.merged_content
            else:
                memory_bank.telemetry.logger.warning(
                    f"Failed to parse merge response for category '{category}', concatenating memories"
                )
                return f"{existing_memory}\n\n{new_memory}"

        except openai.APIConnectionError as e:
            raise memory_bank.exceptions.RetryableError(e) from e
        except openai.APIStatusError as e:
            match e.status_code:
                case 404:
                    if e.message == "Engine not found":
                        raise memory_bank.exceptions.RetryableError(e) from e
                case 409 | 429 | 502 | 503 | 504:
                    raise memory_bank.exceptions.RetryableError(e) from e
            raise e
        except Exception as e:
            memory_bank.telemetry.logger.error(
                f"Unexpected error during memory merge for category '{category}': {e}"
            )
            return f"{existing_memory}\n\n{new_memory}"
