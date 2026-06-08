# Feature: Organisation Management

Status: implemented
Added: 2026-06-06

## Summary

Authenticated users can create organisations, list their memberships, and view org details (name, members, service count). Organisation Owners and Admins can invite new members by email token or shareable invite code. Members join via invite and are assigned one of four org roles: `owner`, `admin`, `developer`, or `viewer`. Org-level RBAC is enforced in middleware and the service layer.

## Roles

| Role        | Permissions |
|-------------|-------------|
| `owner`     | Full control. Assigned automatically to org creator. Can invite/remove members and use all developer capabilities. |
| `admin`     | Invite/remove members, view all services and logs, full service management. |
| `developer` | Create/delete services, manage API keys, search and use AI features. Cannot send org invites. |
| `viewer`    | Read-only. Can list/view services (no API key info), search logs, view dashboards. |

Invitable roles: `admin`, `developer`, `viewer` (owner is not assignable via invite).

## API Endpoints

All routes require `Authorization: Bearer <access_token>` unless noted.

| Method | Path | Org role required | Description |
|--------|------|-------------------|-------------|
| POST   | /orgs | Any authenticated user | Create organisation (caller becomes owner) |
| GET    | /orgs | Any authenticated user | List organisations the user belongs to |
| GET    | /orgs/:id | Org member | Get org details, members, services count |
| POST   | /orgs/:id/invites | Owner or Admin | Send email invite, returns one-time token |
| POST   | /orgs/:id/invite-codes | Owner or Admin | Generate shareable 6-char invite code |
| POST   | /orgs/join/token | Any authenticated user | Join org via email invite token |
| POST   | /orgs/join/code | Any authenticated user | Join org via invite code |

### Request bodies

Create organisation:
```json
{
  "name": "Acme Corp"
}
```

Send email invite:
```json
{
  "email": "dev@example.com",
  "role": "developer"
}
```

Join via token:
```json
{
  "token": "<invite-uuid-token>"
}
```

Join via code:
```json
{
  "code": "ABC123"
}
```

### Success responses

Create organisation:
```json
{
  "success": true,
  "data": {
    "id": "<uuid>",
    "name": "Acme Corp",
    "created_by": "<user-uuid>",
    "created_at": "2026-06-08T10:00:00Z",
    "role": "owner"
  }
}
```

Get organisation:
```json
{
  "success": true,
  "data": {
    "id": "<uuid>",
    "name": "Acme Corp",
    "created_at": "2026-06-08T10:00:00Z",
    "members": [
      {
        "user_id": "<uuid>",
        "email": "owner@example.com",
        "role": "owner",
        "joined_at": "2026-06-08T10:00:00Z"
      }
    ],
    "services_count": 3
  }
}
```

Send invite (token shown once — share with invitee out of band):
```json
{
  "success": true,
  "data": {
    "invite_id": "<uuid>",
    "email": "dev@example.com",
    "role": "developer",
    "expires_at": "2026-06-09T10:00:00Z",
    "token": "<raw-invite-token>"
  }
}
```

### Error responses

| Status | Condition | Error message |
|--------|-----------|---------------|
| 400 | Invalid JSON body | invalid request body |
| 400 | Empty org name | organization name is required |
| 400 | Invalid email | invalid email format |
| 400 | Invalid invite role | invalid invite role |
| 401 | Missing/invalid Bearer token | unauthorized / missing authorization header |
| 403 | Not an org member | not an organization member |
| 403 | Insufficient role (e.g. developer sending invite) | insufficient permissions |
| 404 | Org not found | organization not found |
| 404 | Invite/code not found | invite not found / invite code not found |
| 409 | Already a member | already an organization member |
| 410 | Expired/used invite | invite expired / invite already accepted / invite code is inactive |
| 500 | Unexpected error | internal server error |

## Flows

### Create organisation

```
Client                    API                     Postgres
  |                        |                          |
  |-- POST /orgs --------->|                          |
  |                        |-- INSERT organization -->|
  |                        |-- INSERT org_member ---->|  (role: owner)
  |<-- 201 org + role -----|                          |
```

### Email invite + join

```
Owner/Admin               API                     Postgres        Redis (optional)
  |                        |                          |                |
  |-- POST /orgs/:id/invites                          |                |
  |                        |-- INSERT org_invites --->|                |
  |                        |-- cache invite token ------------------->|
  |<-- token (once) -------|                          |                |
  |                        |                          |                |
Invitee                    |                          |                |
  |-- POST /orgs/join/token|                          |                |
  |                        |-- validate token ------->| (cache first)  |
  |                        |-- INSERT org_member ---->|                |
  |                        |-- mark invite accepted ->|                |
  |<-- joined org ---------|                          |                |
```

