package gemini

import (
	"armyknife/internal/mcp"
	"encoding/json"
	"os"
	"os/exec"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

type Handler struct {
	mcp.DefaultHandler
}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) GetServerInfo() mcp.ServerInfo {
	return mcp.ServerInfo{
		Name:    "gemini",
		Version: "1.0.0",
	}
}

func (h *Handler) GetTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "gemini",
			Description: "AI agent that brings the power of Gemini directly into your terminal. Ground your queries with the Google Search tool, built in to Gemini.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"prompt": {
						Type:        "string",
						Description: "Prompt.",
					},
				},
				Required: []string{"prompt"},
			},
		},
	}
}

func (h *Handler) CallTool(name string, arguments json.RawMessage) (mcp.ToolCallResult, error) {
	switch name {
	case "gemini":
		var args struct {
			Prompt string `json:"prompt"`
		}

		if err := json.Unmarshal(arguments, &args); err != nil {
			return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
		}

		cmd := exec.Command("gemini", "--prompt", args.Prompt)
		cmd.Env = append(os.Environ(), "NODE_OPTIONS=--no-deprecation")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return mcp.ToolCallResult{}, xerrors.Errorf("failed to execute command: %w: %s", err, string(output))
		}

		return mcp.ToolCallResult{
			Content: []mcp.ContentItem{
				{
					Type: "text",
					Text: string(output),
				},
			},
		}, nil
	default:
		return mcp.ToolCallResult{}, xerrors.Errorf("unknown tool: %s", name)
	}
}

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	handler := NewHandler()

	server := mcp.NewServer(handler)

	return server.Run()
}
