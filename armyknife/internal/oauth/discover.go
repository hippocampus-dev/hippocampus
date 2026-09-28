package oauth

import (
	"encoding/json"
	"net/http"

	"golang.org/x/xerrors"
)

type Endpoints struct {
	AuthorizationEndpoint       string
	TokenEndpoint               string
	DeviceAuthorizationEndpoint string
}

type authorizationServerMetadata struct {
	AuthorizationEndpoint       string `json:"authorization_endpoint"`
	TokenEndpoint               string `json:"token_endpoint"`
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
}

func Discover(issuerURL string) (*Endpoints, error) {
	request, err := http.NewRequest(http.MethodGet, issuerURL+"/.well-known/oauth-authorization-server", nil)
	if err != nil {
		return nil, xerrors.Errorf("failed to create metadata request: %w", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, xerrors.Errorf("failed to fetch authorization server metadata: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode >= 400 {
		return nil, xerrors.Errorf("failed to fetch authorization server metadata: status=%d", response.StatusCode)
	}

	var metadata authorizationServerMetadata
	if err := json.NewDecoder(response.Body).Decode(&metadata); err != nil {
		return nil, xerrors.Errorf("failed to decode authorization server metadata: %w", err)
	}

	if metadata.TokenEndpoint == "" {
		return nil, xerrors.Errorf("authorization server metadata is missing token_endpoint")
	}

	return &Endpoints{
		AuthorizationEndpoint:       metadata.AuthorizationEndpoint,
		TokenEndpoint:               metadata.TokenEndpoint,
		DeviceAuthorizationEndpoint: metadata.DeviceAuthorizationEndpoint,
	}, nil
}