### Invite code join

```
Admin                     API                     Postgres
  |                        |                          |
  |-- POST /orgs/:id/invite-codes                     |
  |<-- 6-char code --------|                          |
  |                        |                          |
User                      |                          |
  |-- POST /orgs/join/code |                          |
  |                        |-- lookup code ----------->|
  |                        |-- INSERT org_member ---->|  (default_role: developer)
  |<-- joined org ---------|                          |
```

## Security

- All `/orgs/*` routes require a valid JWT (see `auth.md`)
- `RequireOrgAdmin` middleware guards invite endpoints (owner or admin only)
- Email invite tokens: UUID, SHA-256 hashed in DB; raw token returned once on creation
- Invite token TTL: 24 hours
- Invite codes: 6 characters, uppercase alphanumeric (no ambiguous chars)
- Org membership checked on every org-scoped read

## Configuration

No org-specific environment variables. Uses shared auth config from `auth.md`.

| Constant | Value | Description |
|----------|-------|-------------|
| inviteTokenTTL | 24h | Email invite expiry |
| inviteCodeLen | 6 | Shareable code length |
| default invite code role | developer | Role assigned on code join |

## Database

| Migration | Table | Purpose |
|-----------|-------|---------|
| 003 | `organizations` | Org name, creator, timestamps |
| 004 | `org_members` | User ↔ org membership with role |
| 005 | `org_invites` | Email invites with hashed token |
| 006 | `org_invite_codes` | Shareable join codes |

```sql
organizations (id, name, created_by FK, created_at, updated_at)
org_members   (org_id FK, user_id FK, role, joined_at)  PK(org_id, user_id)
org_invites   (id, org_id FK, invited_by FK, email, role, token_hash, expires_at, accepted_at, created_at)
org_invite_codes (id, org_id FK, code UNIQUE, created_by FK, default_role, is_active, created_at)
```

Run migrations:
```bash
docker exec -i loglens-postgres-1 psql -U loglens -d loglens < migrations/003_create_organizations.sql
docker exec -i loglens-postgres-1 psql -U loglens -d loglens < migrations/004_create_org_members.sql
docker exec -i loglens-postgres-1 psql -U loglens -d loglens < migrations/005_create_org_invites.sql
docker exec -i loglens-postgres-1 psql -U loglens -d loglens < migrations/006_create_org_invite_codes.sql
```

## Code map

| Layer | File | Responsibility |
|-------|------|----------------|
| Entry | cmd/api/main.go | Wire deps, register /orgs routes |
| Handler | internal/org/handler.go | HTTP bind/validate, map errors |
| Service | internal/org/service.go | Business logic, invite generation |
| Repository | internal/org/repository.go | Postgres CRUD |
| Models | internal/org/models.go | Org, member, invite types |
| Errors | internal/org/errors.go | Domain error definitions |
| Middleware | internal/org/middleware.go | RequireOrgAdmin |
| Cache | internal/org/cache.go | Redis invite token fast-path |
| Response | pkg/response/response.go | `{ success, data, error }` envelope |

## Example curls

```bash
BASE=http://localhost:8080
TOKEN="<access_token>"
ORG_ID="<org_uuid>"

# Create org
curl -s -X POST "$BASE/orgs" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"My Org"}'

# List my orgs
curl -s "$BASE/orgs" -H "Authorization: Bearer $TOKEN"

# Get org details
curl -s "$BASE/orgs/$ORG_ID" -H "Authorization: Bearer $TOKEN"

# Invite developer by email
curl -s -X POST "$BASE/orgs/$ORG_ID/invites" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email":"dev@example.com","role":"developer"}'

# Join via invite token (invitee's token)
curl -s -X POST "$BASE/orgs/join/token" \
  -H "Authorization: Bearer $INVITEE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"token":"<invite-token>"}'

# Generate invite code
curl -s -X POST "$BASE/orgs/$ORG_ID/invite-codes" \
  -H "Authorization: Bearer $TOKEN"

# Join via code
curl -s -X POST "$BASE/orgs/join/code" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"code":"ABC123"}'
```

## Not yet implemented

- Remove/update member roles (FR3 partial — invite exists, remove not built)
- Org rename / delete
- Email delivery for invites (token returned in API response only)
- Audit trail for org invites (FR18 — audit table exists, org events not wired yet)
