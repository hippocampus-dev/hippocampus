package codex

import (
	"armyknife/internal/mcp"
	"encoding/json"
	"os"
	"os/exec"
	"strings"

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
		Name:    "codex",
		Version: "1.0.0",
	}
}

func (h *Handler) GetTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "codex",
			Description: "AI agent that brings the power of Codex directly into your terminal.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"prompt": {
						Type:        "string",
						Description: "Prompt.",
					},
					"profile": {
						Type:        "string",
						Description: "gpt-5-codex or gpt-5",
					},
				},
				Required: []string{"prompt"},
			},
		},
	}
}

type Line struct {
	Type string `json:"type"`
	Item Item   `json:"item"`
}

type Item struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Text string `json:"text"`
}

func (h *Handler) CallTool(name string, arguments json.RawMessage) (mcp.ToolCallResult, error) {
	switch name {
	case "codex":
		var args struct {
			Prompt  string `json:"prompt"`
			Profile string `json:"profile"`
		}

		if err := json.Unmarshal(arguments, &args); err != nil {
			return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
		}

		if args.Profile == "" {
			args.Profile = "gpt-5-codex"
		}

		cmd := exec.Command("codex", "exec", "--json", "--profile", args.Profile, args.Prompt)
		cmd.Env = append(os.Environ(), "RUST_LOG=")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return mcp.ToolCallResult{}, xerrors.Errorf("failed to execute command: %w", err)
		}

		text := ""
		lines := strings.Lines(string(output))
		for line := range lines {
			var l Line
			if err := json.Unmarshal([]byte(line), &l); err != nil {
				return mcp.ToolCallResult{}, xerrors.Errorf("failed to unmarshal line: %w", err)
			}

			if l.Item.Type == "agent_message" {
				text = l.Item.Text
			}
		}

		return mcp.ToolCallResult{
			Content: []mcp.ContentItem{
				{
					Type: "text",
					Text: text,
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
