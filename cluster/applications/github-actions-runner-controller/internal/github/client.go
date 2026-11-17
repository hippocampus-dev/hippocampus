package github

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/xerrors"
)

const (
	apiURL = "https://api.github.com"

	StatusCompleted  = "completed"
	StatusInProgress = "in_progress"

	jobsPerPage = 100
)

type WorkflowRun struct {
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	RunAttempt int    `json:"run_attempt"`
}

type WorkflowJob struct {
	ID         int64  `json:"id"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	RunnerName string `json:"runner_name"`
}

type workflowJobsResponse struct {
	TotalCount int           `json:"total_count"`
	Jobs       []WorkflowJob `json:"jobs"`
}

type Client struct {
	httpClient *http.Client
	token      string
}

func NewClientWithPersonalAccessToken(token string) *Client {
	return &Client{httpClient: http.DefaultClient, token: token}
}

func NewClientWithGitHubApp(ctx context.Context, clientID string, installationID string, privateKey string) (*Client, error) {
	block, _ := pem.Decode([]byte(privateKey))
	if block == nil {
		return nil, xerrors.New("failed to decode private key")
	}
	rsaPrivateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse private key: %w", err)
	}

	now := time.Now()
	jwtToken, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(10 * time.Minute).Unix(),
		"iss": clientID,
	}).SignedString(rsaPrivateKey)
	if err != nil {
		return nil, xerrors.Errorf("failed to sign token: %w", err)
	}

	// Scoped down the way createTokenSecret in internal/controllers/runner_controller.go scopes its own, since an unscoped body mints a token carrying every permission the installation holds.
	requestBody, err := json.Marshal(struct {
		Permissions map[string]string `json:"permissions"`
	}{Permissions: map[string]string{"actions": "write", "metadata": "read"}})
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal body: %w", err)
	}

	client := &Client{httpClient: http.DefaultClient, token: jwtToken}
	var accessToken struct {
		Token string `json:"token"`
	}
	status, body, err := client.do(ctx, http.MethodPost, fmt.Sprintf("/app/installations/%s/access_tokens", installationID), requestBody, &accessToken)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, xerrors.Errorf("API error: status=%d, body=%s", status, body)
	}

	return &Client{httpClient: http.DefaultClient, token: accessToken.Token}, nil
}

// A run GitHub no longer holds leaves the returned run nil, while a refusal stays an error, since GitHub answers an exhausted rate limit with the same 403 as a credential missing the permission.
func (c *Client) GetWorkflowRun(ctx context.Context, owner string, repo string, runID int64) (*WorkflowRun, error) {
	var run WorkflowRun
	status, body, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/actions/runs/%d", owner, repo, runID), nil, &run)
	if err != nil {
		return nil, err
	}
	switch {
	case status == http.StatusNotFound || status == http.StatusGone:
		return nil, nil
	case status >= 400:
		return nil, xerrors.Errorf("API error: status=%d, body=%s", status, body)
	}
	return &run, nil
}

// Returns every job of the run's latest attempt, which carries a job the attempt did not re-run under a new identifier while keeping the runner name it had.
func (c *Client) ListWorkflowRunJobs(ctx context.Context, owner string, repo string, runID int64) ([]WorkflowJob, error) {
	var jobs []WorkflowJob
	for page := 1; ; page++ {
		var response workflowJobsResponse
		status, body, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/actions/runs/%d/jobs?per_page=%d&page=%d", owner, repo, runID, jobsPerPage, page), nil, &response)
		if err != nil {
			return nil, err
		}
		switch {
		case status == http.StatusNotFound || status == http.StatusGone:
			return nil, nil
		case status >= 400:
			return nil, xerrors.Errorf("API error: status=%d, body=%s", status, body)
		}
		jobs = append(jobs, response.Jobs...)
		if len(response.Jobs) == 0 || page*jobsPerPage >= response.TotalCount {
			return jobs, nil
		}
	}
}

// GitHub keeps a run whose runner stopped answering in progress until the job's timeout-minutes elapses, so the rerun waits behind this rather than behind that timeout.
// Reports whether GitHub accepted the cancel, since it answers a run it will not cancel right now with 409 and the next pass can ask again.
func (c *Client) ForceCancelWorkflowRun(ctx context.Context, owner string, repo string, runID int64) (bool, string, error) {
	status, body, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/actions/runs/%d/force-cancel", owner, repo, runID), nil, nil)
	if err != nil {
		return false, "", err
	}
	switch {
	case status == http.StatusConflict || status == http.StatusForbidden || status == http.StatusNotFound || status == http.StatusGone:
		return false, fmt.Sprintf("status=%d, body=%s", status, body), nil
	case status >= 400:
		return false, "", xerrors.Errorf("API error: status=%d, body=%s", status, body)
	}
	return true, "", nil
}

// Reports whether GitHub accepted the rerun and, where it refused, what it answered, since a credential without write on Actions and a job identifier a newer attempt has replaced are both refusals no retry changes and only that answer tells them apart.
func (c *Client) RerunWorkflowJob(ctx context.Context, owner string, repo string, jobID int64) (bool, string, error) {
	status, body, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/actions/jobs/%d/rerun", owner, repo, jobID), nil, nil)
	if err != nil {
		return false, "", err
	}
	switch {
	case status == http.StatusNotFound || status == http.StatusForbidden || status == http.StatusGone:
		return false, fmt.Sprintf("status=%d, body=%s", status, body), nil
	case status >= 400:
		return false, "", xerrors.Errorf("API error: status=%d, body=%s", status, body)
	}
	return true, "", nil
}

func (c *Client) do(ctx context.Context, method string, path string, requestBody []byte, result any) (int, string, error) {
	var reader io.Reader
	if requestBody != nil {
		reader = bytes.NewReader(requestBody)
	}
	request, err := http.NewRequestWithContext(ctx, method, apiURL+path, reader)
	if err != nil {
		return 0, "", xerrors.Errorf("failed to create request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return 0, "", xerrors.Errorf("failed to do request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()

	if response.StatusCode >= 400 {
		responseBody, _ := io.ReadAll(response.Body)
		return response.StatusCode, string(responseBody), nil
	}
	if result == nil {
		return response.StatusCode, "", nil
	}
	if err := json.NewDecoder(response.Body).Decode(result); err != nil {
		return response.StatusCode, "", xerrors.Errorf("failed to decode response: %w", err)
	}
	return response.StatusCode, "", nil
}
