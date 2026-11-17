import asyncio
import collections.abc
import os
import typing

import httpx
import memory_bank.llm.openai
import memory_bank.exceptions
import memory_bank.telemetry
import openai
import pydantic


class DocumentCategorySegment(pydantic.BaseModel):
    document_index: int
    category: str
    content: str
    reasoning: str


class DocumentCategorySegments(pydantic.BaseModel):
    segments: collections.abc.Sequence[DocumentCategorySegment]


class Categorizer:
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

    async def _categorize_document(
        self,
        document_index: int,
        document: str,
        categories: collections.abc.Sequence[str],
    ) -> collections.abc.Mapping[str, str]:
        system_prompt = f"""You are a document content categorization assistant. Your task is to analyze the document and split its content into segments based on the available categories.

Available categories: {", ".join(categories)}

For this document:
1. Identify content that belongs to each category
2. Extract relevant segments for each applicable category
3. The document's content can be split across multiple categories
4. Only include categories that have relevant content
5. Provide reasoning for each categorization

When extracting content for each category, format it using Markdown structure with 5W1H elements:
- Use ## for main topic headers within the category
- Use ### for subtopics
- Use bullet points (-) for lists of related items
- Use **bold** for key terms, entities, or important facts (especially Who, What, Where)
- Use > for notable quotes or critical information
- Use numbered lists for sequential steps or chronological events
- Structure information hierarchically
- Preserve the logical flow and organization of the original content

Ensure the extracted content captures the 5W1H elements when present:
- **Who**: Identify people, organizations, roles, or entities involved
- **What**: Describe actions, events, objects, or concepts
- **When**: Include dates, times, durations, or temporal references
- **Where**: Note locations, places, or geographical references
- **Why**: Capture reasons, motivations, or causes
- **How**: Document processes, methods, or procedures

Important:
- Split the document content logically - don't duplicate content across categories unless it genuinely relates to multiple categories.
- Content that doesn't fit any of the available categories should be discarded and not included in any segment."""

        user_prompt = f"Please analyze and split the following document content into appropriate categories:\n\n{document}"

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
                response_format=DocumentCategorySegments,
                service_tier=self.service_tier
                if self.service_tier is not None
                else openai._types.NOT_GIVEN,
            )

            result = {}
            if response.choices[0].message.parsed:
                for segment in response.choices[0].message.parsed.segments:
                    if segment.category in categories:
                        if segment.category not in result:
                            result[segment.category] = segment.content
                        else:
                            result[segment.category] += " " + segment.content

                        memory_bank.telemetry.logger.debug(
                            f"Document {document_index} segment categorized as '{segment.category}': {segment.reasoning}"
                        )
                    else:
                        memory_bank.telemetry.logger.warning(
                            f"Invalid category '{segment.category}' for document {document_index}, skipping"
                        )

            if not result:
                result = {categories[0]: document}
                memory_bank.telemetry.logger.warning(
                    f"No categories found for document {document_index}, using full content in default category '{categories[0]}'"
                )

            return result

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

    async def categorize_documents(
        self,
        documents: collections.abc.Sequence[str],
        categories: collections.abc.Sequence[str],
    ) -> collections.abc.Mapping[int, collections.abc.Mapping[str, str]]:
        tasks = [
            self._categorize_document(i, doc, categories)
            for i, doc in enumerate(documents)
        ]

        results = await asyncio.gather(*tasks, return_exceptions=True)

        final_result = {}
        for i, result in enumerate(results):
            if isinstance(result, Exception):
                memory_bank.telemetry.logger.error(
                    f"Failed to categorize document {i}: {result}"
                )
                final_result[i] = {categories[0]: documents[i]}
            else:
                final_result[i] = result

        return final_result
