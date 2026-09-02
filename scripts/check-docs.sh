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

if [ "$failed" -ne 0 ]; then
  exit 1
fi
printf 'DOCUMENTATION CHECK PASS\n'
