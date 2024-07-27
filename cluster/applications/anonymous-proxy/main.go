package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"
)

const KubernetesServiceAccountCaCert = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
const KubernetesServiceAccountToken = "/var/run/secrets/kubernetes.io/serviceaccount/token"

func main() {
	var address string
	var terminationGracePeriodSeconds int
	var lameduck int
	var keepAlive bool
	flag.StringVar(&address, "address", "0.0.0.0:8080", "")

	flag.IntVar(&terminationGracePeriodSeconds, "termination-grace-period-seconds", 10, "The duration in seconds the application needs to terminate gracefully")
	flag.IntVar(&lameduck, "lameduck", 1, "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.BoolVar(&keepAlive, "http-keepalive", true, "")

	router := http.NewServeMux()
	router.HandleFunc("/openid/v1/jwks", func(w http.ResponseWriter, r *http.Request) {
		token, err := os.ReadFile(KubernetesServiceAccountToken)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(http.StatusText(http.StatusInternalServerError)))
			return
		}

		caCert, err := os.ReadFile(KubernetesServiceAccountCaCert)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(http.StatusText(http.StatusInternalServerError)))
			return
		}

		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: x509.NewCertPool(),
			},
		}

		transport.TLSClientConfig.RootCAs.AppendCertsFromPEM(caCert)

		client := &http.Client{
			Transport: transport,
		}

		u, err := url.JoinPath("https://kubernetes.default.svc.cluster.local", r.URL.Path)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(http.StatusText(http.StatusBadRequest)))
			return
		}

		request, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(http.StatusText(http.StatusInternalServerError)))
			return
		}

		request.Header.Set("Authorization", "Bearer "+string(token))

		response, err := client.Do(request)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(http.StatusText(http.StatusInternalServerError)))
			return
		}
		defer response.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(response.StatusCode)
		_, _ = w.Write([]byte(http.StatusText(response.StatusCode)))
	})

	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
	})

	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen: %+v", err)
	}

	server := &http.Server{
		Handler: router,
	}
	server.SetKeepAlivesEnabled(keepAlive)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v\n%s", err, debug.Stack())
			}
		}()
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Failed to listen: %+v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	<-quit
	time.Sleep(time.Duration(lameduck) * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(terminationGracePeriodSeconds)*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("failed to shutdown: %+v", err)
	}
}
