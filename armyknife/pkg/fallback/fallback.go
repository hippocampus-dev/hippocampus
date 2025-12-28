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
	"sync"
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
			return errors.New("not found")
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
		r = clonedRequest.WithContext(context.WithValue(r.Context(), clonedRequestContextKey, clonedRequest))
		r.Body = io.NopCloser(bytes.NewReader(body))
		proxy.ServeHTTP(w, r)
	})

	server := &http.Server{
		Handler: mux,
	}
	server.SetKeepAlivesEnabled(a.Keepalive)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v\n%s", err, debug.Stack())
			}
		}()
		if err := server.Serve(netutil.LimitListener(listener, a.MaxConnections)); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("failed to listen: %+v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	<-quit
	time.Sleep(time.Duration(a.Lameduck) * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.TerminationGracePeriodSeconds)*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return xerrors.Errorf("failed to shutdown: %w", err)
	}

	return nil
}

func runTCPServer(listener net.Listener, target *url.URL, a *Args) error {
	shutdown := make(chan struct{}, 1)
	semaphore := make(chan struct{}, a.MaxConnections)
	wg := sync.WaitGroup{}

	go func() {
		for {
			local, err := listener.(*net.TCPListener).AcceptTCP()
			if err != nil {
				select {
				case <-shutdown:
					return
				default:
					continue
				}
			}
			semaphore <- struct{}{}
			wg.Add(1)
			go func() {
				defer func() {
					<-semaphore
					wg.Done()
				}()

				defer local.Close()

				remoteAddress := target.Host
				if target.Port() == "" {
					switch target.Scheme {
					case "http":
						remoteAddress = net.JoinHostPort(target.Hostname(), "80")
					case "https":
						remoteAddress = net.JoinHostPort(target.Hostname(), "443")
					}
				}
				remote, err := net.DialTimeout("tcp", remoteAddress, 10*time.Second)
				if err != nil {
					return
				}
				defer remote.Close()

				c := make(chan struct{}, 2)

				f := func(c chan struct{}, dst io.Writer, src io.Reader) {
					_, _ = io.Copy(dst, src)
					c <- struct{}{}
				}
				go f(c, remote, local)
				go f(c, local, remote)

				select {
				case <-c:
				case <-shutdown:
					local.CloseWrite()
				}
			}()
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	<-quit
	time.Sleep(time.Duration(a.Lameduck) * time.Second)

	close(shutdown)
	listener.Close()

	wg.Wait()

	return nil
}
