package remotty

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
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

func newHostBridge(auth string, envPatterns []string) (*hostBridge, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, xerrors.Errorf("failed to create listener: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/open", routes.Open(opener.Open))
	mux.HandleFunc("GET /v1/env", routes.Env(envPatterns))

	server := &http.Server{Handler: middlewares.BasicAuth(auth, mux)}

	// A panic here is left to crash the process, matching the log.Fatalf below: recovering it would return a bridge whose server is already dead, and the caller has no way to notice
	go func() {
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
