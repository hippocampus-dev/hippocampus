## Checklist

DON'T use third party Operator.
DON'T use upstream manifests directly.

### Namespaces

- namespace.yaml
  - labels.name
- network_policy.yaml
  - default-deny
  - allow-envoy-stats-scrape
- istio-proxy(internal)
  - virtual_service.yaml
  - destination_rule.yaml
- istio-proxy(external)
  - service_entry.yaml
  - virtual_service.yaml
  - destination_rule.yaml

### Workload

- deployment.yaml
  - base
    - revisionHistoryLimit: 1
    - lifecycle if http workload
    - automountServiceAccountToken
    - securityContext
    - containerPorts
  - overlays
    - prometheus.io
    - replicas if HPA is not found
    - strategy
    - topologySpreadConstraints
    - resources.requests.cpu if HPA
    - fsGroup if that uses dedicated volume as PV
- stateful_set.yaml
  - base
    - podManagementPolicy
    - lifecycle if l7 workload
    - automountServiceAccountToken
    - securityContext
  - overlays
    - prometheus.io
    - replicas
    - updateStrategy
    - topologySpreadConstraints
    - resources.requests.cpu if HPA
    - volumeClaimTemplates
- daemon_set.yaml
  - base 
    - system-node-critical
    - tolerations
  - overlays
    - updateStrategy
- pod_disruption_budget.yaml
  - overlays
    - maxUnavailable
- istio-proxy
  - peer_authentication.yaml
  - sidecar.yaml
  - telemetry.yaml
- service.yaml
  - overlays
    - trafficDistribution: PreferClose
- horizontal_pod_autoscaler.yaml
  - base
    - metrics
  - overlays
    - minReplicas/maxReplicas
- ingressgateway
  - destination_rule.yaml
  - virtual_service.yaml
  - gateway.yaml
