package remotty

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"armyknife/pkg/remotty/middlewares"

	"golang.org/x/xerrors"
)

type keyBridge struct {
	listener net.Listener
	server   *http.Server
}

func newKeyBridge(auth string, clientPrivateKey *ecdsa.PrivateKey) (*keyBridge, error) {
	encoded, err := encodePrivateKey(clientPrivateKey)
	if err != nil {
		return nil, xerrors.Errorf("failed to encode private key: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/key", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-pem-file")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(encoded)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, xerrors.Errorf("failed to create key bridge listener: %w", err)
	}

	server := &http.Server{Handler: middlewares.BasicAuth(auth, mux)}

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v\n%s", err, debug.Stack())
			}
		}()
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("failed to serve key bridge: %+v", err)
		}
	}()

	return &keyBridge{
		listener: listener,
		server:   server,
	}, nil
}

func (k *keyBridge) Addr() net.Addr {
	return k.listener.Addr()
}

func (k *keyBridge) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return k.server.Shutdown(ctx)
}

func encodePrivateKey(key *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal EC private key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: der,
	}), nil
}
