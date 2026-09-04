#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
failed=0

cd "$root"
./scripts/check-docs.sh
printf 'PASS R-DOCUMENTATION-CONTRACT: operational documentation guard\n'

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
assert_contains Q-ELIXIR-PATH scripts/qualify.sh 'elixir|elixir-companion)'
assert_file Q-ELIXIR-CROSS-FILE qualification/elixir/lib/cross_module_callers.ex
assert_contains Q-ELIXIR-MULTI-CLAUSE qualification/elixir/lib/calls.ex 'def leaf(value) when'
assert_contains Q-ELIXIR-ALIAS qualification/elixir/lib/cross_module_callers.ex 'alias LspTraceQualification.Calls'
assert_contains Q-ELIXIR-QUALIFIED qualification/elixir/lib/cross_module_callers.ex 'LspTraceQualification.Calls.leaf'
assert_contains Q-ELIXIR-BLOCKED scripts/qualify.sh 'missing cross-module caller edge'
assert_contains Q-ELIXIR-EVIDENCE qualification/README.md 'protocol support, same-module resolution, cross-module resolution, and multi-clause resolution'
assert_contains Q-ELIXIR-COMPANION-MODE scripts/qualify.sh 'elixir-companion'
assert_contains Q-ELIXIR-COMPANION-ROOT scripts/qualify.sh 'ELIXIR_CALL_HIERARCHY_ROOT'
assert_contains Q-ELIXIR-COMPANION-DEFAULT scripts/qualify.sh '../elixir-call-hierarchy'
assert_contains Q-ELIXIR-COMPANION-REPOSITORY qualification/README.md 'https://github.com/jaresty/elixir-call-hierarchy'
assert_contains Q-ELIXIR-COMPANION-EXTERNAL-CONTRACT qualification/README.md "external server's own README defines its profiling, trust, completeness, and clause-coalescing contracts"
assert_contains Q-ELIXIR-COMPANION-CI-REPOSITORY .github/workflows/ci.yml 'repository: jaresty/elixir-call-hierarchy'
assert_contains Q-ELIXIR-COMPANION-CI-PIN .github/workflows/ci.yml 'ref: aab27746f1ab03a3ce24fa3e636bbc252c0236ff'
assert_contains Q-ELIXIR-COMPANION-CI-PATH .github/workflows/ci.yml 'path: elixir-call-hierarchy'
assert_contains Q-ELIXIR-LS-DISTINCT qualification/README.md 'does not replace or rewrite the retained ElixirLS BLOCKED evidence'
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
assert_contains R-ELIXIR-CI-ESCRIPT .github/workflows/ci.yml 'mix escript.build'
assert_file R-SECURITY docs/SECURITY.md
assert_file R-SEMANTICS docs/SEMANTICS.md
assert_file R-SCHEMA docs/SCHEMA_POLICY.md
assert_file R-RELEASE docs/RELEASING.md
assert_file R-PLATFORM .goreleaser.yaml
for version in v1 v2 v3; do
  assert_file "R-SCHEMA-$version" "internal/schema/schemas/lsp-trace.graph.$version.schema.json"
