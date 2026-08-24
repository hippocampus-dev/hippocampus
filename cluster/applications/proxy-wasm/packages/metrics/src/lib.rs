#[derive(Clone, Debug, PartialEq, Eq, std::hash::Hash, serde::Serialize, serde::Deserialize)]
pub enum MetricType {
    Counter = 0,
    Gauge = 1,
    Histogram = 2,
}

#[derive(Clone, Debug, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct Metric {
    pub name: String,
    pub label: std::collections::HashMap<String, String>,
    pub metric_type: MetricType,
}

impl std::hash::Hash for Metric {
    fn hash<H: std::hash::Hasher>(&self, state: &mut H) {
        self.name.hash(state);
        for (k, v) in &self.label {
            k.hash(state);
            v.hash(state);
        }
        self.metric_type.hash(state);
    }
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub enum Operation {
    IncrementMetric(i64),
    RecordMetric(u64),
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct Task {
    pub operation: Operation,
    pub metric: Metric,
}
