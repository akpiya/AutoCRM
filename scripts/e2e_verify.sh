#!/usr/bin/env bash
# Install deps, run pytest, then optionally: python3 -m autocrm.main

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
python3 -m pip install -q -e ".[dev]"
python3 -m pytest tests/ -q
python3 -m autocrm.main
echo "OK: autocrm.main completed."
