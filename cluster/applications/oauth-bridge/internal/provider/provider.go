package provider

import (
	"context"
	"net/http"
)

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    *int   `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type Provider interface {
	Configure(providerURL string) error
	AuthorizationURL(clientID string, scope string, state string, redirectURI string, codeChallenge string) string
	CallbackCode(r *http.Request) string
	ExchangeCode(ctx context.Context, clientID string, clientSecret string, code string, redirectURI string, codeVerifier string) (map[string]interface{}, error)
	RefreshAccessToken(ctx context.Context, clientID string, clientSecret string, refreshToken string) (map[string]interface{}, error)
	NormalizeTokenResponse(response map[string]interface{}, scope string) (*TokenResponse, error)
}
