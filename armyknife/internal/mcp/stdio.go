package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"golang.org/x/xerrors"
)

func (s *Server) ServeStdio() error {
	scanner := bufio.NewScanner(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)

	for scanner.Scan() {
		line := scanner.Text()

		var request JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			if err := writeResponse(writer, JSONRPCResponse{
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

		if err := writeResponse(writer, s.dispatch(request)); err != nil {
			return xerrors.Errorf("failed to send response: %w", err)
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return xerrors.Errorf("scanner error: %w", err)
	}

	return nil
}

func writeResponse(writer *bufio.Writer, response JSONRPCResponse) error {
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return xerrors.Errorf("failed to marshal response: %w", err)
	}

	if _, err := writer.WriteString(string(responseBytes) + "\n"); err != nil {
		return xerrors.Errorf("failed to write response: %w", err)
	}

	if err := writer.Flush(); err != nil {
		return xerrors.Errorf("failed to flush writer: %w", err)
	}

	return nil
}
