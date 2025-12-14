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
	"sync"
	"sync/atomic"

	"golang.org/x/xerrors"
)

type Client struct {
	connection net.Conn
	reader     *bufio.Reader
	writer     *bufio.Writer
	requestID  atomic.Int64
	pending    sync.Map
	done       chan struct{}
	closeOnce  sync.Once
}

func NewClient(address string) (*Client, error) {
	connection, err := net.Dial("tcp", address)
	if err != nil {
		return nil, xerrors.Errorf("failed to connect to LSP server: %w", err)
	}

	client := &Client{
		connection: connection,
		reader:     bufio.NewReader(connection),
		writer:     bufio.NewWriter(connection),
		done:       make(chan struct{}),
	}

	go client.readLoop()

	return client, nil
}

func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.done)

		c.pending.Range(func(key, value interface{}) bool {
			if channel, ok := value.(chan *Response); ok {
				select {
				case <-channel:
				default:
					close(channel)
				}
			}
			return true
		})

		err = c.connection.Close()
	})
	return err
}

func (c *Client) readLoop() {
	defer c.Close()

	for {
		select {
		case <-c.done:
			return
		default:
		}

		response, err := c.readResponse()
		if err != nil {
			return
		}

		if response.ID != nil {
			if value, ok := c.pending.LoadAndDelete(*response.ID); ok {
				if channel, ok := value.(chan *Response); ok {
					select {
					case channel <- response:
					case <-c.done:
						return
					}
				}
			}
		}
	}
}

func (c *Client) parseHeaders() (textproto.MIMEHeader, error) {
	tp := textproto.NewReader(c.reader)

	headers, err := tp.ReadMIMEHeader()
	if err != nil {
		return nil, xerrors.Errorf("failed to read headers: %w", err)
	}

	return headers, nil
}

func (c *Client) readResponse() (*Response, error) {
	headers, err := c.parseHeaders()
	if err != nil {
		return nil, xerrors.Errorf("failed to parse headers: %w", err)
	}

	const maxContentLength = 10 * 1024 * 1024

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
	requestID := c.requestID.Add(1)
	request := Request{
		JSONRPC: "2.0",
		ID:      &requestID,
		Method:  method,
		Params:  params,
	}

	channel := make(chan *Response, 1)
	c.pending.Store(requestID, channel)

	content, err := json.Marshal(request)
	if err != nil {
		c.pending.Delete(requestID)
		return nil, xerrors.Errorf("failed to marshal request: %w", err)
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(content))
	if _, err := c.writer.WriteString(header); err != nil {
		c.pending.Delete(requestID)
		return nil, xerrors.Errorf("failed to write header: %w", err)
	}
	if _, err := c.writer.Write(content); err != nil {
		c.pending.Delete(requestID)
		return nil, xerrors.Errorf("failed to write content: %w", err)
	}
	if err := c.writer.Flush(); err != nil {
		c.pending.Delete(requestID)
		return nil, xerrors.Errorf("failed to flush writer: %w", err)
	}

	select {
	case response := <-channel:
		return response, nil
	case <-c.done:
		return nil, xerrors.Errorf("client closed")
	}
}

func (c *Client) sendNotification(method string, params interface{}) error {
	select {
	case <-c.done:
		return xerrors.Errorf("client closed")
	default:
	}

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
					DynamicRegistration: false,
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
