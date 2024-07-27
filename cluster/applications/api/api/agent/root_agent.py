import asyncio
import collections.abc
import json
import os
import re

import aiohttp
import duckduckgo_search
import google.oauth2.credentials
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


def boost(result: embedding_retrieval.model.DocumentChunkWithScore) -> embedding_retrieval.model.DocumentChunkWithScore:
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


def filter(result: embedding_retrieval.model.DocumentChunkWithScore, context: cortex.llm.openai.agent.Context) -> bool:
    if result.metadata.source is not None:
        for pattern, capability in REQUIRED_CAPABILITIES["source"].items():
            if isinstance(pattern, str) and pattern == result.metadata.source:
                return capability & context.capability > 0

            if isinstance(pattern, re.Pattern) and pattern.match(result.metadata.source):
                return capability & context.capability > 0

    if result.metadata.source_id is not None:
        for pattern, capability in REQUIRED_CAPABILITIES["source_id"].items():
            if isinstance(pattern, str) and pattern == result.metadata.source_id:
                return capability & context.capability > 0

            if isinstance(pattern, re.Pattern) and pattern.match(result.metadata.source_id):
                return capability & context.capability > 0

    return True


class RootAgent(cortex.llm.openai.agent.Agent):
    embedding_retrieval_endpoint: str
    github_token: str
    slack_token: str
    google_credentials: google.oauth2.credentials.Credentials
    bing_subscription_key: str | None

    def __init__(
        self,
        embedding_retrieval_endpoint: str,
        github_token: str,
        slack_token: str,
        google_credentials: google.oauth2.credentials.Credentials,
        bing_subscription_key: str | None,
        model: cortex.llm.openai.model.CompletionModel,
        encoder: tiktoken.Encoding,
    ):
        self.embedding_retrieval_endpoint = embedding_retrieval_endpoint
        self.github_token = github_token
        self.slack_token = slack_token
        self.google_credentials = google_credentials
        self.bing_subscription_key = bing_subscription_key

        self.model = model
        self.encoder = encoder
        self.functions = {
            "retrieval_unknown": cortex.llm.openai.agent.FunctionDefinition(
                func=self.retrieval_unknown,
                cache=cortex.llm.openai.agent.FunctionCache(enabled=True, similarity_threshold=0.9),
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
                func=self.web_search_bing if self.bing_subscription_key else self.web_search_ddgs,
                cache=cortex.llm.openai.agent.FunctionCache(enabled=True, similarity_threshold=1.0),
                description="ONLY DO this if you do not have enough information to answer. Send a search query to Bing and get back search results that include links to webpages.",
                parameters={
                    "type": "object",
                    "properties": {
                        "query": {
                            "type": "string",
                            "description": "Search query term",
                        },
                    },
                    "required": ["query"],
                },
            ),
            "open_url": cortex.llm.openai.agent.FunctionDefinition(
                func=self.open_url,
                cache=cortex.llm.openai.agent.FunctionCache(enabled=True, similarity_threshold=1.0),
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
            "launch_browser_agent": cortex.llm.openai.agent.FunctionDefinition(
                func=self.launch_browser_agent,
                description="Launch an agent to manipulate the browser to retrieve information from a web page. The agent returns the processing results in natural language.",
                parameters={
                    "type": "object",
                    "properties": {
                        "query": {
                            "type": "string",
                            "description": "Natural language instructions on what to do in the browser",
                        },
                    },
                    "required": ["query"],
                },
            ),
        }

    async def retrieval_unknown(
        self,
        query: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        try:
            async with aiohttp.ClientSession(trust_env=True) as session:
                async with session.post(
                    f"{self.embedding_retrieval_endpoint}/query",
                    json=embedding_retrieval.model.QueryRequest(
                        queries=[embedding_retrieval.model.Query(query=query, top_k=10)],
                    ).dict(),
                ) as response:
                    if response.status != 200:
                        e = ValueError(f"invalid response status: {response.status}")
                        match response.status:
                            case (409 | 429 | 502 | 503 | 504):
                                raise cortex.exceptions.RetryableError(e) from e
                        raise e
                    query_response = embedding_retrieval.model.QueryResponse.parse_obj(await response.json())
                    results = query_response.results[0].results

                    boosted_results = sorted([boost(result) for result in results], key=lambda x: x.score, reverse=True)
                    filtered_results = [result for result in boosted_results if filter(result, context)]

                    return cortex.llm.openai.agent.FunctionResult(
                        instruction=(
                            "You MUST cite the most relevant results that answer the query. Do not mention any irrelevant results."
                            "You MUST ADHERE to the following instructions for citing search results: to cite a search result, you MUST provide the URL of the result."
                            "In some cases, you should try to call other function, if the results are unsatisfactory, and you believe that you can refine the query to get better results."
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
    ) -> cortex.llm.openai.agent.FunctionResult:
        try:
            async with aiohttp.ClientSession(trust_env=True) as session:
                async with session.get(
                    "https://api.bing.microsoft.com/v7.0/search",
                    params={"q": query, "mkt": "ja-JP", "count": 3},
                    headers={"Ocp-Apim-Subscription-Key": self.bing_subscription_key},
                ) as response:
                    if response.status != 200:
                        e = ValueError(f"invalid response status: {response.status}")
                        match response.status:
                            case (409 | 429 | 502 | 503 | 504):
                                raise cortex.exceptions.RetryableError(e) from e
                        raise e
                    search_response = SearchResponse.parse_obj(await response.json())
                    web_pages = search_response.webPages.value

                    return cortex.llm.openai.agent.FunctionResult(
                        instruction=(
                            "You MUST cite the most relevant results that answer the query. Do not mention any irrelevant results."
                            "You MUST ADHERE to the following instructions for citing search results: to cite a search result, you MUST provide the URL of the result."
                            "In some cases, you should try to call other function, if the results are unsatisfactory, and you believe that you can refine the query to get better results."
                        ),
                        response=json.dumps(
                            [{"url": web_page.url, "name": web_page.name} for web_page in web_pages],
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
    ) -> cortex.llm.openai.agent.FunctionResult:
        async with duckduckgo_search.AsyncDDGS(
            proxies={
                "http://": os.getenv("HTTP_PROXY"),
                "https://": os.getenv("HTTPS_PROXY"),
            },
        ) as session:
            results = [r async for r in session.text(query, max_results=3)]

            return cortex.llm.openai.agent.FunctionResult(
                instruction=(
                    "You MUST cite the most relevant results that answer the query. Do not mention any irrelevant results."
                    "You MUST ADHERE to the following instructions for citing search results: to cite a search result, you MUST provide the URL of the result."
                    "In some cases, you should try to call other function, if the results are unsatisfactory, and you believe that you can refine the query to get better results."
                ),
                response=json.dumps(
                    [{"url": result["href"], "name": result["title"]} for result in results],
                    ensure_ascii=False,
                ),
            )

    async def open_url(
        self,
        url: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        response = await cortex.llm.openai.agent.url_open_agent.URLOpenAgent(
            github_token=self.github_token,
            slack_token=self.slack_token,
            google_credentials=self.google_credentials,
            model=self.model.replace(".", "") if os.getenv("OPENAI_API_TYPE") == "azure" else self.model,
            encoder=self.encoder,
        ).chat_completion_loop(
            [{
                "role": "user",
                "content": url,
            }],
            context,
            streaming=False,
        )

        context.increment_completion_tokens(self.model, response.usage.completion_tokens)
        return cortex.llm.openai.agent.FunctionResult(
            instruction=f"SHOULD include {url} in the answer. If the search result are irrelevant information, you should try to call other function.",
            response=response.choices[0].message.content,
        )

    async def launch_browser_agent(
        self,
        query: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        async with playwright.async_api.async_playwright() as pw:
            browser = await pw.chromium.launch(
                proxy={"server": os.getenv("HTTPS_PROXY"), "bypass": "*"} if os.getenv("HTTPS_PROXY") else None,
            )
            response = await cortex.llm.openai.agent.browser_agent.BrowserAgent(
                browser=browser,
                model=self.model.replace(".", "") if os.getenv("OPENAI_API_TYPE") == "azure" else self.model,
                encoder=self.encoder,
            ).chat_completion_loop(
                [{
                    "role": "user",
                    "content": query,
                }],
                context,
                streaming=False,
            )

            context.increment_completion_tokens(self.model, response.usage.completion_tokens)
            return cortex.llm.openai.agent.FunctionResult(response=response.choices[0].message.content)
