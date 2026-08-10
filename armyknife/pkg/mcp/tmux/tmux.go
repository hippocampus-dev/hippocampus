package tmux

import (
	"armyknife/internal/mcp"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

const maxTitleLength = 256

var panePattern = regexp.MustCompile(`^%[0-9]+$`)

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
	pane := mcp.Property{
		Type:        "string",
		Description: "Target tmux pane id, e.g. the session's $TMUX_PANE",
	}

	return []mcp.Tool{
		{
			Name:        "set_title",
			Description: "Set a tmux pane's title (the work summary shown in tmux and used as the notification label)",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"pane": pane,
					"title": {
						Type:        "string",
						Description: "The new pane title",
					},
				},
				Required: []string{"pane", "title"},
			},
		},
		{
			Name:        "set_monitor_silence",
			Description: "Set a tmux window's monitor-silence option, which the host completion detector reads (0 suppresses the silence alert and host power-off, a positive value such as 3 restores both)",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"pane": pane,
					"value": {
						Type:        "integer",
						Description: "monitor-silence seconds threshold (0 to suppress, 3 to restore)",
					},
				},
				Required: []string{"pane", "value"},
			},
		},
		{
			Name:        "get_title",
			Description: "Return a tmux pane's current title",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"pane": pane,
				},
				Required: []string{"pane"},
			},
		},
	}
}

func (h *Handler) CallTool(name string, arguments json.RawMessage) (mcp.ToolCallResult, error) {
	switch name {
	case "set_title":
		var args struct {
			Pane  string `json:"pane"`
			Title string `json:"title"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
		}
		pane, err := sanitizePane(args.Pane)
		if err != nil {
			return mcp.ToolCallResult{}, err
		}
		title, err := sanitizeTitle(args.Title)
		if err != nil {
			return mcp.ToolCallResult{}, err
		}

		if err := exec.Command("tmux", "select-pane", "-t", pane, "-T", title).Run(); err != nil {
			return mcp.ToolCallResult{}, xerrors.Errorf("failed to set pane title: %w", err)
		}

		return textResult(fmt.Sprintf("Set title: %s", title)), nil
	case "set_monitor_silence":
		var args struct {
			Pane  string `json:"pane"`
			Value int    `json:"value"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
		}
		pane, err := sanitizePane(args.Pane)
		if err != nil {
			return mcp.ToolCallResult{}, err
		}
		if args.Value < 0 {
			return mcp.ToolCallResult{}, xerrors.Errorf("value must not be negative")
		}

		if err := exec.Command("tmux", "setw", "-t", pane, "monitor-silence", strconv.Itoa(args.Value)).Run(); err != nil {
			return mcp.ToolCallResult{}, xerrors.Errorf("failed to set monitor-silence: %w", err)
		}

		return textResult(fmt.Sprintf("Set monitor-silence: %d", args.Value)), nil
	case "get_title":
		var args struct {
			Pane string `json:"pane"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
		}
		pane, err := sanitizePane(args.Pane)
		if err != nil {
			return mcp.ToolCallResult{}, err
		}

		out, err := exec.Command("tmux", "display-message", "-t", pane, "-p", "#{pane_title}").Output()
		if err != nil {
			return mcp.ToolCallResult{}, xerrors.Errorf("failed to get pane title: %w", err)
		}

		return textResult(strings.TrimRight(string(out), "\n")), nil
	default:
		return mcp.ToolCallResult{}, xerrors.Errorf("unknown tool: %s", name)
	}
}

// A %N pane id is required so a value tmux would read as a flag is rejected and
// the request cannot name other targets
func sanitizePane(pane string) (string, error) {
	if !panePattern.MatchString(pane) {
		return "", xerrors.Errorf("pane must be a tmux pane id like %%0")
	}
	return pane, nil
}

func sanitizeTitle(title string) (string, error) {
	if title == "" {
		return "", xerrors.Errorf("title cannot be empty")
	}

	runes := []rune(title)
	for _, r := range runes {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return "", xerrors.Errorf("title must not contain control characters")
		}
	}
	if len(runes) > maxTitleLength {
		runes = runes[:maxTitleLength]
	}

	return string(runes), nil
}

func textResult(text string) mcp.ToolCallResult {
	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type: "text",
				Text: text,
			},
		},
	}
}

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	server := mcp.NewServer(NewHandler())
	if a.Address != "" {
		return server.ServeSSE(a.Address)
	}
	return server.ServeStdio()
}
