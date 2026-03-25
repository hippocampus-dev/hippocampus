package provider

import (
	"context"
)

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    *int   `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type Provider interface {
	AuthorizationURL(clientID string, scope string, state string, redirectURI string) string
	ExchangeCode(ctx context.Context, clientID string, clientSecret string, code string, redirectURI string) (map[string]interface{}, error)
	NormalizeTokenResponse(response map[string]interface{}, scope string) (*TokenResponse, error)
}
