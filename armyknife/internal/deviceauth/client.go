package deviceauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/skratchdot/open-golang/open"
	"golang.org/x/oauth2"
	"golang.org/x/xerrors"
)

type Client struct {
	mu     sync.Mutex
	config oauth2.Config
}

func NewClient(address string, clientID string) *Client {
	return &Client{
		config: oauth2.Config{
			ClientID: clientID,
			Endpoint: oauth2.Endpoint{
				DeviceAuthURL: address + "/device/code",
				TokenURL:      address + "/token",
				AuthStyle:     oauth2.AuthStyleInParams,
			},
		},
	}
}

func (c *Client) GetToken(scope string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if token := c.restore(scope); token.Valid() {
		return token.AccessToken, nil
	}

	token, err := c.authorize(scope)
	if err != nil {
		return "", err
	}

	if err := c.save(scope, token); err != nil {
		return "", err
	}

	return token.AccessToken, nil
}

func (c *Client) authorize(scope string) (*oauth2.Token, error) {
	config := c.config
	config.Scopes = []string{scope}

	deviceAuth, err := config.DeviceAuth(context.Background())
	if err != nil {
		return nil, xerrors.Errorf("failed to request device authorization: %w", err)
	}

	if err := open.Start(deviceAuth.VerificationURI); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Please visit this URL to authorize: "+deviceAuth.VerificationURI)
	}

	token, err := config.DeviceAccessToken(context.Background(), deviceAuth)
	if err != nil {
		return nil, xerrors.Errorf("failed to get device access token: %w", err)
	}

	return token, nil
}

func (c *Client) restore(scope string) *oauth2.Token {
	b, err := os.ReadFile(filepath.Join(c.directory(), url.PathEscape(scope)))
	if err != nil {
		return nil
	}
	var token oauth2.Token
	if err := json.Unmarshal(b, &token); err != nil {
		return nil
	}
	return &token
}

func (c *Client) save(scope string, token *oauth2.Token) error {
	b, err := json.Marshal(token)
	if err != nil {
		return err
	}
	d := c.directory()
	if err := os.MkdirAll(d, 0700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d, url.PathEscape(scope)), b, 0600)
}

func (c *Client) directory() string {
	home := os.Getenv("XDG_DATA_HOME")
	if home == "" {
		u, err := os.UserHomeDir()
		if err != nil {
			return os.TempDir()
		}
		home = filepath.Join(u, ".local", "share")
	}
	return filepath.Join(home, "device-auth")
}
