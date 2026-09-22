#!/bin/sh
set -eu
umask 077

REPO=${REPO:-/Users/schwa/dev/lsp-trace}
export LSP_TRACE_PRIVATE_TEST_DIAGNOSTIC=1
MODEL=${MODEL:-/tmp/lsp-trace-adr0007-qwen-coder7b-v7-model/Qwen2.5-Coder-7B-Instruct-Q4_K_M.gguf}
WORKER=${WORKER:-$REPO/docs/pilot/adr0007/experiment/pilot-worker/worker}
RUNTIME_ROOT=${RUNTIME_ROOT:-/tmp/adr0007-llama-lib}
RUNTIME_MANIFEST=${RUNTIME_MANIFEST:-$RUNTIME_ROOT/yzma-manifest.json}
CLI=${CLI:-/tmp/lsp-trace-adr0007-qualification-cli}
GOPLS=${GOPLS:-/Users/schwa/.local/bin/gopls}
MAX_OBJECT_BYTES=${MAX_OBJECT_BYTES:-67108864}
RUN_ROOT=${RUN_ROOT:-$(mktemp -d /tmp/lsp-trace-adr0007-qualification.XXXXXX)}
PUB=$RUN_ROOT/publication
HISTORICAL_PUB=$RUN_ROOT/historical-publication
HOST_CONFIG=$RUN_ROOT/host.json
ACQUISITION_CONFIG=$RUN_ROOT/acquisition.toml
PROFILE=$RUN_ROOT/network-deny.sb
GRAMMAR=$RUN_ROOT/semantic.gbnf
HISTORICAL_OUT=$RUN_ROOT/historical-census.stdout.json
HISTORICAL_ERR=$RUN_ROOT/historical-census.stderr.log
OUT=$RUN_ROOT/catalog.stdout.json
ERR=$RUN_ROOT/catalog.stderr.log
META=$RUN_ROOT/run.meta

mkdir -p "$PUB" "$HISTORICAL_PUB"
chmod 700 "$PUB" "$HISTORICAL_PUB"
test -x "$GOPLS"
printf '%s\n' '(version 1)' '(allow default)' '(deny network*)' > "$PROFILE"
python3 - "$REPO/internal/describerequest/v2_production.go" "$GRAMMAR" <<'PY'
from pathlib import Path
import sys
src=Path(sys.argv[1]).read_text()
start=src.index('const PinnedRuntimeGrammarV2 = `')+len('const PinnedRuntimeGrammarV2 = `')
end=src.index('`\n', start)
Path(sys.argv[2]).write_text(src[start:end])
PY

hash() { shasum -a 256 "$1" | awk '{print $1}'; }
pin() { test -f "$1"; got=$(hash "$1"); test "$got" = "$2"; printf 'PIN_OK %s %s\n' "$1" "$got"; }
pin "$MODEL" 1664fccab734674a50763490a8c6931b70e3f2f8ec10031b54806d30e5f956b6
pin "$WORKER" b9ad7048c3e1b36ca97006142a0402c450c94a179392cbe97eaea28b18b08a61
pin "$RUNTIME_MANIFEST" b95e8680b4d30761492bbc2d4a6fed656f124c5756dd4387cf02c30d27c13d90
(cd / && shasum -a 256 -c "$REPO/docs/pilot/adr0007/runtime/SHA256SUMS")

SANDBOX=/usr/bin/sandbox-exec
SANDBOX_SHA=$(hash "$SANDBOX")
PROFILE_SHA=$(hash "$PROFILE")
GRAMMAR_SHA=$(hash "$GRAMMAR")

set +e
"$SANDBOX" -f "$PROFILE" /usr/bin/nc -G 1 -z 1.1.1.1 53 >"$RUN_ROOT/network-probe.stdout" 2>"$RUN_ROOT/network-probe.stderr"
NET_CODE=$?
set -e
test "$NET_CODE" -ne 0
printf 'NETWORK_DENIAL_PROVEN exit=%s\n' "$NET_CODE"

