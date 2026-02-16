package prompts

import (
	"armyknife/internal/mcp"
	"fmt"
	"sort"
	"strings"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

type Handler struct {
	mcp.DefaultHandler
	prompts map[string]promptDefinition
}

type promptDefinition struct {
	description string
	arguments   []mcp.PromptArgument
	template    string
}

func NewHandler() *Handler {
	return &Handler{
		prompts: map[string]promptDefinition{
			"code-review": {
				description: "Generate a code review for the provided code",
				arguments: []mcp.PromptArgument{
					{
						Name:        "code",
						Description: "The code to review",
						Required:    true,
					},
					{
						Name:        "language",
						Description: "Programming language of the code",
						Required:    false,
					},
				},
				template: "Please review the following {{.language}} code and provide feedback on:\n" +
					"1. Code quality and best practices\n" +
					"2. Potential bugs or issues\n" +
					"3. Performance considerations\n" +
					"4. Security concerns\n" +
					"5. Suggestions for improvement\n\n" +
					"Code to review:\n" +
					"```{{.language}}\n" +
					"{{.code}}\n" +
					"```",
			},
			"explain-code": {
				description: "Explain how the provided code works",
				arguments: []mcp.PromptArgument{
					{
						Name:        "code",
						Description: "The code to explain",
						Required:    true,
					},
					{
						Name:        "language",
						Description: "Programming language of the code",
						Required:    false,
					},
				},
				template: "Please explain how the following {{.language}} code works. Break down the logic step by step and explain any complex concepts in simple terms.\n\n" +
					"Code to explain:\n" +
					"```{{.language}}\n" +
					"{{.code}}\n" +
					"```",
			},
			"debug-error": {
				description: "Help debug an error message",
				arguments: []mcp.PromptArgument{
					{
						Name:        "error",
						Description: "The error message or stack trace",
						Required:    true,
					},
					{
						Name:        "context",
						Description: "Additional context about when the error occurs",
						Required:    false,
					},
				},
				template: "I'm encountering the following error:\n\n" +
					"```\n" +
					"{{.error}}\n" +
					"```\n\n" +
					"{{if .context}}Context: {{.context}}{{end}}\n\n" +
					"Please help me understand:\n" +
					"1. What this error means\n" +
					"2. Common causes of this error\n" +
					"3. How to fix it\n" +
					"4. How to prevent it in the future",
			},
			"git-commit": {
				description: "Generate a Git commit message",
				arguments: []mcp.PromptArgument{
					{
						Name:        "diff",
						Description: "The git diff or description of changes",
						Required:    true,
					},
				},
				template: "Based on the following changes, please generate a concise and descriptive Git commit message following conventional commit standards:\n\n" +
					"Changes:\n" +
					"```diff\n" +
					"{{.diff}}\n" +
					"```\n\n" +
					"The commit message should:\n" +
					"- Start with a type (feat, fix, docs, style, refactor, test, chore)\n" +
					"- Be written in present tense\n" +
					"- Be no longer than 72 characters\n" +
					"- Clearly describe what was changed and why",
			},
		},
	}
}

func (h *Handler) GetServerInfo() mcp.ServerInfo {
	return mcp.ServerInfo{
		Name:    "prompts",
		Version: "1.0.0",
	}
}

func (h *Handler) GetPrompts() []mcp.Prompt {
	prompts := make([]mcp.Prompt, 0, len(h.prompts))
	names := make([]string, 0, len(h.prompts))
	for name := range h.prompts {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		prompt := h.prompts[name]
		prompts = append(prompts, mcp.Prompt{
			Name:        name,
			Description: prompt.description,
			Arguments:   prompt.arguments,
		})
	}
	return prompts
}

func (h *Handler) GetPrompt(name string, arguments map[string]interface{}) (mcp.PromptResult, error) {
	prompt, exists := h.prompts[name]
	if !exists {
		return mcp.PromptResult{}, xerrors.Errorf("prompt not found: %s", name)
	}

	// Validate required arguments
	for _, arg := range prompt.arguments {
		if arg.Required {
			if _, exists := arguments[arg.Name]; !exists {
				return mcp.PromptResult{}, xerrors.Errorf("required argument missing: %s", arg.Name)
			}
		}
	}

	// Replace template placeholders with arguments
	content := prompt.template
	for key, value := range arguments {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		// Escape special characters to prevent injection
		valueStr := fmt.Sprintf("%v", value)
		content = strings.ReplaceAll(content, placeholder, valueStr)
	}

	// Handle optional arguments that weren't provided
	for _, arg := range prompt.arguments {
		if !arg.Required {
			placeholder := fmt.Sprintf("{{.%s}}", arg.Name)
			content = strings.ReplaceAll(content, placeholder, "")
			// Also handle conditional blocks
			ifStart := fmt.Sprintf("{{if .%s}}", arg.Name)
			ifEnd := "{{end}}"
			for strings.Contains(content, ifStart) {
				startIdx := strings.Index(content, ifStart)
				endIdx := strings.Index(content[startIdx:], ifEnd)
				if endIdx != -1 {
					content = content[:startIdx] + content[startIdx+endIdx+len(ifEnd):]
				} else {
					break
				}
			}
		}
	}

	return mcp.PromptResult{
		Description: prompt.description,
		Messages: []mcp.PromptMessage{
			{
				Role: "user",
				Content: mcp.PromptMessageContent{
					Type: "text",
					Text: content,
				},
			},
		},
	}, nil
}

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	handler := NewHandler()

	server := mcp.NewServer(handler)

	return server.Run()
}
