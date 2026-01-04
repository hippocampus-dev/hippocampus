package claude

import (
	"armyknife/internal/mcp"
	"encoding/json"
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
		Name:    "claude",
		Version: "1.0.0",
	}
}

func (h *Handler) GetTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "claude",
			Description: "AI agent that brings the power of Claude directly into your terminal.",
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
	case "claude":
		var args struct {
			Prompt string `json:"prompt"`
		}

		if err := json.Unmarshal(arguments, &args); err != nil {
			return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
		}

		cmd := exec.Command("claude", "--print", "--no-session-persistence", args.Prompt)
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
