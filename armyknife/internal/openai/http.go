package openai

import (
	"armyknife/internal/bakery"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"golang.org/x/xerrors"
)

func getOpenAIAPIBase() string {
	base, ok := os.LookupEnv("OPENAI_BASE_URL")
	if ok {
		return base
	}
	return "https://cortex-api.minikube.127.0.0.1.nip.io/v1"
}

func CreateHTTPRequest(ctx context.Context, bakeryClient *bakery.Client, body io.Reader) (*http.Request, error) {
	destination, err := url.JoinPath(getOpenAIAPIBase(), "/chat/completions")
	if err != nil {
		return nil, xerrors.Errorf("failed to join url: %w", err)
	}
	u, err := url.Parse(destination)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse url: %w", err)
	}

	if strings.HasPrefix(u.Host, "127.") || strings.HasPrefix(u.Host, "localhost") {
		request, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			destination,
			body,
		)
		if err != nil {
			return nil, xerrors.Errorf("failed to create request: %w", err)
		}
		request.Header.Set("Content-Type", "application/json")
		return request, nil
	}

	openaiAPIKey, ok := os.LookupEnv("OPENAI_API_KEY")
	if u.Host == "api.openai.com" && ok {
		request, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			destination,
			body,
		)
		if err != nil {
			return nil, xerrors.Errorf("failed to create request: %w", err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+openaiAPIKey)
		return request, nil
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		destination,
		body,
	)
	if err != nil {
		return nil, xerrors.Errorf("failed to create request: %w", err)
	}
	request.Host = u.Host
	request.Header.Set("Content-Type", "application/json")

	token, err := bakeryClient.GetValue("_oauth2_proxy")
	if err != nil {
		return nil, xerrors.Errorf("failed to get token: %w", err)
	}
	request.AddCookie(&http.Cookie{
		Name:  "_oauth2_proxy",
		Value: token,
	})

	return request, nil
}
