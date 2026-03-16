package provider

import (
	"context"
)

type Provider interface {
	AuthorizationURL(clientID string, scope string, state string, redirectURI string) string
	ExchangeCode(ctx context.Context, clientID string, clientSecret string, code string, redirectURI string) (map[string]interface{}, error)
	NormalizeTokenResponse(response map[string]interface{}, scope string) map[string]interface{}
}
