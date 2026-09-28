package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"

	"alerthandler/handler"

	"github.com/google/go-github/v68/github"
	"golang.org/x/net/netutil"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

func main() {
	var address string
	var keepAlived bool
	var maxConnections int
	var grafanaAlertsURL string

	flag.StringVar(&address, "address", envOrDefaultValue("ADDRESS", "0.0.0.0:8080"), "HTTP server address")
	flag.BoolVar(&keepAlived, "enable-keep-alive", envOrDefaultValue("ENABLE_KEEP_ALIVE", true), "Enable HTTP keep-alive")
	flag.IntVar(&maxConnections, "max-connections", envOrDefaultValue("MAX_CONNECTIONS", 65536), "Maximum number of concurrent connections")
	flag.StringVar(&grafanaAlertsURL, "grafana-alerts-url", envOrDefaultValue("GRAFANA_ALERTS_URL", ""), "Grafana alert groups URL rendered in the issue body")

	flag.Parse()

	configuration, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to get in-cluster config: %+v", err)
	}
	client, err := kubernetes.NewForConfig(configuration)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %+v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(configuration)
	if err != nil {
		log.Fatalf("Failed to create dynamic client: %+v", err)
	}

	gitHubClient := github.NewClient(nil).WithAuthToken(os.Getenv("GITHUB_TOKEN"))
	dispatcher := handler.NewDispatcher(client, dynamicClient, gitHubClient, grafanaAlertsURL)

	router := http.NewServeMux()
	router.HandleFunc("/", func(responseWriter http.ResponseWriter, request *http.Request) {
		bytes, err := io.ReadAll(request.Body)
		if err != nil {
			log.Printf("%+v", err)
			http.Error(responseWriter, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		var alertManagerRequest handler.AlertManagerRequest
		if err := json.Unmarshal(bytes, &alertManagerRequest); err != nil {
			log.Printf("%+v", err)
			http.Error(responseWriter, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if err := dispatcher.Handle(&alertManagerRequest); err != nil {
			log.Printf("%+v", err)
			http.Error(responseWriter, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		responseWriter.Header().Set("Content-Type", "text/plain")
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(http.StatusText(http.StatusOK)))
	})

	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Failed to create listener %s: %+v", address, err)
	}

	server := &http.Server{
		Handler: router,
	}
	server.SetKeepAlivesEnabled(keepAlived)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v", err)
				log.Printf("%s", debug.Stack())
			}
		}()
		if err := server.Serve(netutil.LimitListener(listener, maxConnections)); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to listen: %+v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	<-quit
	log.Printf("Attempt to shutdown instance...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Failed to shutdown: %+v", err)
	}
	select {
	case <-ctx.Done():
		log.Printf("Server shutdown timed out in 10 seconds")
	default:
	}
	log.Printf("Server has been shutdown")
}
