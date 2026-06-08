# Feature: Search & Investigation

Status: implemented
Added: 2026-06-08

## Summary

Structured log search for FR10–FR12. Org members (including Viewers) can filter logs by service, severity, time range, and keyword. Filters are ANDed server-side. Results are paginated (max 1,000 per page). Individual log detail is available by ID at org scope or service scope.

## API Endpoints

| Method | Path | Role | Description |
|--------|------|------|-------------|
| GET | /orgs/:id/logs/search | Member | Search logs (FR10, FR11) |
| GET | /orgs/:id/logs/:logId | Member | Log detail by org (FR12) |
| GET | /orgs/:id/services/:serviceId/logs/:logId | Member | Log detail by service (FR12) |

### Search query parameters

| Param | Required | Example | Notes |
|-------|----------|---------|-------|
| service_id | No | uuid or uuid1,uuid2 | Filter by one or more services |
| severity | No | ERROR,WARN | Comma-separated severities |
| from | No | 2026-06-01T00:00:00Z | RFC3339, inclusive |
| to | No | 2026-06-08T23:59:59Z | RFC3339, inclusive |
| q | No | payment failed | Full-text search on message |
| page | No | 1 | Default 1 |
| limit | No | 100 | Default 100, max 1000 |

All active filters are combined with AND logic.

### Example

```bash
curl -s "http://localhost:8080/orgs/$ORG_ID/logs/search?service_id=$SERVICE_ID&severity=ERROR&from=2026-06-07T00:00:00Z&q=payment&limit=50" \
  -H "Authorization: Bearer $TOKEN"
```

Response:
```json
{
  "success": true,
  "data": {
    "logs": [ { "id": "...", "severity": "ERROR", "message": "payment failed", ... } ],
    "pagination": { "page": 1, "limit": 50, "total": 12, "total_pages": 1 }
  }
}
```

## Database indexes

Migration `010_create_logs.sql` includes indexes for search:

| Index | Columns | Used for |
|-------|---------|----------|
| idx_logs_org_time | (org_id, timestamp DESC) | Org-wide search + time sort |
| idx_logs_service_time | (service_id, timestamp DESC) | Per-service time filter |
| idx_logs_service_severity_time | (service_id, severity, timestamp DESC) | Service + severity filter |
| idx_logs_message_fts | GIN (message_tsv) | Keyword full-text search |
| idx_logs_timestamp | (timestamp) | Time range bounds |

`message_tsv` is a generated `tsvector` column on `message`.

## Code map

| File | Responsibility |
|------|----------------|
| internal/telemetry/search.go | Query param parsing and validation |
| internal/telemetry/repository.go | SearchLogs, GetLogByOrgID |
| internal/telemetry/handler.go | SearchLogs, GetLogByOrg handlers |
| cmd/api/main.go | Route registration |

## Validation

- `limit` max 1000 (PRD FR10)
- `from` must be before `to`
- Time range max 30 days when both bounds set
- Invalid UUIDs or severities return 400

## Not yet implemented

- Keyset (cursor) pagination for very large result sets
- Metadata JSON field search
- Search highlighting / snippets in results
