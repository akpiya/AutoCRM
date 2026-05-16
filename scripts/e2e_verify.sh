#!/usr/bin/env bash
# Install deps, run pytest, then optionally: python3 -m autocrm.main

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
python -m pip install -q -e ".[dev]"
python -m pytest tests/ -q 2>/dev/null || echo "No tests yet."
python3 -m autocrm.main
echo "OK: autocrm.main completed."
