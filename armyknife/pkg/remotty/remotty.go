package remotty

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"armyknife/internal/bakery"
	"armyknife/pkg/remotty/routers"

	chclient "github.com/jpillora/chisel/client"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

const hostBridgePort = 65532

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return xerrors.Errorf("failed to create listener: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/open", routers.Open(open))

	server := &http.Server{Handler: mux}

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

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	b := bakery.NewClient(a.BakeryURL, a.ListenPort)
	cookie, err := b.GetValue(a.CookieName)
	if err != nil {
		return xerrors.Errorf("failed to get cookie: %w", err)
	}

	remotes := append(
		[]string{fmt.Sprintf("R:127.0.0.1:%d:127.0.0.1:%d", hostBridgePort, listener.Addr().(*net.TCPAddr).Port)},
		a.Remotes...,
	)

	client, err := chclient.NewClient(&chclient.Config{
		Server:  a.Server,
		Remotes: remotes,
		Auth:    a.Auth,
		Headers: http.Header{
			"Cookie": []string{a.CookieName + "=" + cookie},
		},
	})
	if err != nil {
		return xerrors.Errorf("failed to create chisel client: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := client.Start(ctx); err != nil {
		return xerrors.Errorf("failed to start chisel client: %w", err)
	}

	if err := client.Wait(); err != nil {
		return xerrors.Errorf("chisel client error: %w", err)
	}

	return nil
}
