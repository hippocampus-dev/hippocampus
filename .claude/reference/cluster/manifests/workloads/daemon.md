# Daemon Workload

Node-level agents that run on every node.

## When to Use

- Log collectors (Fluentd, Fluent Bit)
- Metrics exporters (node-exporter, cAdvisor)
- Network agents (CNI components, eBPF tools)
- Security agents, monitoring agents

## Example

MUST copy from: `cluster/manifests/additional-cadvisor/`

## Files

| Directory | File | Purpose |
|-----------|------|---------|
| base/ | daemon_set.yaml | Pod template with tolerations |

Note: DaemonSets do not support PodDisruptionBudget as they run on every node by design.

## Key Modifications

- `kustomization.yaml`: Update image name and digest
- `daemon_set.yaml`: Update labels, container name, hostPath mounts
- `tolerations`: Adjust based on which nodes to run on
