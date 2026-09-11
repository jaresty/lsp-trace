#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
workflow="$root/.github/workflows/ci.yml"
failed=0

require() {
  id=$1
  text=$2
  if grep -F -- "$text" "$workflow" >/dev/null; then
    printf 'PASS %s\n' "$id"
  else
    printf 'FAIL %s: missing %s\n' "$id" "$text"
    failed=1
  fi
}

require CI-FORMAT 'gofmt -l .'
require CI-TEST 'go test ./...'
require CI-TEST-UNIX-ONLY "if: runner.os != 'Windows'"
require CI-WINDOWS-BUILD 'windows-latest'
require CI-VET 'go vet ./...'
require CI-BUILD 'go build ./...'
require CI-PYTHON 'python3 -m py_compile scripts/retain-qualification.py scripts/check-program-c-profiles.py scripts/test-program-c-profiles.py scripts/check-program-c-gate-ii.py scripts/test-program-c-gate-ii.py'
require CI-PROGRAM-C-PROFILES 'python3 scripts/test-program-c-profiles.py'
require CI-PROGRAM-C-GATE-II-TESTS 'python3 scripts/test-program-c-gate-ii.py'
require CI-PROGRAM-C-GATE-II-CHECK 'python3 scripts/check-program-c-gate-ii.py --root . --outcomes qualification/program-c/gate-ii-current-outcomes.v3.tsv'
require CI-SHELL 'sh -n scripts/qualify.sh scripts/release-check.sh scripts/check-ci.sh'
require CI-RELEASE './scripts/release-check.sh'
require CI-BUN 'oven-sh/setup-bun@v2'
require CI-ADAPTER 'pi-mcp-adapter@2.32.1'
require CI-PI-RUNTIME '@earendil-works/pi-coding-agent@0.84.1'
require CI-PI-AI-RUNTIME '@earendil-works/pi-ai@0.84.1'
require CI-TYPEBOX 'typebox@1.3.3'
require CI-CLEAN 'git status --porcelain'
if grep -F -- 'go build -trimpath' "$root/scripts/release-check.sh" >/dev/null; then
  printf 'PASS CI-RELEASE-DRY-RUN\n'
else
  printf 'FAIL CI-RELEASE-DRY-RUN: release-check must execute go build -trimpath\n'
  failed=1
fi

exit "$failed"