cat > "$HOST_CONFIG" <<JSON
{"version":1,"continuation":{"publication_root":"$PUB","max_object_bytes":$MAX_OBJECT_BYTES,"worker":{"worker":{"path":"$WORKER","sha256":"sha256:$(hash "$WORKER")"},"model":{"path":"$MODEL","sha256":"sha256:$(hash "$MODEL")"},"library":{"path":"$RUNTIME_MANIFEST","sha256":"sha256:$(hash "$RUNTIME_MANIFEST")"},"sandbox_executable":{"path":"$SANDBOX","sha256":"sha256:$SANDBOX_SHA"},"sandbox_profile":{"path":"$PROFILE","sha256":"sha256:$PROFILE_SHA"},"grammar":{"path":"$GRAMMAR","sha256":"sha256:$GRAMMAR_SHA"},"runtime_identity":"llama.cpp-v0.4.0-yzma-347c6ee0b893cc0bcac50ff7fb12ece0611fd01a","adapter_identity":"adr0007-pilot-worker-sha256:b9ad7048c3e1b36ca97006142a0402c450c94a179392cbe97eaea28b18b08a61","model_identity":"qwen2.5-coder-7b-instruct-q4_k_m-sha256:1664fccab734674a50763490a8c6931b70e3f2f8ec10031b54806d30e5f956b6","limits":{"timeout_ms":90000,"max_tokens":384,"context_tokens":16384,"stdout_bytes":1048576,"stderr_bytes":1048576,"work_bytes":16777216,"temp_bytes":16777216}}}}
JSON
cat > "$ACQUISITION_CONFIG" <<TOML
[profiles.qualification-go]
command = "$GOPLS"
language_ids = ["go"]
TOML

(cd "$REPO" && go build -o "$CLI" ./cmd/lsp-trace)
printf 'cli_sha256=%s\n' "$(hash "$CLI")" > "$META"
"$CLI" version >> "$META"
HISTORICAL_START=$(python3 -c 'import time; print(time.monotonic_ns())')
set +e
"$CLI" census --machine --workspace "$REPO" --profile qualification-go --config "$ACQUISITION_CONFIG" --publication-root "$HISTORICAL_PUB" --source internal/censuscontinuation/pipeline.go --source internal/censuscontinuation/capture.go --down-depth 1 --up-depth 0 --max-nodes 100 --batch-targets 16 --timeout 60s --request-timeout 30s >"$HISTORICAL_OUT" 2>"$HISTORICAL_ERR"
HISTORICAL_CODE=$?
set -e
HISTORICAL_END=$(python3 -c 'import time; print(time.monotonic_ns())')
printf 'historical_exit_code=%s\nhistorical_elapsed_ns=%s\n' "$HISTORICAL_CODE" "$((HISTORICAL_END-HISTORICAL_START))" >> "$META"
if test "$HISTORICAL_CODE" -ne 0; then
  cat "$HISTORICAL_ERR" >&2
  printf 'QUALIFICATION_BLOCKED run_root=%s historical_exit_code=%s\n' "$RUN_ROOT" "$HISTORICAL_CODE" >&2
  exit "$HISTORICAL_CODE"
fi
printf 'HISTORICAL_CENSUS_COMPLETE\n'

START=$(python3 -c 'import time; print(time.monotonic_ns())')
set +e
"$CLI" census --machine --catalog --workspace "$REPO" --profile qualification-go --config "$ACQUISITION_CONFIG" --catalog-config "$HOST_CONFIG" --publication-root "$PUB" --source internal/censuscontinuation/pipeline.go --source internal/censuscontinuation/capture.go --down-depth 1 --up-depth 0 --max-nodes 100 --batch-targets 16 --timeout 60s --request-timeout 30s >"$OUT" 2>"$ERR"
CODE=$?
set -e
END=$(python3 -c 'import time; print(time.monotonic_ns())')
printf 'catalog_exit_code=%s\ncatalog_elapsed_ns=%s\n' "$CODE" "$((END-START))" >> "$META"
grep '^PRIVATE_HANDOFF_PREFLIGHT canonical_bytes=[0-9][0-9]* max_object_bytes=[0-9][0-9]*$' "$ERR" >> "$META"
if test "$CODE" -ne 0; then
  cat "$ERR" >&2
  printf 'QUALIFICATION_BLOCKED run_root=%s catalog_exit_code=%s\n' "$RUN_ROOT" "$CODE" >&2
  exit "$CODE"
fi

python3 - "$OUT" <<'PY'
import json,re,sys
p=json.load(open(sys.argv[1]))
assert p['schema_version']=='lsp-trace.census-feature-catalog-result.v2'
c=p['catalog']
assert c['kind']=='ADR_0007_FEATURE_CATALOG'
assert c['authority']==0 and c['accepted'] is False and c['completeness']=='UNKNOWN'
assert c['status'] in ('COMPLETE','DEGRADED')
rx=re.compile(r'^g-[0-9a-f]{64}\.selector\.json$')
for k in ('checkpoint_selector','composite_selector','catalog_selector'):
    assert rx.fullmatch(c[k]), (k,c[k])
print('SUCCESSOR_TUPLE_VALID')
print(c['checkpoint_selector'])
PY
printf 'QUALIFICATION_FRESH_COMPLETE run_root=%s\n' "$RUN_ROOT"
