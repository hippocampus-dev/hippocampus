package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"
)

const prefix = "/url_shortener"

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
	var address string
	var httpKVSURL string
	var terminationGracePeriod time.Duration
	var lameduck time.Duration
	var keepAlive bool
	flag.StringVar(&address, "address", envOrDefaultValue("ADDRESS", "0.0.0.0:8080"), "")
	flag.StringVar(&httpKVSURL, "http-kvs-url", envOrDefaultValue("HTTP_KVS_URL", "https://http-kvs.minikube.127.0.0.1.nip.io"), "")

	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "The duration the application needs to terminate gracefully")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.BoolVar(&keepAlive, "http-keepalive", envOrDefaultValue("HTTP_KEEPALIVE", true), "")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			response, err := http.DefaultClient.Get(httpKVSURL + path.Join(prefix, r.RequestURI))
			if err != nil {
				log.Printf("%+v", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			defer func() {
				_ = response.Body.Close()
			}()
			b, err := io.ReadAll(response.Body)
			if err != nil {
				log.Printf("%+v", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			if response.StatusCode != http.StatusOK {
				http.Error(w, http.StatusText(response.StatusCode), response.StatusCode)
				return
			}

			url := string(b)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.Redirect(w, r, url, http.StatusMovedPermanently)
		case http.MethodPost:
			defer func() {
				_ = r.Body.Close()
			}()
			url, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("%+v", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			shorten := fmt.Sprintf("%x", sha256.Sum256(url))

			response, err := http.DefaultClient.Post(httpKVSURL+path.Join(prefix, shorten), "text/plain", bytes.NewReader(url))
			if err != nil {
				log.Printf("%+v", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			defer func() {
				_ = response.Body.Close()
			}()

			if response.StatusCode != http.StatusOK {
				http.Error(w, http.StatusText(response.StatusCode), response.StatusCode)
				return
			}

			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(shorten))
		default:
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}
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
