#!/usr/bin/env bash
# Run all SQL migrations against the production Postgres container.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.prod.yml"
ENV_FILE="$SCRIPT_DIR/.env"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "error: $ENV_FILE not found (needs POSTGRES_PASSWORD)" >&2
  exit 1
fi

# shellcheck disable=SC1090
source "$ENV_FILE"

for migration in "$REPO_ROOT"/migrations/*.sql; do
  echo "==> $(basename "$migration")"
  docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" \
    exec -T postgres psql -U loglens -d loglens -v ON_ERROR_STOP=1 < "$migration"
done

echo "migrations complete"
