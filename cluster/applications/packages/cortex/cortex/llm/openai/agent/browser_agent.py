import bs4
import playwright.async_api
import tiktoken

import cortex.llm.openai.agent
import cortex.llm.openai.model


class BrowserAgent(cortex.llm.openai.agent.Agent):
    browser: playwright.async_api.Browser
    page: playwright.async_api.Page | None

    def __init__(
        self,
        browser: playwright.async_api.Browser,
        model: cortex.llm.openai.model.CompletionModel,
        encoder: tiktoken.Encoding,
    ):
        self.browser = browser
        self.page = None

        self.model = model
        self.encoder = encoder
        self.functions = {
            "navigate": cortex.llm.openai.agent.FunctionDefinition(
                func=self.navigate,
                description="Navigate a browser to the specified URL.",
                parameters={
                    "type": "object",
                    "properties": {
                        "url": {
                            "type": "string",
                            "description": "url to navigate to",
                        },
                    },
                    "required": ["url"],
                },
            ),
            "navigate_back": cortex.llm.openai.agent.FunctionDefinition(
                func=self.navigate_back,
                description="Navigate back to the previous page in the browser history.",
                parameters={
                    "type": "object",
                    "properties": {},
                },
            ),
            "extract_text": cortex.llm.openai.agent.FunctionDefinition(
                func=self.extract_text,
                direct_return=True,  # Prompt be duplicated if without planner
                description="Extract all the text on the current web page.",
                parameters={
                    "type": "object",
                    "properties": {},
                },
            ),
            "screenshot": cortex.llm.openai.agent.FunctionDefinition(
                func=self.screenshot,
                description="Take a screenshot and store it in memory.",
                parameters={
                    "type": "object",
                    "properties": {},
                },
            ),
        }

    async def navigate(
        self,
        url: str,
        context: cortex.llm.openai.agent.Context,
    ) -> cortex.llm.openai.agent.FunctionResult:
        p = await self._get_page(self.browser)
        try:
            response = await p.goto(url, timeout=30000, wait_until="networkidle")
        except playwright.async_api.TimeoutError:
            return cortex.llm.openai.agent.FunctionResult(response="Timeout when navigating")
        except playwright.async_api.Error:
            return cortex.llm.openai.agent.FunctionResult(response="Error when navigating")
        if response:
            return cortex.llm.openai.agent.FunctionResult(
                response=f"Navigating to {url} returned status code {response.status}")
        else:
            return cortex.llm.openai.agent.FunctionResult(response="Unable to navigate")

    async def navigate_back(self, context: cortex.llm.openai.agent.Context) -> cortex.llm.openai.agent.FunctionResult:
        p = await self._get_page(self.browser)
        try:
            response = await p.go_back(timeout=30000, wait_until="networkidle")
        except playwright.async_api.TimeoutError:
            return cortex.llm.openai.agent.FunctionResult(response="Timeout when navigating")
        except playwright.async_api.Error:
            return cortex.llm.openai.agent.FunctionResult(response="Error when navigating")
        if response:
            return cortex.llm.openai.agent.FunctionResult(
                response=f"Navigated back to the previous page with URL '{response.url}'",
            )
        else:
            return cortex.llm.openai.agent.FunctionResult(response="Unable to navigate back")

    async def extract_text(self, context: cortex.llm.openai.agent.Context) -> cortex.llm.openai.agent.FunctionResult:
        p = await self._get_page(self.browser)
        html_content = await p.content()
        soup = bs4.BeautifulSoup(html_content, "html.parser")

        for script in soup(["script", "style"]):
            script.decompose()

        for img_tag in soup.find_all("img"):
            img_tag.replace_with(img_tag.get("alt", ""))

        return cortex.llm.openai.agent.FunctionResult(response=soup.get_text(separator=" ", strip=True))

    async def screenshot(self, context: cortex.llm.openai.agent.Context) -> cortex.llm.openai.agent.FunctionResult:
        p = await self._get_page(self.browser)
        await p.set_viewport_size({'width': 1920, 'height': 1080})
        result = await p.screenshot()

        await context.cache_history(
            f"Screenshot of {p.url}",
            self.functions["screenshot"].cache.expire,
            cortex.llm.openai.agent.History(body=result),
            namespace="content",
        )

        return cortex.llm.openai.agent.FunctionResult(response="Screenshot taken is succeeded but it need to upload")

    async def _get_page(self, browser: playwright.async_api.Browser) -> playwright.async_api.Page:
        if self.page is None or self.page.is_closed():
            context = await browser.new_context()
            self.page = await context.new_page()
        return self.page
