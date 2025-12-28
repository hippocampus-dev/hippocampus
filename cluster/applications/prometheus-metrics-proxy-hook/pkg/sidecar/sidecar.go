package sidecar

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

var (
	lastScrape  atomic.Value
	terminating atomic.Bool
	scrapeChan  chan struct{}
)

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		var errs validator.ValidationErrors
		errors.As(err, &errs)
		var messages []string
		for _, e := range errs {
			if e.ActualTag() == "oneof" {
				messages = append(messages, fmt.Sprintf("%s must be one of these [%s]", e.Field(), e.Param()))
			}
		}
		if len(messages) > 0 {
			err = xerrors.Errorf("%s: %w", strings.Join(messages, ", "), err)
		}
		return xerrors.Errorf("validation error: %w", err)
	}

	lastScrape.Store(time.Time{})
	scrapeChan = make(chan struct{}, 1)

	targetURL, err := url.Parse(a.TargetURL)
	if err != nil {
		return xerrors.Errorf("failed to parse target URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	listener, err := net.Listen("tcp", a.Address)
	if err != nil {
		return xerrors.Errorf("failed to listen: %w", err)
	}

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lastScrape.Store(time.Now())
			if terminating.Load() {
				select {
				case scrapeChan <- struct{}{}:
				default:
				}
			}
			proxy.ServeHTTP(w, r)
		}),
	}

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
	terminating.Store(true)
	time.Sleep(a.Lameduck)

	t, ok := lastScrape.Load().(time.Time)
	if ok && !t.IsZero() {
		timeSinceLastScrape := time.Since(t)

		if timeSinceLastScrape > 1*time.Second {
			ctx, cancel := context.WithTimeout(context.Background(), a.TerminationGracePeriod)
			defer cancel()

			select {
			case <-scrapeChan:
			case <-ctx.Done():
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.TerminationGracePeriod)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("failed to shutdown: %+v", err)
	}
	return nil
}
