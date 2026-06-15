#!/usr/bin/env bash
# Run Go tests, then cmd/autocrm (requires Mac + optional Notion env).

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
go test ./... -count=1
go run ./cmd/autocrm
echo "OK: autocrm completed."
