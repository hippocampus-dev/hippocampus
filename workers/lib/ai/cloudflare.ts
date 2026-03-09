import type {
  AiProvider,
  ChatCompletionOptions,
  ChatCompletionResponse,
} from "./types";

const DEFAULT_MODEL = "@cf/meta/llama-3.1-8b-instruct";
const DEFAULT_MAX_TOKENS = 1024;

export class CloudflareAiProvider implements AiProvider {
  private ai: Ai;

  constructor(ai: Ai) {
    this.ai = ai;
  }

  async createChatCompletion(
    options: ChatCompletionOptions
  ): Promise<ChatCompletionResponse> {
    const model = options.model ?? DEFAULT_MODEL;

    const response = (await this.ai.run(
      model as Parameters<typeof this.ai.run>[0],
      {
        messages: options.messages.map((m) => ({
          role: m.role,
          content: m.content,
        })),
        max_tokens: options.maxTokens ?? DEFAULT_MAX_TOKENS,
        temperature: options.temperature,
      }
    )) as { response?: string };

    return {
      content: response.response ?? "",
      model,
    };
  }
}
