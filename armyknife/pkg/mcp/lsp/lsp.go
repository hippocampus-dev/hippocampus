package lsp

import (
	"armyknife/internal/lsp"
	"armyknife/internal/mcp"
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

type Handler struct {
	mcp.DefaultHandler
	clients []*lsp.Client
}

type ServerResponse struct {
	ServerIndex int                     `json:"server_index"`
	Success     bool                    `json:"success"`
	Error       string                  `json:"error,omitempty"`
	Symbols     []lsp.SymbolInformation `json:"symbols,omitempty"`
}

func NewHandler(lspServers []string) (*Handler, error) {
	clients := make([]*lsp.Client, 0, len(lspServers))

	cwd, err := os.Getwd()
	if err != nil {
		return nil, xerrors.Errorf("failed to get current working directory: %w", err)
	}

	workspaceURI := fmt.Sprintf("file://%s", cwd)

	for _, serverAddress := range lspServers {
		client, err := lsp.NewClient(serverAddress)
		if err != nil {
			for _, c := range clients {
				c.Close()
			}
			return nil, xerrors.Errorf("failed to create LSP client for %s: %w", serverAddress, err)
		}

		if _, err := client.Initialize(workspaceURI); err != nil {
			client.Close()
			for _, c := range clients {
				c.Close()
			}
			return nil, xerrors.Errorf("failed to initialize LSP client for %s: %w", serverAddress, err)
		}

		clients = append(clients, client)
	}

	return &Handler{
		clients: clients,
	}, nil
}

func (h *Handler) GetServerInfo() mcp.ServerInfo {
	return mcp.ServerInfo{
		Name:    "lsp",
		Version: "1.0.0",
	}
}

func (h *Handler) GetTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "workspace_symbol",
			Description: "Search for symbols in the workspace",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"query": {
						Type:        "string",
						Description: "Search query for symbols",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "text_document_document_symbol",
			Description: "Get symbols from a specific document",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"uri": {
						Type:        "string",
						Description: "File URI (e.g., file:///path/to/file.go)",
					},
				},
				Required: []string{"uri"},
			},
		},
		{
			Name:        "get_code_from_range",
			Description: "Extract code from a file given a range",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"uri": {
						Type:        "string",
						Description: "File URI",
					},
					"start_line": {
						Type:        "integer",
						Description: "Start line (0-indexed)",
					},
					"start_character": {
						Type:        "integer",
						Description: "Start character position",
					},
					"end_line": {
						Type:        "integer",
						Description: "End line (0-indexed)",
					},
					"end_character": {
						Type:        "integer",
						Description: "End character position",
					},
				},
				Required: []string{"uri", "start_line", "start_character", "end_line", "end_character"},
			},
		},
	}
}

func (h *Handler) CallTool(name string, arguments json.RawMessage) (mcp.ToolCallResult, error) {
	switch name {
	case "workspace_symbol":
		return h.workspaceSymbol(arguments)
	case "text_document_document_symbol":
		return h.textDocumentDocumentSymbol(arguments)
	case "get_code_from_range":
		return h.getCodeFromRange(arguments)
	default:
		return mcp.ToolCallResult{}, xerrors.Errorf("unknown tool: %s", name)
	}
}

func (h *Handler) workspaceSymbol(arguments json.RawMessage) (mcp.ToolCallResult, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
	}

	type Response struct {
		Query   string           `json:"query"`
		Servers []ServerResponse `json:"servers"`
	}

	response := Response{
		Query:   args.Query,
		Servers: make([]ServerResponse, 0, len(h.clients)),
	}

	for i, client := range h.clients {
		symbols, err := client.WorkspaceSymbol(args.Query)
		serverResponse := ServerResponse{
			ServerIndex: i + 1,
		}

		if err == nil {
			serverResponse.Success = true
			serverResponse.Symbols = symbols
		} else {
			serverResponse.Success = false
			serverResponse.Error = err.Error()
		}

		response.Servers = append(response.Servers, serverResponse)
	}

	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("failed to marshal response: %w", err)
	}

	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type: "text",
				Text: string(jsonData),
			},
		},
	}, nil
}

func (h *Handler) textDocumentDocumentSymbol(arguments json.RawMessage) (mcp.ToolCallResult, error) {
	var args struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
	}

	type Response struct {
		URI     string           `json:"uri"`
		Servers []ServerResponse `json:"servers"`
	}

	response := Response{
		URI:     args.URI,
		Servers: make([]ServerResponse, 0, len(h.clients)),
	}

	for i, client := range h.clients {
		symbols, err := client.TextDocumentDocumentSymbol(args.URI)
		serverResp := ServerResponse{
			ServerIndex: i + 1,
		}

		if err == nil {
			serverResp.Success = true
			serverResp.Symbols = symbols
		} else {
			serverResp.Success = false
			serverResp.Error = err.Error()
		}

		response.Servers = append(response.Servers, serverResp)
	}

	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("failed to marshal response: %w", err)
	}

	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type: "text",
				Text: string(jsonData),
			},
		},
	}, nil
}

func (h *Handler) getCodeFromRange(arguments json.RawMessage) (mcp.ToolCallResult, error) {
	var args struct {
		URI            string `json:"uri"`
		StartLine      int    `json:"start_line"`
		StartCharacter int    `json:"start_character"`
		EndLine        int    `json:"end_line"`
		EndCharacter   int    `json:"end_character"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
	}

	codeRange := lsp.Range{
		Start: lsp.Position{
			Line:      args.StartLine,
			Character: args.StartCharacter,
		},
		End: lsp.Position{
			Line:      args.EndLine,
			Character: args.EndCharacter,
		},
	}

	code, err := lsp.GetCodeFromRange(args.URI, codeRange)
	if err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("failed to get code from range: %w", err)
	}

	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type: "text",
				Text: code,
			},
		},
	}, nil
}

func (h *Handler) Close() error {
	for _, client := range h.clients {
		if err := client.Close(); err != nil {
			return err
		}
	}
	return nil
}

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	handler, err := NewHandler(a.LSPServers)
	if err != nil {
		return xerrors.Errorf("failed to create handler: %w", err)
	}
	defer handler.Close()

	server := mcp.NewServer(handler)

	return server.Run()
}
