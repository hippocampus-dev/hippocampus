package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"golang.org/x/xerrors"

	"github-actions-exporter/internal/swr"
)

func p[T any](v T) *T {
	return &v
}

func envOrDefaultValue[T any](key string, defaultValue T) T {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}

	switch any(defaultValue).(type) {
	case string:
		return any(value).(T)
	case int:
		if intValue, err := strconv.Atoi(value); err == nil {
			return any(intValue).(T)
		}
	case int64:
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return any(intValue).(T)
		}
	case uint:
		if uintValue, err := strconv.ParseUint(value, 10, 0); err == nil {
			return any(uint(uintValue)).(T)
		}
	case uint64:
		if uintValue, err := strconv.ParseUint(value, 10, 64); err == nil {
			return any(uintValue).(T)
		}
	case float64:
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return any(floatValue).(T)
		}
	case bool:
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return any(boolValue).(T)
		}
	case time.Duration:
		if durationValue, err := time.ParseDuration(value); err == nil {
			return any(durationValue).(T)
		}
	}

	return defaultValue
}

type TokenProvider interface {
	GetToken(ctx context.Context) (string, error)
}

type PatTokenProvider struct {
	token string
}

func NewPatTokenProvider(token string) *PatTokenProvider {
	return &PatTokenProvider{token: token}
}

func (p *PatTokenProvider) GetToken(ctx context.Context) (string, error) {
	return p.token, nil
}

type AppTokenProvider struct {
	clientId       string
	installationId string
	privateKey     string

	cache *swr.Cache[string]
}

func NewAppTokenProvider(clientId string, installationId string, privateKey string) *AppTokenProvider {
	return &AppTokenProvider{
		clientId:       clientId,
		installationId: installationId,
		privateKey:     privateKey,
		cache:          swr.New[string](),
	}
}

func (p *AppTokenProvider) GetToken(ctx context.Context) (string, error) {
	return p.cache.Get("token", func() (swr.FetchResult[string], error) {
		err, jwtToken := signJwt(p.privateKey, p.clientId)
		if err != nil {
			return swr.FetchResult[string]{}, xerrors.Errorf("failed to sign jwt: %w", err)
		}

		accessToken, expiresAt, err := p.getAccessToken(context.Background(), *jwtToken)
		if err != nil {
			return swr.FetchResult[string]{}, xerrors.Errorf("failed to get access token: %w", err)
		}

		expiresIn := time.Until(expiresAt)
		return swr.FetchResult[string]{
			Value:       accessToken,
			StaleAfter:  max(expiresIn-60*time.Second, 0),
			ExpireAfter: max(expiresIn-30*time.Second, 0),
		}, nil
	})
}

func (p *AppTokenProvider) getAccessToken(ctx context.Context, jwtToken string) (string, time.Time, error) {
	body := struct {
		Permissions map[string]string `json:"permissions"`
	}{
		Permissions: map[string]string{
			"actions":  "read",
			"metadata": "read",
		},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", time.Time{}, xerrors.Errorf("failed to marshal body: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("https://api.github.com/app/installations/%s/access_tokens", p.installationId), strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", time.Time{}, xerrors.Errorf("failed to create request: %w", err)
	}

	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwtToken))
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", time.Time{}, xerrors.Errorf("failed to do request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode >= 400 {
		responseBody, _ := io.ReadAll(response.Body)
		return "", time.Time{}, xerrors.Errorf("failed to get access token: status=%d, body=%s", response.StatusCode, string(responseBody))
	}

	var accessToken struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.NewDecoder(response.Body).Decode(&accessToken); err != nil {
		return "", time.Time{}, xerrors.Errorf("failed to decode access token: %w", err)
	}

	expiresAt, err := time.Parse(time.RFC3339, accessToken.ExpiresAt)
	if err != nil {
		return "", time.Time{}, xerrors.Errorf("failed to parse expires_at: %w", err)
	}

	return accessToken.Token, expiresAt, nil
}

