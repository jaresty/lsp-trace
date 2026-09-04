#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
provider=${LSP_TRACE_EXTERNAL_PROVIDER_PATH:-${1:-}}

fail() {
  printf 'FAIL %s: %s\n' "$1" "$2" >&2
  exit 1
}

[ -n "$provider" ] || fail ASSERT_EXTERNAL_PROVIDER_ABSOLUTE_PATH 'set LSP_TRACE_EXTERNAL_PROVIDER_PATH or pass an absolute executable path'
case "$provider" in
  /*) ;;
  *) fail ASSERT_EXTERNAL_PROVIDER_ABSOLUTE_PATH "provider path is not absolute: $provider" ;;
esac
[ -f "$provider" ] && [ -x "$provider" ] || fail ASSERT_EXTERNAL_PROVIDER_EXECUTABLE "not an executable file: $provider"

canonical=$(CDPATH= cd -- "$(dirname -- "$provider")" && pwd)/$(basename -- "$provider")
case "$canonical" in
  *'/testdata/'*|*'/fake-'*|*'/fake_'*) fail ASSERT_EXTERNAL_PROVIDER_REAL_PACKAGE_PATH "fake/testdata providers cannot qualify production: $canonical" ;;
  "$root"/*) fail ASSERT_EXTERNAL_PROVIDER_REAL_PACKAGE_PATH "provider must be independently installed outside the core repository: $canonical" ;;
esac

printf 'PASS ASSERT_EXTERNAL_PROVIDER_ABSOLUTE_PATH: %s\n' "$canonical"
printf 'PASS ASSERT_EXTERNAL_PROVIDER_REAL_PACKAGE_PATH: independently installed executable\n'
LSP_TRACE_EXTERNAL_PROVIDER_PATH="$canonical" \
LSP_TRACE_RETAIN_EXTERNAL_QUALIFICATION=1 \
  go test ./internal/provider -run TestProductionExternalProviderCompletesManagedLifecycle -count=1 -v
