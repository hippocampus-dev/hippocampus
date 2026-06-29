package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/xerrors"
)

type Google struct{}

func (g *Google) AuthorizationURL(clientID string, scope string, state string, redirectURI string) string {
	return fmt.Sprintf("https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&response_type=code&scope=%s&state=%s&redirect_uri=%s&access_type=offline&prompt=consent",
		url.QueryEscape(clientID),
		url.QueryEscape(scope),
		url.QueryEscape(state),
		url.QueryEscape(redirectURI),
	)
}

func (g *Google) ExchangeCode(ctx context.Context, clientID string, clientSecret string, code string, redirectURI string) (map[string]interface{}, error) {
	values := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(values.Encode()))
	if err != nil {
		return nil, xerrors.Errorf("failed to create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, xerrors.Errorf("failed to do request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()

	if response.StatusCode >= 400 {
		body, _ := io.ReadAll(response.Body)
		return nil, xerrors.Errorf("API error: status=%d, body=%s", response.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, xerrors.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

func (g *Google) RefreshAccessToken(ctx context.Context, clientID string, clientSecret string, refreshToken string) (map[string]interface{}, error) {
	values := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(values.Encode()))
	if err != nil {
		return nil, xerrors.Errorf("failed to create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, xerrors.Errorf("failed to do request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()

	if response.StatusCode >= 400 {
		body, _ := io.ReadAll(response.Body)
		return nil, xerrors.Errorf("API error: status=%d, body=%s", response.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, xerrors.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

func (g *Google) NormalizeTokenResponse(googleResponse map[string]interface{}, scope string) (*TokenResponse, error) {
	accessToken, ok := googleResponse["access_token"].(string)
	if !ok || accessToken == "" {
		return nil, xerrors.Errorf("missing access_token in response")
	}

	token := &TokenResponse{
		AccessToken: accessToken,
		TokenType:   "bearer",
		Scope:       scope,
	}

	if responseScope, ok := googleResponse["scope"].(string); ok {
		token.Scope = responseScope
	}
	if expiresIn, ok := googleResponse["expires_in"].(float64); ok && expiresIn > 0 {
		v := int(expiresIn)
		token.ExpiresIn = &v
	}
	if refreshToken, ok := googleResponse["refresh_token"].(string); ok {
		token.RefreshToken = refreshToken
	}

	return token, nil
}
