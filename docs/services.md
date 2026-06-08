# Feature: Service Management

Status: implemented
Added: 2026-06-08

## Summary

Developers register named application services within an organisation (FR5). Each service can have multiple API keys for log ingestion authentication (FR6). Services use soft delete. API keys are bcrypt-hashed and the raw secret is returned only on generate and rotate. Viewers can list and view services but cannot see key metadata or counts. All critical actions write append-only audit events.

## API Endpoints

All routes require `Authorization: Bearer <access_token>` and org membership.

Base path: `/orgs/:id/services`

### Services (FR5)

| Method | Path | Org role required | Description |
|--------|------|-------------------|-------------|
| POST   | /orgs/:id/services | Developer+ | Create service |
| GET    | /orgs/:id/services | Member | List services in org |
| GET    | /orgs/:id/services/:serviceId | Member | Get single service |
| PATCH  | /orgs/:id/services/:serviceId | Developer+ | Update name/description |
| DELETE | /orgs/:id/services/:serviceId | Developer+ | Soft delete service |

Developer+ = `owner`, `admin`, or `developer`.

### API keys (FR6)

| Method | Path | Org role required | Description |
|--------|------|-------------------|-------------|
| POST   | /orgs/:id/services/:serviceId/api-keys | Developer+ | Generate key (secret shown once) |
| GET    | /orgs/:id/services/:serviceId/api-keys | Developer+ | List keys (metadata only) |
| DELETE | /orgs/:id/services/:serviceId/api-keys/:keyId | Developer+ | Revoke key immediately |
| POST   | /orgs/:id/services/:serviceId/api-keys/:keyId/rotate | Developer+ | Atomic rotate (new key + revoke old) |

### Request bodies

Create / update service:
```json
{
  "name": "Payment Service",
  "description": "Handles checkout and refunds"
}
```

Generate API key:
```json
{
  "label": "production"
}
```

`label` is optional.

### Success responses

Create service:
```json
{
  "success": true,
  "data": {
    "id": "<uuid>",
    "org_id": "<uuid>",
    "name": "Payment Service",
    "description": "Handles checkout and refunds",
    "created_by": "<user-uuid>",
    "created_at": "2026-06-08T10:00:00Z",
    "updated_at": "2026-06-08T10:00:00Z",
    "active_api_keys_count": 0
  }
}
```

Generate API key (only time `api_key` is returned):
```json
{
  "success": true,
  "data": {
    "id": "<uuid>",
    "service_id": "<uuid>",
    "prefix": "ll_a1b2c3d4",
    "api_key": "ll_a1b2c3d4_<32-char-hex-secret>",
    "label": "production",
    "created_at": "2026-06-08T10:00:00Z",
    "created_by": "<user-uuid>"
  }
}
```

List API keys (no secret):
```json
{
  "success": true,
  "data": [
    {
      "id": "<uuid>",
      "service_id": "<uuid>",
      "prefix": "ll_a1b2c3d4",
      "label": "production",
      "created_at": "2026-06-08T10:00:00Z",
      "created_by": "<user-uuid>",
      "revoked_at": null,
      "last_used_at": null
    }
  ]
}
```

Delete service:
```json
{
  "success": true,
  "data": {
    "message": "service deleted"
  }
}
```

### Role-based response differences

| Field | Owner / Admin / Developer | Viewer |
|-------|---------------------------|--------|
| `active_api_keys_count` on service | Included | Omitted |
| API key endpoints | Allowed | 403 Forbidden |

### Error responses

| Status | Condition | Error message |
|--------|-----------|---------------|
| 400 | Invalid JSON body | invalid request body |
| 400 | Empty service name | service name is required |
| 400 | Name > 255 chars | service name must be at most 255 characters |
| 401 | Missing/invalid Bearer token | unauthorized |
| 403 | Not org member | not an organization member |
| 403 | Viewer or insufficient role | insufficient permissions |
| 404 | Service not found (or wrong org) | service not found |
| 404 | API key not found | api key not found |
| 409 | Duplicate name in org | service name already exists in this organization |
| 410 | Revoke already-revoked key | api key already revoked |
| 500 | Unexpected error | internal server error |

## Flows

### Create service + API key

```
Developer                 API                     Postgres           audit_events
  |                        |                          |                  |
  |-- POST .../services -->|                          |                  |
  |                        |-- INSERT services ------->|                  |
  |                        |-- audit service.created ------------------>|
  |<-- 201 service --------|                          |                  |
  |                        |                          |                  |
  |-- POST .../api-keys -->|                          |                  |
  |                        |-- generate ll_prefix_secret                 |
  |                        |-- bcrypt(hash)           |                  |
  |                        |-- INSERT service_api_keys|                  |
  |                        |-- audit api_key.created ------------------>|
  |<-- 201 + api_key ------|  (shown once)            |                  |
```

### Rotate API key (atomic)

```
Developer                 API                     Postgres
  |                        |                          |
  |-- POST .../rotate ---->|                          |
  |                        |-- BEGIN TX               |
  |                        |-- UPDATE old revoked_at ->|
  |                        |-- INSERT new key -------->|
  |                        |-- COMMIT                 |
  |                        |-- audit api_key.rotated  |
  |<-- 201 + new api_key --|                          |
```

### Revoke API key

```
Developer                 API                     Postgres
  |                        |                          |
  |-- DELETE .../keyId --->|                          |
  |                        |-- SET revoked_at = now()->|
  |                        |-- audit api_key.revoked  |
  |<-- 200 key metadata ---|                          |
```

