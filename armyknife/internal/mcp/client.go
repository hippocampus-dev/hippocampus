package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/xerrors"
)

func CallToolSSE(sseURL string, name string, arguments map[string]interface{}) (ToolCallResult, error) {
	request, err := http.NewRequest(http.MethodGet, sseURL, nil)
	if err != nil {
		return ToolCallResult{}, xerrors.Errorf("failed to create request: %w", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return ToolCallResult{}, xerrors.Errorf("failed to open sse %s: %w", sseURL, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ToolCallResult{}, xerrors.Errorf("sse %s returned %s", sseURL, response.Status)
	}

	reader := bufio.NewReader(response.Body)

	endpoint, err := readEndpoint(reader)
	if err != nil {
		return ToolCallResult{}, err
	}
	messagesURL, err := resolveEndpoint(sseURL, endpoint)
	if err != nil {
		return ToolCallResult{}, err
	}

	argumentBytes, err := json.Marshal(arguments)
	if err != nil {
		return ToolCallResult{}, xerrors.Errorf("failed to marshal arguments: %w", err)
	}
	paramBytes, err := json.Marshal(ToolCallParams{Name: name, Arguments: argumentBytes})
	if err != nil {
		return ToolCallResult{}, xerrors.Errorf("failed to marshal params: %w", err)
	}
	requestBytes, err := json.Marshal(JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: paramBytes})
	if err != nil {
		return ToolCallResult{}, xerrors.Errorf("failed to marshal request: %w", err)
	}

	postRequest, err := http.NewRequest(http.MethodPost, messagesURL, bytes.NewReader(requestBytes))
	if err != nil {
		return ToolCallResult{}, xerrors.Errorf("failed to create request: %w", err)
	}
	postRequest.Header.Set("Content-Type", "application/json")
	post, err := http.DefaultClient.Do(postRequest)
	if err != nil {
		return ToolCallResult{}, xerrors.Errorf("failed to post message: %w", err)
	}
	post.Body.Close()
	if post.StatusCode != http.StatusAccepted {
		return ToolCallResult{}, xerrors.Errorf("messages endpoint returned %s", post.Status)
	}

	for {
		event, data, err := readEvent(reader)
		if err != nil {
			return ToolCallResult{}, err
		}
		if event != "message" {
			continue
		}

		var jsonResponse JSONRPCResponse
		if err := json.Unmarshal([]byte(data), &jsonResponse); err != nil {
			return ToolCallResult{}, xerrors.Errorf("failed to parse response: %w", err)
		}
		if jsonResponse.Error != nil {
			return ToolCallResult{}, xerrors.Errorf("tool error: %s", jsonResponse.Error.Message)
		}

		resultBytes, err := json.Marshal(jsonResponse.Result)
		if err != nil {
			return ToolCallResult{}, xerrors.Errorf("failed to re-marshal result: %w", err)
		}
		var result ToolCallResult
		if err := json.Unmarshal(resultBytes, &result); err != nil {
			return ToolCallResult{}, xerrors.Errorf("failed to parse result: %w", err)
		}
		return result, nil
	}
}

func readEvent(reader *bufio.Reader) (string, string, error) {
	var event, data string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", "", xerrors.Errorf("failed to read sse stream: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			return event, data, nil
		}
		if value, ok := strings.CutPrefix(line, "event:"); ok {
			event = strings.TrimSpace(value)
		} else if value, ok := strings.CutPrefix(line, "data:"); ok {
			data = strings.TrimSpace(value)
		}
	}
}

func readEndpoint(reader *bufio.Reader) (string, error) {
	for {
		event, data, err := readEvent(reader)
		if err != nil {
			return "", err
		}
		if event == "endpoint" {
			return data, nil
		}
	}
}

func resolveEndpoint(sseURL string, endpoint string) (string, error) {
	base, err := url.Parse(sseURL)
	if err != nil {
		return "", xerrors.Errorf("failed to parse sse url: %w", err)
	}
	reference, err := url.Parse(endpoint)
	if err != nil {
		return "", xerrors.Errorf("failed to parse endpoint: %w", err)
	}
	return base.ResolveReference(reference).String(), nil
}
