package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"
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

func newReverseProxy(remoteAddress string) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetXForwarded()
			r.Out.URL.Scheme = "http"
			r.Out.URL.Host = remoteAddress
			// Chromium answers 500 unless Host is an IP address or localhost, and 403 to a WebSocket handshake carrying an Origin
			r.Out.Host = remoteAddress
			r.Out.Header.Del("Origin")
		},
		ModifyResponse: func(response *http.Response) error {
			if response.StatusCode != http.StatusOK {
				return nil
			}

			body, err := io.ReadAll(response.Body)
			if err != nil {
				return err
			}
			if err := response.Body.Close(); err != nil {
				return err
			}

			// /json/* builds webSocketDebuggerUrl and devtoolsFrontendUrl out of the Host it received
			body = bytes.ReplaceAll(body, []byte(remoteAddress), []byte(response.Request.Header.Get("X-Forwarded-Host")))

			response.Body = io.NopCloser(bytes.NewReader(body))
			response.ContentLength = int64(len(body))
			response.Header.Set("Content-Length", strconv.Itoa(len(body)))

			return nil
		},
	}
}

func main() {
	var localAddress string
	var remoteAddress string
	var terminationGracePeriod time.Duration
	var lameduck time.Duration
	var keepAlive bool

	flag.StringVar(&localAddress, "local-address", envOrDefaultValue("LOCAL_ADDRESS", "0.0.0.0:59223"), "Local listen address")
	flag.StringVar(&remoteAddress, "remote-address", envOrDefaultValue("REMOTE_ADDRESS", "127.0.0.1:9222"), "Remote Chrome DevTools Protocol address")
	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "The duration the application needs to terminate gracefully")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.BoolVar(&keepAlive, "http-keepalive", envOrDefaultValue("HTTP_KEEPALIVE", true), "Enable HTTP keep-alive")
	flag.Parse()

	listener, err := net.Listen("tcp", localAddress)
	if err != nil {
		log.Fatalf("failed to listen: %+v", err)
	}

	server := &http.Server{
		Handler: newReverseProxy(remoteAddress),
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
