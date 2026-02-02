package grafana

import "time"

type ElasticsearchQueryBuilder struct {
	datasourceUID string
	refID         string
	query         string
	metrics       []ElasticsearchMetric
	bucketAggs    []ElasticsearchBucketAgg
	timeField     string
}

func NewElasticsearchQueryBuilder() *ElasticsearchQueryBuilder {
	return &ElasticsearchQueryBuilder{
		refID:     "A",
		query:     "*",
		timeField: "@timestamp",
	}
}

func (b *ElasticsearchQueryBuilder) WithDatasourceUID(uid string) *ElasticsearchQueryBuilder {
	b.datasourceUID = uid
	return b
}

func (b *ElasticsearchQueryBuilder) WithRefID(refID string) *ElasticsearchQueryBuilder {
	b.refID = refID
	return b
}

func (b *ElasticsearchQueryBuilder) WithQuery(query string) *ElasticsearchQueryBuilder {
	b.query = query
	return b
}

func (b *ElasticsearchQueryBuilder) WithMetrics(metrics []ElasticsearchMetric) *ElasticsearchQueryBuilder {
	b.metrics = metrics
	return b
}

func (b *ElasticsearchQueryBuilder) WithBucketAggs(bucketAggs []ElasticsearchBucketAgg) *ElasticsearchQueryBuilder {
	b.bucketAggs = bucketAggs
	return b
}

func (b *ElasticsearchQueryBuilder) WithTimeField(timeField string) *ElasticsearchQueryBuilder {
	b.timeField = timeField
	return b
}

func (b *ElasticsearchQueryBuilder) Build() (ElasticsearchQuery, error) {
	return ElasticsearchQuery{
		RefID:         b.refID,
		Query:         b.query,
		DatasourceUID: b.datasourceUID,
		Metrics:       b.metrics,
		BucketAggs:    b.bucketAggs,
		TimeField:     b.timeField,
	}, nil
}

func BuildElasticsearchQueryRequest(queries []ElasticsearchQuery, from time.Time, to time.Time) QueryRequest {
	return buildQueryRequestWithQueries(queries, from, to, DefaultIntervalMs)
}
