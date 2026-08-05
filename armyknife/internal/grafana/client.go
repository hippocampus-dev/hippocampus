package grafana

import (
	"armyknife/internal/bakery"
	"armyknife/internal/retry"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/xerrors"
)

type Client struct {
	baseURL      string
	bakeryClient *bakery.Client
	httpClient   *http.Client
}

func NewClient(baseURL string, bakeryClient *bakery.Client) *Client {
	transport := &retry.Transport{
		Base:          http.DefaultTransport,
		RetryStrategy: retry.NewExponentialBackOff(1*time.Second, 30*time.Second, 3, nil),
		RetryOn:       retry.NewDefaultRetryOn(),
	}

	return &Client{
		baseURL:      baseURL,
		bakeryClient: bakeryClient,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	endpoint, err := url.JoinPath(c.baseURL, path)
	if err != nil {
		return nil, xerrors.Errorf("failed to join url: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, xerrors.Errorf("failed to create request: %w", err)
	}

	if c.bakeryClient != nil {
		token, err := c.bakeryClient.GetValue("_oauth2_proxy")
		if err != nil {
			return nil, xerrors.Errorf("failed to get token from bakery: %w", err)
		}
		request.AddCookie(&http.Cookie{
			Name:  "_oauth2_proxy",
			Value: token,
		})
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, xerrors.Errorf("failed to execute request: %w", err)
	}

	if response.StatusCode >= 400 {
		defer func() {
			_ = response.Body.Close()
		}()
		body, _ := io.ReadAll(response.Body)
		return nil, xerrors.Errorf("API error: status=%d, body=%s", response.StatusCode, string(body))
	}

	return response, nil
}

func (c *Client) GetDatasources(ctx context.Context) ([]Datasource, error) {
	response, err := c.doRequest(ctx, http.MethodGet, "/api/datasources", nil)
	if err != nil {
		return nil, xerrors.Errorf("failed to get datasources: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	var datasources []Datasource
	if err := json.NewDecoder(response.Body).Decode(&datasources); err != nil {
		return nil, xerrors.Errorf("failed to decode response: %w", err)
	}

	return datasources, nil
}

func (c *Client) QueryPrometheus(ctx context.Context, queries []Query, from time.Time, to time.Time) (*QueryResponse, error) {
	if len(queries) == 0 {
		return nil, xerrors.New("at least one query is required")
	}

	request := BuildPrometheusQueryRequest(queries, from, to)
	return c.executeQuery(ctx, request)
}

func (c *Client) executeQuery(ctx context.Context, request QueryRequest) (*QueryResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal request: %w", err)
	}

	response, err := c.doRequest(ctx, "POST", "/api/ds/query", bytes.NewReader(body))
	if err != nil {
		return nil, xerrors.Errorf("failed to execute query: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	var queryResponse QueryResponse
	if err := json.NewDecoder(response.Body).Decode(&queryResponse); err != nil {
		return nil, xerrors.Errorf("failed to decode response: %w", err)
	}

	return &queryResponse, nil
}

func (c *Client) getDatasourceByType(ctx context.Context, datasourceType string, name string) (*Datasource, error) {
	datasources, err := c.GetDatasources(ctx)
	if err != nil {
		return nil, err
	}

	for _, ds := range datasources {
		if ds.Type == datasourceType {
			if name == "" || ds.Name == name {
				return &ds, nil
			}
		}
	}

	if name != "" {
		return nil, xerrors.Errorf("%s datasource not found: %s", datasourceType, name)
	}
	return nil, xerrors.Errorf("%s datasource not found", datasourceType)
}

func (c *Client) GetPrometheusDatasource(ctx context.Context, name string) (*Datasource, error) {
	return c.getDatasourceByType(ctx, DatasourceTypePrometheus, name)
}

func (c *Client) GetLokiDatasource(ctx context.Context, name string) (*Datasource, error) {
	return c.getDatasourceByType(ctx, DatasourceTypeLoki, name)
}

func (c *Client) GetTempoDatasource(ctx context.Context, name string) (*Datasource, error) {
	return c.getDatasourceByType(ctx, DatasourceTypeTempo, name)
}

func (c *Client) GetPyroscopeDatasource(ctx context.Context, name string) (*Datasource, error) {
	return c.getDatasourceByType(ctx, DatasourceTypePyroscope, name)
}

func (c *Client) GetAlertmanagerDatasource(ctx context.Context, name string) (*Datasource, error) {
	return c.getDatasourceByType(ctx, DatasourceTypeAlertmanager, name)
}

func (c *Client) GetElasticsearchDatasource(ctx context.Context, name string) (*Datasource, error) {
	return c.getDatasourceByType(ctx, DatasourceTypeElasticsearch, name)
}

func (c *Client) QueryLoki(ctx context.Context, queries []LokiQuery, from time.Time, to time.Time) (*QueryResponse, error) {
	if len(queries) == 0 {
		return nil, xerrors.New("at least one query is required")
	}

	request := BuildLokiQueryRequest(queries, from, to)
	return c.executeQuery(ctx, request)
}

func (c *Client) QueryTempo(ctx context.Context, queries []TempoQuery, from time.Time, to time.Time) (*QueryResponse, error) {
	if len(queries) == 0 {
		return nil, xerrors.New("at least one query is required")
	}

	request := BuildTempoQueryRequest(queries, from, to)
	return c.executeQuery(ctx, request)
}

func (c *Client) QueryPyroscope(ctx context.Context, queries []PyroscopeQuery, from time.Time, to time.Time) (*QueryResponse, error) {
	if len(queries) == 0 {
		return nil, xerrors.New("at least one query is required")
	}

	request := BuildPyroscopeQueryRequest(queries, from, to)
	return c.executeQuery(ctx, request)
}

func (c *Client) QueryElasticsearch(ctx context.Context, queries []ElasticsearchQuery, from time.Time, to time.Time) (*QueryResponse, error) {
	if len(queries) == 0 {
		return nil, xerrors.New("at least one query is required")
	}

	request := BuildElasticsearchQueryRequest(queries, from, to)
	return c.executeQuery(ctx, request)
}
