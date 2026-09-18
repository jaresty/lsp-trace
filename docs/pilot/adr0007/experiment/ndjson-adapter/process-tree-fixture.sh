#!/bin/sh
set -eu
child_file="${ADR0007_CHILD_PID_FILE:?}"
(sleep 120) &
child=$!
printf '%s\n' "$child" > "$child_file"
trap 'kill "$child" 2>/dev/null || true; wait "$child" 2>/dev/null || true; exit 143' TERM INT
while :; do sleep 1; done
