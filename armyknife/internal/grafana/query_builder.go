package grafana

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/xerrors"
)

type QueryBuilder struct {
	datasourceID  int64
	datasourceUID string
	refID         string
	expr          string
	instant       bool
	interval      string
	maxDataPoints int
}

func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{
		refID:         "A",
		instant:       false,
		interval:      "30s",
		maxDataPoints: 300,
	}
}

func (qb *QueryBuilder) WithDatasourceID(id int64) *QueryBuilder {
	qb.datasourceID = id
	return qb
}

func (qb *QueryBuilder) WithDatasourceUID(uid string) *QueryBuilder {
	qb.datasourceUID = uid
	return qb
}

func (qb *QueryBuilder) WithRefID(refID string) *QueryBuilder {
	qb.refID = refID
	return qb
}

func (qb *QueryBuilder) WithExpression(expr string) *QueryBuilder {
	qb.expr = expr
	return qb
}

func (qb *QueryBuilder) WithInstant(instant bool) *QueryBuilder {
	qb.instant = instant
	return qb
}

func (qb *QueryBuilder) WithInterval(interval string) *QueryBuilder {
	qb.interval = interval
	return qb
}

func (qb *QueryBuilder) WithMaxDataPoints(points int) *QueryBuilder {
	qb.maxDataPoints = points
	return qb
}

func (qb *QueryBuilder) Build() (Query, error) {
	if qb.expr == "" {
		return Query{}, xerrors.New("expression is required")
	}

	intervalMs, err := parseIntervalToMs(qb.interval)
	if err != nil {
		return Query{}, xerrors.Errorf("failed to parse interval: %w", err)
	}

	return Query{
		RefID:         qb.refID,
		Expr:          qb.expr,
		Range:         !qb.instant,
		Instant:       qb.instant,
		DatasourceID:  qb.datasourceID,
		DatasourceUID: qb.datasourceUID,
		IntervalMs:    intervalMs,
		MaxDataPoints: qb.maxDataPoints,
	}, nil
}

func BuildQueryRequest(queries []Query, from, to time.Time) QueryRequest {
	intervalMs := 30000
	if len(queries) > 0 && queries[0].IntervalMs > 0 {
		intervalMs = queries[0].IntervalMs
	}

	return QueryRequest{
		From:    from.Format(time.RFC3339),
		To:      to.Format(time.RFC3339),
		Queries: queries,
		Range: TimeRange{
			From: from,
			To:   to,
			Raw: RawTimeRange{
				From: "now-1h",
				To:   "now",
			},
		},
		Interval:      fmt.Sprintf("%dms", intervalMs),
		IntervalMs:    intervalMs,
		MaxDataPoints: 300,
	}
}

func parseIntervalToMs(interval string) (int, error) {
	interval = strings.TrimSpace(interval)
	if interval == "" {
		return 30000, nil
	}

	if strings.HasSuffix(interval, "ms") {
		var ms int
		_, err := fmt.Sscanf(interval, "%dms", &ms)
		return ms, err
	}

	if strings.HasSuffix(interval, "s") {
		var s int
		_, err := fmt.Sscanf(interval, "%ds", &s)
		return s * 1000, err
	}

	if strings.HasSuffix(interval, "m") {
		var m int
		_, err := fmt.Sscanf(interval, "%dm", &m)
		return m * 60 * 1000, err
	}

	if strings.HasSuffix(interval, "h") {
		var h int
		_, err := fmt.Sscanf(interval, "%dh", &h)
		return h * 60 * 60 * 1000, err
	}

	return 0, xerrors.Errorf("invalid interval format: %s", interval)
}
