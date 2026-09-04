#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
failed=0

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

assert_heading_order() {
  id=$1
  path=$2
  shift 2
  previous=0
  for heading in "$@"; do
    line=$(grep -n -F -x "$heading" "$root/$path" | cut -d: -f1 || true)
    case "$line" in
      ''|*[!0-9]*)
        printf 'FAIL %s: %s must contain exactly one %s heading\n' "$id" "$path" "$heading"
        failed=1
        return
        ;;
    esac
    if [ "$line" -le "$previous" ]; then
      printf 'FAIL %s: %s top-level heading order breaks at %s\n' "$id" "$path" "$heading"
      failed=1
      return
    fi
    previous=$line
  done
  printf 'PASS %s: %s has stable task-first top-level heading order\n' "$id" "$path"
}

assert_heading_order DOC-SKILL-TASK-FIRST cmd/lsp-trace/SKILL.md \
  '## Quick start' \
  '## Choose a command' \
  '## Operational workflows' \
  '## Interpretation boundaries' \
  '## Reference'
assert_contains DOC-SKILL-ROUTER-INCOMING cmd/lsp-trace/SKILL.md '`incoming`: start from exact callee positions and trace callers upward.'
assert_contains DOC-SKILL-ROUTER-SLICE cmd/lsp-trace/SKILL.md '`slice`: discover bounded outgoing nodes first, then trace incoming callers from the exact frontier and server-reported leaves.'
assert_contains DOC-SKILL-ROUTER-INSPECT cmd/lsp-trace/SKILL.md '`inspect`: admit and project one seed or all retained seeds without changing evidence authority.'
assert_contains DOC-SKILL-ROUTER-FILTER cmd/lsp-trace/SKILL.md '`filter`: mechanically compare exactly two seeds from an admitted all-seeds inspection.'

assert_contains DOC-README README.md '## Inspect retained seeds'
assert_contains DOC-SEMANTICS docs/SEMANTICS.md '## Seed inspection operational contract'
assert_contains DOC-SKILL cmd/lsp-trace/SKILL.md '## Build technical inputs for a feature inventory'
assert_contains DOC-ADR docs/adr/0001-technical-evidence-packet-projections.md '# ADR 0001: Add versioned all-seed inspection projections'
assert_contains DOC-FILTER-ADR docs/adr/0002-deterministic-seed-evidence-filtering.md '# ADR 0002: Add deterministic pairwise seed-evidence comparison'
assert_contains DOC-FILTER-README README.md '## Compare retained seed evidence'
assert_contains DOC-FILTER-SEMANTICS docs/SEMANTICS.md '## Pairwise seed-evidence filter operational contract'
assert_contains DOC-FILTER-SKILL cmd/lsp-trace/SKILL.md '## Compare two retained seed-evidence sets'
assert_contains DOC-FILTER-COMMAND README.md 'lsp-trace filter evidence-inspection.json --compare-seeds LEFT_LABEL --compare-seeds RIGHT_LABEL --json'
assert_contains DOC-FILTER-TYPED docs/SEMANTICS.md 'ReferenceKey = (namespace, value)'
assert_contains DOC-FILTER-CEILING cmd/lsp-trace/SKILL.md 'Shared references do not establish shared feature or workflow identity'
assert_contains DOC-ALL-SEEDS cmd/lsp-trace/SKILL.md 'lsp-trace inspect evidence.selector.json --all-seeds --json'
assert_contains DOC-SEED cmd/lsp-trace/SKILL.md 'lsp-trace inspect SELECTOR_OR_ARTIFACT --seed LABEL --json'
assert_contains DOC-TRAVERSAL cmd/lsp-trace/SKILL.md '### Choose a traversal'
assert_contains DOC-STATUS cmd/lsp-trace/SKILL.md '### Handle traversal status'
assert_contains DOC-RECONCILE cmd/lsp-trace/SKILL.md '### Reconcile all stored seeds'
assert_contains DOC-INPUT README.md 'lsp-trace inspect SELECTOR_OR_ARTIFACT --seed LABEL --json'
assert_contains DOC-ALL-INPUT README.md 'lsp-trace inspect SELECTOR_OR_ARTIFACT --all-seeds --json'
assert_contains DOC-PRECEDENCE docs/SEMANTICS.md 'custody verification precedes structural validation, structural validation precedes semantic validation, and inspection follows successful admission'
assert_contains DOC-AUTHORITY docs/SEMANTICS.md '`NON_AUTHORITATIVE_DERIVED_VIEW`'
assert_contains DOC-RELEASE scripts/release-check.sh './scripts/check-docs.sh'
assert_contains DOC-PROFILE-README README.md '## Named server profiles'
assert_contains DOC-PROFILE-SEMANTICS docs/SEMANTICS.md '## Named server profile resolution'
assert_contains DOC-PROFILE-SKILL cmd/lsp-trace/SKILL.md '## Use named server profiles'
assert_contains DOC-PROFILE-SECRET README.md 'Graph invocation output records environment names/references, never values.'
assert_contains DOC-MCP-README README.md '## MCP offline evidence server'
assert_contains DOC-MCP-SKILL cmd/lsp-trace/SKILL.md '### Use the MCP offline evidence server'
assert_contains DOC-MCP-STDIO README.md 'lsp-trace-mcp --publication-root'
assert_contains DOC-MCP-CAPABILITIES README.md 'lsp_trace_v1_capabilities'
assert_contains DOC-MCP-ALWAYS-LOCAL README.md 'local-development-only'
assert_contains DOC-MCP-TWELVE-TOOLS README.md 'twelve canonical tools'
assert_contains DOC-MCP-INCOMING cmd/lsp-trace/SKILL.md 'bounded incoming traversal are enabled by default'
assert_contains DOC-MCP-SLICE-ENABLED cmd/lsp-trace/SKILL.md 'Slice traversal is enabled by default'
assert_contains DOC-MCP-WARNING README.md "developer's permissions"
assert_contains DOC-MCP-NO-SANDBOX README.md 'not sandboxed'
assert_contains DOC-MCP-LOCAL-ACCESS README.md 'local files and network'
assert_contains DOC-MCP-TRUST README.md 'must be trusted'
assert_contains DOC-MCP-ADR docs/adr/0003-always-local-stage2.md '# ADR 0003: Activate always-local Stage 2 lifecycle tools'
assert_contains DOC-MCP-HISTORICAL-ADR docs/adr/0003-persistent-mcp-language-server-sessions.md '**Status:** Superseded by [ADR 0003: Activate always-local Stage 2 lifecycle tools](0003-always-local-stage2.md)'

if [ "$failed" -ne 0 ]; then
  exit 1
fi
printf 'DOCUMENTATION CHECK PASS\n'
