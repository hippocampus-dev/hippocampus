package remotty

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"armyknife/pkg/remotty/internal/opener"
	"armyknife/pkg/remotty/middlewares"
	"armyknife/pkg/remotty/routes"

	"golang.org/x/xerrors"
)

type hostBridge struct {
	listener net.Listener
	server   *http.Server
}

func newHostBridge(auth string) (*hostBridge, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, xerrors.Errorf("failed to create listener: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/open", routes.Open(opener.Open))

	server := &http.Server{Handler: middlewares.BasicAuth(auth, mux)}

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v\n%s", err, debug.Stack())
			}
		}()
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("failed to serve host bridge: %+v", err)
		}
	}()

	return &hostBridge{
		listener: listener,
		server:   server,
	}, nil
}

func (h *hostBridge) Addr() net.Addr {
	return h.listener.Addr()
}

func (h *hostBridge) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return h.server.Shutdown(ctx)
}
