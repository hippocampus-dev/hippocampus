---
paths:
  - "cluster/manifests/prometheus-adapter/**/config.yaml"
---

* Wrap `metricsQuery` with `round()` if the PromQL expression can return decimal values

## Custom Metrics for HPA

Decimal values in custom metrics cause confusing milli-unit display in HPA (e.g., `143750m/20` instead of `144/20`).

| PromQL Pattern | Can Return Decimals | Action |
|----------------|---------------------|--------|
| `rate()`, `irate()` | Yes | Add `round()` |
| `max_over_time()`, `avg_over_time()` | Yes | Add `round()` |
| Quantile metrics (`quantile="..."`) | Yes | Add `round()` |
| Counter/Gauge direct query | Typically no | Optional |

Example: `round(sum(max_over_time(<<.Series>>{<<.LabelMatchers>>}[2m])) by (<<.GroupBy>>))`
