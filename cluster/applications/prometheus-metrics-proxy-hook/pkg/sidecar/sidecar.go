package sidecar

import (
	"context"
	"errors"
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
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

type CachedResponse struct {
	Body        []byte
	ContentType string
}

var (
	lastScrape    atomic.Value
	terminating   atomic.Bool
	scrapeChan    chan struct{}
	cachedMetrics atomic.Pointer[CachedResponse]
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
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if cached := cachedMetrics.Load(); cached != nil {
			w.Header().Set("Content-Type", cached.ContentType)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(cached.Body)
			return
		}
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), a.TerminationGracePeriod)
	defer cancel()

	t, ok := lastScrape.Load().(time.Time)
	if ok && !t.IsZero() {
		timeSinceLastScrape := time.Since(t)

		if timeSinceLastScrape > a.ScrapeWaitThreshold {
			if request, err := http.NewRequestWithContext(ctx, http.MethodGet, a.TargetURL, nil); err == nil {
				if response, err := http.DefaultClient.Do(request); err == nil {
					defer func() {
						_ = response.Body.Close()
					}()
					if response.StatusCode < 400 {
						if body, err := io.ReadAll(response.Body); err == nil {
							cachedMetrics.Store(&CachedResponse{
								Body:        body,
								ContentType: response.Header.Get("Content-Type"),
							})
						}
					}
				}
			}

			terminating.Store(true)

			select {
			case <-scrapeChan:
			case <-ctx.Done():
			}
		}
	}

	time.Sleep(a.Lameduck)

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("failed to shutdown: %+v", err)
	}
	return nil
}