done
assert_file R-INSPECTION-SCHEMA internal/schema/schemas/lsp-trace.inspect.v1.schema.json
assert_contains R-SCHEMA-GET-DOC README.md 'lsp-trace schema get --schema v1|v2|v3'
assert_contains R-VALIDATE-DOC README.md 'lsp-trace validate [--schema v1|v2|v3] PATH|-'
assert_contains R-INSPECT-DOC README.md 'lsp-trace inspect SELECTOR_OR_ARTIFACT --seed LABEL'
assert_contains R-INSPECT-ALL-DOC README.md 'lsp-trace inspect SELECTOR_OR_ARTIFACT --all-seeds --json'
assert_contains R-INSPECT-AUTHORITY docs/SEMANTICS.md '`NON_AUTHORITATIVE_DERIVED_VIEW`'
assert_contains R-INSPECT-DIAGNOSTIC-CORRELATION docs/SEMANTICS.md '`TOOL_DERIVED_NODE_CORRELATION`'
assert_contains R-SKILL-INSPECT cmd/lsp-trace/SKILL.md '## Inspect retained seeds'
assert_contains R-SKILL-INSPECT-ALL cmd/lsp-trace/SKILL.md 'lsp-trace inspect SELECTOR_OR_ARTIFACT --all-seeds --json'
assert_contains R-SLICE-DOC README.md 'lsp-trace slice'
assert_contains R-SLICE-DOWN README.md '`--down-depth` and `--up-depth` count call edges'
assert_contains R-SLICE-START-MODES README.md 'Choose exactly one starting mode'
assert_contains R-SLICE-AT README.md 'repeatable `--at PATH:LINE:COLUMN`'
assert_contains R-SLICE-SEED-FILE README.md '`--seed-file FILE` uses the existing labeled seed JSON format'
assert_contains R-SLICE-SEMANTICS docs/SEMANTICS.md 'Nodes at exactly `--down-depth` form the exact frontier.'
assert_contains R-SLICE-UPWARD-UNION docs/SEMANTICS.md '`upward_start_node_ids` must equal the sorted deduplicated union'
assert_contains R-SLICE-FAILURE-NOT-LEAF docs/SEMANTICS.md 'A null or failed outgoing response is not a server-reported leaf'
assert_contains R-SLICE-SEED-CLOSURE docs/SEMANTICS.md 'Each successful slice seed records its deterministic causal closure'
assert_contains R-SLICE-SEED-UNION docs/SEMANTICS.md 'union of successful seed memberships to equal the deduplicated global node/relation sets'
assert_contains R-SLICE-POLICY docs/SCHEMA_POLICY.md 'V3 may include an optional `slice` object'
assert_contains R-SLICE-SCHEMA internal/schema/schemas/lsp-trace.graph.v3.schema.json '"slice": {"$ref": "#/$defs/slice"}'
assert_contains R-SLICE-SCHEMA-UPWARD internal/schema/schemas/lsp-trace.graph.v3.schema.json '"upward_start_node_ids"'
assert_contains R-SKILL-VALIDATE-PRESERVES cmd/lsp-trace/SKILL.md 'Validation does not canonicalize or rewrite input.'
assert_contains R-SKILL-VALIDATION-LAYERS cmd/lsp-trace/SKILL.md 'V1 and v2 validation is structural; v3 runs structural validation before deeper semantic validation.'
assert_contains R-SKILL-NO-AUTH cmd/lsp-trace/SKILL.md 'Validation and verification do not authenticate producer identity or prove that a declared process executed.'
assert_contains R-SKILL-AUTHORITY cmd/lsp-trace/SKILL.md 'Invocation provenance is caller-supplied; normalized identities, digests, and receipts are tool-derived.'
assert_contains R-SKILL-NO-RAW-CUSTODY cmd/lsp-trace/SKILL.md 'The raw server-stderr stream is not retained as a standalone artifact'
assert_contains R-SKILL-SLICE-DESCRIPTION cmd/lsp-trace/SKILL.md 'description: Trace and slice server-reported call hierarchies'
assert_contains R-SKILL-SERVER-TRUST cmd/lsp-trace/SKILL.md 'Use only trusted language-server binaries and workspaces.'
assert_contains R-SKILL-SERVER-STARTUP cmd/lsp-trace/SKILL.md '`language-server` is a placeholder.'
assert_contains R-SKILL-SEED-GRAMMAR cmd/lsp-trace/SKILL.md 'Unknown seed-file fields are rejected.'
assert_contains R-SKILL-PUBLISH-STDOUT-FALLBACK cmd/lsp-trace/SKILL.md 'exit code `1` may carry the complete marshaled graph on stdout'
assert_contains R-SKILL-FAILED-SEED-STDERR cmd/lsp-trace/SKILL.md "stderr lists each failed seed's label, failure phase, and failure message"
assert_contains R-SKILL-RANGE-WARNING cmd/lsp-trace/SKILL.md '`SERVER_CALL_SITE_OUTSIDE_CALLER_RANGE` is advisory'
assert_contains R-SKILL-ARTIFACT-DETERMINISM cmd/lsp-trace/SKILL.md 'With identical invocation inputs and server observations, canonical artifact bytes are deterministic'
assert_contains R-SKILL-GENERATION-NAME cmd/lsp-trace/SKILL.md 'generation directory basenames are opaque publication coordinates, not content identities'
assert_contains R-SKILL-SEED-FAILURE-FIELDS cmd/lsp-trace/SKILL.md '`failure.phase` and `failure.message`'
assert_contains R-SKILL-SLICE-OPTIONS cmd/lsp-trace/SKILL.md 'Slice accepts `--server-arg`, `--server-env`, `--language-id`, `--down-depth`, `--up-depth`, `--max-nodes`, `--timeout`, `--request-timeout`, `--output`, and `--pretty`.'
assert_contains R-SKILL-TRACE-SENSITIVITY cmd/lsp-trace/SKILL.md '`--trace-lsp PATH` writes an opt-in JSON Lines transcript'
assert_contains R-SLICE-STDERR-RELAY docs/SEMANTICS.md 'captured stderr is relayed before the transport/lifecycle error'
assert_contains R-SLICE-STDERR-DIAGNOSTIC docs/SEMANTICS.md 'retained and printed as a sensitive `server-stderr` diagnostic'
assert_contains R-V3-CLOSED-RELATION-KINDS docs/SEMANTICS.md 'Native relation kinds are closed to call relations, dispatch associations, and sibling candidates'
assert_contains R-V3-EXECUTION-BUNDLE docs/SEMANTICS.md 'This common provenance does not make co-bundled relations independent observations and adds no confidence or support.'
assert_contains R-V3-LOCATOR-CEILING docs/SEMANTICS.md 'they do not establish runtime behavior or feature correspondence'
assert_contains R-V3-AUTHORITY-SCOPE docs/SEMANTICS.md 'Source identity is limited to successfully resolved seed URI/content-digest pairs'
assert_contains R-V3-HISTORICAL-ADDITIVE docs/SCHEMA_POLICY.md 'historical v3 documents that omit them remain structurally and semantically valid'
assert_contains R-SLICE-RANGE-WARNING-RECOVERABLE docs/SEMANTICS.md '`SERVER_CALL_SITE_OUTSIDE_CALLER_RANGE` is a recoverable server-quality diagnostic'
assert_contains R-SLICE-RANGE-WARNING-PUBLISH docs/SEMANTICS.md 'this diagnostic alone does not make the slice incomplete or prevent selector publication'
assert_contains R-SLICE-INVALID-RELATION-CLOSED docs/SEMANTICS.md 'malformed or unattributable callers, node-identity collisions, and dangling relation references remain fail-closed structural errors'
assert_contains R-GENERATION-NAME-NOT-IDENTITY README.md 'Generation directory basenames are opaque publication coordinates, not content identities and not part of the determinism contract.'
assert_contains R-ARTIFACT-BYTE-DETERMINISM docs/SEMANTICS.md 'With identical invocation inputs and server observations, canonical artifact bytes are deterministic; immutable generation directory basenames may differ between publications.'
assert_contains R-FLAGS README.md '## Flags'
for flag in workspace config profile server at server-arg server-env language-id max-depth max-nodes timeout request-timeout concurrency log-level trace-lsp output pretty; do
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
assert_file R-MCP-ADR docs/adr/0003-persistent-mcp-language-server-sessions.md
assert_file R-MCP-SERVER cmd/lsp-trace-mcp/main.go
assert_file R-MCP-MANIFEST internal/mcpcontract/testdata/stage1-manifest.v1.json
assert_file R-MCP-ENVELOPE-SCHEMA internal/mcpcontract/testdata/schemas/envelope-result.v1.schema.json
assert_contains R-MCP-PROTOCOL-PIN internal/mcpcontract/testdata/stage1-manifest.v1.json '"revision":"2025-06-18"'
assert_contains R-MCP-GORELEASER-MAIN .goreleaser.yaml 'main: ./cmd/lsp-trace-mcp'
assert_contains R-MCP-GORELEASER-BINARY .goreleaser.yaml 'binary: lsp-trace-mcp'
assert_contains R-MCP-ADR-SUPERSEDED docs/adr/0003-persistent-mcp-language-server-sessions.md '**Status:** Superseded by'
assert_contains R-MCP-ADR-SUCCESSOR docs/adr/0003-persistent-mcp-language-server-sessions.md '[ADR 0003: Activate always-local Stage 2 lifecycle tools](0003-always-local-stage2.md)'
assert_contains R-MCP-RELEASE-DOC docs/RELEASING.md 'lsp-trace-mcp'
assert_contains R-MCP-BOOTSTRAP-DOC README.md 'lsp-trace-mcp --bootstrap-config /absolute/path/bootstrap.json'
assert_contains R-MCP-BOOTSTRAP-HOST-AUTHORITY README.md 'the host—not the MCP caller—provisions trusted sessions'
assert_contains R-MCP-PRODUCTION-BOOTSTRAP-TEST cmd/lsp-trace-mcp/bootstrap_process_test.go 'func TestProductionBootstrapBlocksStdioUntilHostConfiguredProcessIsReady'

