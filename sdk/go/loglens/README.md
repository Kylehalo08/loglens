# LogLens Go SDK

Send structured logs from any Go application to [LogLens](https://loglens-ten.vercel.app).

## Install

```bash
go get github.com/Kylehalo08/loglens/sdk/go/loglens@v0.1.0
```

Use `@latest` or a newer tag after release.

## Prerequisites

1. [Sign up](https://loglens-ten.vercel.app/register) for LogLens
2. Create an **organization** and **service**
3. **Generate an API key** (copy the full `ll_...` secret — shown once)

## Environment variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `LOGLENS_API_KEY` | Yes* | — | Service API key (`ll_<prefix>_<secret>`) |
| `INGESTOR_URL` or `LOGLENS_INGESTOR_URL` | No | `http://localhost:8081` | Log ingestor base URL |

\* Pass directly to `NewClient(apiKey)` if you prefer not to use env vars.

**Production ingestor:**

```bash
export INGESTOR_URL=https://ingest.madhavmaheshwaricreations.in
export LOGLENS_API_KEY=ll_your_prefix_your_secret
```

## Usage

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/Kylehalo08/loglens/sdk/go/loglens"
)

func main() {
	client := loglens.NewClient(os.Getenv("LOGLENS_API_KEY"))
	ctx := context.Background()

	if err := client.Error(ctx, "payment failed", map[string]any{
		"order_id": "123",
		"amount":   499,
	}); err != nil {
		log.Fatal(err)
	}

	log.Println("log sent")
}
```

### Severity helpers

- `client.Debug(ctx, message, metadata)`
- `client.Info(ctx, message, metadata)`
- `client.Warn(ctx, message, metadata)`
- `client.Error(ctx, message, metadata)`
- `client.Fatal(ctx, message, metadata)`

### Low-level API

```go
client.Log(ctx, "ERROR", "custom severity log", map[string]any{"key": "value"})
```

Valid severities: `DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`.

## View logs

Open the LogLens dashboard → select your org → search or filter logs by service and severity.

## HTTP alternative (no SDK)

```bash
curl -s -X POST "$INGESTOR_URL/v1/logs" \
  -H "Authorization: Bearer $LOGLENS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"severity":"ERROR","message":"payment failed","metadata":{"order_id":"123"}}'
```

## Local development (this repo)

From the monorepo root:

```bash
export LOGLENS_API_KEY=ll_...
export INGESTOR_URL=http://localhost:8081
go run ./examples/send_log
```

Or in another module:

```go
replace github.com/Kylehalo08/loglens/sdk/go/loglens => /path/to/loglens/sdk/go/loglens
```

## Releases

Nested modules need a **prefixed tag** (from repo root):

```bash
git tag sdk/go/loglens/v0.1.0
git push origin sdk/go/loglens/v0.1.0
```

Consumers install with:

```bash
go get github.com/Kylehalo08/loglens/sdk/go/loglens@v0.1.0
```

Do **not** use a root `v0.1.0` tag only — `go get` will not find the SDK package.
