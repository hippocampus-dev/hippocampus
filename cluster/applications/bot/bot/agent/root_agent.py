import asyncio
import collections.abc
import datetime
import json
import mimetypes
import os
import re

import aiohttp
import aiohttp.client_exceptions
import duckduckgo_search
import google.oauth2.credentials
import magic
import openai.types.chat
import playwright.async_api
import pydantic
import tiktoken

import bot.slack
import cortex.exceptions
import cortex.llm.openai.agent
import cortex.llm.openai.agent.audio_agent
import cortex.llm.openai.agent.browser_agent
import cortex.llm.openai.agent.grafana_agent
import cortex.llm.openai.agent.image_agent
import cortex.llm.openai.agent.playwright_agent
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
    google_custom_search_api_key: str | None
    google_custom_search_engine_id: str | None
    image_model: cortex.llm.openai.model.ImageModel
    audio_model: cortex.llm.openai.model.AudioModel

    def __init__(
        self,
        # **kwargs: collections.abc.MutableMapping[str, typing.Any],
        browser: playwright.async_api.Browser,
        embedding_retrieval_url: str,
        grafana_mcp_url: str,
        playwright_mcp_url: str,
        github_token: str,
        slack_token: str,
        google_credentials: google.oauth2.credentials.Credentials,
        bing_subscription_key: str | None,
        google_custom_search_api_key: str | None,
        google_custom_search_engine_id: str | None,
        image_model: cortex.llm.openai.model.ImageModel,
        audio_model: cortex.llm.openai.model.AudioModel,
        model: cortex.llm.openai.model.CompletionModel,
        reasoning_effort: openai.types.chat.ChatCompletionReasoningEffort,
        encoder: tiktoken.Encoding,
    ):
        self.browser = browser
        self.embedding_retrieval_url = embedding_retrieval_url
        self.grafana_mcp_url = grafana_mcp_url
        self.playwright_mcp_url = playwright_mcp_url
        self.github_token = github_token
        self.slack_token = slack_token
        self.google_credentials = google_credentials
        self.bing_subscription_key = bing_subscription_key
        self.google_custom_search_api_key = google_custom_search_api_key
        self.google_custom_search_engine_id = google_custom_search_engine_id

        self.model = model
        self.reasoning_effort = reasoning_effort
        self.encoder = encoder
        self.image_model = image_model
        self.audio_model = audio_model
        self.functions = {
            "retrieval_unknown": cortex.llm.openai.agent.FunctionDefinition(
                func=self.retrieval_unknown,
                cache=cortex.llm.openai.agent.FunctionCache(
                    enabled=True, similarity_threshold=0.9
                ),
                description="Do this if you do not have enough information to answer. Semantic Search for knowledge you don't know.",
                parameters={
                    "type": "object",
                    "properties": {
                        "query": {
                            "type": "string",
                            "description": "Natural language as search query",
                        },
                    },
                    "required": ["query"],
                },
                capability=cortex.llm.openai.agent.Capability.INTERNAL,
            ),
            "web_search": cortex.llm.openai.agent.FunctionDefinition(
                func=self.web_search_google
                if self.google_custom_search_api_key and self.google_custom_search_engine_id
                else self.web_search_bing
                if self.bing_subscription_key
                else self.web_search_ddgs,
                cache=cortex.llm.openai.agent.FunctionCache(
                    enabled=True, similarity_threshold=1.0
                ),
                description="ONLY DO this if you do not have enough information to answer. Send a search query to Search Engine and get back search results that include links to webpages.",
                parameters={
                    "type": "object",
                    "properties": {
                        "query": {
                            "type": "string",
                            "description": "Search query term",
                        },
                        "date_start": {
                            "type": "string",
                            "description": "Start date for date range filter (format: YYYYMMDD).",
                        },
                        "date_end": {
                            "type": "string",
                            "description": "End date for date range filter (format: YYYYMMDD).",
                        },
                        "date_restrict": {
                            "type": "string",
                            "description": "Restrict results to recent content (e.g., 'd30' for last 30 days, 'w1' for last week, 'm1' for last month).",
                        },
                        "page": {
                            "type": "integer",
                            "description": "Page number for pagination (1-based). Used to fetch additional results.",
                            "default": 1,
                        },
                    },
                    "required": ["query", "page"],
                },
            ),
            "open_url": cortex.llm.openai.agent.FunctionDefinition(
                func=self.open_url,
                cache=cortex.llm.openai.agent.FunctionCache(
                    enabled=True, similarity_threshold=1.0
                ),
                description="If a URL is given, perhaps this needs to be done first. Launch an agent to open URL and returning its content.",
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
            "launch_browser_agent": cortex.llm.openai.agent.FunctionDefinition(
                func=self.launch_browser_agent,
                description="Launch an agent to manipulate the browser to retrieve information from a web page. The agent returns the processing results in natural language.",
                parameters={
                    "type": "object",
                    "properties": {
                        "query": {
                            "type": "string",
                            "description": "Natural language instructions with URL on what to do in the browser",
                        },
                    },
                    "required": ["query"],
                },
            ),
            "launch_image_agent": cortex.llm.openai.agent.FunctionDefinition(
                func=self.launch_image_agent,
                description="Launch an agent to generate or manipulate an image. The agent returns the processing results in natural language.",
                parameters={
                    "type": "object",
                    "properties": {
                        "query": {
                            "type": "string",
                            "description": "Natural language instructions on what to do for the image.",
                        },
                    },
                    "required": ["query"],
                },
            ),
            "launch_audio_agent": cortex.llm.openai.agent.FunctionDefinition(
                func=self.launch_audio_agent,
                description="Launch an agent to manipulate an audio.",
                parameters={
                    "type": "object",
                    "properties": {
                        "query": {
                            "type": "string",
                            "description": "Natural language instructions on what to do for the audio.",
                        },
                    },
                    "required": ["query"],
                },
            ),
            "launch_grafana_agent": cortex.llm.openai.agent.FunctionDefinition(
                func=self.launch_grafana_agent,
                description="Launch an agent to query metrics, logs, and alerts from Grafana. The agent returns the processing results in natural language.",
                parameters={
                    "type": "object",
                    "properties": {
                        "query": {
                            "type": "string",
                            "description": "Natural language instructions on what to do for metrics, logs, and alerts from Grafana.",
                        },
                    },
                },
            ),
            "file_upload": cortex.llm.openai.agent.FunctionDefinition(
                func=self.file_upload,
                description="Upload file to slack.",
                parameters={
                    "type": "object",
                    "properties": {
                        "content": {
                            "type": "string",
                            "description": "What content you need to upload. e.g. 'screenshotted image', 'generated audio file'",
                        },
                    },
                    "required": ["content"],
                },
            ),
        }
        self.functions["file_upload"].dependencies = {
            self.functions["open_url"],
            self.functions["launch_browser_agent"],
            self.functions["launch_image_agent"],
            self.functions["launch_audio_agent"],
        }

    async def retrieval_unknown(
        self,
        query: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        try:
            async with aiohttp.ClientSession(trust_env=True) as session:
                async with session.post(
                    f"{self.embedding_retrieval_url}/query",
                    json=embedding_retrieval.model.QueryRequest(
                        queries=[
                            embedding_retrieval.model.Query(query=query, top_k=10)
                        ],
                    ).model_dump(),
                ) as response:
                    if response.status != 200:
                        e = ValueError(f"invalid response status: {response.status}")
                        match response.status:
                            case 409 | 429 | 502 | 503 | 504:
                                raise cortex.exceptions.RetryableError(e) from e
                        raise e
                    query_response = (
                        embedding_retrieval.model.QueryResponse.model_validate(
                            await response.json()
                        )
                    )
                    results = query_response.results[0].results

                    boosted_results = sorted(
                        [boost(result) for result in results],
                        key=lambda x: x.score,
                        reverse=True,
                    )
                    filtered_results = [
                        result for result in boosted_results if filter(result, context)
                    ]

                    return cortex.llm.openai.agent.FunctionResult(
                        instruction=(
                            "In some cases, you should try to call other function, if the results are unsatisfactory, and you believe that you can refine the query to get better results."
                            "SHOULD include `url` in the answer."
                        ),
                        response=json.dumps(
                            [
                                {
                                    "source": result.metadata.source,
                                    "url": result.metadata.source_id,
                                    "text": result.text,
                                }
                                for i, result in enumerate(filtered_results)
                                if i < 3
                            ],
                            ensure_ascii=False,
                        ),
                    )
        except (
                aiohttp.ClientConnectionError,  # ECONNREFUSED, EPIPE, ECONNRESET
                aiohttp.client_exceptions.ServerDisconnectedError,
                asyncio.TimeoutError,
        ) as e:
            raise cortex.exceptions.RetryableError(e) from e

    async def web_search_bing(
        self,
        query: str,
        context: cortex.llm.openai.agent.Context,
        date_start: str | None = None,
        date_end: str | None = None,
        date_restrict: str | None = None,
        page: int = 1,
    ) -> cortex.llm.openai.agent.FunctionResult:
        try:
            async with aiohttp.ClientSession(trust_env=True) as session:
                params = {"q": query, "mkt": "ja-JP", "count": 3}

                if page > 1:
                    params["offset"] = (page - 1) * 3

                if date_start and date_end:
                    freshness = f"{date_start[:4]}-{date_start[4:6]}-{date_start[6:]}..{date_end[:4]}-{date_end[4:6]}-{date_end[6:]}"
                    params["freshness"] = freshness
                elif date_restrict:
                    if date_restrict.startswith("d"):
                        params["freshness"] = "Day"
                    elif date_restrict.startswith("w"):
                        params["freshness"] = "Week"
                    elif date_restrict.startswith("m"):
                        params["freshness"] = "Month"

                async with session.get(
                    "https://api.bing.microsoft.com/v7.0/search",
                    params=params,
                    headers={"Ocp-Apim-Subscription-Key": self.bing_subscription_key},
                ) as response:
                    if response.status != 200:
                        e = ValueError(f"invalid response status: {response.status}")
                        match response.status:
                            case 409 | 429 | 502 | 503 | 504:
                                raise cortex.exceptions.RetryableError(e) from e
                        raise e

                    d = await response.json()
                    web_pages: collections.abc.Sequence[ValueItem] = []
                    if "webPages" in d:
                        search_response = SearchResponse.model_validate(d)
                        web_pages = search_response.webPages.value

                    return cortex.llm.openai.agent.FunctionResult(
                        instruction=(
                            "In some cases, you should repeat to search with other query, if the results are unsatisfactory, and you believe that you can refine the query to get better results."
                        ),
                        response=json.dumps(
                            [
                                {"url": web_page.url, "name": web_page.name}
                                for web_page in web_pages
                            ],
                            ensure_ascii=False,
                        ),
                    )
        except (
                aiohttp.ClientConnectionError,  # ECONNREFUSED, EPIPE, ECONNRESET
                aiohttp.client_exceptions.ServerDisconnectedError,
                asyncio.TimeoutError,
        ) as e:
            raise cortex.exceptions.RetryableError(e) from e

    async def web_search_google(
        self,
        query: str,
        context: cortex.llm.openai.agent.Context,
        date_start: str | None = None,
        date_end: str | None = None,
        date_restrict: str | None = None,
        page: int = 1,
    ) -> cortex.llm.openai.agent.FunctionResult:
        try:
            async with aiohttp.ClientSession(trust_env=True) as session:
                params = {
                    "key": self.google_custom_search_api_key,
                    "cx": self.google_custom_search_engine_id,
                    "q": query,
                    "num": 3,
                }

                if page > 1:
                    params["start"] = (page - 1) * 3 + 1

                if date_start and date_end:
                    params["sort"] = f"date:r:{date_start}:{date_end}"
                elif date_restrict:
                    params["dateRestrict"] = date_restrict

                async with session.get(
                    "https://www.googleapis.com/customsearch/v1",
                    params=params,
                ) as response:
                    if response.status != 200:
                        e = ValueError(f"invalid response status: {response.status}")
                        match response.status:
                            case 409 | 429 | 502 | 503 | 504:
                                raise cortex.exceptions.RetryableError(e) from e
                        raise e

                    data = await response.json()
                    items = data.get("items", [])

                    return cortex.llm.openai.agent.FunctionResult(
                        instruction=(
                            "In some cases, you should repeat to search with other query, if the results are unsatisfactory, and you believe that you can refine the query to get better results."
                        ),
                        response=json.dumps(
                            [
                                {"url": item["link"], "name": item["title"]}
                                for item in items
                            ],
                            ensure_ascii=False,
                        ),
                    )
        except (
                aiohttp.ClientConnectionError,  # ECONNREFUSED, EPIPE, ECONNRESET
                aiohttp.client_exceptions.ServerDisconnectedError,
                asyncio.TimeoutError,
        ) as e:
            raise cortex.exceptions.RetryableError(e) from e

    async def web_search_ddgs(
        self,
        query: str,
        context: cortex.llm.openai.agent.Context,
        date_start: str | None = None,
        date_end: str | None = None,
        date_restrict: str | None = None,
        page: int = 1,
    ) -> cortex.llm.openai.agent.FunctionResult:
        timelimit = None
        if date_restrict:
            if date_restrict.startswith("d"):
                timelimit = "d"
            elif date_restrict.startswith("w"):
                timelimit = "w"
            elif date_restrict.startswith("m"):
                timelimit = "m"
            elif date_restrict.startswith("y"):
                timelimit = "y"

        async with duckduckgo_search.AsyncDDGS(
            proxies={
                "http://": os.getenv("HTTP_PROXY"),
                "https://": os.getenv("HTTPS_PROXY"),
            },
        ) as session:
            skip_results = (page - 1) * 3 if page > 1 else 0
            max_results = 3 + skip_results
            all_results = [r async for r in session.text(query, max_results=max_results, timelimit=timelimit)]
            results = all_results[skip_results:]

            return cortex.llm.openai.agent.FunctionResult(
                instruction=(
                    "In some cases, you should repeat to search with other query, if the results are unsatisfactory, and you believe that you can refine the query to get better results."
                ),
                response=json.dumps(
                    [
                        {"url": result["href"], "name": result["title"]}
                        for result in results
                    ],
                    ensure_ascii=False,
                ),
            )

    async def open_url(
        self,
        url: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        agent = cortex.llm.openai.agent.url_open_agent.URLOpenAgent(
            browser=self.browser,
            github_token=self.github_token,
            slack_token=self.slack_token,
            google_credentials=self.google_credentials,
            model=self.model,
            encoder=self.encoder,
        )
        response = await agent.chat_completion_loop(
            [
                {
                    "role": "user",
                    "content": url,
                }
            ],
            context,
        )

        context.increment_completion_tokens(
            self.model,
            response.usage.completion_tokens,
        )
        return cortex.llm.openai.agent.FunctionResult(
            instruction="If the result are irrelevant information, you should try to call other function.",
            response=response.choices[0].message.content,
        )

    async def launch_browser_agent(
        self,
        query: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        # agent = cortex.llm.openai.agent.browser_agent.BrowserAgent(
        #     browser=self.browser,
        #     model=self.model,
        #     encoder=self.encoder,
        # )

        async with cortex.llm.openai.agent.playwright_agent.PlaywrightAgent(
            url=self.playwright_mcp_url,
            model=self.model,
            encoder=self.encoder,
        ) as agent:
            response = await agent.chat_completion_loop(
                [
                    *context.current_messages,
                    {
                        "role": "user",
                        "content": query,
                    },
                ],
                context,
            )

            context.increment_completion_tokens(
                self.model,
                response.usage.completion_tokens,
            )
            return cortex.llm.openai.agent.FunctionResult(
                response=response.choices[0].message.content
            )

    async def launch_image_agent(
        self,
        query: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        agent = cortex.llm.openai.agent.image_agent.ImageAgent(
            model=self.model,
            encoder=self.encoder,
            image_model=self.image_model,
        )
        response = await agent.chat_completion_loop(
            [
                {
                    "role": "user",
                    "content": query,
                }
            ],
            context,
        )

        context.increment_completion_tokens(
            self.model,
            response.usage.completion_tokens,
        )
        return cortex.llm.openai.agent.FunctionResult(
            response=response.choices[0].message.content
        )

    async def launch_audio_agent(
        self,
        query: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        agent = cortex.llm.openai.agent.audio_agent.AudioAgent(
            model=self.model,
            encoder=self.encoder,
            audio_model=self.audio_model,
        )
        response = await agent.chat_completion_loop(
            [
                {
                    "role": "user",
                    "content": query,
                }
            ],
            context,
        )

        context.increment_completion_tokens(
            self.model,
            response.usage.completion_tokens,
        )
        return cortex.llm.openai.agent.FunctionResult(
            response=response.choices[0].message.content
        )

    async def launch_grafana_agent(
        self,
        query: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        async with cortex.llm.openai.agent.grafana_agent.GrafanaAgent(
            url=self.grafana_mcp_url,
            model=self.model,
            encoder=self.encoder,
        ) as agent:
            response = await agent.chat_completion_loop(
                [
                    *context.current_messages,
                    {
                        "role": "user",
                        "content": query,
                    },
                ],
                context,
            )

            context.increment_completion_tokens(
                self.model,
                response.usage.completion_tokens,
            )
            return cortex.llm.openai.agent.FunctionResult(
                response=response.choices[0].message.content
            )

    async def file_upload(
        self,
        content: str,
        context: bot.slack.SlackContext,
    ) -> cortex.llm.openai.agent.FunctionResult:
        formatted_now = str(datetime.datetime.now().timestamp())

        histories = await context.restore_histories(
            query=content, similarity_threshold=0.4, namespace="content"
        )
        if len(histories) == 0:
            return cortex.llm.openai.agent.FunctionResult(response="Content not found")

        latest_history = sorted(histories, key=lambda x: x.created_at)[-1]

        mimetype = magic.from_buffer(latest_history.body, mime=True)
        extension = mimetypes.guess_extension(mimetype)

        await context.client.files_upload_v2(
            channel=context.progress["channel"],
            thread_ts=context.progress.get("thread_ts") or context.progress["ts"],
            file=latest_history.body,
            filename=formatted_now + extension if extension else formatted_now,
            title=formatted_now,
        )

        return cortex.llm.openai.agent.FunctionResult(
            instruction="MUST not include the uploaded file in the answer.",
            response="Uploaded",
        )
