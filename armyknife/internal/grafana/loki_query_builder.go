package grafana

import (
	"time"

	"golang.org/x/xerrors"
)

type LokiQueryBuilder struct {
	datasourceUID string
	refID         string
	expr          string
	queryType     string
	maxLines      int
	interval      string
	maxDataPoints int
	direction     string
	legendFormat  string
}

func NewLokiQueryBuilder() *LokiQueryBuilder {
	return &LokiQueryBuilder{
		refID:         "A",
		queryType:     "range",
		maxLines:      100,
		interval:      "30s",
		maxDataPoints: 300,
		direction:     "backward",
	}
}

func (b *LokiQueryBuilder) WithDatasourceUID(uid string) *LokiQueryBuilder {
	b.datasourceUID = uid
	return b
}

func (b *LokiQueryBuilder) WithRefID(refID string) *LokiQueryBuilder {
	b.refID = refID
	return b
}

func (b *LokiQueryBuilder) WithExpression(expr string) *LokiQueryBuilder {
	b.expr = expr
	return b
}

func (b *LokiQueryBuilder) WithQueryType(queryType string) *LokiQueryBuilder {
	b.queryType = queryType
	return b
}

func (b *LokiQueryBuilder) WithMaxLines(maxLines int) *LokiQueryBuilder {
	b.maxLines = maxLines
	return b
}

func (b *LokiQueryBuilder) WithInterval(interval string) *LokiQueryBuilder {
	b.interval = interval
	return b
}

func (b *LokiQueryBuilder) WithMaxDataPoints(points int) *LokiQueryBuilder {
	b.maxDataPoints = points
	return b
}

func (b *LokiQueryBuilder) WithDirection(direction string) *LokiQueryBuilder {
	b.direction = direction
	return b
}

func (b *LokiQueryBuilder) WithLegendFormat(legendFormat string) *LokiQueryBuilder {
	b.legendFormat = legendFormat
	return b
}

func (b *LokiQueryBuilder) Build() (LokiQuery, error) {
	if b.expr == "" {
		return LokiQuery{}, xerrors.New("expression is required")
	}

	intervalMs, err := parseIntervalToMs(b.interval)
	if err != nil {
		return LokiQuery{}, xerrors.Errorf("failed to parse interval: %w", err)
	}

	return LokiQuery{
		RefID:     b.refID,
		Expr:      b.expr,
		QueryType: b.queryType,
		Datasource: &QueryDatasource{
			UID:  b.datasourceUID,
			Type: DatasourceTypeLoki,
		},
		DatasourceUID: b.datasourceUID,
		MaxLines:      b.maxLines,
		IntervalMs:    intervalMs,
		MaxDataPoints: b.maxDataPoints,
		Direction:     b.direction,
		LegendFormat:  b.legendFormat,
	}, nil
}

func BuildLokiQueryRequest(queries []LokiQuery, from time.Time, to time.Time) QueryRequest {
	intervalMs := DefaultIntervalMs
	if len(queries) > 0 && queries[0].IntervalMs > 0 {
		intervalMs = queries[0].IntervalMs
	}

	return buildQueryRequestWithQueries(queries, from, to, intervalMs)
}
