package grafana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/xerrors"
)

// Client is a Grafana API client
type Client struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
	username   string
	password   string
}

// NewClient creates a new Grafana client
func NewClient(baseURL string, apiKey string, username string, password string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: http.DefaultClient,
		apiKey:     apiKey,
		username:   username,
		password:   password,
	}
}

// DashboardRequest is the request body for creating/updating a dashboard
type DashboardRequest struct {
	Dashboard json.RawMessage `json:"dashboard"`
	FolderUID string          `json:"folderUid,omitempty"`
	Overwrite bool            `json:"overwrite"`
}

// DashboardResponse is the response from creating/updating a dashboard
type DashboardResponse struct {
	UID     string `json:"uid"`
	URL     string `json:"url"`
	Version int    `json:"version"`
	Status  string `json:"status"`
}

// UpsertDashboard creates or updates a dashboard
func (c *Client) UpsertDashboard(ctx context.Context, dashboardJSON []byte, folderUID string) (*DashboardResponse, error) {
	reqBody := DashboardRequest{
		Dashboard: dashboardJSON,
		FolderUID: folderUID,
		Overwrite: true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/dashboards/db", bytes.NewReader(body))
	if err != nil {
		return nil, xerrors.Errorf("failed to create request: %w", err)
	}

	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, xerrors.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, xerrors.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, xerrors.Errorf("grafana API error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var dashResp DashboardResponse
	if err := json.Unmarshal(respBody, &dashResp); err != nil {
		return nil, xerrors.Errorf("failed to unmarshal response: %w", err)
	}

	return &dashResp, nil
}

// DeleteDashboard deletes a dashboard by UID
func (c *Client) DeleteDashboard(ctx context.Context, uid string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/api/dashboards/uid/"+uid, nil)
	if err != nil {
		return xerrors.Errorf("failed to create request: %w", err)
	}

	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return xerrors.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 404 is acceptable (dashboard already deleted)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		respBody, _ := io.ReadAll(resp.Body)
		return xerrors.Errorf("grafana API error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	return nil
}

// GetFolderByTitle gets a folder by title and returns its UID
func (c *Client) GetFolderByTitle(ctx context.Context, title string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/folders", nil)
	if err != nil {
		return "", xerrors.Errorf("failed to create request: %w", err)
	}

	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", xerrors.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", xerrors.Errorf("grafana API error: status=%d", resp.StatusCode)
	}

	var folders []struct {
		UID   string `json:"uid"`
		Title string `json:"title"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&folders); err != nil {
		return "", xerrors.Errorf("failed to decode response: %w", err)
	}

	for _, f := range folders {
		if f.Title == title {
			return f.UID, nil
		}
	}

	return "", nil
}

// CreateFolder creates a folder and returns its UID
func (c *Client) CreateFolder(ctx context.Context, title string) (string, error) {
	body, _ := json.Marshal(map[string]string{"title": title})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/folders", bytes.NewReader(body))
	if err != nil {
		return "", xerrors.Errorf("failed to create request: %w", err)
	}

	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", xerrors.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", xerrors.Errorf("grafana API error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var folder struct {
		UID string `json:"uid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&folder); err != nil {
		return "", xerrors.Errorf("failed to decode response: %w", err)
	}

	return folder.UID, nil
}

// EnsureFolder ensures a folder exists and returns its UID
func (c *Client) EnsureFolder(ctx context.Context, title string) (string, error) {
	if title == "" {
		return "", nil
	}

	uid, err := c.GetFolderByTitle(ctx, title)
	if err != nil {
		return "", err
	}
	if uid != "" {
		return uid, nil
	}

	return c.CreateFolder(ctx, title)
}

func (c *Client) setAuth(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	} else if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
}
