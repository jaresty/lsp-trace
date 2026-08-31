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
assert_contains Q-TYPESCRIPT-LEAF qualification/typescript/src/calls.ts 'export function leaf'
assert_contains Q-TYPESCRIPT-LEFT qualification/typescript/src/calls.ts 'export function left'
assert_contains Q-TYPESCRIPT-RIGHT qualification/typescript/src/calls.ts 'export function right'
assert_contains Q-TYPESCRIPT-RECURSION qualification/typescript/src/calls.ts 'recursive(value - 1)'
assert_contains Q-TYPESCRIPT-STATIC qualification/typescript/src/calls.ts 'staticButNotExecuted'
assert_file Q-CSHARP qualification/csharp/Calls.cs
assert_contains Q-CSHARP-LEAF qualification/csharp/Calls.cs 'int Leaf'
assert_contains Q-CSHARP-LEFT qualification/csharp/Calls.cs 'int Left'
assert_contains Q-CSHARP-RIGHT qualification/csharp/Calls.cs 'int Right'
assert_contains Q-CSHARP-RECURSION qualification/csharp/Calls.cs 'Recursive(value - 1)'
assert_contains Q-CSHARP-STATIC qualification/csharp/Calls.cs 'StaticButNotExecuted'
assert_file Q-ELIXIR qualification/elixir/lib/calls.ex
assert_contains Q-ELIXIR-LEAF qualification/elixir/lib/calls.ex 'def leaf'
assert_contains Q-ELIXIR-LEFT qualification/elixir/lib/calls.ex 'def left'
assert_contains Q-ELIXIR-RIGHT qualification/elixir/lib/calls.ex 'def right'
assert_contains Q-ELIXIR-RECURSION qualification/elixir/lib/calls.ex 'recursive_loop(value - 1)'
assert_contains Q-ELIXIR-STATIC qualification/elixir/lib/calls.ex 'static_but_not_executed'
assert_contains Q-ELIXIR-PATH scripts/qualify.sh 'elixir)'
assert_contains Q-STATE qualification/README.md 'PASS, BLOCKED, or FAIL'
assert_contains Q-EVIDENCE qualification/README.md 'A PASS requires server version'
assert_contains Q-RETAIN qualification/README.md './scripts/retain-qualification.py'
for language in typescript csharp elixir; do
  assert_contains "Q-RETAINED-$language-STATUS" "qualification/retained/$language/status.txt" 'PASS'
  assert_contains "Q-RETAINED-$language-ASSERTIONS" "qualification/retained/$language/assertions.txt" 'all caller/callee edges retain exact non-empty call-site ranges'
  assert_contains "Q-RETAINED-$language-GRAPH" "qualification/retained/$language/graph.json" '"schema_version": "lsp-trace.graph.v1"'
done
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
