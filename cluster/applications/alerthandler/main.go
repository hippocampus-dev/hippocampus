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
	"syscall"
	"time"

	"alerthandler/handler"

	"github.com/google/go-github/v68/github"
	"golang.org/x/net/netutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	address := flag.String("address", "0.0.0.0:8080", "")
	keepAlived := flag.Bool("enable-keep-alive", true, "")
	maxConnections := flag.Int("max-connections", 65536, "")

	flag.Parse()

	configuration, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to get in-cluster config: %+v", err)
	}
	client, err := kubernetes.NewForConfig(configuration)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %+v", err)
	}

	gitHubClient := github.NewClient(nil).WithAuthToken(os.Getenv("GITHUB_TOKEN"))
	dispatcher := handler.NewDispatcher(client, gitHubClient)

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

	listener, err := net.Listen("tcp", *address)
	if err != nil {
		log.Fatalf("Failed to create listener %s: %+v", *address, err)
	}

	server := &http.Server{
		Handler: router,
	}
	server.SetKeepAlivesEnabled(*keepAlived)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v", err)
				log.Printf("%s", debug.Stack())
			}
		}()
		if err := server.Serve(netutil.LimitListener(listener, *maxConnections)); err != nil && err != http.ErrServerClosed {
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
