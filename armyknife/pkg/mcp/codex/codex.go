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
	profile string
}

func NewHandler(profile string) *Handler {
	return &Handler{
		profile: profile,
	}
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
			Prompt string `json:"prompt"`
		}

		if err := json.Unmarshal(arguments, &args); err != nil {
			return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
		}

		a := []string{"exec", "--json"}
		if h.profile != "" {
			a = append(a, "--profile", h.profile)
		}
		a = append(a, args.Prompt)

		cmd := exec.Command("codex", a...)
		// Read by files/home/kai/.config/claudex/config/hooks/Stop.sh
		cmd.Env = append(os.Environ(), "RUST_LOG=", "LAST_REPLY_ONLY=1")
		output, err := cmd.Output()
		if err != nil {
			return mcp.ToolCallResult{}, xerrors.Errorf("failed to execute command: %w: %s", err, string(output))
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

	handler := NewHandler(a.Profile)

	server := mcp.NewServer(handler)

	if a.Address != "" {
		return server.ServeSSE(a.Address)
	}
	return server.ServeStdio()
}
