package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"os"
	"strconv"

	"golang.org/x/xerrors"
)

type Client struct {
	connection net.Conn
	reader     *bufio.Reader
	writer     *bufio.Writer
	requestID  int64
}

func NewClient(address string) (*Client, error) {
	connection, err := net.Dial("tcp", address)
	if err != nil {
		return nil, xerrors.Errorf("failed to connect to LSP server: %w", err)
	}

	return &Client{
		connection: connection,
		reader:     bufio.NewReader(connection),
		writer:     bufio.NewWriter(connection),
	}, nil
}

func (c *Client) Close() error {
	return c.connection.Close()
}

func (c *Client) readResponse() (*Response, error) {
	tp := textproto.NewReader(c.reader)

	headers, err := tp.ReadMIMEHeader()
	if err != nil {
		return nil, xerrors.Errorf("failed to read headers: %w", err)
	}

	const maxContentLength = 100 * 1024 * 1024

	lengthString := headers.Get("Content-Length")
	if lengthString == "" {
		return nil, xerrors.Errorf("missing Content-Length header")
	}

	contentLength, err := strconv.Atoi(lengthString)
	if err != nil {
		return nil, xerrors.Errorf("invalid Content-Length header: %s", lengthString)
	}

	if contentLength <= 0 {
		return nil, xerrors.Errorf("invalid Content-Length: %d", contentLength)
	}

	if contentLength > maxContentLength {
		return nil, xerrors.Errorf("content length too large: %d bytes (max: %d bytes)", contentLength, maxContentLength)
	}

	content := make([]byte, contentLength)
	if _, err := io.ReadFull(c.reader, content); err != nil {
		return nil, xerrors.Errorf("failed to read content: %w", err)
	}

	var response Response
	if err := json.Unmarshal(content, &response); err != nil {
		return nil, xerrors.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

func (c *Client) sendRequest(method string, params interface{}) (*Response, error) {
	c.requestID++
	requestID := c.requestID

	request := Request{
		JSONRPC: "2.0",
		ID:      &requestID,
		Method:  method,
		Params:  params,
	}

	content, err := json.Marshal(request)
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal request: %w", err)
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(content))
	if _, err := c.writer.WriteString(header); err != nil {
		return nil, xerrors.Errorf("failed to write header: %w", err)
	}
	if _, err := c.writer.Write(content); err != nil {
		return nil, xerrors.Errorf("failed to write content: %w", err)
	}
	if err := c.writer.Flush(); err != nil {
		return nil, xerrors.Errorf("failed to flush writer: %w", err)
	}

	for {
		response, err := c.readResponse()
		if err != nil {
			return nil, err
		}
		if response.ID == nil {
			continue
		}
		if *response.ID == requestID {
			return response, nil
		}
	}
}

func (c *Client) sendNotification(method string, params interface{}) error {
	request := Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	content, err := json.Marshal(request)
	if err != nil {
		return xerrors.Errorf("failed to marshal notification: %w", err)
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(content))
	if _, err := c.writer.WriteString(header); err != nil {
		return xerrors.Errorf("failed to write header: %w", err)
	}
	if _, err := c.writer.Write(content); err != nil {
		return xerrors.Errorf("failed to write content: %w", err)
	}
	if err := c.writer.Flush(); err != nil {
		return xerrors.Errorf("failed to flush writer: %w", err)
	}

	return nil
}

func (c *Client) Initialize(rootURI string) (*InitializeResult, error) {
	processID := os.Getpid()
	params := InitializeParams{
		ProcessID: &processID,
		RootURI:   &rootURI,
		Capabilities: ClientCapabilities{
			Workspace: WorkspaceClientCapabilities{
				Symbol: WorkspaceSymbolClientCapabilities{
					DynamicRegistration: false,
				},
			},
			TextDocument: TextDocumentClientCapabilities{
				DocumentSymbol: DocumentSymbolClientCapabilities{
					DynamicRegistration:               false,
					HierarchicalDocumentSymbolSupport: false,
				},
			},
		},
	}

	response, err := c.sendRequest("initialize", params)
	if err != nil {
		return nil, xerrors.Errorf("failed to send initialize request: %w", err)
	}

	if response.Error != nil {
		return nil, xerrors.Errorf("initialize error: %s", response.Error.Message)
	}

	var result InitializeResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, xerrors.Errorf("failed to unmarshal initialize result: %w", err)
	}

	if err := c.sendNotification("initialized", struct{}{}); err != nil {
		return nil, xerrors.Errorf("failed to send initialized notification: %w", err)
	}

	return &result, nil
}

func (c *Client) DidOpen(documentURI string, languageID string, text string) error {
	params := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        documentURI,
			LanguageID: languageID,
			Version:    1,
			Text:       text,
		},
	}
	return c.sendNotification("textDocument/didOpen", params)
}

func (c *Client) DidClose(documentURI string) error {
	params := DidCloseTextDocumentParams{
		TextDocument: TextDocumentIdentifier{
			URI: documentURI,
		},
	}
	return c.sendNotification("textDocument/didClose", params)
}

func (c *Client) WorkspaceSymbol(query string) ([]SymbolInformation, error) {
	params := WorkspaceSymbolParams{
		Query: query,
	}

	response, err := c.sendRequest("workspace/symbol", params)
	if err != nil {
		return nil, xerrors.Errorf("failed to send workspace/symbol request: %w", err)
	}

	if response.Error != nil {
		return nil, xerrors.Errorf("workspace/symbol error: %s", response.Error.Message)
	}

	var symbols []SymbolInformation
	if err := json.Unmarshal(response.Result, &symbols); err != nil {
		return nil, xerrors.Errorf("failed to unmarshal workspace symbols: %w", err)
	}

	return symbols, nil
}

func (c *Client) TextDocumentDocumentSymbol(documentURI string) ([]SymbolInformation, error) {
	params := DocumentSymbolParams{
		TextDocument: TextDocumentIdentifier{
			URI: documentURI,
		},
	}

	response, err := c.sendRequest("textDocument/documentSymbol", params)
	if err != nil {
		return nil, xerrors.Errorf("failed to send textDocument/documentSymbol request: %w", err)
	}

	if response.Error != nil {
		return nil, xerrors.Errorf("textDocument/documentSymbol error: %s", response.Error.Message)
	}

	if response.Result == nil || string(response.Result) == "null" {
		return nil, nil
	}

	var symbols []SymbolInformation
	if err := json.Unmarshal(response.Result, &symbols); err != nil {
		return nil, xerrors.Errorf("failed to unmarshal document symbols: %w", err)
	}

	return symbols, nil
}

func (c *Client) Shutdown() error {
	response, err := c.sendRequest("shutdown", nil)
	if err != nil {
		return xerrors.Errorf("failed to send shutdown request: %w", err)
	}

	if response.Error != nil {
		return xerrors.Errorf("shutdown error: %s", response.Error.Message)
	}

	if err := c.sendNotification("exit", nil); err != nil {
		return xerrors.Errorf("failed to send exit notification: %w", err)
	}

	return nil
}
