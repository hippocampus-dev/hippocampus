package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"golang.org/x/xerrors"
)

// Handler defines the interface for MCP server implementations
type Handler interface {
	// GetServerInfo returns server name and version
	GetServerInfo() ServerInfo

	// GetTools returns the list of tools this server provides
	GetTools() []Tool

	// CallTool executes a tool and returns the result
	CallTool(name string, arguments json.RawMessage) (ToolCallResult, error)

	// GetPrompts returns the list of prompts this server provides
	GetPrompts() []Prompt

	// GetPrompt returns a specific prompt with the given arguments
	GetPrompt(name string, arguments map[string]interface{}) (PromptResult, error)
}

// Server represents an MCP server that handles JSON-RPC communication
type Server struct {
	handler Handler
	scanner *bufio.Scanner
	writer  *bufio.Writer
}

// NewServer creates a new MCP server with the given handler
func NewServer(handler Handler) *Server {
	return &Server{
		handler: handler,
		scanner: bufio.NewScanner(os.Stdin),
		writer:  bufio.NewWriter(os.Stdout),
	}
}

// Run starts the MCP server and handles incoming requests
func (s *Server) Run() error {
	for s.scanner.Scan() {
		line := s.scanner.Text()

		var request JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			if err := s.sendResponse(JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &JSONError{
					Code:    ErrorCodeParseError,
					Message: fmt.Sprintf("Parse error: %s", err.Error()),
				},
			}); err != nil {
				return xerrors.Errorf("failed to send response: %w", err)
			}
			continue
		}

		if request.ID == nil {
			continue
		}

		response := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      request.ID,
		}

		switch request.Method {
		case "initialize":
			response.Result = s.handleInitialize()
		case "ping":
			response.Result = map[string]interface{}{}
		case "tools/list":
			response.Result = s.handleToolsList()
		case "tools/call":
			result, err := s.handleToolsCall(request.Params)
			if err != nil {
				response.Error = &JSONError{
					Code:    ErrorCodeInvalidParams,
					Message: err.Error(),
				}
			} else {
				response.Result = result
			}
		case "prompts/list":
			response.Result = s.handlePromptsList()
		case "prompts/get":
			result, err := s.handlePromptsGet(request.Params)
			if err != nil {
				response.Error = &JSONError{
					Code:    ErrorCodeInvalidParams,
					Message: err.Error(),
				}
			} else {
				response.Result = result
			}
		default:
			response.Error = &JSONError{
				Code:    ErrorCodeMethodNotFound,
				Message: "Method not found",
			}
		}

		if err := s.sendResponse(response); err != nil {
			return xerrors.Errorf("failed to send response: %w", err)
		}
	}

	if err := s.scanner.Err(); err != nil && err != io.EOF {
		return xerrors.Errorf("scanner error: %w", err)
	}

	return nil
}

func (s *Server) handleInitialize() InitializeResult {
	return InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: map[string]interface{}{
			"tools":     map[string]interface{}{},
			"resources": map[string]interface{}{},
			"prompts":   map[string]interface{}{},
		},
		ServerInfo: s.handler.GetServerInfo(),
	}
}

func (s *Server) handleToolsList() map[string]interface{} {
	return map[string]interface{}{
		"tools": s.handler.GetTools(),
	}
}

func (s *Server) handleToolsCall(params json.RawMessage) (interface{}, error) {
	var callParams ToolCallParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		return nil, xerrors.Errorf("invalid params: %w", err)
	}

	result, err := s.handler.CallTool(callParams.Name, callParams.Arguments)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Server) handlePromptsList() map[string]interface{} {
	return map[string]interface{}{
		"prompts": s.handler.GetPrompts(),
	}
}

func (s *Server) handlePromptsGet(params json.RawMessage) (interface{}, error) {
	var getParams PromptGetParams
	if err := json.Unmarshal(params, &getParams); err != nil {
		return nil, xerrors.Errorf("invalid params: %w", err)
	}

	result, err := s.handler.GetPrompt(getParams.Name, getParams.Arguments)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Server) sendResponse(response JSONRPCResponse) error {
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return xerrors.Errorf("failed to marshal response: %w", err)
	}

	if _, err := s.writer.WriteString(string(responseBytes) + "\n"); err != nil {
		return xerrors.Errorf("failed to write response: %w", err)
	}

	if err := s.writer.Flush(); err != nil {
		return xerrors.Errorf("failed to flush writer: %w", err)
	}

	return nil
}
