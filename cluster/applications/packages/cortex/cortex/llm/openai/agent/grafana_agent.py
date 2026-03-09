import base64
import contextlib
import functools
import typing

import mcp.client.sse
import mcp.shared.exceptions
import tiktoken

import cortex.llm.openai.model


def _with_default_datasource(
    func: typing.Callable[..., typing.Awaitable[cortex.llm.openai.agent.FunctionResult]],
    datasource: str,
) -> typing.Callable[..., typing.Awaitable[cortex.llm.openai.agent.FunctionResult]]:
    async def wrapper(context: cortex.llm.openai.agent.Context, **arguments) -> cortex.llm.openai.agent.FunctionResult:
        arguments["datasourceUid"] = datasource
        return await func(context, **arguments)

    return wrapper


class GrafanaAgent(cortex.llm.openai.agent.Agent):
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
            if tool.name in ["list_contact_points"]:
                continue

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

            func = _func
            direct_return = False

            match tool.name:
                case "query_prometheus" | "query_loki_logs":
                    direct_return = True

            if "loki" in tool.name:
                if tool.inputSchema["properties"].get("logql") is not None:
                    tool.inputSchema["properties"]["logql"][
                        "description"] += "Only the `grouping` label is supported, and it must follow the format `kubernetes.<NAMESPACE>.<app.kubernetes.io/name>`. e.g. `{grouping=\"kubernetes.<NAMESPACE>.<app.kubernetes.io/name>\"}`. If either the `NAMESPACE` or `app.kubernetes.io/name` is missing or unknown, please ask the user to provide the required information. At minimum, you must specify the `NAMESPACE`. Once provided, you can use a grouping selector like `{grouping=~\"kubernetes.<NAMESPACE>.*\"}`."
                if tool.inputSchema["properties"].get("datasourceUid") is not None:
                    func = _with_default_datasource(func, "loki")
                    del tool.inputSchema["properties"]["datasourceUid"]
                    tool.inputSchema["required"].remove("datasourceUid")

            if "prometheus" in tool.name:
                if tool.inputSchema["properties"].get("datasourceUid") is not None:
                    func = _with_default_datasource(func, "prometheus")
                    del tool.inputSchema["properties"]["datasourceUid"]
                    tool.inputSchema["required"].remove("datasourceUid")

            self.functions[tool.name] = cortex.llm.openai.agent.FunctionDefinition(
                func=func,
                direct_return=direct_return,
                description=tool.description,
                parameters=tool.inputSchema,
            )

        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self._stack.aclose()
