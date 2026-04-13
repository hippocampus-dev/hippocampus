package remotty

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"os/signal"
	"syscall"

	"armyknife/internal/bakery"

	chclient "github.com/jpillora/chisel/client"

	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/ssh"
	"golang.org/x/xerrors"
)

const (
	hostBridgePort = 65532
	sftpBridgePort = 65533
	keyBridgePort  = 65534
)

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	bridge, err := newHostBridge(a.Auth)
	if err != nil {
		return xerrors.Errorf("failed to start host bridge: %w", err)
	}
	defer func() {
		_ = bridge.Close()
	}()

	remotes := []string{
		fmt.Sprintf("R:127.0.0.1:%d:127.0.0.1:%d", hostBridgePort, bridge.Addr().(*net.TCPAddr).Port),
	}

	if a.Sync != "" {
		clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return xerrors.Errorf("failed to generate client key: %w", err)
		}

		clientSigner, err := ssh.NewSignerFromKey(clientKey)
		if err != nil {
			return xerrors.Errorf("failed to create client signer: %w", err)
		}

		sftpBridge, err := newSFTPBridge(a.Sync, clientSigner.PublicKey())
		if err != nil {
			return xerrors.Errorf("failed to start SFTP bridge: %w", err)
		}
		defer func() {
			_ = sftpBridge.Close()
		}()

		keyBridge, err := newKeyBridge(a.Auth, clientKey)
		if err != nil {
			return xerrors.Errorf("failed to start key bridge: %w", err)
		}
		defer func() {
			_ = keyBridge.Close()
		}()

		remotes = append(remotes,
			fmt.Sprintf("R:127.0.0.1:%d:127.0.0.1:%d", sftpBridgePort, sftpBridge.Addr().(*net.TCPAddr).Port),
			fmt.Sprintf("R:127.0.0.1:%d:127.0.0.1:%d", keyBridgePort, keyBridge.Addr().(*net.TCPAddr).Port),
		)
	}

	b := bakery.NewClient(a.BakeryURL, a.ListenPort)
	cookie, err := b.GetValue(a.CookieName)
	if err != nil {
		return xerrors.Errorf("failed to get cookie: %w", err)
	}

	remotes = append(remotes, a.Remotes...)

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
