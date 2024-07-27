package openai

import (
	"encoding/json"
	"errors"
)

var ErrInvalidChatRole = errors.New("invalid chat role")

type ChatRole uint

const (
	System ChatRole = iota
	User
	Assistant
)

func (c ChatRole) MarshalJSON() ([]byte, error) {
	var s string
	switch c {
	case System:
		s = "system"
	case User:
		s = "user"
	case Assistant:
		s = "assistant"
	default:
		return nil, ErrInvalidChatRole
	}
	return json.Marshal(s)
}

func (c *ChatRole) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "system":
		*c = System
	case "user":
		*c = User
	case "assistant":
		*c = Assistant
	default:
		return ErrInvalidChatRole
	}
	return nil
}

type ChatMessage struct {
	Role    ChatRole `json:"role"`
	Content string   `json:"content"`
}

type ChatCompletionRequestBody struct {
	Model            string        `json:"model"`
	Messages         []ChatMessage `json:"messages"`
	Temperature      float64       `json:"temperature"`
	TopP             float64       `json:"top_p"`
	N                int           `json:"n"`
	MaxTokens        *int          `json:"max_tokens,omitempty"`
	PresencePenalty  float64       `json:"presence_penalty"`
	FrequencyPenalty float64       `json:"frequency_penalty"`
}

type ChatCompletionResponseBody struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Choices []struct {
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
		Index        int         `json:"index"`
	} `json:"choices"`
}

type ErrorResponseBody struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}
