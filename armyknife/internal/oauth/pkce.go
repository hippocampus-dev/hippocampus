package oauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/skratchdot/open-golang/open"
	"golang.org/x/oauth2"
	"golang.org/x/xerrors"
)

type PKCEClient struct {
	mu         sync.Mutex
	config     oauth2.Config
	listenPort uint
}

func NewPKCEClient(authorizationURL string, tokenURL string, clientID string, listenPort uint) *PKCEClient {
	return &PKCEClient{
		config: oauth2.Config{
			ClientID: clientID,
			Endpoint: oauth2.Endpoint{
				AuthURL:   authorizationURL,
				TokenURL:  tokenURL,
				AuthStyle: oauth2.AuthStyleInParams,
			},
		},
		listenPort: listenPort,
	}
}

func (c *PKCEClient) GetToken(scope string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if token := restoreToken(pkceDirectory(), scope); token.Valid() {
		return token.AccessToken, nil
	}

	token, err := c.authorize(scope)
	if err != nil {
		return "", err
	}

	if err := saveToken(pkceDirectory(), scope, token); err != nil {
		return "", err
	}

	return token.AccessToken, nil
}

func (c *PKCEClient) authorize(scope string) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", c.listenPort))
	if err != nil {
		return nil, xerrors.Errorf("failed to start callback server: %w", err)
	}
	defer func() {
		_ = listener.Close()
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d", port)

	verifier := oauth2.GenerateVerifier()

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, xerrors.Errorf("failed to generate state: %w", err)
	}
	state := hex.EncodeToString(stateBytes)

	config := c.config
	config.Scopes = []string{scope}
	config.RedirectURL = redirectURI

	authURL := config.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))

	_ = open.Start(authURL)
	_, _ = fmt.Fprintln(os.Stderr, "Please visit this URL to authorize: "+authURL)

	type callbackResult struct {
		code string
		err  error
	}
	ch := make(chan callbackResult, 1)

	go http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			select {
			case ch <- callbackResult{err: xerrors.Errorf("state mismatch in callback")}:
			default:
			}
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			select {
			case ch <- callbackResult{err: xerrors.Errorf("missing authorization code in callback")}:
			default:
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<script>window.open("about:blank","_self").close()</script>`))
		select {
		case ch <- callbackResult{code: code}:
		default:
		}
	}))

	result := <-ch
	if result.err != nil {
		return nil, result.err
	}

	token, err := config.Exchange(context.Background(), result.code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, xerrors.Errorf("failed to exchange authorization code: %w", err)
	}

	return token, nil
}
