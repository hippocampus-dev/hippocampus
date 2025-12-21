package bakery

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/skratchdot/open-golang/open"
	"golang.org/x/xerrors"
)

type Client struct {
	url        string
	listenPort uint
}

func NewClient(address string, listenPort uint) *Client {
	return &Client{
		url:        address,
		listenPort: listenPort,
	}
}

func (c *Client) GetValue(cookieName string) (string, error) {
	cookie := c.restore(cookieName)
	value, exists := cookie["value"]
	expires, expiresExists := cookie["expires"]
	t, terr := time.Parse(time.RFC3339, expires)
	if !exists || !expiresExists || terr != nil || t.Before(time.Now()) {
		cookie, err := c.challenge(cookieName)
		if err != nil {
			return "", err
		}
		if err := c.save(cookieName, cookie); err != nil {
			return "", err
		}
		return cookie["value"], nil
	}
	return value, nil
}

func (c *Client) challenge(cookieName string) (map[string]string, error) {
	listener, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", c.listenPort))
	if err != nil {
		return nil, err
	}

	quit := make(chan map[string]string, 1)
	go http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<script>window.open("about:blank","_self").close()</script>`))
		m := make(map[string]string)
		m["value"] = r.URL.Query().Get("value")
		m["expires"] = r.URL.Query().Get("expires")
		quit <- m
	}))

	u, err := url.ParseRequestURI(c.url)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse request uri: %w", err)
	}
	queries := u.Query()
	queries.Set("redirect_url", fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port))
	queries.Set("cookie_name", cookieName)
	u.RawQuery = queries.Encode()
	if err := open.Start(u.String()); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Please visit this URL to authorize this application: "+u.String())
	}

	return <-quit, nil
}

func (c *Client) restore(cookieName string) map[string]string {
	var cookie map[string]string
	b, err := os.ReadFile(filepath.Join(c.directory(), cookieName))
	if err != nil {
		return make(map[string]string)
	}
	_ = json.Unmarshal(b, &cookie)
	return cookie
}

func (c *Client) save(cookieName string, cookie map[string]string) error {
	b, err := json.Marshal(cookie)
	if err != nil {
		return err
	}
	d := c.directory()
	if err := os.MkdirAll(d, 0700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d, cookieName), b, 0600)
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
	return filepath.Join(home, "bakery")
}
