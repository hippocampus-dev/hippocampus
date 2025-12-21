# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Fluentd aggregator service that provides custom log processing plugins for Kubernetes environments. It extends the official Fluentd v1.16 image with additional plugins for Prometheus, Grafana Loki, and Elasticsearch integration, plus four custom Ruby plugins.

## Common Development Commands

### Building the Docker Image
```bash
docker build -t fluentd-aggregator .
```

### Testing Plugins Locally
Since these are Fluentd plugins, testing requires a Fluentd environment:
```bash
# Run Fluentd with custom plugins
docker run -v $(pwd)/plugins:/fluentd/plugins fluentd-aggregator -c /path/to/test-config.conf
```

## Architecture

### Plugin System
The service implements four custom Fluentd plugins in Ruby:

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

### Integration Points
- **Prometheus**: Metrics export via fluent-plugin-prometheus v2.0.3
- **Grafana Loki**: Log aggregation via fluent-plugin-grafana-loki v1.2.20
- **Elasticsearch**: Search backend via fluent-plugin-elasticsearch v5.4.3

### Development Notes
- Plugins follow Fluentd's plugin architecture conventions
- All filters use `filter_stream` for processing records
- The output plugin uses `emit` for routing to different labels
- Plugins are loaded from `/fluentd/plugins` in the container