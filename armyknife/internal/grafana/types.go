package grafana

import "time"

type Datasource struct {
	ID              int64                  `json:"id"`
	UID             string                 `json:"uid"`
	OrgID           int64                  `json:"orgId"`
	Name            string                 `json:"name"`
	Type            string                 `json:"type"`
	TypeLogoURL     string                 `json:"typeLogoUrl"`
	Access          string                 `json:"access"`
	URL             string                 `json:"url"`
	Password        string                 `json:"password,omitempty"`
	User            string                 `json:"user,omitempty"`
	Database        string                 `json:"database,omitempty"`
	BasicAuth       bool                   `json:"basicAuth"`
	BasicAuthUser   string                 `json:"basicAuthUser,omitempty"`
	BasicAuthPass   string                 `json:"basicAuthPassword,omitempty"`
	WithCredentials bool                   `json:"withCredentials"`
	IsDefault       bool                   `json:"isDefault"`
	JSONData        map[string]interface{} `json:"jsonData"`
	SecureJSONData  map[string]interface{} `json:"secureJsonData,omitempty"`
	Version         int                    `json:"version"`
	ReadOnly        bool                   `json:"readOnly"`
}

type QueryRequest struct {
	From          string    `json:"from"`
	To            string    `json:"to"`
	Queries       []Query   `json:"queries"`
	Range         TimeRange `json:"range"`
	Interval      string    `json:"interval"`
	IntervalMs    int       `json:"intervalMs"`
	MaxDataPoints int       `json:"maxDataPoints"`
}

type Query struct {
	RefID         string `json:"refId"`
	Expr          string `json:"expr"`
	Range         bool   `json:"range"`
	Instant       bool   `json:"instant"`
	DatasourceID  int64  `json:"datasourceId,omitempty"`
	DatasourceUID string `json:"datasourceUid,omitempty"`
	IntervalMs    int    `json:"intervalMs"`
	MaxDataPoints int    `json:"maxDataPoints"`
	Format        string `json:"format,omitempty"`
}

type TimeRange struct {
	From time.Time    `json:"from"`
	To   time.Time    `json:"to"`
	Raw  RawTimeRange `json:"raw"`
}

type RawTimeRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type QueryResponse struct {
	Results map[string]QueryResult `json:"results"`
}

type QueryResult struct {
	RefID      string       `json:"refId"`
	Series     []TimeSeries `json:"series,omitempty"`
	Tables     []Table      `json:"tables,omitempty"`
	Dataframes []DataFrame  `json:"dataframes,omitempty"`
}

type TimeSeries struct {
	Target     string      `json:"target"`
	Datapoints [][]float64 `json:"datapoints"`
}

type Table struct {
	Columns []Column        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
}

type Column struct {
	Text string `json:"text"`
}

type DataFrame struct {
	Schema Schema          `json:"schema"`
	Data   [][]interface{} `json:"data"`
}

type Schema struct {
	RefID  string  `json:"refId"`
	Meta   Meta    `json:"meta"`
	Fields []Field `json:"fields"`
}

type Meta struct {
	TypeVersion  []int                  `json:"typeVersion"`
	Custom       map[string]interface{} `json:"custom"`
	Stats        []Stat                 `json:"stats"`
	PreferredVis string                 `json:"preferredVisualisationType"`
}

type Stat struct {
	DisplayName string      `json:"displayName"`
	Value       interface{} `json:"value"`
}

type Field struct {
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	TypeInfo TypeInfo               `json:"typeInfo"`
	Config   map[string]interface{} `json:"config"`
	Labels   map[string]string      `json:"labels,omitempty"`
}

type TypeInfo struct {
	Frame string `json:"frame"`
}
