import base64
import contextlib

import mcp.client.sse
import mcp.shared.exceptions
import tiktoken

import cortex.llm.openai.model


class MCPAgent(cortex.llm.openai.agent.Agent):
    url: str

    def __init__(
        self,
        model: cortex.llm.openai.model.CompletionModel,
        encoder: tiktoken.Encoding,
        url: str,
    ):
        self.model = model
        self.encoder = encoder
        self.url = url
        self.functions = {}

    async def __aenter__(self):
        self._stack = contextlib.AsyncExitStack()

        read, write = await self._stack.enter_async_context(
            mcp.client.sse.sse_client(self.url)
        )
        self._session = await self._stack.enter_async_context(
            mcp.ClientSession(read, write)
        )

        await self._session.initialize()

        response = await self._session.list_tools()
        for tool in response.tools:
            async def _func(
                context: cortex.llm.openai.agent.Context,
                *,
                _session=self._session,
                _tool_name=tool.name,
                **arguments
            ) -> cortex.llm.openai.agent.FunctionResult:
                try:
                    result = await _session.call_tool(_tool_name, arguments=arguments)
                    return cortex.llm.openai.agent.FunctionResult(
                        response="\n".join([content.text for content in result.content if content.type == "text"]),
                    )
                except mcp.shared.exceptions.McpError as e:
                    return cortex.llm.openai.agent.FunctionResult(response=e.error.message)

            self.functions[tool.name] = cortex.llm.openai.agent.FunctionDefinition(
                func=_func,
                description=tool.description,
                parameters=tool.inputSchema,
            )

        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self._stack.aclose()
