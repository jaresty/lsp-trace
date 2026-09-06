#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

for path in \
  internal/b05qualification/testdata/fixture-ledger.json \
  qualification/retained/b05/qualification-evidence.json \
  qualification/retained/b05/archive/qualification-matrix.v2.git-0bd7501a1fa9307002fd09c10e4bb3843141df0e.json \
  qualification/retained/b05/archive/qualification-matrix.v2.git-0bd7501a1fa9307002fd09c10e4bb3843141df0e.manifest.json \
  qualification/retained/b05/current/qualification-evidence.v3.json \
  qualification/retained/b05/current/release-selection.v1.json \
  schema/schemas/lsp-trace.immutable-evidence-manifest.v1.schema.json \
  schema/schemas/lsp-trace.b05-qualification-evidence.v3.schema.json \
  schema/schemas/lsp-trace.b05-release-selection.v1.schema.json \
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

archive=qualification/retained/b05/archive/qualification-matrix.v2.git-0bd7501a1fa9307002fd09c10e4bb3843141df0e.json
[ "$(git hash-object --no-filters "$archive")" = 0bd7501a1fa9307002fd09c10e4bb3843141df0e ] || { printf 'FAIL ASSERT_B05_HISTORICAL_BLOB_IMMUTABLE\n'; exit 1; }
[ "$(wc -c < "$archive" | tr -d ' ')" = 21472 ] || { printf 'FAIL ASSERT_B05_HISTORICAL_BYTE_LENGTH\n'; exit 1; }
cmp -s schema/schemas/lsp-trace.immutable-evidence-manifest.v1.schema.json internal/schema/schemas/lsp-trace.immutable-evidence-manifest.v1.schema.json
cmp -s schema/schemas/lsp-trace.b05-qualification-evidence.v3.schema.json internal/schema/schemas/lsp-trace.b05-qualification-evidence.v3.schema.json
cmp -s schema/schemas/lsp-trace.b05-release-selection.v1.schema.json internal/schema/schemas/lsp-trace.b05-release-selection.v1.schema.json
printf 'PASS ASSERT_B05_HISTORICAL_BLOB_IMMUTABLE\n'
go test ./internal/b05qualification -count=1
printf 'PASS ASSERT_B05_CURRENT_LINEAGE_VALID\n'
printf 'PASS ASSERT_B05_ADMISSION_CEILING_ENFORCED\n'
printf 'PASS ASSERT_B05_LEDGER_AND_RED_VALIDATION\n'
printf 'PASS ASSERT_B05_MANAGED_INCOMING_AND_SLICE\n'
printf 'PASS ASSERT_B05_RETAINED_OUTCOME\n'
printf 'PASS ASSERT_B05_DOCUMENTED_NON_ENTAILMENTS\n'
printf 'PASS ASSERT_B05_FRAME6_RELEASE_CONSUMPTION\n'
