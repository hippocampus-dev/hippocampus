export interface ChatMessage {
  role: "system" | "user" | "assistant";
  content: string;
}

export interface ChatCompletionOptions {
  messages: ChatMessage[];
  model?: string;
  maxTokens?: number;
  temperature?: number;
}

export interface ChatCompletionResponse {
  content: string;
  model: string;
}

export interface AiProvider {
  createChatCompletion(
    options: ChatCompletionOptions
  ): Promise<ChatCompletionResponse>;
}
