#!/usr/bin/env bash
# Build production binaries into ./bin/ (run on the VM).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUT_DIR="${1:-$REPO_ROOT/bin}"

mkdir -p "$OUT_DIR"

export CGO_ENABLED=0
export GOOS=linux

# Oracle Ampere VMs are arm64; change to amd64 if you use an x86 VM.
ARCH="${GOARCH:-$(uname -m)}"
case "$ARCH" in
  aarch64|arm64) export GOARCH=arm64 ;;
  x86_64|amd64)  export GOARCH=amd64 ;;
  *) echo "unsupported arch: $ARCH" >&2; exit 1 ;;
esac

echo "building for linux/$GOARCH -> $OUT_DIR"

cd "$REPO_ROOT"
go build -ldflags="-s -w" -o "$OUT_DIR/loglens-api"      ./cmd/api
go build -ldflags="-s -w" -o "$OUT_DIR/loglens-ingestor" ./cmd/ingestor
go build -ldflags="-s -w" -o "$OUT_DIR/loglens-consumer" ./cmd/consumer

echo "done:"
ls -lh "$OUT_DIR"/loglens-*