if [ "$failed" -ne 0 ]; then
  exit 1
fi

if go test ./internal/schema -run Inspection -count=1 &&
   go test ./cmd/lsp-trace -run 'TestInspect|TestProjectAllSeedInspection|TestValidateAllSeedAccounting' -count=1; then
  printf 'PASS R-INSPECTION-CONTRACT: schema, modes, custody, accounting, determinism, and equivalence\n'
else
  printf 'FAIL R-INSPECTION-CONTRACT: hermetic inspection contract suite failed\n'
  exit 1
fi

if go test ./cmd/lsp-trace-mcp -run 'TestProductionBootstrap|TestRunStdioOnly' -count=1; then
  printf 'PASS R-MCP-PRODUCTION-BOOTSTRAP-STDIO: production bootstrap, trusted-local stderr, and clean MCP stdout\n'
else
  printf 'FAIL R-MCP-PRODUCTION-BOOTSTRAP-STDIO: production bootstrap or stdio channel contract failed\n'
  exit 1
fi
if go test ./internal/mcp -run TestLifecycleExecutorFamilyIsEnabledAndAdvertisedByDefault -count=1; then
  printf 'PASS R-MCP-EXACT-TWELVE-TOOLS: exactly twelve canonical tools\n'
else
  printf 'FAIL R-MCP-EXACT-TWELVE-TOOLS: canonical tool cardinality contract failed\n'
  exit 1
