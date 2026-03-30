#!/usr/bin/env bash
# Manual E2E (Notion):
#   export AUTOCRM_NOTION_TOKEN=... AUTOCRM_NOTION_DATABASE_ID=...
#   pip install -e '.[dev]' && python3 -m autocrm.main
#   Share the Notion DB with the integration; properties: Last contacted (date),
#   Last channel (text), Phones, Emails — match AUTOCRM_NOTION_*_PROP if overridden.

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
python -m pip install -q -e ".[dev]"
python -m pytest tests/ -q
echo "pytest OK. See comments above for manual E2E."