## Security

- API key format: `ll_<8-char-prefix>_<32-char-hex-secret>`
- Keys stored as bcrypt hashes (cost 12); raw key never stored or returned after create/rotate
- Prefix indexed for future ingestor lookup (FR7)
- Multiple active keys per service allowed (e.g. staging + production, zero-downtime rotation)
- Service names unique per org among non-deleted services
- Soft delete: `deleted_at` set; service excluded from list/get

## RBAC matrix

| Action | Owner | Admin | Developer | Viewer |
|--------|-------|-------|-----------|--------|
| List / get services | ✓ | ✓ | ✓ | ✓ |
| See `active_api_keys_count` | ✓ | ✓ | ✓ | ✗ |
| Create / update / delete service | ✓ | ✓ | ✓ | ✗ |
| Generate / list / revoke / rotate keys | ✓ | ✓ | ✓ | ✗ |

## Database

| Migration | Table | Purpose |
|-----------|-------|---------|
| 007 | `services` | Registered applications per org |
| 008 | `service_api_keys` | Hashed ingestion keys |
| 009 | `audit_events` | Append-only audit log |

```sql
services (id, org_id FK, name, description, metadata JSONB, created_by FK,
          created_at, updated_at, deleted_at)
  UNIQUE (org_id, name) WHERE deleted_at IS NULL

service_api_keys (id, service_id FK, org_id FK, prefix UNIQUE, key_hash,
                  label, created_by FK, created_at, revoked_at, last_used_at,
                  rotated_from_id FK)

audit_events (id, org_id FK, actor_id FK, actor_type, action, resource_type,
              resource_id, metadata JSONB, ip_address, created_at)
```

Run migrations:
```bash
docker exec -i loglens-postgres-1 psql -U loglens -d loglens < migrations/007_create_services.sql
docker exec -i loglens-postgres-1 psql -U loglens -d loglens < migrations/008_create_service_api_keys.sql
docker exec -i loglens-postgres-1 psql -U loglens -d loglens < migrations/009_create_audit_events.sql
```

### Audit actions written

| Action | Resource type | Trigger |
|--------|---------------|---------|
| `service.created` | service | POST /services |
| `service.updated` | service | PATCH /services/:id |
| `service.deleted` | service | DELETE /services/:id |
| `api_key.created` | api_key | POST /api-keys |
| `api_key.revoked` | api_key | DELETE /api-keys/:id |
| `api_key.rotated` | api_key | POST /api-keys/:id/rotate |

## Code map

| Layer | File | Responsibility |
|-------|------|----------------|
| Entry | cmd/api/main.go | Wire deps, register /orgs/:id/services routes |
| Handler | internal/service/handler.go | HTTP bind/validate, map errors |
| Service | internal/service/service.go | Business logic, RBAC, audit calls |
| Repository | internal/service/repository.go | Postgres CRUD, rotate transaction |
| Models | internal/service/models.go | Service/API key response types |
| Errors | internal/service/errors.go | Domain error definitions |
| Middleware | internal/service/middleware.go | RequireOrgMember, RequireOrgDeveloper |
| API keys | internal/auth/apikey.go | Generate, bcrypt hash, validate |
| Audit | internal/audit/writer.go | Append-only audit event writer |
| Org integration | internal/org/repository.go | CountServicesByOrgID for org detail |
| Response | pkg/response/response.go | `{ success, data, error }` envelope |

## Example curls

```bash
BASE=http://localhost:8080
TOKEN="<access_token>"
ORG_ID="<org_uuid>"
SERVICE_ID="<service_uuid>"
KEY_ID="<api_key_uuid>"

# Create service
curl -s -X POST "$BASE/orgs/$ORG_ID/services" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Payment Service","description":"Handles payments"}'

# List services
curl -s "$BASE/orgs/$ORG_ID/services" \
  -H "Authorization: Bearer $TOKEN"

# Get service
curl -s "$BASE/orgs/$ORG_ID/services/$SERVICE_ID" \
  -H "Authorization: Bearer $TOKEN"

# Update service
curl -s -X PATCH "$BASE/orgs/$ORG_ID/services/$SERVICE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Payment Service v2","description":"Updated"}'

# Delete service (soft delete)
curl -s -X DELETE "$BASE/orgs/$ORG_ID/services/$SERVICE_ID" \
  -H "Authorization: Bearer $TOKEN"

# Generate API key — copy api_key from response immediately
curl -s -X POST "$BASE/orgs/$ORG_ID/services/$SERVICE_ID/api-keys" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"label":"production"}'

# List API keys (metadata only)
curl -s "$BASE/orgs/$ORG_ID/services/$SERVICE_ID/api-keys" \
  -H "Authorization: Bearer $TOKEN"

# Revoke API key
curl -s -X DELETE "$BASE/orgs/$ORG_ID/services/$SERVICE_ID/api-keys/$KEY_ID" \
  -H "Authorization: Bearer $TOKEN"

# Rotate API key
curl -s -X POST "$BASE/orgs/$ORG_ID/services/$SERVICE_ID/api-keys/$KEY_ID/rotate" \
  -H "Authorization: Bearer $TOKEN"
```

## Not yet implemented

- API key validation on log ingest (FR7 — ingestor not built; `internal/auth/apikey.go` ready)
- Redis cache for hot API key lookup during ingestion
- `last_used_at` updates on key use
- `metadata` JSONB field exposed via API (column exists, not in request/response yet)
- Audit log read API (FR18)
- Per-service active key limit