fi
if go test ./internal/mcpcontract ./internal/mcp ./internal/operation ./cmd/lsp-trace-mcp; then
  printf 'PASS R-MCP-CONTRACT: MCP Stage 1 contract, transport, registry, and real-process suites\n'
else
  printf 'FAIL R-MCP-CONTRACT: MCP Stage 1 contract suite failed\n'
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
go build -trimpath -o "$release_tmp/lsp-trace-mcp" ./cmd/lsp-trace-mcp
if [ ! -s "$release_tmp/lsp-trace-mcp" ]; then
  printf 'FAIL R-MCP-DRY-BUILD: MCP release binary is missing or empty\n'
  exit 1
fi
printf 'PASS R-MCP-DRY-BUILD: hermetic non-publishing MCP release binary\n'
embedded_skill=$($release_tmp/lsp-trace skill get)
case "$embedded_skill" in
  *'## Compare two retained seed-evidence sets'*'Shared references do not establish shared feature or workflow identity'*)
    printf 'PASS R-EMBEDDED-FILTER-CONTRACT: release binary embeds filter-v1 operational guidance\n'
    ;;
  *)
    printf 'FAIL R-EMBEDDED-FILTER-CONTRACT: release binary skill lacks filter-v1 operational guidance\n'
    exit 1
    ;;
esac
for version in v1 v2 v3; do
  "$release_tmp/lsp-trace" schema get --schema "$version" > "$release_tmp/$version.schema.json"
  if cmp -s "$release_tmp/$version.schema.json" "$root/internal/schema/schemas/lsp-trace.graph.$version.schema.json"; then
    printf 'PASS R-SCHEMA-%s-BYTES: release binary embeds exact committed schema\n' "$version"
  else
    printf 'FAIL R-SCHEMA-%s-BYTES: release binary schema differs from committed bytes\n' "$version"
    exit 1
  fi
done
if "$release_tmp/lsp-trace" validate --schema v2 "$root/qualification/retained/typescript/graph.json" >/dev/null; then
  printf 'PASS R-VALIDATE-V2: retained qualification graph passes release validator\n'
else
  printf 'FAIL R-VALIDATE-V2: retained qualification graph failed release validator\n'
  exit 1
fi
printf 'RELEASE CHECK PASS\n'
