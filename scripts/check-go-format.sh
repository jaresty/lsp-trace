#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

files=$(mktemp)
trap 'rm -f "$files"' EXIT HUP INT TERM

git ls-files '*.go' | while IFS= read -r path; do
  case "$path" in
    .pi/evidence/* | docs/pilot/*/source/* | */testdata/* | cmd/*freeze*/*)
      continue
      ;;
  esac
  printf '%s\n' "$path"
done >"$files"

unformatted=$(xargs gofmt -l <"$files")
if [ -n "$unformatted" ]; then
  printf '%s\n' "$unformatted"
  exit 1
fi
