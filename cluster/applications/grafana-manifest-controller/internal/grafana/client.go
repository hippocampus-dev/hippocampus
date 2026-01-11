package grafana

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"golang.org/x/xerrors"
)

// Client is a Grafana API client
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: http.DefaultClient,
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

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/dashboards/db", bytes.NewReader(body))
	if err != nil {
		return nil, xerrors.Errorf("failed to create request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, xerrors.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode >= http.StatusBadRequest {
		return nil, xerrors.Errorf("API error: status=%d", response.StatusCode)
	}

	var dashResp DashboardResponse
	if err := json.NewDecoder(response.Body).Decode(&dashResp); err != nil {
		return nil, xerrors.Errorf("failed to decode response: %w", err)
	}

	return &dashResp, nil
}

// DeleteDashboard deletes a dashboard by UID
func (c *Client) DeleteDashboard(ctx context.Context, uid string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/api/dashboards/uid/"+uid, nil)
	if err != nil {
		return xerrors.Errorf("failed to create request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return xerrors.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	// 404 is acceptable (dashboard already deleted)
	if response.StatusCode >= http.StatusBadRequest && response.StatusCode != http.StatusNotFound {
		return xerrors.Errorf("API error: status=%d", response.StatusCode)
	}

	return nil
}

// GetFolderByTitle gets a folder by title and returns its UID
func (c *Client) GetFolderByTitle(ctx context.Context, title string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/folders", nil)
	if err != nil {
		return "", xerrors.Errorf("failed to create request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", xerrors.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode >= http.StatusBadRequest {
		return "", xerrors.Errorf("API error: status=%d", response.StatusCode)
	}

	var folders []struct {
		UID   string `json:"uid"`
		Title string `json:"title"`
	}
	if err := json.NewDecoder(response.Body).Decode(&folders); err != nil {
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
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/folders", bytes.NewReader(body))
	if err != nil {
		return "", xerrors.Errorf("failed to create request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", xerrors.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(response.Body)
		return "", xerrors.Errorf("API error: status=%d, body=%s", response.StatusCode, string(body))
	}

	var folder struct {
		UID string `json:"uid"`
	}
	if err := json.NewDecoder(response.Body).Decode(&folder); err != nil {
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
