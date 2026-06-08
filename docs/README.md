# LogLens Feature Docs

Each file in this directory documents one feature added to the project.

## Naming convention

```
docs/<feature-name>.md
```

Examples:
- `auth.md` — user authentication
- `infrastructure.md` — database, redis, docker, env config
- `log-ingestion.md` — (future) log upload and storage

## File template

When adding a new feature, create a `.md` file with:

1. **Summary** — what the feature does in one paragraph
2. **Status** — implemented / in-progress / planned
3. **API endpoints** — if applicable
4. **Flows** — how data moves through the system
5. **Database** — tables/migrations if applicable
6. **Code map** — which files implement the feature
7. **Env vars** — configuration needed
8. **Not yet implemented** — known gaps or follow-ups

## Current features

| File               | Status      | Description                                        |
|--------------------|-------------|----------------------------------------------------|
| auth.md            | implemented | Register, login, refresh, logout                   |
| infrastructure.md  | implemented | Postgres, Redis, docker-compose                    |
| organisation.md    | implemented | Orgs, members, invites, RBAC roles                 |
| services.md        | implemented | Service CRUD, API keys, audit events               |
| telemetry.md       | implemented | Log ingest, Kafka, storage, WebSocket stream       |



open -a Docker
# 2. Start DB containers
cd /Users/madhavmaheshwari/loglens
docker compose up -d
# 3. Start API
go run ./cmd/api
