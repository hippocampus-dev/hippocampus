package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"spotify-mcp-server/internal/spotify"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"
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

type contextKey string

const (
	spotifyTokenKey        contextKey = "spotifyToken"
	spotifyUnauthorizedKey contextKey = "spotifyUnauthorized"
)

const defaultAuthorizationServer = "https://hippocampus-dev-spotify-oauth-bridge-public.kaidotio.dev"

type challengeWriter struct {
	http.ResponseWriter
	challenge    string
	unauthorized *atomic.Bool
	discarded    bool
}

func (c *challengeWriter) WriteHeader(statusCode int) {
	if c.unauthorized.Load() {
		c.discarded = true
		c.Header().Del("Content-Type")
		c.Header().Set("WWW-Authenticate", c.challenge)
		c.ResponseWriter.WriteHeader(http.StatusUnauthorized)
		return
	}
	c.ResponseWriter.WriteHeader(statusCode)
}

func (c *challengeWriter) Write(b []byte) (int, error) {
	if c.discarded {
		return len(b), nil
	}
	return c.ResponseWriter.Write(b)
}

// mcp-go answers a GET stream with 405 unless the ResponseWriter implements http.Flusher
func (c *challengeWriter) Flush() {
	if flusher, ok := c.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func main() {
	var (
		address                string
		baseURL                string
		authorizationServer    string
		terminationGracePeriod time.Duration
		lameduck               time.Duration
		keepAlive              bool
	)

	flag.StringVar(&address, "address", envOrDefaultValue("ADDRESS", "0.0.0.0:8080"), "HTTP server address")
	flag.StringVar(&baseURL, "base-url", envOrDefaultValue("BASE_URL", "http://127.0.0.1:8080"), "Public base URL of this server")
	flag.StringVar(&authorizationServer, "authorization-server", envOrDefaultValue("AUTHORIZATION_SERVER", defaultAuthorizationServer), "OAuth authorization server URL")
	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "Termination grace period")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "Lameduck period")
	flag.BoolVar(&keepAlive, "http-keepalive", envOrDefaultValue("HTTP_KEEPALIVE", true), "Enable HTTP keep-alive")
	flag.Parse()

	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = http.DefaultTransport.(*http.Transport).MaxIdleConns

	spotifyClient, err := spotify.NewClient(spotifyAPIBaseURL, &contextTokenSource{}, spotify.WithClient(&unauthorizedReporter{}))
	if err != nil {
		log.Fatalf("failed to create spotify client: %+v", err)
	}

	mcpServer := server.NewMCPServer(
		"spotify-mcp-server",
		"0.1.0",
		server.WithToolCapabilities(false),
	)

	tools(mcpServer, spotifyClient)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /.well-known/oauth-protected-resource", func(w http.ResponseWriter, r *http.Request) {
		metadata := map[string]interface{}{
			"resource":              baseURL,
			"authorization_servers": []string{authorizationServer},
			"scopes_supported": []string{
				"user-read-currently-playing",
				"user-read-recently-played",
				"user-top-read",
				"user-library-read",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metadata)
	})

	mux.Handle("/mcp", requireAuthorization(baseURL+"/.well-known/oauth-protected-resource", server.NewStreamableHTTPServer(mcpServer,
		server.WithEndpointPath("/mcp"),
	)))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
	})

	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen: %+v", err)
	}

	httpSrv := &http.Server{
		Handler: mux,
	}
	httpSrv.SetKeepAlivesEnabled(keepAlive)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v\n%s", err, debug.Stack())
			}
		}()
		if err := httpSrv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("failed to listen: %+v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	<-quit
	time.Sleep(lameduck)

	ctx, cancel := context.WithTimeout(context.Background(), terminationGracePeriod)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Fatalf("failed to shutdown: %+v", err)
	}
}

// https://www.rfc-editor.org/rfc/rfc9728#name-www-authenticate-response
func requireAuthorization(resourceMetadataURL string, next http.Handler) http.Handler {
	challenge := fmt.Sprintf("Bearer resource_metadata=%q", resourceMetadataURL)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			w.Header().Set("WWW-Authenticate", challenge)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		unauthorized := &atomic.Bool{}
		ctx := context.WithValue(r.Context(), spotifyTokenKey, parts[1])
		ctx = context.WithValue(ctx, spotifyUnauthorizedKey, unauthorized)

		next.ServeHTTP(&challengeWriter{ResponseWriter: w, challenge: challenge, unauthorized: unauthorized}, r.WithContext(ctx))
	})
}
