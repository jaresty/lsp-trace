#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"
PYTHONDONTWRITEBYTECODE=1 python3 scripts/test-ci-shards.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/test-ci-affected-packages.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/test-summarize-go-test-json.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/test-ci-topology.py
python3 scripts/check-ci-shards.py
python3 scripts/check-ci-topology.py
if grep -F -- 'go build -trimpath' scripts/release-check.sh >/dev/null; then
  printf 'PASS CI-RELEASE-DRY-RUN\n'
else
  printf 'FAIL CI-RELEASE-DRY-RUN\n' >&2
  exit 1
fi
