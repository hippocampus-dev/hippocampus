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

func (s *Slack) NormalizeTokenResponse(slackResponse map[string]interface{}, scope string) map[string]interface{} {
	token := map[string]interface{}{
		"token_type": "bearer",
		"scope":      scope,
	}

	if authedUser, ok := slackResponse["authed_user"].(map[string]interface{}); ok {
		if accessToken, ok := authedUser["access_token"].(string); ok {
			token["access_token"] = accessToken
		}
		if userScope, ok := authedUser["scope"].(string); ok {
			token["scope"] = userScope
		}
	}

	if _, hasUserToken := token["access_token"]; !hasUserToken {
		if accessToken, ok := slackResponse["access_token"].(string); ok {
			token["access_token"] = accessToken
		}
		if topScope, ok := slackResponse["scope"].(string); ok {
			token["scope"] = topScope
		}
	}

	return token
}
