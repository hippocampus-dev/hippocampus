package grafana

import (
	"time"

	"golang.org/x/xerrors"
)

type PrometheusQueryBuilder struct {
	datasourceID  int64
	datasourceUID string
	refID         string
	expr          string
	instant       bool
	interval      string
	maxDataPoints int
	legendFormat  string
	exemplar      bool
}

func NewPrometheusQueryBuilder() *PrometheusQueryBuilder {
	return &PrometheusQueryBuilder{
		refID:         "A",
		instant:       false,
		interval:      "30s",
		maxDataPoints: 300,
	}
}

func (b *PrometheusQueryBuilder) WithDatasourceID(id int64) *PrometheusQueryBuilder {
	b.datasourceID = id
	return b
}

func (b *PrometheusQueryBuilder) WithDatasourceUID(uid string) *PrometheusQueryBuilder {
	b.datasourceUID = uid
	return b
}

func (b *PrometheusQueryBuilder) WithRefID(refID string) *PrometheusQueryBuilder {
	b.refID = refID
	return b
}

func (b *PrometheusQueryBuilder) WithExpression(expr string) *PrometheusQueryBuilder {
	b.expr = expr
	return b
}

func (b *PrometheusQueryBuilder) WithInstant(instant bool) *PrometheusQueryBuilder {
	b.instant = instant
	return b
}

func (b *PrometheusQueryBuilder) WithInterval(interval string) *PrometheusQueryBuilder {
	b.interval = interval
	return b
}

func (b *PrometheusQueryBuilder) WithMaxDataPoints(points int) *PrometheusQueryBuilder {
	b.maxDataPoints = points
	return b
}

func (b *PrometheusQueryBuilder) WithLegendFormat(legendFormat string) *PrometheusQueryBuilder {
	b.legendFormat = legendFormat
	return b
}

func (b *PrometheusQueryBuilder) WithExemplar(exemplar bool) *PrometheusQueryBuilder {
	b.exemplar = exemplar
	return b
}

func (b *PrometheusQueryBuilder) Build() (Query, error) {
	if b.expr == "" {
		return Query{}, xerrors.New("expression is required")
	}

	intervalMs, err := parseIntervalToMs(b.interval)
	if err != nil {
		return Query{}, xerrors.Errorf("failed to parse interval: %w", err)
	}

	return Query{
		RefID:         b.refID,
		Expr:          b.expr,
		Range:         !b.instant,
		Instant:       b.instant,
		DatasourceID:  b.datasourceID,
		DatasourceUID: b.datasourceUID,
		IntervalMs:    intervalMs,
		MaxDataPoints: b.maxDataPoints,
		LegendFormat:  b.legendFormat,
		Exemplar:      b.exemplar,
	}, nil
}

func BuildPrometheusQueryRequest(queries []Query, from time.Time, to time.Time) QueryRequest {
	intervalMs := DefaultIntervalMs
	if len(queries) > 0 && queries[0].IntervalMs > 0 {
		intervalMs = queries[0].IntervalMs
	}

	return buildQueryRequestWithQueries(queries, from, to, intervalMs)
}
