package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/xerrors"
)

type Spotify struct{}

func (s *Spotify) AuthorizationURL(clientID string, scope string, state string, redirectURI string) string {
	return fmt.Sprintf("https://accounts.spotify.com/authorize?client_id=%s&response_type=code&scope=%s&state=%s&redirect_uri=%s",
		url.QueryEscape(clientID),
		url.QueryEscape(scope),
		url.QueryEscape(state),
		url.QueryEscape(redirectURI),
	)
}

func (s *Spotify) ExchangeCode(ctx context.Context, clientID string, clientSecret string, code string, redirectURI string) (map[string]interface{}, error) {
	values := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://accounts.spotify.com/api/token", strings.NewReader(values.Encode()))
	if err != nil {
		return nil, xerrors.Errorf("failed to create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(clientID+":"+clientSecret)))

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

func (s *Spotify) RefreshAccessToken(ctx context.Context, clientID string, clientSecret string, refreshToken string) (map[string]interface{}, error) {
	values := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://accounts.spotify.com/api/token", strings.NewReader(values.Encode()))
	if err != nil {
		return nil, xerrors.Errorf("failed to create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(clientID+":"+clientSecret)))

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

func (s *Spotify) NormalizeTokenResponse(spotifyResponse map[string]interface{}, scope string) (*TokenResponse, error) {
	accessToken, ok := spotifyResponse["access_token"].(string)
	if !ok || accessToken == "" {
		return nil, xerrors.Errorf("missing access_token in response")
	}

	token := &TokenResponse{
		AccessToken: accessToken,
		TokenType:   "bearer",
		Scope:       scope,
	}

	if responseScope, ok := spotifyResponse["scope"].(string); ok {
		token.Scope = responseScope
	}
	if expiresIn, ok := spotifyResponse["expires_in"].(float64); ok && expiresIn > 0 {
		v := int(expiresIn)
		token.ExpiresIn = &v
	}
	if refreshToken, ok := spotifyResponse["refresh_token"].(string); ok {
		token.RefreshToken = refreshToken
	}

	return token, nil
}
