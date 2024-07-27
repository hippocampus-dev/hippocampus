import os
import unittest

import aiohttp
import dotenv
import tiktoken

import bot.agent.root_agent
import cortex.llm.openai.agent
import cortex.llm.openai.model


class DummyContext(cortex.llm.openai.agent.Context):
    def __init__(self):
        super().__init__(
            "dummy",
            cortex.llm.openai.agent.MemoryType.Redis,
            cortex.llm.openai.model.EmbeddingModel.ADA_V2,
            tiktoken.get_encoding("cl100k_base"),
        )

    async def report_progress(self, message: str, stage: cortex.llm.openai.agent.ProgressStage):
        pass


class TestCase(unittest.IsolatedAsyncioTestCase):
    async def test_simple_query(self) -> None:
        context = DummyContext()
        await context.acquire_budget(10)
        result = await bot.agent.root_agent.RootAgent(
            embedding_retrieval_endpoint=os.getenv("EMBEDDING_RETRIEVAL_ENDPOINT"),
            github_token=os.getenv("GITHUB_TOKEN"),
            slack_token=os.getenv("SLACK_TOKEN"),
            bing_subscription_key=None,
            model=cortex.llm.openai.model.CompletionModel.GPT4,
            encoder=tiktoken.get_encoding("cl100k_base"),
            image_model=cortex.llm.openai.model.ImageModel.x256,
            audio_model=cortex.llm.openai.model.AudioModel.Whisper1,
        ).chat_completion_loop(
            [{
                "role": "user",
                "content": "Hi",
            }],
            context,
        )
        self.assertIsNotNone(result.choices[0].message.content)

    async def test_using_browser_agent(self) -> None:
        context = DummyContext()
        await context.acquire_budget(10)
        result = await bot.agent.root_agent.RootAgent(
            embedding_retrieval_endpoint=os.getenv("EMBEDDING_RETRIEVAL_ENDPOINT"),
            github_token=os.getenv("GITHUB_TOKEN"),
            slack_token=os.getenv("SLACK_TOKEN"),
            bing_subscription_key=None,
            model=cortex.llm.openai.model.CompletionModel.GPT4,
            encoder=tiktoken.get_encoding("cl100k_base"),
            image_model=cortex.llm.openai.model.ImageModel.x256,
            audio_model=cortex.llm.openai.model.AudioModel.Whisper1,
        ).chat_completion_loop(
            [{
                "role": "user",
                "content": "get my IP address from https://httpbin.org/ip",
            }],
            context,
        )
        async with aiohttp.ClientSession(trust_env=True) as session:
            async with session.get("https://httpbin.org/ip") as response:
                j = await response.json()
                self.assertRegex(result.choices[0].message.content, j["origin"])


if __name__ == "__main__":
    dotenv.load_dotenv(override=True)

    unittest.main()
