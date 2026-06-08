# Feature: Telemetry Collection

Status: implemented
Added: 2026-06-08

## Summary

Log ingestion pipeline for FR7–FR9. Applications send logs via the Go SDK to a dedicated ingestor (API key auth). Logs are committed to Kafka before `200 OK`, consumed into PostgreSQL, and broadcast on Redis for live WebSocket streaming. Retention is configurable (default 2 hours in dev).

## Architecture

```
Go SDK → Ingestor → Kafka → Consumer → PostgreSQL
                              └──────→ Redis pub/sub → WebSocket (API)
```

| Binary | Port | Role |
|--------|------|------|
| `cmd/api` | 8080 | JWT, logs by ID, WebSocket stream |
| `cmd/ingestor` | 8081 | API key auth, Kafka produce |
| `cmd/consumer` | — | Kafka consume, Postgres insert, retention |

## API Endpoints

### Ingestor (API key auth)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | /v1/logs | Bearer API key | Ingest one log entry |
| GET | /health | No | Health check |

Request:
```json
{
  "timestamp": "2026-06-08T10:00:00Z",
  "severity": "ERROR",
  "message": "payment failed",
  "metadata": { "order_id": "123" }
}
```

`timestamp` is optional (defaults to now). `severity`: DEBUG, INFO, WARN, ERROR, FATAL. `message` max 64KB.

Success:
```json
{
  "success": true,
  "data": { "id": "<log-uuid>" }
}
```

### API (JWT auth)

| Method | Path | Role | Description |
|--------|------|------|-------------|
| GET | /orgs/:id/logs/search | Member | Search logs (FR10–FR11) — see search.md |
| GET | /orgs/:id/logs/:logId | Member | Get log by ID, org scope (FR12) |
| GET | /orgs/:id/services/:serviceId/logs/:logId | Member | Get log by ID (FR8) |
| GET | /orgs/:id/services/:serviceId/logs/stream | Member | WebSocket live stream (FR9) |
| GET | /health | No | Health check |

WebSocket sends JSON log entries as text frames matching `LogEntry` schema.

## Ingestion flow

```
SDK                   Ingestor              Redis           Postgres        Kafka
 |                       |                   |                |              |
 |-- POST /v1/logs ----->|                   |                |              |
 |                       |-- prefix lookup ->|                |              |
 |                       |-- bcrypt verify   |                |              |
 |                       |-- produce ---------------------->|              |
 |                       |                   |                |              |-- ack
 |<-- 200 {id} ----------|                   |                |              |
```

API key resolution:
1. Extract prefix from `ll_<prefix>_<secret>`
2. Redis cache → Postgres on miss
3. bcrypt verify full key
4. Attach `service_id` + `org_id` from key row

On revoke/rotate, API invalidates `apikey:prefix:*` and sets `apikey:revoked:*` in Redis.

## Consumer flow

```
Kafka → Consumer → INSERT logs → COMMIT offset → Redis PUBLISH logs:service:{id}
```

Retention goroutine (in consumer):
- Runs every `LOG_RETENTION_INTERVAL_MINUTES` (default 15)
- Deletes logs where `timestamp < now() - LOG_RETENTION_HOURS`

## Go SDK

```go
import "loglens/sdk/go/loglens"

client := loglens.NewClient(os.Getenv("LOGLENS_API_KEY"))
_ = client.Error(ctx, "payment failed", map[string]any{"order_id": "123"})
```

Env: `INGESTOR_URL` or `LOGLENS_INGESTOR_URL` (default `http://localhost:8081`).

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| INGESTOR_PORT | 8081 | Ingestor HTTP port |
| INGESTOR_URL | http://localhost:8081 | SDK target URL |
| KAFKA_BROKERS | localhost:9092 | Kafka broker list |
| KAFKA_TOPIC_LOGS | loglens.logs | Log topic |
| KAFKA_CONSUMER_GROUP | loglens-consumer | Consumer group |
| LOG_RETENTION_HOURS | 2 | Max log age in Postgres |
| LOG_RETENTION_INTERVAL_MINUTES | 15 | Retention job frequency |
| API_KEY_CACHE_TTL_SECONDS | 300 | Prefix metadata cache TTL |
| API_KEY_VALIDATION_CACHE_TTL_SECONDS | 120 | Skip repeat bcrypt window |

## Database

Migration: `migrations/010_create_logs.sql`

```sql
logs (id, org_id, service_id, timestamp, severity, message, metadata JSONB,
      ingested_at, message_tsv tsvector)
```

Indexes: `(service_id, timestamp)`, `(org_id, timestamp)`, `(service_id, severity, timestamp)`, GIN FTS on `message_tsv`.

## Code map

| Layer | File | Responsibility |
|-------|------|----------------|
| Ingestor | cmd/ingestor/main.go | Ingestor bootstrap |
| Ingest handler | internal/ingest/handler.go | POST /v1/logs |
| Key lookup | internal/ingest/keylookup.go | Prefix → service adapter |
| API key cache | internal/auth/keycache.go | Redis + bcrypt validation |
| Models | internal/telemetry/models.go | LogEntry, severities |
| Kafka | internal/telemetry/kafka.go | Producer + consumer |
| Repository | internal/telemetry/repository.go | Postgres log CRUD |
| Pub/sub | internal/telemetry/pubsub.go | Redis publish |
| Consumer | cmd/consumer/main.go | Consume, store, retain |
| Log API | internal/telemetry/handler.go | GET log by ID |
| WebSocket | internal/stream/handler.go | Live stream |
| SDK | sdk/go/loglens/client.go | Client library |

## Running locally

```bash
docker compose up -d

docker exec -i loglens-postgres-1 psql -U loglens -d loglens < migrations/010_create_logs.sql

go run ./cmd/api        # :8080
go run ./cmd/ingestor   # :8081
go run ./cmd/consumer
```

## Example curls

```bash
# Ingest (API key from service management)
curl -s -X POST http://localhost:8081/v1/logs \
  -H "Authorization: Bearer ll_<prefix>_<secret>" \
  -H "Content-Type: application/json" \
  -d '{"severity":"INFO","message":"hello from payment service"}'

# Get log by ID (JWT)
curl -s http://localhost:8080/orgs/$ORG_ID/services/$SERVICE_ID/logs/$LOG_ID \
  -H "Authorization: Bearer $TOKEN"

# WebSocket stream (use wscat or browser)
# ws://localhost:8080/orgs/$ORG_ID/services/$SERVICE_ID/logs/stream
# Header: Authorization: Bearer $TOKEN
```

## Not yet implemented

- Batch ingest endpoint
- `last_used_at` update on API key use
- Metadata field search (message FTS only; see search.md)
- Partitioned logs table for long retention
- Ingestor/consumer Docker services in compose (run via `go run` for now)
