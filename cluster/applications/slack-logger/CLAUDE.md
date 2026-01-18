# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Slack Logger is a Go-based HTTP server that receives Slack chat messages via Events API, encrypts them using channel-specific keys, and stores them in a MySQL database. It provides a Slack Web API-compatible interface for retrieving messages.

## Development Commands

### Build and Run
- `make dev` - Run the application with hot-reload using watchexec
- `go run *.go` - Run the application directly

### Code Quality
- `make fmt` - Format code using `go fmt` and `goimports`
- `make lint` - Run `go vet` for static analysis
- `make tidy` - Update and clean up go.mod dependencies
- `make test` - Run tests with race detector and benchmarks
- `make all` - Run fmt, lint, tidy, and test in sequence

### Database Management
- `make generate` - Generate database code using Atlas and SQLC (validates first)
- `make validate` - Validate migrations and check SQLC schema compatibility
- `make migrate` - Apply database migrations to remote database (requires environment variables)
- Run a specific test: `go test -run TestName ./...`

### Environment Variables
Required for database operations:
- `MYSQL_USER` - MySQL username
- `MYSQL_PASSWORD` - MySQL password
- `MYSQL_ADDRESS` - MySQL server address
- `MYSQL_DATABASE` - Database name

Application configuration:
- `ADDRESS` - Server listen address (default: `0.0.0.0:8080`)
- `TERMINATION_GRACE_PERIOD` - Graceful shutdown timeout
- `PYROSCOPE_ENDPOINT` - Profiling endpoint (optional)

## Project Architecture

### Directory Structure
- `/internal/db/` - Database models and SQLC-generated code
- `/internal/encryption/` - AES-256-GCM encryption implementation
- `/internal/routes/` - HTTP route handlers (events API, conversations history)
- `/internal/storage/` - MySQL storage implementation
- `/internal/types/` - Shared type definitions
- `/queries/` - SQL queries for SQLC
- `/migrations/` - Atlas database migrations

### Key Technologies
- **Database**: MySQL with Atlas for migrations and SQLC for type-safe queries
- **Encryption**: AES-256-GCM using channel IDs as key source
- **Observability**: OpenTelemetry tracing, Prometheus metrics, Pyroscope profiling
- **Development**: watchexec for hot-reload, dotenvx for environment management

### API Endpoints
- `POST /slack/events` - Receives Slack Events API webhooks
- `POST /api/conversations.history` - Retrieves encrypted messages (Slack-compatible)
- `GET /metrics` - Prometheus metrics
- `GET /healthz` - Health check

### Security Design
Messages are encrypted using AES-256-GCM with keys derived from Slack channel IDs:
1. Channel ID → SHA-256 → 32-byte encryption key
2. Each message gets a unique 12-byte nonce
3. Channel isolation ensures messages can only be decrypted with the correct channel ID

## Development Workflow

1. Use `make dev` for local development with auto-reload
2. Ensure MySQL is running and configured via environment variables
3. Run `make validate` before making database schema changes
4. Use `make generate` to update database code after schema changes
5. Run `make all` before committing to ensure code quality

## Change Data Capture (CDC)

### Overview
Slack Logger uses TiCDC to capture database changes in real-time and publish them to Kafka for downstream processing and logging.

### Architecture
```
TiDB → TiCDC → Kafka → KafkaSource → cloudevents-logger → Logs
```

### Components
1. **TiCDC**: Captures changes from `slack_logger.encrypted_messages` table
2. **Kafka Cluster**: Message broker (Strimzi-managed, 1 replica for dev)
3. **Kafka Topic**: `slack-logger-cdc` (3 partitions, 7-day retention)
4. **KafkaSource**: Knative Eventing component that reads from Kafka
5. **cloudevents-logger**: 既存の Knative Service (knative-eventing namespace) で CDC イベントをログ

### CDC Commands

#### Check TiCDC Status
```bash
kubectl exec -it slack-logger-ticdc-0 -n slack-logger -- \
  /cdc cli capture list --server=http://127.0.0.1:8301
```

#### List Changefeeds
```bash
kubectl exec -it slack-logger-ticdc-0 -n slack-logger -- \
  /cdc cli changefeed list --server=http://127.0.0.1:8301
```

#### Query Changefeed Details
```bash
kubectl exec -it slack-logger-ticdc-0 -n slack-logger -- \
  /cdc cli changefeed query --server=http://127.0.0.1:8301 \
  --changefeed-id=slack-logger-to-kafka
```

#### View CDC Logs
```bash
# Follow cloudevents-logger logs (CDC events will appear here)
kubectl logs -n knative-eventing -l app.kubernetes.io/name=cloudevents-logger -f

# Check KafkaSource status
kubectl get kafkasource slack-logger-cdc -n slack-logger
```

#### Debug Kafka
```bash
# Check Kafka cluster status
kubectl get kafka slack-logger -n slack-logger

# Check Kafka topic
kubectl get kafkatopic slack-logger-cdc -n slack-logger

# View messages in Kafka topic (requires kafka client pod)
kubectl run kafka-consumer -n slack-logger --rm -i --tty \
  --image=quay.io/strimzi/kafka:latest-kafka-3.9.0 -- \
  /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server slack-logger-kafka-bootstrap:9092 \
  --topic slack-logger-cdc \
  --from-beginning
```

### CDC Event Format
TiCDC publishes events in Canal-JSON format:
```json
{
  "id": 1,
  "database": "slack_logger",
  "table": "encrypted_messages",
  "type": "INSERT",
  "ts": 1234567890,
  "data": [{
    "id": 123,
    "channel_name": "general",
    "message_ts": "1234567890.123456",
    ...
  }]
}
```

### Troubleshooting

1. **Changefeed not running**: Check TiCDC Pod logs
   ```bash
   kubectl logs -n slack-logger slack-logger-ticdc-0
   ```

2. **No events in Kafka**: Verify changefeed is running and try inserting test data
   ```bash
   # Insert test message via slack-logger API
   # Then check changefeed status
   kubectl exec -it slack-logger-ticdc-0 -n slack-logger -- \
     /cdc cli changefeed query --server=http://127.0.0.1:8301 \
     --changefeed-id=slack-logger-to-kafka
   ```

3. **cloudevents-logger not receiving events**: Check KafkaSource status
   ```bash
   kubectl get kafkasource slack-logger-cdc -n slack-logger
   kubectl describe kafkasource slack-logger-cdc -n slack-logger
   kubectl get ksvc cloudevents-logger -n knative-eventing
   ```
