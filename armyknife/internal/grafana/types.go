package grafana

import "time"

const (
	DatasourceTypePrometheus    = "prometheus"
	DatasourceTypeLoki          = "loki"
	DatasourceTypeTempo         = "tempo"
	DatasourceTypePyroscope     = "grafana-pyroscope-datasource"
	DatasourceTypeAlertmanager  = "alertmanager"
	DatasourceTypeElasticsearch = "elasticsearch"
)

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
	From          string      `json:"from"`
	To            string      `json:"to"`
	Queries       interface{} `json:"queries"`
	Range         TimeRange   `json:"range"`
	Interval      string      `json:"interval"`
	IntervalMs    int         `json:"intervalMs"`
	MaxDataPoints int         `json:"maxDataPoints"`
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
	LegendFormat  string `json:"legendFormat,omitempty"`
	Exemplar      bool   `json:"exemplar,omitempty"`
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

type LokiQuery struct {
	RefID         string `json:"refId"`
	Expr          string `json:"expr"`
	QueryType     string `json:"queryType"`
	DatasourceUID string `json:"datasourceUid,omitempty"`
	MaxLines      int    `json:"maxLines,omitempty"`
	IntervalMs    int    `json:"intervalMs,omitempty"`
	MaxDataPoints int    `json:"maxDataPoints,omitempty"`
	Direction     string `json:"direction,omitempty"`
	LegendFormat  string `json:"legendFormat,omitempty"`
}

type TempoQuery struct {
	RefID         string `json:"refId"`
	Query         string `json:"query"`
	QueryType     string `json:"queryType"`
	DatasourceUID string `json:"datasourceUid,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	ServiceName   string `json:"serviceName,omitempty"`
	SpanName      string `json:"spanName,omitempty"`
	Search        string `json:"search,omitempty"`
	MinDuration   string `json:"minDuration,omitempty"`
	MaxDuration   string `json:"maxDuration,omitempty"`
}

type PyroscopeQuery struct {
	RefID         string   `json:"refId"`
	LabelSelector string   `json:"labelSelector"`
	ProfileTypeID string   `json:"profileTypeId"`
	DatasourceUID string   `json:"datasourceUid,omitempty"`
	GroupBy       []string `json:"groupBy,omitempty"`
	MaxNodes      int      `json:"maxNodes,omitempty"`
}

type ElasticsearchQuery struct {
	RefID         string                   `json:"refId"`
	Query         string                   `json:"query"`
	DatasourceUID string                   `json:"datasourceUid,omitempty"`
	Metrics       []ElasticsearchMetric    `json:"metrics,omitempty"`
	BucketAggs    []ElasticsearchBucketAgg `json:"bucketAggs,omitempty"`
	TimeField     string                   `json:"timeField,omitempty"`
}

type ElasticsearchMetric struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Field string `json:"field,omitempty"`
}

type ElasticsearchBucketAgg struct {
	ID       string                          `json:"id"`
	Type     string                          `json:"type"`
	Field    string                          `json:"field,omitempty"`
	Settings *ElasticsearchBucketAggSettings `json:"settings,omitempty"`
}

type ElasticsearchBucketAggSettings struct {
	Interval    string `json:"interval,omitempty"`
	MinDocCount string `json:"min_doc_count,omitempty"`
}
