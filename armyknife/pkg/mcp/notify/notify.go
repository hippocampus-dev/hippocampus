package notify

import (
	"armyknife/internal/mcp"
	"encoding/json"
	"fmt"

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
		Name:    "notify",
		Version: "1.0.0",
	}
}

func (h *Handler) GetTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "notify",
			Description: "Send a desktop notification (cross-platform: Linux, macOS, Windows)",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"summary": {
						Type:        "string",
						Description: "Notification summary.",
					},
					"body": {
						Type:        "string",
						Description: "Notification body.",
					},
					"urgency": {
						Type:        "string",
						Description: "Specifies the urgency level (low, normal, critical).",
					},
					"expire-time": {
						Type:        "integer",
						Description: "Specifies the timeout in milliseconds at which to expire the notification.",
					},
				},
				Required: []string{"summary", "body"},
			},
		},
	}
}

func (h *Handler) CallTool(name string, arguments json.RawMessage) (mcp.ToolCallResult, error) {
	switch name {
	case "notify":
		var args struct {
			Summary    string  `json:"summary"`
			Body       string  `json:"body"`
			Urgency    *string `json:"urgency,omitempty"`
			ExpireTime *uint   `json:"expire-time,omitempty"`
		}

		if err := json.Unmarshal(arguments, &args); err != nil {
			return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
		}

		return h.sendNotification(args.Summary, args.Body, args.Urgency, args.ExpireTime)
	default:
		return mcp.ToolCallResult{}, xerrors.Errorf("unknown tool: %s", name)
	}
}

func (h *Handler) sendNotification(summary string, body string, urgency *string, expireTime *uint) (mcp.ToolCallResult, error) {
	err := h.sendPlatformNotification(summary, body, urgency, expireTime)
	if err != nil {
		return mcp.ToolCallResult{}, err
	}

	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Notification sent successfully: %s - %s", summary, body),
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
