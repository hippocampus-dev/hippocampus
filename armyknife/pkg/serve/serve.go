package serve

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"golang.org/x/net/netutil"
	"golang.org/x/xerrors"
)

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(a.Directory)))
	listener, err := net.Listen("tcp", a.Address)
	if err != nil {
		return xerrors.Errorf("failed to create listener: %w", err)
	}

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
