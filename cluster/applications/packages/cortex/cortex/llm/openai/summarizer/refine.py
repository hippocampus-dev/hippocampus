import tiktoken

import cortex.llm.openai.summarizer

INITIAL_SYSTEM_PROMPT_TEMPLATE = (
    "Your task is to create a concise running summary of actions and information results in the provided text while keeping the input language, focusing on key and potentially important information to remember.\n"
    "---\n"
    "{text}\n"
    "---"
)

REFINE_SYSTEM_PROMPT_TEMPLATE = (
    "Your task is to create a concise running summary of actions and information results in the provided text while keeping the input language, focusing on key and potentially important information to remember.\n"
    "We have provided an existing summary up to a certain point: {existing_answer}\n"
    "We have the opportunity to refine the existing summary (only if needed) with some more context below.\n"
    "---\n"
    "{text}\n"
    "---\n"
    "Given the new context, refine the original summary.\n"
    "If the context isn't useful, return the original summary."
)


class RefineSummarizer(cortex.llm.openai.summarizer.Summarizer):
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
        initial_prompt_tokens = self.encoder.encode(INITIAL_SYSTEM_PROMPT_TEMPLATE.format(text=""),
                                                    disallowed_special=())
        initial_prompt_tokens_count = len(initial_prompt_tokens)
        refine_prompt_tokens = self.encoder.encode(REFINE_SYSTEM_PROMPT_TEMPLATE.format(existing_answer="", text=""),
                                                   disallowed_special=())
        refine_prompt_tokens_count = len(refine_prompt_tokens)
        messages_format_token = len(self.encoder.encode('{"role":"system","content":""}', disallowed_special=()))

        default_chunk_size = int(
            (
                self.model.max_tokens - initial_prompt_tokens_count - refine_prompt_tokens_count - messages_format_token
            ) / 2
        )

        tokens = self.encoder.encode(content, disallowed_special=())
        chunk_size = default_chunk_size
        if len(tokens) < chunk_size:
            return await self._summary(INITIAL_SYSTEM_PROMPT_TEMPLATE.format(text=content))

        summary = ""
        challenge = 1
        while tokens:
            # Avoid infinite loop
            if chunk_size <= 0 or challenge > self.max_challenge:
                return summary
            challenge += 1

            chunk = tokens[:chunk_size]
            chunk_text = self._punctuate(self.encoder.decode(chunk))

            tokens = tokens[len(self.encoder.encode(chunk_text, disallowed_special=())):]

            prompt = INITIAL_SYSTEM_PROMPT_TEMPLATE.format(
                text=chunk_text
            ) if summary == "" else REFINE_SYSTEM_PROMPT_TEMPLATE.format(
                existing_answer=summary,
                text=chunk_text
            )
            summary = await self._summary(prompt)

            chunk_size = default_chunk_size - len(self.encoder.encode(summary, disallowed_special=()))

        return summary
