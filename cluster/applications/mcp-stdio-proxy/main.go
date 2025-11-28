package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

func envOrDefaultValue[T any](key string, defaultValue T) T {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}

	switch any(defaultValue).(type) {
	case string:
		return any(value).(T)
	case int:
		if intValue, err := strconv.Atoi(value); err == nil {
			return any(intValue).(T)
		}
	case int64:
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return any(intValue).(T)
		}
	case uint:
		if uintValue, err := strconv.ParseUint(value, 10, 0); err == nil {
			return any(uint(uintValue)).(T)
		}
	case uint64:
		if uintValue, err := strconv.ParseUint(value, 10, 64); err == nil {
			return any(uintValue).(T)
		}
	case float64:
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return any(floatValue).(T)
		}
	case bool:
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return any(boolValue).(T)
		}
	case time.Duration:
		if durationValue, err := time.ParseDuration(value); err == nil {
			return any(durationValue).(T)
		}
	}

	return defaultValue
}

type MCPMethod string

// https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/schema/2025-03-26/schema.ts
const (
	CancelledNotification           MCPMethod = "notifications/cancelled"
	InitializeRequest                         = "initialize"
	InitializedNotification                   = "notifications/initialized"
	PingRequest                               = "ping"
	ProgressNotification                      = "notifications/progress"
	ListResourcesRequest                      = "resources/list"
	ListResourceTemplatesRequest              = "resources/templates/list"
	ReadResourceRequest                       = "resources/read"
	ResourceListChangedNotification           = "notifications/resource/list/changed"
	SubscribeRequest                          = "resources/subscribe"
	UnsubscribeRequest                        = "resources/unsubscribe"
	ResourceUpdatedNotification               = "notifications/resource/updated"
	ListPromptsRequest                        = "prompts/list"
	GetPromptRequest                          = "prompts/get"
	PromptListChangedNotification             = "notifications/prompt/list/changed"
	ListToolsRequest                          = "tools/list"
	CallToolRequest                           = "tools/call"
	ToolListChangedNotification               = "notifications/tool/list/changed"
)

type MCPMessage struct {
	JSONRPC string    `json:"jsonrpc"`
	Method  MCPMethod `json:"method"`
	ID      any       `json:"id,omitempty"`
}

type session struct {
	id            string
	responseQueue chan []byte
	requestQueue  chan []byte
}

// https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/specification/2024-11-05/basic/transports.mdx#http-with-sse
func main() {
	var address string
	var terminationGracePeriod time.Duration
	var lameduck time.Duration
	var keepAlive bool
	var verbose bool
	flag.StringVar(&address, "address", envOrDefaultValue("ADDRESS", "0.0.0.0:8080"), "")
	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "The duration the application needs to terminate gracefully")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.BoolVar(&keepAlive, "http-keepalive", envOrDefaultValue("HTTP_KEEPALIVE", true), "")
	flag.BoolVar(&verbose, "verbose", envOrDefaultValue("VERBOSE", false), "")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		log.Fatalf("command not specified")
	}
	name := args[0]
	var arg []string
	if len(args) > 1 {
		arg = args[1:]
	}

	sessions := &sync.Map{}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		cmd := exec.Command(name, arg...)

		stdin, err := cmd.StdinPipe()
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if err := cmd.Start(); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		s := &session{
			id:            uuid.New().String(),
			requestQueue:  make(chan []byte, 65536),
			responseQueue: make(chan []byte, 65536),
		}

		sessions.Store(s.id, s)

		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				s.responseQueue <- scanner.Bytes()
			}
		}()

		defer func() {
			sessions.Delete(s.id)
			_ = cmd.Process.Kill()
			_ = cmd.Wait()

			close(s.requestQueue)
			close(s.responseQueue)
		}()

		_, _ = fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", fmt.Sprintf("http://%s/messages?sessionId=%s", r.Host, s.id))
		flusher.Flush()

		for {
			select {
			case data := <-s.requestQueue:
				_, _ = stdin.Write(data)
			case data := <-s.responseQueue:
				_, _ = fmt.Fprint(w, fmt.Sprintf("event: message\ndata: %s\n\n", data))
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})

	mux.HandleFunc("POST /messages", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("sessionId")
		if id == "" {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		sany, ok := sessions.Load(id)
		if !ok {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		s := sany.(*session)

		var rawMessage json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&rawMessage); err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		var mcpMessage MCPMessage
		if err := json.Unmarshal(rawMessage, &mcpMessage); err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if verbose {
			log.Printf("%s: %s", id, mcpMessage.Method)
		}

		s.requestQueue <- rawMessage
		s.requestQueue <- []byte("\n")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(http.StatusText(http.StatusAccepted)))
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
	})

	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen: %+v", err)
	}

	server := &http.Server{
		Handler: mux,
	}
	server.SetKeepAlivesEnabled(keepAlive)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v\n%s", err, debug.Stack())
			}
		}()
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("failed to listen: %+v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	<-quit
	time.Sleep(lameduck)

	ctx, cancel := context.WithTimeout(context.Background(), terminationGracePeriod)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("failed to shutdown: %+v", err)
	}
}
