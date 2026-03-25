package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/uuid"
)

var (
	sink                   string
	oidcTokenPath          string
	defaultSource          string
	port                   int
	terminationGracePeriod time.Duration
	lameduck               time.Duration
)

type oidcTransport struct {
	base      http.RoundTripper
	tokenPath string
}

func (t *oidcTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := os.ReadFile(t.tokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read OIDC token from %s: %w", t.tokenPath, err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
	return t.base.RoundTrip(req)
}

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

func main() {
	flag.StringVar(&sink, "sink", envOrDefaultValue("K_SINK", ""), "Sink URL to send CloudEvents to (K_SINK)")
	flag.StringVar(&oidcTokenPath, "oidc-token-path", envOrDefaultValue("K_OIDC_TOKEN_PATH", ""), "Path to OIDC token file for broker authentication (K_OIDC_TOKEN_PATH)")
	flag.StringVar(&defaultSource, "default-source", envOrDefaultValue("DEFAULT_SOURCE", "cloudevents-ingress"), "Default CloudEvent source")
	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "The duration the application needs to terminate gracefully")
	flag.IntVar(&port, "port", envOrDefaultValue("PORT", 8080), "")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.Parse()

	if sink == "" {
		log.Fatal("--sink or K_SINK is required")
	}

	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = http.DefaultTransport.(*http.Transport).MaxIdleConns

	ceOpts := []cehttp.Option{cehttp.WithTarget(sink)}
	if oidcTokenPath != "" {
		ceOpts = append(ceOpts, cehttp.WithRoundTripper(&oidcTransport{
			base:      http.DefaultTransport,
			tokenPath: oidcTokenPath,
		}))
	}

	sender, err := cloudevents.NewClientHTTP(ceOpts...)
	if err != nil {
		log.Fatalf("failed to create CloudEvents client: %+v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		event := cloudevents.NewEvent()
		event.SetID(headerOrDefault(r, "Ce-Id", uuid.New().String()))
		event.SetType(headerOrDefault(r, "Ce-Type", r.Method))
		event.SetSource(headerOrDefault(r, "Ce-Source", defaultSource))
		event.SetSubject(headerOrDefault(r, "Ce-Subject", r.URL.Path))

		contentType := headerOrDefault(r, "Content-Type", "application/json")
		if err := event.SetData(contentType, body); err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		result := sender.Send(r.Context(), event)
		if cloudevents.IsUndelivered(result) {
			http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
	})

	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %+v", err)
	}

	server := &http.Server{Handler: mux}

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

func headerOrDefault(r *http.Request, key string, defaultValue string) string {
	if v := r.Header.Get(key); v != "" {
		return v
	}
	return defaultValue
}
