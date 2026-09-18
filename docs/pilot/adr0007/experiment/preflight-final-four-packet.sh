#!/bin/sh
# ADR 0007 local-only preflight. No network access or packet execution.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
MODEL=${ADR0007_MODEL:?set ADR0007_MODEL to the pinned model artifact}
ADAPTER="$ROOT/ndjson-adapter/adapter"
WORKER="$ROOT/pilot-worker/worker"

expect_model=1664fccab734674a50763490a8c6931b70e3f2f8ec10031b54806d30e5f956b6
expect_adapter=af22f92bddbaa925cfced6fc93032c398ba671310404a3648c330436a8a171c1
expect_worker=623ad4341720192d57fe95372343ad7d87450ef8bfb5baa6ecc0c05288d16d83

hash_file() {
  shasum -a 256 "$1" | awk '{print $1}'
}

[ -f "$MODEL" ] || { echo "PREFLIGHT_FAILURE: model artifact missing: $MODEL" >&2; exit 3; }
[ -f "$ADAPTER" ] || { echo "PREFLIGHT_FAILURE: adapter missing: $ADAPTER" >&2; exit 3; }
[ -f "$WORKER" ] || { echo "PREFLIGHT_FAILURE: worker missing: $WORKER" >&2; exit 3; }

model_hash=$(hash_file "$MODEL")
adapter_hash=$(hash_file "$ADAPTER")
worker_hash=$(hash_file "$WORKER")

[ "$model_hash" = "$expect_model" ] || { echo "PREFLIGHT_FAILURE: model digest mismatch: $model_hash" >&2; exit 4; }
[ "$adapter_hash" = "$expect_adapter" ] || { echo "PREFLIGHT_FAILURE: adapter digest mismatch: $adapter_hash" >&2; exit 4; }
[ "$worker_hash" = "$expect_worker" ] || { echo "PREFLIGHT_FAILURE: worker digest mismatch: $worker_hash" >&2; exit 4; }

echo "PREFLIGHT_OK: model=$model_hash adapter=$adapter_hash worker=$worker_hash"
