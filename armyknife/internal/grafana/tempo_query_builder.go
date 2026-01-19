package grafana

import (
	"time"

	"golang.org/x/xerrors"
)

type TempoQueryBuilder struct {
	datasourceUID string
	refID         string
	query         string
	queryType     string
	limit         int
	serviceName   string
	spanName      string
	search        string
	minDuration   string
	maxDuration   string
}

func NewTempoQueryBuilder() *TempoQueryBuilder {
	return &TempoQueryBuilder{
		refID:     "A",
		queryType: "traceql",
		limit:     20,
	}
}

func (b *TempoQueryBuilder) WithDatasourceUID(uid string) *TempoQueryBuilder {
	b.datasourceUID = uid
	return b
}

func (b *TempoQueryBuilder) WithRefID(refID string) *TempoQueryBuilder {
	b.refID = refID
	return b
}

func (b *TempoQueryBuilder) WithQuery(query string) *TempoQueryBuilder {
	b.query = query
	return b
}

func (b *TempoQueryBuilder) WithQueryType(queryType string) *TempoQueryBuilder {
	b.queryType = queryType
	return b
}

func (b *TempoQueryBuilder) WithLimit(limit int) *TempoQueryBuilder {
	b.limit = limit
	return b
}

func (b *TempoQueryBuilder) WithServiceName(serviceName string) *TempoQueryBuilder {
	b.serviceName = serviceName
	return b
}

func (b *TempoQueryBuilder) WithSpanName(spanName string) *TempoQueryBuilder {
	b.spanName = spanName
	return b
}

func (b *TempoQueryBuilder) WithSearch(search string) *TempoQueryBuilder {
	b.search = search
	return b
}

func (b *TempoQueryBuilder) WithMinDuration(minDuration string) *TempoQueryBuilder {
	b.minDuration = minDuration
	return b
}

func (b *TempoQueryBuilder) WithMaxDuration(maxDuration string) *TempoQueryBuilder {
	b.maxDuration = maxDuration
	return b
}

func (b *TempoQueryBuilder) Build() (TempoQuery, error) {
	if b.query == "" && b.serviceName == "" && b.search == "" {
		return TempoQuery{}, xerrors.New("query, serviceName, or search is required")
	}

	return TempoQuery{
		RefID:         b.refID,
		Query:         b.query,
		QueryType:     b.queryType,
		DatasourceUID: b.datasourceUID,
		Limit:         b.limit,
		ServiceName:   b.serviceName,
		SpanName:      b.spanName,
		Search:        b.search,
		MinDuration:   b.minDuration,
		MaxDuration:   b.maxDuration,
	}, nil
}

func BuildTempoQueryRequest(queries []TempoQuery, from time.Time, to time.Time) QueryRequest {
	return buildQueryRequestWithQueries(queries, from, to, DefaultIntervalMs)
}
