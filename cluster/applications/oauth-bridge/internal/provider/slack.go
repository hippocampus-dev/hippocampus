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

type Slack struct{}

func (s *Slack) AuthorizationURL(clientID string, scope string, state string, redirectURI string) string {
	slackScope := strings.ReplaceAll(scope, " ", ",")
	return fmt.Sprintf("https://slack.com/oauth/v2/authorize?client_id=%s&user_scope=%s&state=%s&redirect_uri=%s",
		url.QueryEscape(clientID),
		url.QueryEscape(slackScope),
		url.QueryEscape(state),
		url.QueryEscape(redirectURI),
	)
}

func (s *Slack) ExchangeCode(ctx context.Context, clientID string, clientSecret string, code string, redirectURI string) (map[string]interface{}, error) {
	values := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://slack.com/api/oauth.v2.access", strings.NewReader(values.Encode()))
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

	ok, _ := result["ok"].(bool)
	if !ok {
		return nil, xerrors.Errorf("Slack API error: %v", result["error"])
	}

	return result, nil
}

func (s *Slack) RefreshAccessToken(_ context.Context, _ string, _ string, _ string) (map[string]interface{}, error) {
	return nil, xerrors.Errorf("refresh_token grant type is not supported for Slack")
}

func (s *Slack) NormalizeTokenResponse(slackResponse map[string]interface{}, scope string) (*TokenResponse, error) {
	authedUser, ok := slackResponse["authed_user"].(map[string]interface{})
	if !ok {
		return nil, xerrors.Errorf("missing authed_user in response")
	}

	accessToken, ok := authedUser["access_token"].(string)
	if !ok || accessToken == "" {
		return nil, xerrors.Errorf("missing access_token in authed_user")
	}

	token := &TokenResponse{
		AccessToken: accessToken,
		TokenType:   "bearer",
		Scope:       scope,
	}

	if userScope, ok := authedUser["scope"].(string); ok {
		token.Scope = userScope
	}
	if expiresIn, ok := authedUser["expires_in"].(float64); ok && expiresIn > 0 {
		v := int(expiresIn)
		token.ExpiresIn = &v
	}
	if refreshToken, ok := authedUser["refresh_token"].(string); ok {
		token.RefreshToken = refreshToken
	}

	return token, nil
}
