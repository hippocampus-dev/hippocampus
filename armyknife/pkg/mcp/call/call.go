package call

import (
	"armyknife/internal/mcp"
	"encoding/json"
	"fmt"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	var arguments map[string]interface{}
	if a.Arguments != "" {
		if err := json.Unmarshal([]byte(a.Arguments), &arguments); err != nil {
			return xerrors.Errorf("failed to parse arguments: %w", err)
		}
	}

	result, err := mcp.CallToolSSE(a.URL, a.Tool, arguments)
	if err != nil {
		return xerrors.Errorf("failed to call %s: %w", a.Tool, err)
	}

	for _, item := range result.Content {
		if item.Type == "text" {
			fmt.Println(item.Text)
			break
		}
	}

	return nil
}
