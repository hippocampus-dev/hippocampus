package fallback

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"golang.org/x/net/netutil"
	"golang.org/x/xerrors"
)

const (
	clonedRequestContextKey = "clonedRequest"
	clonedBodyKey           = "clonedBody"
)

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	listener, err := net.Listen("tcp", a.Address)
	if err != nil {
		return xerrors.Errorf("failed to create listener: %w", err)
	}

	target, err := url.Parse(a.Target)
	if err != nil {
		return xerrors.Errorf("failed to parse target: %w", err)
	}

	fallback, err := url.Parse(a.Fallback)
	if err != nil {
		return xerrors.Errorf("failed to parse fallback: %w", err)
	}

	return runHTTPServer(listener, target, fallback, a)
}

func runHTTPServer(listener net.Listener, target *url.URL, fallback *url.URL, a *Args) error {
	proxy := httputil.NewSingleHostReverseProxy(target)
	fallbackProxy := httputil.NewSingleHostReverseProxy(fallback)
	proxy.ModifyResponse = func(response *http.Response) error {
		if response.StatusCode == http.StatusNotFound {
			return xerrors.New("not found")
		}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		body := r.Context().Value(clonedBodyKey).([]byte)
		log.Print(string(body))

		clonedRequest := r.Context().Value(clonedRequestContextKey).(*http.Request)
		clonedRequest.Host = fallback.Host
		clonedRequest.URL.Host = fallback.Host
		fallbackProxy.ServeHTTP(w, clonedRequest)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		r.Host = target.Host
		r.URL.Host = target.Host

		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), clonedBodyKey, body))

		clonedRequest := r.Clone(r.Context())
		clonedRequest.Body = io.NopCloser(bytes.NewReader(body))
		clonedRequest.ContentLength = int64(len(body))
		r = clonedRequest.WithContext(context.WithValue(r.Context(), clonedRequestContextKey, clonedRequest))
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))
		proxy.ServeHTTP(w, r)
	})

	server := &http.Server{
		Handler: mux,
	}
	server.SetKeepAlivesEnabled(a.Keepalive)

	// This goroutine owns the only listener, so whatever ends it has to reach the select below: a failure that stayed here would leave the process idling with nothing served until SIGTERM
	errCh := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errCh <- xerrors.Errorf("panic: %+v\n%s", r, debug.Stack())
			}
		}()
		if err := server.Serve(netutil.LimitListener(listener, a.MaxConnections)); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- xerrors.Errorf("failed to listen: %w", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	select {
	case err := <-errCh:
		return err
	case <-quit:
	}
	time.Sleep(time.Duration(a.Lameduck) * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.TerminationGracePeriodSeconds)*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return xerrors.Errorf("failed to shutdown: %w", err)
	}

	return nil
}
