#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

for path in \
  internal/b05qualification/testdata/fixture-ledger.json \
  qualification/retained/b05/qualification-evidence.json \
  qualification/retained/b05/qualification-matrix.v2.json \
  scripts/qualify-b05-frame6.sh \
  qualification/B05.md
do
  if [ ! -s "$path" ]; then
    printf 'FAIL ASSERT_B05_ARTIFACT: missing or empty %s\n' "$path"
    exit 1
  fi
done

if ! grep -F '"schema_version": "lsp-trace.b05-qualification.v1"' internal/b05qualification/testdata/fixture-ledger.json >/dev/null; then
  printf 'FAIL ASSERT_B05_LEDGER_SCHEMA\n'
  exit 1
fi
if ! grep -F 'external_provider_status' qualification/retained/b05/qualification-evidence.json >/dev/null; then
  printf 'FAIL ASSERT_B05_RETAINED_OUTCOME\n'
  exit 1
fi
if ! grep -F 'No B05 artifact establishes runtime execution' qualification/B05.md >/dev/null; then
  printf 'FAIL ASSERT_B05_DOCUMENTED_NON_ENTAILMENTS\n'
  exit 1
fi

go test ./internal/b05qualification -count=1
printf 'PASS ASSERT_B05_LEDGER_AND_RED_VALIDATION\n'
printf 'PASS ASSERT_B05_MANAGED_INCOMING_AND_SLICE\n'
printf 'PASS ASSERT_B05_RETAINED_OUTCOME\n'
printf 'PASS ASSERT_B05_DOCUMENTED_NON_ENTAILMENTS\n'
printf 'PASS ASSERT_B05_FRAME6_RELEASE_CONSUMPTION\n'
