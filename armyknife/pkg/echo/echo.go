package echo

import (
	"context"
	"errors"
	"io"
	"log"
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

	router := http.NewServeMux()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		r.Header.Add("RawBody", string(b))
		log.Printf("%+v\n", r)
	})
	listener, err := net.Listen("tcp", a.Address)
	if err != nil {
		return xerrors.Errorf("failed to create listener: %w", err)
	}

	server := &http.Server{
		Handler: router,
	}
	server.SetKeepAlivesEnabled(a.Keepalive)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v\n%s", err, debug.Stack())
			}
		}()
		if err := server.Serve(netutil.LimitListener(listener, a.MaxConnections)); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Failed to listen: %+v", err)
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
