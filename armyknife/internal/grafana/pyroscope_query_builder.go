package grafana

import (
	"time"

	"golang.org/x/xerrors"
)

type PyroscopeQueryBuilder struct {
	datasourceUID string
	refID         string
	labelSelector string
	profileTypeID string
	groupBy       []string
	maxNodes      int
}

func NewPyroscopeQueryBuilder() *PyroscopeQueryBuilder {
	return &PyroscopeQueryBuilder{
		refID:         "A",
		profileTypeID: "process_cpu:cpu:nanoseconds:cpu:nanoseconds",
		maxNodes:      16384,
	}
}

func (b *PyroscopeQueryBuilder) WithDatasourceUID(uid string) *PyroscopeQueryBuilder {
	b.datasourceUID = uid
	return b
}

func (b *PyroscopeQueryBuilder) WithRefID(refID string) *PyroscopeQueryBuilder {
	b.refID = refID
	return b
}

func (b *PyroscopeQueryBuilder) WithLabelSelector(labelSelector string) *PyroscopeQueryBuilder {
	b.labelSelector = labelSelector
	return b
}

func (b *PyroscopeQueryBuilder) WithProfileTypeID(profileTypeID string) *PyroscopeQueryBuilder {
	b.profileTypeID = profileTypeID
	return b
}

func (b *PyroscopeQueryBuilder) WithGroupBy(groupBy []string) *PyroscopeQueryBuilder {
	b.groupBy = groupBy
	return b
}

func (b *PyroscopeQueryBuilder) WithMaxNodes(maxNodes int) *PyroscopeQueryBuilder {
	b.maxNodes = maxNodes
	return b
}

func (b *PyroscopeQueryBuilder) Build() (PyroscopeQuery, error) {
	if b.profileTypeID == "" {
		return PyroscopeQuery{}, xerrors.New("profileTypeId is required")
	}

	return PyroscopeQuery{
		RefID:         b.refID,
		LabelSelector: b.labelSelector,
		ProfileTypeID: b.profileTypeID,
		DatasourceUID: b.datasourceUID,
		GroupBy:       b.groupBy,
		MaxNodes:      b.maxNodes,
	}, nil
}

func BuildPyroscopeQueryRequest(queries []PyroscopeQuery, from time.Time, to time.Time) QueryRequest {
	return buildQueryRequestWithQueries(queries, from, to, DefaultIntervalMs)
}
