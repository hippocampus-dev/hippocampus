import collections.abc
import os
import re

import google.oauth2.credentials
import openai.types.chat
import playwright.async_api
import pydantic
import tiktoken

import cortex.exceptions
import cortex.llm.openai.agent
import cortex.llm.openai.agent.browser_agent
import cortex.llm.openai.agent.url_open_agent
import cortex.llm.openai.model
import embedding_retrieval.model


class InnerDeepLink(pydantic.BaseModel):
    name: str
    url: str


class DeepLink(pydantic.BaseModel):
    name: str
    url: str
    snippet: str | None = None
    deepLinks: collections.abc.Sequence[InnerDeepLink] | None = None


class License(pydantic.BaseModel):
    name: str
    url: str


class ContractualRule(pydantic.BaseModel):
    _type: str
    targetPropertyName: str
    targetPropertyIndex: int
    mustBeCloseToContent: bool
    license: License
    licenseNotice: str


class ValueItem(pydantic.BaseModel):
    id: str
    name: str
    url: str
    isFamilyFriendly: bool
    displayUrl: str
    snippet: str
    deepLinks: collections.abc.Sequence[DeepLink] | None = None
    dateLastCrawled: str
    language: str
    isNavigational: bool
    contractualRules: collections.abc.Sequence[ContractualRule] | None = None
    thumbnailUrl: str | None = None


class WebPages(pydantic.BaseModel):
    webSearchUrl: str | None = None
    totalEstimatedMatches: int
    value: collections.abc.Sequence[ValueItem]


class SearchResponse(pydantic.BaseModel):
    _type: str
    webPages: WebPages


BOOSTS = {
    "source": {},
    "source_id": {},
}


def boost(
    result: embedding_retrieval.model.DocumentChunkWithScore,
) -> embedding_retrieval.model.DocumentChunkWithScore:
    for pattern, source_boost in BOOSTS["source"].items():
        if isinstance(pattern, str) and pattern == result.metadata.source:
            result.score *= source_boost

        if isinstance(pattern, re.Pattern) and pattern.match(result.metadata.source):
            result.score *= source_boost

    for pattern, source_id_boost in BOOSTS["source_id"].items():
        if isinstance(pattern, str) and pattern == result.metadata.source_id:
            result.score *= source_id_boost

        if isinstance(pattern, re.Pattern) and pattern.match(result.metadata.source_id):
            result.score *= source_id_boost

    return result


REQUIRED_CAPABILITIES = {
    "source": {},
    "source_id": {},
}


def filter(
    result: embedding_retrieval.model.DocumentChunkWithScore,
    context: cortex.llm.openai.agent.Context,
) -> bool:
    if result.metadata.source is not None:
        for pattern, capability in REQUIRED_CAPABILITIES["source"].items():
            if isinstance(pattern, str) and pattern == result.metadata.source:
                return capability & context.capability > 0

            if isinstance(pattern, re.Pattern) and pattern.match(
                result.metadata.source
            ):
                return capability & context.capability > 0

    if result.metadata.source_id is not None:
        for pattern, capability in REQUIRED_CAPABILITIES["source_id"].items():
            if isinstance(pattern, str) and pattern == result.metadata.source_id:
                return capability & context.capability > 0

            if isinstance(pattern, re.Pattern) and pattern.match(
                result.metadata.source_id
            ):
                return capability & context.capability > 0

    return True


class RootAgent(cortex.llm.openai.agent.Agent):
    browser: playwright.async_api.Browser
    embedding_retrieval_url: str
    github_token: str
    slack_token: str
    google_credentials: google.oauth2.credentials.Credentials
    bing_subscription_key: str | None

    def __init__(
        self,
        # **kwargs: collections.abc.MutableMapping[str, typing.Any],
        browser: playwright.async_api.Browser,
        github_token: str,
        slack_token: str,
        google_credentials: google.oauth2.credentials.Credentials,
        model: cortex.llm.openai.model.CompletionModel,
        reasoning_effort: openai.types.chat.ChatCompletionReasoningEffort,
        encoder: tiktoken.Encoding,
    ):
        self.browser = browser
        self.github_token = github_token
        self.slack_token = slack_token
        self.google_credentials = google_credentials

        self.model = model
        self.reasoning_effort = reasoning_effort
        self.encoder = encoder
        self.functions = {
            "open_url": cortex.llm.openai.agent.FunctionDefinition(
                func=self.open_url,
                cache=cortex.llm.openai.agent.FunctionCache(
                    enabled=True, similarity_threshold=1.0
                ),
                description="Launch an agent to open URL and returning its content.",
                parameters={
                    "type": "object",
                    "properties": {
                        "url": {
                            "type": "string",
                            "description": "Target URL",
                        },
                    },
                    "required": ["url"],
                },
            ),
        }

    async def open_url(
        self,
        url: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        response = await cortex.llm.openai.agent.url_open_agent.URLOpenAgent(
            browser=self.browser,
            github_token=self.github_token,
            slack_token=self.slack_token,
            google_credentials=self.google_credentials,
            model=self.model.replace(".", "")
            if os.getenv("OPENAI_API_TYPE") == "azure"
            else self.model,
            encoder=self.encoder,
        ).chat_completion_loop(
            [
                {
                    "role": "user",
                    "content": url,
                }
            ],
            context,
        )

        context.increment_completion_tokens(
            self.model, response.usage.completion_tokens
        )
        return cortex.llm.openai.agent.FunctionResult(
            instruction="If the result are irrelevant information, you should try to call other function.",
            response=response.choices[0].message.content,
        )
