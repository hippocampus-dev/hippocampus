package oauth

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/skratchdot/open-golang/open"
	"golang.org/x/oauth2"
	"golang.org/x/xerrors"
)

type DeviceClient struct {
	mu     sync.Mutex
	config oauth2.Config
}

func NewDeviceClient(deviceAuthURL string, tokenURL string, clientID string) *DeviceClient {
	return &DeviceClient{
		config: oauth2.Config{
			ClientID: clientID,
			Endpoint: oauth2.Endpoint{
				DeviceAuthURL: deviceAuthURL,
				TokenURL:      tokenURL,
				AuthStyle:     oauth2.AuthStyleInParams,
			},
		},
	}
}

func (c *DeviceClient) GetToken(scope string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if token := restoreToken(deviceDirectory(), scope); token.Valid() {
		return token.AccessToken, nil
	}

	token, err := c.authorize(scope)
	if err != nil {
		return "", err
	}

	if err := saveToken(deviceDirectory(), scope, token); err != nil {
		return "", err
	}

	return token.AccessToken, nil
}

func (c *DeviceClient) authorize(scope string) (*oauth2.Token, error) {
	config := c.config
	config.Scopes = []string{scope}

	deviceAuth, err := config.DeviceAuth(context.Background())
	if err != nil {
		return nil, xerrors.Errorf("failed to request device authorization: %w", err)
	}

	_ = open.Start(deviceAuth.VerificationURI)
	_, _ = fmt.Fprintln(os.Stderr, "Please visit this URL to authorize: "+deviceAuth.VerificationURI)

	token, err := config.DeviceAccessToken(context.Background(), deviceAuth)
	if err != nil {
		return nil, xerrors.Errorf("failed to get device access token: %w", err)
	}

	return token, nil
}
