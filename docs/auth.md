# Feature: User Authentication

Status: implemented
Added: 2026-06-06

## Summary

JWT-based authentication with email/password registration and login. Issues short-lived access tokens and long-lived refresh tokens. Refresh tokens are rotated on each refresh and can be revoked on logout.

## API Endpoints

| Method | Path            | Auth required | Description                          |
|--------|-----------------|---------------|--------------------------------------|
| POST   | /auth/register  | No            | Create account, return token pair    |
| POST   | /auth/login     | No            | Authenticate, return token pair      |
| POST   | /auth/refresh   | No            | Rotate refresh token, new access token |
| POST   | /auth/logout    | Yes (Bearer)  | Revoke refresh token                 |

### Request bodies

Register / Login:
```json
{
  "email": "user@example.com",
  "password": "min-8-chars"
}
```

Refresh / Logout:
```json
{
  "refresh_token": "<uuid-string>"
}
```

### Success response envelope

```json
{
  "success": true,
  "data": {
    "access_token": "<jwt>",
    "refresh_token": "<uuid>"
  }
}
```

Logout success:
```json
{
  "success": true,
  "data": {
    "message": "logged out"
  }
}
```

### Error responses

| Status | Condition                    | Error message                  |
|--------|------------------------------|--------------------------------|
| 400    | Invalid JSON body            | invalid request body           |
| 400    | Bad email format             | invalid email format           |
| 400    | Password too short           | password must be at least 8 characters |
| 401    | Wrong credentials            | invalid credentials            |
| 401    | Missing/invalid Bearer token | missing/invalid authorization header |
| 401    | Bad/expired refresh token    | invalid/expired refresh token  |
| 404    | User not found (login)       | user not found                 |
| 409    | Duplicate email              | email already taken            |
| 500    | Unexpected error             | internal server error          |

## Auth flows

### Register / Login

```
Client                    API                     Postgres        Redis (optional)
  |                        |                          |                |
  |-- POST /auth/register ->|                          |                |
  |                        |-- validate email/pw ---->|                |
  |                        |-- bcrypt hash ---------->|                |
  |                        |-- INSERT user ---------->|                |
  |                        |-- issue JWT + refresh -->|                |
  |                        |-- store refresh hash --->|                |
  |                        |-- cache refresh token ------------------->|
  |<-- access + refresh ---|                          |                |
```

### Refresh (token rotation)

```
Client                    API                     Postgres        Redis
  |                        |                          |                |
  |-- POST /auth/refresh ->|                          |                |
  |                        |-- lookup refresh hash -->| (cache first)  |
  |                        |-- revoke old token ----->|                |
  |                        |-- issue new pair ------->|                |
  |<-- new access + refresh|                          |                |
```

### Logout

```
Client                    API                     Postgres        Redis
  |                        |                          |                |
  |-- Bearer access token ->|                          |                |
  |-- refresh_token body -->|                          |                |
  |                        |-- verify access JWT      |                |
  |                        |-- delete refresh hash -->|                |
  |<-- logged out ---------|                          |                |
```

## Security

- Passwords hashed with bcrypt (cost factor 12)
- Access tokens: JWT signed with HS256 (`JWT_SECRET`)
- Refresh tokens: random UUID, SHA-256 hashed before storage (raw token never stored)
- Refresh rotation: old token deleted before new one is issued
- Default role on token: `user`

## Token configuration

| Env var                    | Default | Description              |
|----------------------------|---------|--------------------------|
| JWT_SECRET                 | required| Access token signing key |
| JWT_EXPIRY_HOURS           | 1       | Access token TTL         |
| REFRESH_TOKEN_EXPIRY_DAYS  | 7       | Refresh token TTL        |

## Database

Migration: `migrations/001_create_users.sql`
```sql
users (id UUID, email UNIQUE, password_hash, created_at)
```

Migration: `migrations/002_create_refresh_tokens.sql`
```sql
refresh_tokens (id UUID, user_id FK, token_hash UNIQUE, expires_at, created_at)
```

## Code map

| Layer        | File                              | Responsibility                    |
|--------------|-----------------------------------|-----------------------------------|
| Entry        | cmd/api/main.go                   | Wire deps, register /auth routes  |
| Handler      | internal/user/handler.go            | HTTP bind/validate, map errors    |
| Service      | internal/user/service.go          | Business logic, token issuance    |
| Repository   | internal/user/repository.go       | Postgres CRUD for users/tokens    |
| Models       | internal/user/models.go           | User, RefreshToken, AuthTokens    |
| Errors       | internal/user/errors.go           | Domain error definitions          |
| JWT          | internal/auth/jwt.go              | Access/refresh token generation   |
| Middleware   | internal/middleware/auth.go         | Bearer token verification         |
| Redis cache  | internal/user/service.go (adapter)| Fast refresh token lookup         |
| Response     | pkg/response/response.go          | `{ success, data, error }` envelope |

## Dependencies

- Echo v4 — HTTP framework
- pgx v5 — Postgres driver
- go-redis v9 — optional refresh token cache
- golang-jwt/jwt v5 — JWT access tokens
- golang.org/x/crypto/bcrypt — password hashing

## Not yet implemented

- User profile endpoints (GetUserByID exists in repo, unused)
- Bulk session revoke (DeleteAllRefreshTokens exists, unused)
- Role-based access control beyond default `user` role
- Email verification / password reset
