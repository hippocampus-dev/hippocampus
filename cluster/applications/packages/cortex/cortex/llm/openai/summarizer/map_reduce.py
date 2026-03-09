import asyncio

import tiktoken

import cortex.llm.openai.summarizer

SYSTEM_PROMPT_TEMPLATE = (
    "Your task is to create a concise running summary of actions and information results in the provided text while keeping the input language, focusing on key and potentially important information to remember.\n"
    "---\n"
    "{text}\n"
    "---"
)


class MapReduceSummarizer(cortex.llm.openai.summarizer.Summarizer):
    max_challenge: int

    def __init__(
        self,
        model: cortex.llm.openai.model.CompletionModel,
        encoder: tiktoken.Encoding,
        max_challenge: int = 5,
    ):
        super().__init__(model, encoder)
        self.max_challenge = max_challenge

    async def summarize(self, content: str) -> str:
        initial_prompt_tokens = self.encoder.encode(SYSTEM_PROMPT_TEMPLATE.format(text=""), disallowed_special=())
        initial_prompt_tokens_count = len(initial_prompt_tokens)
        messages_format_token = len(self.encoder.encode('{"role":"system","content":""}', disallowed_special=()))

        chunk_size = int((self.model.max_tokens - initial_prompt_tokens_count - messages_format_token) / 2)

        # Map
        chunks = self._get_text_chunks(content, chunk_size)
        mapped_summaries = await asyncio.gather(
            *[self._summary(SYSTEM_PROMPT_TEMPLATE.format(text=chunk)) for chunk in chunks]
        )

        # Reduce
        summary = "".join(mapped_summaries)
        challenge = 1
        while len(self.encoder.encode(summary, disallowed_special=())) > self.model.max_tokens:
            # Avoid infinite loop
            if challenge > self.max_challenge:
                return summary
            challenge += 1

            combined_chunks = self._get_text_chunks(summary, chunk_size)
            reduced_summaries = await asyncio.gather(
                *[self._summary(SYSTEM_PROMPT_TEMPLATE.format(text=chunk)) for chunk in combined_chunks]
            )

            summary = "".join(reduced_summaries)

        return summary
