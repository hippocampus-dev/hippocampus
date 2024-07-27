package main

import (
	"context"
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
	router.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		redirectURL := r.URL.Query().Get("redirect_url")
		u, err := url.ParseRequestURI(redirectURL)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(http.StatusText(http.StatusBadRequest)))
			return
		}

		name := r.URL.Query().Get("cookie_name")
		cookie, err := r.Cookie(name)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(http.StatusText(http.StatusUnauthorized)))
			return
		}

		queries := u.Query()
		queries.Set("value", cookie.Value)
		queries.Set("expires", cookie.Expires.Format(time.RFC3339))
		u.RawQuery = queries.Encode()

		http.Redirect(w, r, u.String(), http.StatusFound)
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
