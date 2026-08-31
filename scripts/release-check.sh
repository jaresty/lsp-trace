#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
failed=0

assert_file() {
  id=$1
  path=$2
  if [ -f "$root/$path" ]; then
    printf 'PASS %s: %s\n' "$id" "$path"
  else
    printf 'FAIL %s: missing %s\n' "$id" "$path"
    failed=1
  fi
}

assert_contains() {
  id=$1
  path=$2
  text=$3
  if [ -f "$root/$path" ] && grep -F "$text" "$root/$path" >/dev/null; then
    printf 'PASS %s: %s contains %s\n' "$id" "$path" "$text"
  else
    printf 'FAIL %s: %s must contain %s\n' "$id" "$path" "$text"
    failed=1
  fi
}

assert_file Q-TYPESCRIPT qualification/typescript/src/calls.ts
assert_file Q-CSHARP qualification/csharp/Calls.cs
assert_file Q-ELIXIR qualification/elixir/lib/calls.ex
assert_contains Q-ELIXIR-PATH scripts/qualify.sh 'elixir)'
assert_contains Q-STATE qualification/README.md 'PASS, BLOCKED, or FAIL'
assert_contains Q-EVIDENCE qualification/README.md 'A PASS requires retained'
assert_file R-CI .github/workflows/ci.yml
assert_file R-SECURITY docs/SECURITY.md
assert_file R-SEMANTICS docs/SEMANTICS.md
assert_file R-SCHEMA docs/SCHEMA_POLICY.md
assert_file R-RELEASE docs/RELEASING.md
assert_file R-PLATFORM .goreleaser.yaml

if [ "$failed" -ne 0 ]; then
  exit 1
fi
printf 'RELEASE CHECK PASS\n'
