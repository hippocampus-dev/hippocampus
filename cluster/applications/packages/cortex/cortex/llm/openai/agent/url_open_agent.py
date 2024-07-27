import asyncio
import base64
import io
import json
import os
import pathlib

import aiohttp
import aiohttp.client_exceptions
import bs4
import google.auth.transport.requests
import google.oauth2.credentials
import google_auth_httplib2
import googleapiclient.discovery
import googleapiclient.errors
import googleapiclient.http
import httplib2
import opentelemetry.context
import opentelemetry.trace
import playwright.async_api
import tiktoken
import unstructured.partition.auto

import cortex.exceptions
import cortex.llm.openai.agent.browser_agent
import cortex.llm.openai.model


class URLOpenAgent(cortex.llm.openai.agent.Agent):
    github_token: str
    slack_token: str
    google_credentials: google.oauth2.credentials.Credentials

    def __init__(
        self,
        github_token: str,
        slack_token: str,
        google_credentials: google.oauth2.credentials.Credentials,
        model: cortex.llm.openai.model.CompletionModel,
        encoder: tiktoken.Encoding,
    ):
        self.github_token = github_token
        self.slack_token = slack_token
        self.google_credentials = google_credentials

        self.model = model
        self.encoder = encoder
        self.system_prompt = """\
Your task is to choice the correct function to open the URL and retrieve its content.
"""
        self.functions = {
            "open_github_file": cortex.llm.openai.agent.FunctionDefinition(
                func=self.open_github_file,
                cache=cortex.llm.openai.agent.FunctionCache(enabled=True, similarity_threshold=1.0),
                direct_return=True,
                description="Open the GitHub file URL and retrieve its content. e.g. https://github.com/torvalds/linux/blob/master/net/ipv4/tcp.c -> owner: torvalds, repo: linux, ref: master, path: net/ipv4/tcp.c",
                parameters={
                    "type": "object",
                    "properties": {
                        "owner": {
                            "type": "string",
                            "description": "The account owner of the repository. The name is not case sensitive.",
                        },
                        "repo": {
                            "type": "string",
                            "description": "The name of the repository without the .git extension. The name is not case sensitive.",
                        },
                        "ref": {
                            "type": "string",
                            "description": "The name of the commit/branch/tag. The name is not case sensitive.",
                        },
                        "path": {
                            "type": "string",
                            "description": "The file path",
                        },
                    },
                    "required": ["owner", "repo", "ref", "path"],
                },
            ),
            "open_github_issue": cortex.llm.openai.agent.FunctionDefinition(
                func=self.open_github_issue,
                cache=cortex.llm.openai.agent.FunctionCache(enabled=True, similarity_threshold=1.0),
                direct_return=True,
                description="Open the GitHub issue URL and retrieve its content. e.g. https://github.com/golang/go/issues/59968",
                parameters={
                    "type": "object",
                    "properties": {
                        "owner": {
                            "type": "string",
                            "description": "The account owner of the repository. The name is not case sensitive.",
                        },
                        "repo": {
                            "type": "string",
                            "description": "The name of the repository without the .git extension. The name is not case sensitive.",
                        },
                        "issue_number": {
                            "type": "string",
                            "description": "The number that identifies the issue",
                        },
                    },
                    "required": ["owner", "repo", "issue_number"],
                },
            ),
            "open_slack_download_url": cortex.llm.openai.agent.FunctionDefinition(
                func=self.open_slack_download_url,
                cache=cortex.llm.openai.agent.FunctionCache(enabled=True, similarity_threshold=1.0),
                direct_return=True,
                description="Open the slack files URL and retrieve its content. e.g. https://files.slack.com/files-pri/T12345-F0S43P1CZ/image.png",
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
            "open_google_drive_url": cortex.llm.openai.agent.FunctionDefinition(
                func=self.open_google_drive_url,
                cache=cortex.llm.openai.agent.FunctionCache(enabled=True, similarity_threshold=1.0),
                direct_return=True,
                description="Open the Google Drive URL and retrieve its content. e.g. https://drive.google.com",
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
            "open_google_docs_url": cortex.llm.openai.agent.FunctionDefinition(
                func=self.open_google_docs_url,
                cache=cortex.llm.openai.agent.FunctionCache(enabled=True, similarity_threshold=1.0),
                direct_return=True,
                description="Open the Google Docs URL and retrieve its content. e.g. https://docs.google.com",
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
            "open_other_url": cortex.llm.openai.agent.FunctionDefinition(
                func=self.open_other_url,
                cache=cortex.llm.openai.agent.FunctionCache(enabled=True, similarity_threshold=1.0),
                direct_return=True,
                description="Open other URL in browser and retrieve its content.",
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

    async def open_github_file(
        self,
        owner: str,
        repo: str,
        ref: str,
        path: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        headers = {
            "Accept": "application/vnd.github+json",
            "Authorization": f"Bearer {self.github_token}",
            "X-GitHub-Api-Version": "2022-11-28",
        }
        try:
            async with aiohttp.ClientSession(headers=headers, trust_env=True) as session:
                async with session.get(
                    f"https://api.github.com/repos/{owner}/{repo}/contents/{path}?ref={ref}",
                ) as response:
                    if response.status != 200:
                        e = ValueError(f"invalid response status: {response.status}")
                        match response.status:
                            case 404:
                                return cortex.llm.openai.agent.FunctionResult(response=f"Not found: {path}")
                            case (409 | 429 | 502 | 503 | 504):
                                raise cortex.exceptions.RetryableError(e) from e
                        raise e
                    j = await response.json()
                    content = base64.b64decode(j["content"])
                    try:
                        if is_unstructured(pathlib.Path(path)):
                            elements = unstructured.partition.auto.partition(file=io.BytesIO(content))
                            return cortex.llm.openai.agent.FunctionResult(
                                instruction=f"SHOULD include `https://github.com/{owner}/{repo}/blob/{ref}/{path}` in the answer.",
                                response="\n".join([str(element) for element in elements]),
                            )
                        else:
                            return cortex.llm.openai.agent.FunctionResult(
                                instruction=f"SHOULD include `https://github.com/{owner}/{repo}/blob/{ref}/{path}` in the answer.",
                                response=content.decode("utf-8"),
                            )
                    except Exception:
                        await context.save_history(
                            f"{path} content",
                            self.functions["open_github_file"].cache.expire,
                            cortex.llm.openai.agent.History(body=content),
                            namespace="content",
                        )
                        return cortex.llm.openai.agent.FunctionResult(
                            instruction=f"""\
MUST include `{path} content` as arguments to call other function about this content.

e.g.
Generate an image of `{path}` other variations.
""",
                            response=f"`{path}` content is successfully saved.",
                        )
        except (
            aiohttp.ClientConnectionError,  # ECONNREFUSED, EPIPE, ECONNRESET
            aiohttp.client_exceptions.ServerDisconnectedError,
            asyncio.TimeoutError,
        ) as e:
            raise cortex.exceptions.RetryableError(e) from e

    async def open_github_issue(
        self,
        owner: str,
        repo: str,
        issue_number: int,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        headers = {
            "Accept": "application/vnd.github+json",
            "Authorization": f"Bearer {self.github_token}",
            "X-GitHub-Api-Version": "2022-11-28",
        }
        try:
            async with aiohttp.ClientSession(headers=headers, trust_env=True) as session:
                async with session.get(
                    f"https://api.github.com/repos/{owner}/{repo}/issues/{issue_number}",
                ) as response:
                    if response.status != 200:
                        e = ValueError(f"invalid response status: {response.status}")
                        match response.status:
                            case 404:
                                return cortex.llm.openai.agent.FunctionResult(response=f"Not found: #{issue_number}")
                            case (409 | 429 | 502 | 503 | 504):
                                raise cortex.exceptions.RetryableError(e) from e
                        raise e
                    j = await response.json()
                    return cortex.llm.openai.agent.FunctionResult(
                        response=json.dumps(
                            {
                                "title": j["title"],
                                "body": j["body"],
                            },
                            ensure_ascii=False,
                        ),
                    )
        except (
            aiohttp.ClientConnectionError,  # ECONNREFUSED, EPIPE, ECONNRESET
            aiohttp.client_exceptions.ServerDisconnectedError,
            asyncio.TimeoutError,
        ) as e:
            raise cortex.exceptions.RetryableError(e) from e

    async def open_slack_download_url(
        self,
        url: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        headers = {
            "Authorization": f"Bearer {self.slack_token}",
        }
        try:
            async with aiohttp.ClientSession(headers=headers, trust_env=True) as session:
                async with session.get(url, allow_redirects=True) as response:
                    if response.status != 200:
                        e = ValueError(f"invalid response status: {response.status}")
                        match response.status:
                            case 404:
                                return cortex.llm.openai.agent.FunctionResult(response=f"Not found: {url}")
                            case (409 | 429 | 502 | 503 | 504):
                                raise cortex.exceptions.RetryableError(e) from e
                        raise e
                    try:
                        if is_unstructured(pathlib.Path(url)):
                            b = await response.read()
                            elements = unstructured.partition.auto.partition(file=io.BytesIO(b))
                            return cortex.llm.openai.agent.FunctionResult(
                                instruction=f"SHOULD include `{url}` in the answer.",
                                response="\n".join([str(element) for element in elements]),
                            )
                        else:
                            return cortex.llm.openai.agent.FunctionResult(
                                instruction=f"SHOULD include `{url}` in the answer.",
                                response=await response.text(),
                            )
                    except Exception:
                        await context.save_history(
                            f"{url} content",
                            self.functions["open_slack_download_url"].cache.expire,
                            cortex.llm.openai.agent.History(body=await response.read()),
                            namespace="content",
                        )
                        return cortex.llm.openai.agent.FunctionResult(
                            instruction=f"""\
    MUST include `{url} content` as arguments to call other function about this content.

    e.g.
    Generate an image of `{url}` other variations.
    """,
                            response=f"`{url}` content is successfully saved.",
                        )
        except (
            aiohttp.ClientConnectionError,  # ECONNREFUSED, EPIPE, ECONNRESET
            aiohttp.client_exceptions.ServerDisconnectedError,
            asyncio.TimeoutError,
        ) as e:
            raise cortex.exceptions.RetryableError(e) from e

    async def open_google_drive_url(
        self,
        url: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        if self.google_credentials.expired:
            self.google_credentials.refresh(google.auth.transport.requests.Request())

        http = google_auth_httplib2.AuthorizedHttp(
            self.google_credentials,
            http=httplib2.Http(ca_certs=os.getenv("HTTPLIB2_CA_CERTS")),
        )
        drive = googleapiclient.discovery.build("drive", "v3", http=http)

        shards = url.split("/")
        file_id = shards[-2]

        buffer = io.BytesIO()

        def run_in_context(ctx: opentelemetry.context.Context):
            opentelemetry.context.attach(ctx)
            downloader = googleapiclient.http.MediaIoBaseDownload(buffer, drive.files().get_media(fileId=file_id))

            done = False
            while done is False:
                status, done = downloader.next_chunk()

        try:
            await asyncio.get_running_loop().run_in_executor(None, run_in_context, opentelemetry.context.get_current())
        except googleapiclient.errors.HttpError as e:
            match e.resp.status:
                case 403:
                    return cortex.llm.openai.agent.FunctionResult(response=f"Forbidden: {url}")
                case 404:
                    return cortex.llm.openai.agent.FunctionResult(response=f"Not found: {url}")
            raise e

        try:
            buffer.seek(0)
            elements = unstructured.partition.auto.partition(file=buffer)
            return cortex.llm.openai.agent.FunctionResult(
                response="\n".join([str(element) for element in elements]),
            )
        except Exception:
            buffer.seek(0)
            await context.save_history(
                f"{url} content",
                self.functions["open_google_drive_url"].cache.expire,
                cortex.llm.openai.agent.History(body=buffer.getvalue()),
                namespace="content",
            )
            return cortex.llm.openai.agent.FunctionResult(
                instruction=f"""\
MUST include `{url} content` as arguments to call other function about this content.

e.g.
Generate an image of `{url}` other variations.
""",
                response=f"`{url}` content is successfully saved.",
            )

    async def open_google_docs_url(
        self,
        url: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        if self.google_credentials.expired:
            self.google_credentials.refresh(google.auth.transport.requests.Request())

        mime_types = {
            "document": "text/plain",
            "spreadsheets": "text/csv",
            "presentation": "application/pdf",
        }

        shards = url.split("/")
        file_id, file_type = shards[-2], shards[-4]

        mime_type = mime_types.get(file_type)
        if mime_type is None:
            return cortex.llm.openai.agent.FunctionResult(response="Unsupported file type")

        http = google_auth_httplib2.AuthorizedHttp(
            self.google_credentials,
            http=httplib2.Http(ca_certs=os.getenv("HTTPLIB2_CA_CERTS")),
        )
        drive = googleapiclient.discovery.build("drive", "v3", http=http)

        buffer = io.BytesIO()

        def run_in_context(ctx: opentelemetry.context.Context):
            opentelemetry.context.attach(ctx)
            document = drive.files().export_media(fileId=file_id, mimeType=mime_type)
            downloader = googleapiclient.http.MediaIoBaseDownload(buffer, document)

            done = False
            while done is False:
                status, done = downloader.next_chunk()

        try:
            await asyncio.get_running_loop().run_in_executor(None, run_in_context, opentelemetry.context.get_current())
        except googleapiclient.errors.HttpError as e:
            match e.resp.status:
                case 403:
                    return cortex.llm.openai.agent.FunctionResult(response=f"Forbidden: {url}")
                case 404:
                    return cortex.llm.openai.agent.FunctionResult(response=f"Not found: {url}")
            raise e

        buffer.seek(0)
        elements = unstructured.partition.auto.partition(file=buffer)
        return cortex.llm.openai.agent.FunctionResult(
            response="\n".join([str(element) for element in elements]),
        )

    async def open_other_url(
        self,
        url: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        if is_unstructured(pathlib.Path(url)):
            async with aiohttp.ClientSession(trust_env=True) as session:
                try:
                    async with session.get(url, allow_redirects=True) as response:
                        if response.status != 200:
                            e = ValueError(f"invalid response status: {response.status}")
                            match response.status:
                                case 404:
                                    return cortex.llm.openai.agent.FunctionResult(response=f"Not found: {url}")
                                case (409 | 429 | 502 | 503 | 504):
                                    raise cortex.exceptions.RetryableError(e) from e
                            raise e
                        b = await response.read()
                        elements = unstructured.partition.auto.partition(file=io.BytesIO(b))
                        return cortex.llm.openai.agent.FunctionResult(
                            instruction=f"SHOULD include `{url}` in the answer.",
                            response="\n".join([str(element) for element in elements]),
                        )
                except aiohttp.client_exceptions.InvalidURL:
                    return cortex.llm.openai.agent.FunctionResult(response="Unable to navigate")

        with opentelemetry.trace.get_tracer("playwright").start_as_current_span(
            "chromium.launch",
            attributes={
                "http.url": url,
            },
        ):
            async with playwright.async_api.async_playwright() as pw:
                browser = await pw.chromium.launch(
                    proxy={"server": os.getenv("HTTPS_PROXY"), "bypass": "*"} if os.getenv("HTTPS_PROXY") else None,
                )
                browser_context: playwright.async_api.BrowserContext = await browser.new_context()
                page = await browser_context.new_page()
                try:
                    response = await page.goto(url, timeout=10000, wait_until="networkidle")
                except playwright.async_api.TimeoutError as e:
                    raise cortex.exceptions.RetryableError(e) from e
                except playwright.async_api.Error:
                    return cortex.llm.openai.agent.FunctionResult(response="Error when navigating")

                if response is None:
                    return cortex.llm.openai.agent.FunctionResult(response="Unable to navigate")

                content_type = response.headers.get("content-type", "")
                if content_type.startswith("text/html"):
                    html_content = await page.content()
                    soup = bs4.BeautifulSoup(html_content, "html.parser")

                    for script in soup(["script", "style"]):
                        script.decompose()

                    for img_tag in soup.find_all("img"):
                        img_tag.replace_with(img_tag.get("alt", ""))

                    message = json.dumps(
                        {
                            "title": soup.title.string if soup.title else "",
                            "text": soup.get_text(separator=" ", strip=True),
                        },
                        ensure_ascii=False,
                    )
                    return cortex.llm.openai.agent.FunctionResult(
                        instruction=f"SHOULD include `{url}` in the answer.",
                        response=message,
                    )
                else:
                    await context.save_history(
                        f"{url} content",
                        self.functions["open_other_url"].cache.expire,
                        cortex.llm.openai.agent.History(body=await response.body()),
                        namespace="content",
                    )
                    return cortex.llm.openai.agent.FunctionResult(
                        instruction=f"""\
MUST include `{url} content` as arguments to call other function about this content.

e.g.
Generate an image of `{url}` other variations.
""",
                        response=f"`{url}` content is successfully saved.",
                    )


def is_unstructured(
    path: pathlib.Path,
) -> bool:
    match path.suffix:
        case ".doc" | ".docx" | ".ppt" | ".pptx" | ".xls" | ".xlsx" | ".pdf":
            return True
        case _:
            return False
