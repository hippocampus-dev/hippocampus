package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"sync"
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

type statusRecorder struct {
	statusCode int
	err        error
}

func (r *statusRecorder) Header() http.Header         { return http.Header{} }
func (r *statusRecorder) Write(b []byte) (int, error) { return len(b), nil }
func (r *statusRecorder) WriteHeader(code int)        { r.statusCode = code }

type PodResult struct {
	Pod     string `json:"pod"`
	Status  int    `json:"status"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type BroadcastResponse struct {
	Total   int         `json:"total"`
	Success int         `json:"success"`
	Failed  int         `json:"failed"`
	Results []PodResult `json:"results"`
}

func main() {
	var address string
	var terminationGracePeriod time.Duration
	var lameduck time.Duration
	var keepAlive bool
	var targetService string
	var targetPort int

	flag.StringVar(&address, "address", envOrDefaultValue("ADDRESS", "0.0.0.0:8080"), "")
	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "The duration the application needs to terminate gracefully")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.BoolVar(&keepAlive, "http-keepalive", envOrDefaultValue("HTTP_KEEPALIVE", true), "")
	flag.StringVar(&targetService, "target-service", envOrDefaultValue("TARGET_SERVICE", ""), "Headless service FQDN to broadcast to")
	flag.IntVar(&targetPort, "target-port", envOrDefaultValue("TARGET_PORT", 80), "Target port on each pod")
	flag.Parse()

	if targetService == "" {
		log.Fatal("--target-service or TARGET_SERVICE is required")
	}

	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = http.DefaultTransport.(*http.Transport).MaxIdleConns

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ips, err := net.DefaultResolver.LookupHost(r.Context(), targetService)
		if err != nil {
			log.Printf("DNS lookup failed for %s: %v", targetService, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "service discovery failed",
			})
			return
		}

		if len(ips) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "no pods found for service",
			})
			return
		}

		results := make([]PodResult, len(ips))
		wg := &sync.WaitGroup{}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "failed to read request body",
			})
			return
		}

		for i, ip := range ips {
			wg.Go(func() {
				target, err := url.Parse(fmt.Sprintf("http://%s:%d", ip, targetPort))
				if err != nil {
					results[i] = PodResult{
						Pod:     ip,
						Success: false,
						Error:   fmt.Sprintf("failed to parse target URL: %v", err),
					}
					return
				}

				proxy := httputil.NewSingleHostReverseProxy(target)
				originalDirector := proxy.Director
				proxy.Director = func(request *http.Request) {
					originalDirector(request)
					request.Host = targetService
				}

				recorder := &statusRecorder{}
				proxy.ErrorHandler = func(_ http.ResponseWriter, _ *http.Request, err error) {
					recorder.err = err
				}

				request := r.Clone(r.Context())
				request.Body = io.NopCloser(bytes.NewReader(body))
				request.ContentLength = int64(len(body))

				proxy.ServeHTTP(recorder, request)

				if recorder.err != nil {
					results[i] = PodResult{
						Pod:     ip,
						Success: false,
						Error:   recorder.err.Error(),
					}
				} else {
					results[i] = PodResult{
						Pod:     ip,
						Status:  recorder.statusCode,
						Success: recorder.statusCode > 0 && recorder.statusCode < 400,
					}
				}
			})
		}

		wg.Wait()

		successCount := 0
		for _, result := range results {
			if result.Success {
				successCount++
			}
		}

		response := BroadcastResponse{
			Total:   len(results),
			Success: successCount,
			Failed:  len(results) - successCount,
			Results: results,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
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
