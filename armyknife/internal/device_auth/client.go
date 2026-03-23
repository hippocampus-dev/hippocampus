package device_auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/skratchdot/open-golang/open"
	"golang.org/x/xerrors"
)

type Client struct {
	mu  sync.Mutex
	url string
}

func NewClient(address string) *Client {
	return &Client{
		url: address,
	}
}

type deviceAuthResponse struct {
	DeviceCode      string `json:"device_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type tokenErrorResponse struct {
	Error    string `json:"error"`
	Interval int    `json:"interval"`
}

type cachedToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    *int   `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

func (c *Client) GetToken(scope string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if token := c.restore(scope); token != "" {
		return token, nil
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

func (c *Client) authorize(scope string) (*cachedToken, error) {
	request, err := http.NewRequest(http.MethodPost, c.url+"/device/authorize", strings.NewReader(url.Values{"scope": {scope}}.Encode()))
	if err != nil {
		return nil, xerrors.Errorf("failed to create device authorization request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, xerrors.Errorf("failed to request device authorization: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode >= 400 {
		body, _ := io.ReadAll(response.Body)
		return nil, xerrors.Errorf("device authorization failed: status=%d, body=%s", response.StatusCode, string(body))
	}

	var authResponse deviceAuthResponse
	if err := json.NewDecoder(response.Body).Decode(&authResponse); err != nil {
		return nil, xerrors.Errorf("failed to decode device authorization response: %w", err)
	}

	if err := open.Start(authResponse.VerificationURI); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Please visit this URL to authorize: "+authResponse.VerificationURI)
	}

	return c.poll(authResponse)
}

func (c *Client) poll(authResponse deviceAuthResponse) (*cachedToken, error) {
	interval := authResponse.Interval
	if interval <= 0 {
		interval = 5
	}
	deadline := time.Now().Add(time.Duration(authResponse.ExpiresIn) * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(time.Duration(interval) * time.Second)

		request, err := http.NewRequest(http.MethodPost, c.url+"/token", strings.NewReader(url.Values{"device_code": {authResponse.DeviceCode}}.Encode()))
		if err != nil {
			return nil, xerrors.Errorf("failed to create token request: %w", err)
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return nil, xerrors.Errorf("failed to poll token: %w", err)
		}

		body, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil {
			return nil, xerrors.Errorf("failed to read token response: %w", err)
		}

		if response.StatusCode == http.StatusOK {
			var token cachedToken
			if err := json.Unmarshal(body, &token); err != nil {
				return nil, xerrors.Errorf("failed to decode token response: %w", err)
			}
			return &token, nil
		}

		var errResponse tokenErrorResponse
		if err := json.Unmarshal(body, &errResponse); err != nil {
			return nil, xerrors.Errorf("failed to decode error response: %w", err)
		}

		switch errResponse.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			if errResponse.Interval > 0 {
				interval = errResponse.Interval
			}
			continue
		default:
			return nil, xerrors.Errorf("token request failed: %s", errResponse.Error)
		}
	}

	return nil, xerrors.Errorf("authorization timed out")
}

func (c *Client) restore(scope string) string {
	b, err := os.ReadFile(filepath.Join(c.directory(), url.PathEscape(scope)))
	if err != nil {
		return ""
	}
	var token cachedToken
	if err := json.Unmarshal(b, &token); err != nil {
		return ""
	}
	if token.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, token.ExpiresAt)
		if err != nil || t.Before(time.Now()) {
			return ""
		}
	}
	return token.AccessToken
}

func (c *Client) save(scope string, token *cachedToken) error {
	if token.ExpiresIn != nil && *token.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(*token.ExpiresIn) * time.Second).Format(time.RFC3339)
	}
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
