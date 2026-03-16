package tmux

import (
	"armyknife/internal/mcp"
	"encoding/json"
	"fmt"
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
		Name:    "tmux",
		Version: "1.0.0",
	}
}

func (h *Handler) GetTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "rename",
			Description: "Rename the current tmux pane's title",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"title": {
						Type:        "string",
						Description: "The new title for the tmux pane",
					},
				},
				Required: []string{"title"},
			},
		},
	}
}

func (h *Handler) CallTool(name string, arguments json.RawMessage) (mcp.ToolCallResult, error) {
	paneID := os.Getenv("TMUX_PANE")

	switch name {
	case "rename":
		var args struct {
			Title string `json:"title"`
		}

		if err := json.Unmarshal(arguments, &args); err != nil {
			return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
		}

		if args.Title == "" {
			return mcp.ToolCallResult{}, xerrors.Errorf("title cannot be empty")
		}

		if err := exec.Command("tmux", "select-pane", "-t", paneID, "-T", args.Title).Run(); err != nil {
			return mcp.ToolCallResult{}, xerrors.Errorf("failed to set pane title: %w", err)
		}

		return mcp.ToolCallResult{
			Content: []mcp.ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Renamed pane to: %s", args.Title),
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
