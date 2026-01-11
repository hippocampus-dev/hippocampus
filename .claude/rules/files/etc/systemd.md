---
paths:
  - "files/etc/systemd/**/*.service"
---

* `Requires=` and `After=` are always used as a pair for dependencies
* When ExecStop requires network access, add `Before=network.target` to ensure network remains available during shutdown
* `After=network-online.target` only controls startup order; it does not guarantee network availability during shutdown

## Dependency Patterns

| Dependency Type | Requires | After | Before |
|-----------------|----------|-------|--------|
| Network + ExecStop with network | network-online.target | network-online.target | network.target |
| Network + no ExecStop | network-online.target | network-online.target | - |
| Other service | {service}.service | {service}.service | - |
| Docker | docker.service | docker.service | - |

## Network-Dependent Stop

| Directive | Effect on Start | Effect on Stop |
|-----------|-----------------|----------------|
| `After=network-online.target` | Waits for network | No effect |
| `Before=network.target` | No effect | Stops before network |

Both directives are required when ExecStop performs network operations:

```ini
[Unit]
Requires=network-online.target
After=network-online.target
Before=network.target
```
