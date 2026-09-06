#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
if [ "$#" -ne 2 ]; then
  echo "usage: $0 PINNED_SOURCE_REPOSITORY CALLER_SELECTED_OUTPUT" >&2
  exit 2
fi
exec node "$ROOT/qualification/type-overlay/run.mjs" \
  --manifest "$ROOT/qualification/type-overlay/b05.manifest.json" \
  --source "$1" \
  --output "$2"