func signJwt(privateKey string, clientId string) (error, *string) {
	block, _ := pem.Decode([]byte(privateKey))
	if block == nil {
		return xerrors.New("failed to decode private key"), nil
	}

	rsaPrivateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return xerrors.Errorf("failed to parse private key: %w", err), nil
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(time.Minute * 10).Unix(),
		"iss": clientId,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	jwtToken, err := token.SignedString(rsaPrivateKey)
	if err != nil {
		return xerrors.Errorf("failed to sign token: %w", err), nil
	}
	return nil, &jwtToken
}

type GitHubClient struct {
	tokenProvider TokenProvider
	owner         string
	repo          string
}

func NewGitHubClient(tokenProvider TokenProvider, owner string, repo string) *GitHubClient {
	return &GitHubClient{
		tokenProvider: tokenProvider,
		owner:         owner,
		repo:          repo,
	}
}

type WorkflowRunsResponse struct {
	TotalCount int `json:"total_count"`
}

type RateLimitInfo struct {
	Remaining int
	Limit     int
}

func (c *GitHubClient) GetQueuedRunsCount(ctx context.Context) (int, *RateLimitInfo, error) {
	token, err := c.tokenProvider.GetToken(ctx)
	if err != nil {
		return 0, nil, xerrors.Errorf("failed to get token: %w", err)
	}

	var url string
	if c.repo == "" {
		url = fmt.Sprintf("https://api.github.com/orgs/%s/actions/runs?status=queued&per_page=100", c.owner)
	} else {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs?status=queued&per_page=100", c.owner, c.repo)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, xerrors.Errorf("failed to create request: %w", err)
	}

	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return 0, nil, xerrors.Errorf("failed to do request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	rateLimitInfo := &RateLimitInfo{}
	if remaining := response.Header.Get("X-RateLimit-Remaining"); remaining != "" {
		if remainingInt, err := strconv.Atoi(remaining); err == nil {
			rateLimitInfo.Remaining = remainingInt
		}
	}
	if limit := response.Header.Get("X-RateLimit-Limit"); limit != "" {
		if limitInt, err := strconv.Atoi(limit); err == nil {
			rateLimitInfo.Limit = limitInt
		}
	}

	if response.StatusCode >= 400 {
		responseBody, _ := io.ReadAll(response.Body)
		return 0, rateLimitInfo, xerrors.Errorf("failed to get queued runs: status=%d, body=%s", response.StatusCode, string(responseBody))
	}

	var runsResponse WorkflowRunsResponse
	if err := json.NewDecoder(response.Body).Decode(&runsResponse); err != nil {
		return 0, rateLimitInfo, xerrors.Errorf("failed to decode response: %w", err)
	}

	return runsResponse.TotalCount, rateLimitInfo, nil
}

func main() {
	var address string
	var owner string
	var repo string
	var githubToken string
	var githubTokenFile string
	var githubAppClientId string
	var githubAppInstallationId string
	var githubAppPrivateKey string
	var terminationGracePeriod time.Duration
	var lameduck time.Duration
	var keepAlive bool

	flag.StringVar(&address, "address", envOrDefaultValue("ADDRESS", "0.0.0.0:8080"), "HTTP server address")
	flag.StringVar(&owner, "owner", envOrDefaultValue("GITHUB_OWNER", ""), "GitHub owner (organization or user)")
	flag.StringVar(&repo, "repo", envOrDefaultValue("GITHUB_REPO", ""), "GitHub repository (empty for org-level)")
	flag.StringVar(&githubToken, "github-token", envOrDefaultValue("GITHUB_TOKEN", ""), "GitHub PAT token")
	flag.StringVar(&githubTokenFile, "github-token-file", envOrDefaultValue("GITHUB_TOKEN_FILE", ""), "GitHub PAT token file")
	flag.StringVar(&githubAppClientId, "github-app-client-id", envOrDefaultValue("GITHUB_APP_CLIENT_ID", ""), "GitHub App Client ID")
	flag.StringVar(&githubAppInstallationId, "github-app-installation-id", envOrDefaultValue("GITHUB_APP_INSTALLATION_ID", ""), "GitHub App Installation ID")
	flag.StringVar(&githubAppPrivateKey, "github-app-private-key", envOrDefaultValue("GITHUB_APP_PRIVATE_KEY", ""), "GitHub App Private Key")
	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "The duration the application needs to terminate gracefully")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "A period that explicitly asks clients to stop sending requests")
	flag.BoolVar(&keepAlive, "http-keepalive", envOrDefaultValue("HTTP_KEEPALIVE", true), "Enable HTTP keep-alive")
	flag.Parse()

	if owner == "" {
		log.Fatal("--owner or GITHUB_OWNER is required")
	}

	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = http.DefaultTransport.(*http.Transport).MaxIdleConns

	var tokenProvider TokenProvider

	if githubTokenFile != "" {
		tokenBytes, err := os.ReadFile(githubTokenFile)
		if err != nil {
			log.Fatalf("failed to read github token file: %+v", err)
		}
		githubToken = strings.TrimSpace(string(tokenBytes))
	}

	if githubToken != "" {
		tokenProvider = NewPatTokenProvider(githubToken)
	} else if githubAppClientId != "" && githubAppInstallationId != "" && githubAppPrivateKey != "" {
		tokenProvider = NewAppTokenProvider(githubAppClientId, githubAppInstallationId, githubAppPrivateKey)
	} else {
		log.Fatal("either --github-token/GITHUB_TOKEN, --github-token-file/GITHUB_TOKEN_FILE, or all GitHub App flags (--github-app-client-id/GITHUB_APP_CLIENT_ID, --github-app-installation-id/GITHUB_APP_INSTALLATION_ID, --github-app-private-key/GITHUB_APP_PRIVATE_KEY) are required")
	}

	client := NewGitHubClient(tokenProvider, owner, repo)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		count, rateLimit, err := client.GetQueuedRunsCount(r.Context())
		if err != nil {
			log.Printf("failed to get queued runs count: %+v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		runsLabels := []*dto.LabelPair{
			{Name: p("owner"), Value: p(owner)},
			{Name: p("status"), Value: p("queued")},
		}
		if repo != "" {
			runsLabels = append(runsLabels, &dto.LabelPair{Name: p("repo"), Value: p(repo)})
		}

		families := []*dto.MetricFamily{
			{
				Name: p("github_actions_runs_total"),
				Help: p("Total number of GitHub Actions workflow runs"),
				Type: p(dto.MetricType_GAUGE),
				Metric: []*dto.Metric{
					{
						Label: runsLabels,
						Gauge: &dto.Gauge{Value: p(float64(count))},
					},
				},
			},
		}

		if rateLimit != nil {
			families = append(families,
				&dto.MetricFamily{
					Name: p("github_api_rate_limit_remaining"),
					Help: p("GitHub API rate limit remaining"),
					Type: p(dto.MetricType_GAUGE),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{{Name: p("resource"), Value: p("core")}},
							Gauge: &dto.Gauge{Value: p(float64(rateLimit.Remaining))},
						},
					},
				},
				&dto.MetricFamily{
					Name: p("github_api_rate_limit_limit"),
					Help: p("GitHub API rate limit"),
					Type: p(dto.MetricType_GAUGE),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{{Name: p("resource"), Value: p("core")}},
							Gauge: &dto.Gauge{Value: p(float64(rateLimit.Limit))},
						},
					},
				},
			)
		}

		w.Header().Set("Content-Type", string(expfmt.NewFormat(expfmt.TypeTextPlain)))
		encoder := expfmt.NewEncoder(w, expfmt.NewFormat(expfmt.TypeTextPlain))
		for _, family := range families {
			_ = encoder.Encode(family)
		}
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
	})

	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen: %+v", err)
	}

	server := &http.Server{
		Handler: mux,
	}
	server.SetKeepAlivesEnabled(keepAlive)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v\n%s", err, debug.Stack())
			}
		}()
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("failed to listen: %+v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	<-quit
	time.Sleep(lameduck)

	ctx, cancel := context.WithTimeout(context.Background(), terminationGracePeriod)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("failed to shutdown: %+v", err)
	}
}
