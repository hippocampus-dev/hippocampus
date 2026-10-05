package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/xerrors"
)

// oauth2-proxy stores the authenticated session under this cookie by default
const oauth2ProxySessionCookieName = "_oauth2_proxy"

type OAuth2Proxy struct {
	providerURL string
}

func (o *OAuth2Proxy) Configure(providerURL string) error {
	if providerURL == "" {
		return xerrors.Errorf("--provider-url or PROVIDER_URL is required for oauth2-proxy")
	}

	o.providerURL = strings.TrimRight(providerURL, "/")

	return nil
}

// https://oauth2-proxy.github.io/oauth2-proxy/features/endpoints
func (o *OAuth2Proxy) AuthorizationURL(_ string, _ string, state string, redirectURI string, _ string) string {
	returnURL := redirectURI + "?" + url.Values{"state": {state}}.Encode()

	return fmt.Sprintf("%s/oauth2/start?%s", o.providerURL, url.Values{"rd": {returnURL}}.Encode())
}

// oauth2-proxy returns no authorization code, so the session cookie carries the credential instead
func (o *OAuth2Proxy) CallbackCode(r *http.Request) string {
	cookie, err := r.Cookie(oauth2ProxySessionCookieName)
	if err != nil {
		return ""
	}

	return cookie.Value
}

// The session cookie is already the credential, so there is nothing to exchange
func (o *OAuth2Proxy) ExchangeCode(_ context.Context, _ string, _ string, code string, _ string, _ string) (map[string]interface{}, error) {
	return map[string]interface{}{"access_token": code}, nil
}

func (o *OAuth2Proxy) RefreshAccessToken(_ context.Context, _ string, _ string, _ string) (map[string]interface{}, error) {
	return nil, xerrors.Errorf("refresh_token grant type is not supported for oauth2-proxy")
}

func (o *OAuth2Proxy) NormalizeTokenResponse(oauth2ProxyResponse map[string]interface{}, scope string) (*TokenResponse, error) {
	accessToken, ok := oauth2ProxyResponse["access_token"].(string)
	if !ok || accessToken == "" {
		return nil, xerrors.Errorf("missing access_token in response")
	}

	return &TokenResponse{
		AccessToken: accessToken,
		TokenType:   "bearer",
		Scope:       scope,
	}, nil
}
