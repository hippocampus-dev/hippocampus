# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Fluentd aggregator service that provides custom log processing plugins for Kubernetes environments. It extends the official Fluentd v1.16 image with additional plugins for Prometheus, Grafana Loki, and Elasticsearch integration, plus five custom Ruby plugins.

## Common Development Commands

### Building the Docker Image
```bash
docker build -t fluentd-aggregator .
```

### Testing Plugins Locally
The `test/` directory contains a docker-compose environment for testing plugins:
```bash
cd test
docker compose up --build
```

This starts a mock Loki server and Fluentd with the custom plugins. The mock server logs all received requests for verification.

Alternatively, run Fluentd directly with custom plugins:
```bash
docker run -v $(pwd)/plugins:/fluentd/plugins fluentd-aggregator -c /path/to/test-config.conf
```

## Architecture

### Plugin System
The service implements five custom Fluentd plugins in Ruby:

1. **filter_metadata.rb** - Enriches logs with Kubernetes metadata
   - Adds `grouping` field based on Kubernetes labels
   - Prioritizes app identification from standard Kubernetes labels

2. **filter_secret.rb** - Security filter for credential redaction
   - Redacts GitHub tokens (gh[pousr]_* patterns)
   - Redacts OpenAI API keys (sk-* patterns)
   - Prevents credential exposure in logs

3. **filter_structural_json.rb** - JSON parsing filter
   - Converts JSON-formatted messages to structured data
   - Creates `structural_message` field for parsed JSON
   - Removes original message to avoid duplication

4. **out_relabel_filter.rb** - Conditional routing output plugin
   - Routes logs to different pipelines based on patterns
   - Supports regex matching and inverted logic
   - Enables complex log routing workflows

5. **out_loki_historical.rb** - Historical log output plugin for Loki
   - Inherits from `out_loki` and registers as `loki_historical`
   - Adds a version label (yyyymmddhh format) based on Fluentd event time
   - Uses `Time.at(time).utc` to convert Fluent::EventTime to UTC timestamp before formatting
   - Configurable via `historical_label_key` (default: `version`)
   - Used as secondary output for logs rejected by Loki due to "too far behind" errors

### Integration Points
- **Prometheus**: Metrics export via fluent-plugin-prometheus v2.0.3
- **Grafana Loki**: Log aggregation via fluent-plugin-grafana-loki v1.2.20 (patched for historical log handling)
- **Elasticsearch**: Search backend via fluent-plugin-elasticsearch v5.4.3

### Patches
- **out_loki_unrecoverable_error.patch**: Modifies the Loki output plugin to raise `Fluent::UnrecoverableError` when Loki rejects logs with 400 errors containing "too far behind" or "out of order" messages. This enables routing historical logs to a secondary output (`loki_historical`) instead of silently dropping them.

### Development Notes
- Plugins follow Fluentd's plugin architecture conventions
- All filters use `filter_stream` for processing records
- The output plugin uses `emit` for routing to different labels
- Plugins are loaded from `/fluentd/plugins` in the container
