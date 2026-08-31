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
assert_contains Q-ELIXIR-RECURSION qualification/elixir/lib/calls.ex 'recursive(value - 1)'
assert_contains Q-ELIXIR-STATIC qualification/elixir/lib/calls.ex 'static_but_not_executed'
assert_contains Q-ELIXIR-PATH scripts/qualify.sh 'elixir)'
assert_file Q-ELIXIR-CROSS-FILE qualification/elixir/lib/cross_module_callers.ex
assert_contains Q-ELIXIR-MULTI-CLAUSE qualification/elixir/lib/calls.ex 'def leaf(value) when'
assert_contains Q-ELIXIR-ALIAS qualification/elixir/lib/cross_module_callers.ex 'alias LspTraceQualification.Calls'
assert_contains Q-ELIXIR-QUALIFIED qualification/elixir/lib/cross_module_callers.ex 'LspTraceQualification.Calls.leaf'
assert_contains Q-ELIXIR-BLOCKED scripts/qualify.sh 'missing cross-module caller edge'
assert_contains Q-ELIXIR-EVIDENCE qualification/README.md 'protocol support, same-module resolution, cross-module resolution, and multi-clause resolution'
assert_contains Q-TERMINAL-PROVENANCE docs/SEMANTICS.md 'Terminal provenance'
assert_contains Q-SOURCE-COMPLETE docs/SEMANTICS.md 'source completeness'
assert_contains Q-SCHEMA-V1 qualification/README.md 'lsp-trace.graph.v1'
assert_contains Q-STATE qualification/README.md 'PASS, BLOCKED, or FAIL'
assert_contains Q-EVIDENCE qualification/README.md 'A PASS requires server version'
assert_contains Q-RETAIN qualification/README.md './scripts/retain-qualification.py'
for language in typescript csharp; do
  assert_contains "Q-RETAINED-$language-STATUS" "qualification/retained/$language/status.txt" 'PASS'
  assert_contains "Q-RETAINED-$language-ASSERTIONS" "qualification/retained/$language/assertions.txt" 'all caller/callee edges retain exact non-empty call-site ranges'
  assert_contains "Q-RETAINED-$language-GRAPH" "qualification/retained/$language/graph.json" '"schema_version": "lsp-trace.graph.v2"'
done
assert_contains Q-RETAINED-elixir-STATUS qualification/retained/elixir/status.txt 'BLOCKED:'
assert_contains Q-RETAINED-elixir-GRAPH qualification/retained/elixir/graph.json '"schema_version": "lsp-trace.graph.v2"'
assert_contains Q-RETAINED-elixir-MISSING qualification/retained/elixir/assertions.txt 'missing exact callers'
assert_file R-CI .github/workflows/ci.yml
assert_contains R-ELIXIR-CI-JOB .github/workflows/ci.yml 'elixir-companion:'
assert_contains R-ELIXIR-CI-SETUP .github/workflows/ci.yml 'erlef/setup-beam@v1'
assert_contains R-ELIXIR-CI-OTP .github/workflows/ci.yml "otp-version: '25.3.2.7'"
assert_contains R-ELIXIR-CI-VERSION .github/workflows/ci.yml "elixir-version: '1.16.2'"
assert_contains R-ELIXIR-CI-DEPS .github/workflows/ci.yml 'mix deps.get --only test'
assert_contains R-ELIXIR-CI-FORMAT .github/workflows/ci.yml 'mix format --check-formatted'
assert_contains R-ELIXIR-CI-TEST .github/workflows/ci.yml 'mix test'
assert_file R-SECURITY docs/SECURITY.md
assert_file R-SEMANTICS docs/SEMANTICS.md
assert_file R-SCHEMA docs/SCHEMA_POLICY.md
assert_file R-RELEASE docs/RELEASING.md
assert_file R-PLATFORM .goreleaser.yaml
assert_contains R-FLAGS README.md '## Flags'
for flag in workspace server at server-arg server-env language-id max-depth max-nodes timeout request-timeout concurrency log-level trace-lsp output pretty; do
  assert_contains "R-FLAG-$flag" README.md "\`--$flag"
done
assert_contains R-COMPLETE docs/SEMANTICS.md '`summary.complete`'
assert_contains R-TRUNCATED docs/SEMANTICS.md '`summary.truncated`'
assert_contains R-REASONS docs/SCHEMA_POLICY.md '## Reason enum'
for reason in NO_INCOMING_CALLS PREPARE_RETURNED_NO_ITEM INCOMING_RETURNED_NULL EXTERNAL_URI UNSUPPORTED_CALL_HIERARCHY SERVER_ERROR INVALID_SERVER_RESPONSE REQUEST_TIMEOUT GLOBAL_TIMEOUT CANCELLED MAX_DEPTH MAX_NODES NODE_ID_COLLISION; do
  assert_contains "R-REASON-$reason" docs/SCHEMA_POLICY.md "\`$reason\`"
done
assert_contains R-TRACE docs/SECURITY.md '`--trace-lsp PATH` writes an opt-in JSON Lines protocol transcript'
assert_contains R-TIMEOUT docs/SEMANTICS.md '`GLOBAL_TIMEOUT`'
assert_contains R-SUPPORT README.md 'retained PASS evidence'
assert_contains R-DRY-RUN docs/RELEASING.md './scripts/release-check.sh'
assert_contains R-NO-SERVERS docs/RELEASING.md 'does not start or install external language servers'
assert_contains R-GORELEASER .goreleaser.yaml 'version: 2'
assert_contains R-RETAIN-STATUS scripts/release-check.sh 'Q-RETAINED-$language-STATUS'

if [ "$failed" -ne 0 ]; then
  exit 1
fi

release_tmp=$(mktemp -d "${TMPDIR:-/tmp}/lsp-trace-release.XXXXXX")
trap 'rm -rf "$release_tmp"' EXIT HUP INT TERM
go build -trimpath -o "$release_tmp/lsp-trace" ./cmd/lsp-trace
if [ ! -s "$release_tmp/lsp-trace" ]; then
  printf 'FAIL R-DRY-BUILD: release binary is missing or empty\n'
  exit 1
fi
printf 'PASS R-DRY-BUILD: hermetic non-publishing release binary\n'
printf 'RELEASE CHECK PASS\n'
