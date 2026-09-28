package grafana

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/xerrors"
)

const DefaultIntervalMs = 30000

func buildQueryRequestWithQueries(queries interface{}, from time.Time, to time.Time, intervalMs int) QueryRequest {
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
		return DefaultIntervalMs, nil
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
