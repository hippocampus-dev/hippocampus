package mcp

import (
	"encoding/json"

	"golang.org/x/xerrors"
)

type Handler interface {
	GetServerInfo() ServerInfo
	GetTools() []Tool
	CallTool(name string, arguments json.RawMessage) (ToolCallResult, error)
	GetPrompts() []Prompt
	GetPrompt(name string, arguments map[string]interface{}) (PromptResult, error)
}

type Server struct {
	handler Handler
}

func NewServer(handler Handler) *Server {
	return &Server{handler: handler}
}

func (s *Server) dispatch(request JSONRPCRequest) JSONRPCResponse {
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

	return response
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
