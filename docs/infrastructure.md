# Feature: Infrastructure

Status: implemented
Added: 2026-06-06

## Summary

Local development stack with Postgres (required) and Redis (optional). Environment-based configuration loaded from `.env` at startup.

## Services (docker-compose)

| Service  | Image        | Port  | Credentials              |
|----------|--------------|-------|--------------------------|
| postgres | postgres:16  | 5432  | loglens / loglens / loglens |
| redis    | redis:7      | 6379  | no auth                  |

Start:
```bash
docker compose up -d
```

## Environment variables

| Variable      | Required | Default | Description                        |
|---------------|----------|---------|------------------------------------|
| DATABASE_URL  | Yes      | —       | Postgres connection string         |
| JWT_SECRET    | Yes      | —       | JWT signing secret                 |
| REDIS_ADDR    | No       | —       | Redis address (e.g. localhost:6379) |
| PORT          | No       | 8080    | HTTP server port                   |

## Database connection

File: `internal/db/postgres.go`

- Reads `DATABASE_URL` from environment
- Creates pgx connection pool
- Pings on startup; fatal if unreachable

## Redis connection

File: `internal/db/redis.go`

- Reads `REDIS_ADDR` from environment
- Graceful degradation: if unset or unreachable, app continues without cache
- Used by auth for refresh token fast-path lookup

## Migrations

Run manually against Postgres:

```bash
psql $DATABASE_URL -f migrations/001_create_users.sql
psql $DATABASE_URL -f migrations/002_create_refresh_tokens.sql
```

| Migration | Creates                              |
|-----------|--------------------------------------|
| 001       | `users` table with pgcrypto UUIDs    |
| 002       | `refresh_tokens` table + user_id index |

No migration runner is integrated yet.

## Running the API

```bash
go run ./cmd/api
```

Middleware enabled: Echo recover + request logger.

## Code map

| File                      | Responsibility              |
|---------------------------|-----------------------------|
| cmd/api/main.go           | App bootstrap and wiring    |
| internal/db/postgres.go   | Postgres pool connection    |
| internal/db/redis.go      | Redis client (optional)     |
| docker-compose.yml        | Local Postgres + Redis      |
| migrations/*.sql          | Schema definitions          |
| .env                      | Local env config (gitignored) |

## Not yet implemented

- Automated migration runner
- Health check endpoint
- Production deployment config
- Connection retry/backoff logic
