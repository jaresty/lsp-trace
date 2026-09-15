#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
failed=0

assert_contains() {
  id=$1
  path=$2
  text=$3
  if [ -f "$root/$path" ] && grep -F -- "$text" "$root/$path" >/dev/null; then
    printf 'PASS %s: %s contains %s\n' "$id" "$path" "$text"
  else
    printf 'FAIL %s: %s must contain %s\n' "$id" "$path" "$text"
    failed=1
  fi
}

assert_same() {
  id=$1
  left=$2
  right=$3
  if cmp -s "$root/$left" "$root/$right"; then
    printf 'PASS %s: %s matches %s byte-for-byte\n' "$id" "$left" "$right"
  else
    printf 'FAIL %s: %s must match %s byte-for-byte\n' "$id" "$left" "$right"
    failed=1
  fi
}

assert_not_contains() {
  id=$1
  path=$2
  text=$3
  if [ -f "$root/$path" ] && ! grep -F -- "$text" "$root/$path" >/dev/null; then
    printf 'PASS %s: %s excludes stale %s\n' "$id" "$path" "$text"
  else
    printf 'FAIL %s: %s must exclude stale %s\n' "$id" "$path" "$text"
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

assert_following_nonempty_line_equals() {
  id=$1
  path=$2
  heading=$3
  expected=$4
  actual=$(awk -v heading="$heading" '
    $0 == heading { headings++; seeking=1; next }
    seeking && NF { print; seeking=0 }
    END { if (headings != 1) exit 1 }
  ' "$root/$path" || true)
  if [ "$actual" = "$expected" ]; then
    printf 'PASS %s: %s binds %s to its exact exhaustive set\n' "$id" "$path" "$heading"
  else
    printf 'FAIL %s: %s must bind %s to exactly %s\n' "$id" "$path" "$heading" "$expected"
    failed=1
  fi
}

assert_exactly_one_line() {
  id=$1
  path=$2
  text=$3
  count=$(grep -F -x -c -- "$text" "$root/$path" || true)
  if [ "$count" -eq 1 ]; then
    printf 'PASS %s: %s contains exactly one %s\n' "$id" "$path" "$text"
  else
    printf 'FAIL %s: %s must contain exactly one %s (found %s)\n' "$id" "$path" "$text" "$count"
    failed=1
  fi
}

assert_heading_order DOC-SKILL-TASK-FIRST cmd/lsp-trace/SKILL.md \
  '## Route the task' \
  '## Command router' \
  '## Shared evidence boundary' \
  '## Retrieval and authority'
assert_contains DOC-SKILL-NAME cmd/lsp-trace/SKILL.md 'name: lsp-trace'
assert_contains DOC-SKILL-DESCRIPTION cmd/lsp-trace/SKILL.md 'description: Structural code analysis with lsp-trace:'
assert_contains DOC-SKILL-ROUTER-INCOMING cmd/lsp-trace/SKILL.md '`incoming`: start from exact callee positions and trace callers upward.'
assert_contains DOC-SKILL-ROUTER-SLICE cmd/lsp-trace/SKILL.md '`slice`: discover bounded outgoing nodes, then trace incoming callers from the exact frontier and genuine server-reported empty outgoing leaves.'
assert_contains DOC-SKILL-ROUTER-INSPECT cmd/lsp-trace/SKILL.md '`inspect`: admit and project one seed or all retained seeds without changing evidence authority.'
assert_contains DOC-SKILL-ROUTER-FILTER cmd/lsp-trace/SKILL.md '`filter`: mechanically compare exactly two seeds from an admitted all-seeds inspection.'
assert_contains DOC-SKILL-REF-LIVE cmd/lsp-trace/references/live-tracing.md '# Live tracing and census'
assert_contains DOC-SKILL-REF-OFFLINE cmd/lsp-trace/references/offline-evidence.md '# Offline evidence operations'
assert_contains DOC-SKILL-REF-TRANSPORT cmd/lsp-trace/references/transport-routing.md '# Transport and routing'
assert_contains DOC-SKILL-REF-BOUNDARIES cmd/lsp-trace/references/evidence-boundaries.md '# Evidence boundaries'
assert_contains DOC-FEATURE-SKILL-NAME .pi/skills/lsp-trace-feature-inventory/SKILL.md 'name: lsp-trace-feature-inventory'
assert_contains DOC-FEATURE-SKILL-DESCRIPTION .pi/skills/lsp-trace-feature-inventory/SKILL.md 'description: Prepare and review correction-safe feature inventories'
assert_contains DOC-FEATURE-REF-PREP .pi/skills/lsp-trace-feature-inventory/references/preparation-and-grouping.md '# Preparation and reversible grouping'
assert_contains DOC-FEATURE-REF-ADJUDICATION .pi/skills/lsp-trace-feature-inventory/references/adjudication-and-acceptance.md '# Adjudication and acceptance'
assert_contains DOC-CALLS-SERVER-ONLY cmd/lsp-trace/references/evidence-boundaries.md 'CALLS records are server-reported Call Hierarchy relations only.'
assert_contains DOC-CAPTURE-SET-BOUNDARY cmd/lsp-trace/references/evidence-boundaries.md 'do not infer cross-capture CALLS'
assert_contains DOC-UNKNOWN-BOUNDARY cmd/lsp-trace/references/evidence-boundaries.md '`source_graph_complete` remains `UNKNOWN`.'
assert_contains DOC-MECHANICAL-NO-ACCEPT .pi/skills/lsp-trace-feature-inventory/SKILL.md 'Mechanical preparation cannot accept feature identity.'
assert_contains DOC-COMMUNITY-INSTABILITY .pi/skills/lsp-trace-feature-inventory/SKILL.md 'communities, centrality, instability'
assert_contains DOC-CENSUS-AVAILABLE-FEATURE-SKILL .pi/skills/lsp-trace-feature-inventory/SKILL.md 'The documented CLI `census` command may be used'
assert_contains DOC-CONTEXT-AVAILABLE-FEATURE-SKILL .pi/skills/lsp-trace-feature-inventory/SKILL.md '`context` is AVAILABLE for bounded transient live structural analysis'
assert_contains DOC-LEIDEN-HEADLINE .pi/skills/lsp-trace-feature-inventory/references/preparation-and-grouping.md 'a community is a structurally notable set worth examining, never a feature'
assert_contains DOC-LEIDEN-INDEPENDENT-CENSUS .pi/skills/lsp-trace-feature-inventory/references/preparation-and-grouping.md 'independent entry-point census'
assert_contains DOC-LEIDEN-INSTABILITY .pi/skills/lsp-trace-feature-inventory/references/preparation-and-grouping.md 'A-08 instability campaigns'
assert_contains DOC-LEIDEN-RECAPTURE .pi/skills/lsp-trace-feature-inventory/references/preparation-and-grouping.md 'Recapture only bytes that are missing, mismatched, or unavailable'
assert_contains DOC-LIVE-ORIENTATION cmd/lsp-trace/references/live-tracing.md 'Graph Provenance and source capture are not prerequisites'
assert_contains DOC-LIVE-CLAIM-CEILING cmd/lsp-trace/references/live-tracing.md 'it is not retained, replayable, or source-grounded evidence'
assert_contains DOC-LIVE-OPTIONAL-ESCALATION cmd/lsp-trace/references/live-tracing.md 'That escalation is never the default prerequisite'
assert_contains DOC-LIVE-NO-CENTRALITY cmd/lsp-trace/references/live-tracing.md 'They do not compute centrality, establish architectural boundaries'
assert_contains DOC-LIVE-CENSUS-AVAILABLE cmd/lsp-trace/references/live-tracing.md 'Current CLI `census` is AVAILABLE'
assert_contains DOC-LIVE-CONTEXT-AVAILABLE cmd/lsp-trace/references/live-tracing.md 'The `context` interface accepted in ADR 0006 is AVAILABLE for bounded transient live analysis'
assert_contains DOC-README-CENSUS-AVAILABLE README.md '### Accountable source-symbol census'
assert_contains DOC-README-CENSUS-AUTHORITY README.md '`authority` is `0`, `source_graph_complete` is `UNKNOWN`'
assert_contains DOC-README-CONTEXT-AVAILABLE README.md '`context` is AVAILABLE for bounded transient live structural analysis'
assert_contains DOC-README-SYMBOL-CHURN README.md 'context-churn-symbols'
assert_contains DOC-README-SYMBOL-CHURN-CEILING README.md '`cross_revision_identity=NOT_EVALUATED`'
assert_contains DOC-SEMANTICS-SYMBOL-CHURN docs/SEMANTICS.md '## Historical symbol churn V2 operational contract'
assert_contains DOC-SEMANTICS-SYMBOL-NO-CALLS docs/SEMANTICS.md 'cannot manufacture `CALLS`'
assert_contains DOC-README-LEGACY-UNSCHEDULED README.md 'Legacy `slice` and `incoming` remain visible and callable; removal is `UNSCHEDULED`.'
assert_contains DOC-MIGRATION-CENSUS-AVAILABLE docs/cli-migration-diagnostics.md 'The CLI `census` replacement is AVAILABLE'
assert_contains DOC-MIGRATION-CONTEXT-AVAILABLE docs/cli-migration-diagnostics.md '`context` is AVAILABLE with authority-zero, non-retained structural output'
assert_contains DOC-MIGRATION-LEGACY-UNSCHEDULED docs/cli-migration-diagnostics.md 'removal remains `UNSCHEDULED`'
assert_not_contains DOC-NO-STALE-CENSUS-CONTEXT-MIGRATION docs/cli-migration-diagnostics.md '`census` and `context` are unavailable proposals'
assert_not_contains DOC-NO-STALE-CENSUS-CONTEXT-LIVE cmd/lsp-trace/references/live-tracing.md 'The `context` and `census` interfaces accepted in ADR 0006 remain `FUTURE/PROPOSED`'
assert_contains DOC-SKILL-EXPORT-GRAMMAR cmd/lsp-trace/SKILL.md '`lsp-trace skill get (lsp-trace|lsp-trace-feature-inventory) DESTINATION`'
assert_contains DOC-SKILL-EXPORT-SAFETY cmd/lsp-trace/SKILL.md 'atomic no-replace final rename descriptor-relatively beneath it'
assert_contains DOC-SKILL-EXPORT-VISIBILITY cmd/lsp-trace/SKILL.md 'atomic for namespace visibility'
assert_contains DOC-SKILL-EXPORT-NO-DURABILITY cmd/lsp-trace/SKILL.md 'does not promise crash durability'
assert_same DOC-FEATURE-EMBEDDED-SKILL .pi/skills/lsp-trace-feature-inventory/SKILL.md cmd/lsp-trace/embedded-skills/lsp-trace-feature-inventory/SKILL.md
assert_same DOC-FEATURE-EMBEDDED-PREP .pi/skills/lsp-trace-feature-inventory/references/preparation-and-grouping.md cmd/lsp-trace/embedded-skills/lsp-trace-feature-inventory/references/preparation-and-grouping.md
assert_same DOC-FEATURE-EMBEDDED-ADJUDICATION .pi/skills/lsp-trace-feature-inventory/references/adjudication-and-acceptance.md cmd/lsp-trace/embedded-skills/lsp-trace-feature-inventory/references/adjudication-and-acceptance.md
assert_contains DOC-EMBEDDED-LSP-CENSUS cmd/lsp-trace/SKILL.md '`census`: use for accountable source-symbol enumeration'

assert_contains DOC-README README.md '## Inspect retained seeds'
assert_contains DOC-SEMANTICS docs/SEMANTICS.md '## Seed inspection operational contract'
assert_contains DOC-ADR docs/adr/0001-technical-evidence-packet-projections.md '# ADR 0001: Add versioned all-seed inspection projections'
assert_contains DOC-SEMANTIC-ADR-HEADING docs/adr/0007-optional-local-semantic-feature-index.md '# ADR 0007: Pilot an optional local engineering-context index'
assert_contains DOC-SEMANTIC-ADR-ACCEPTED docs/adr/0007-optional-local-semantic-feature-index.md '- **Status:** Accepted'
assert_contains DOC-SEMANTIC-ADR-PILOT-AUTHORIZATION docs/adr/0007-optional-local-semantic-feature-index.md 'Acceptance authorizes prerequisite work and, only after every frozen gate in this ADR is satisfied, implementation and execution of the isolated pilot.'
assert_contains DOC-SEMANTIC-ADR-FEATURE-DOWNSTREAM docs/adr/0007-optional-local-semantic-feature-index.md 'Feature inventory is one downstream workflow, not the primary product.'
assert_contains DOC-SEMANTIC-ADR-TYPED-CORPORA docs/adr/0007-optional-local-semantic-feature-index.md '## Strictly typed corpora'
assert_contains DOC-SEMANTIC-ADR-CORPUS-CODE docs/adr/0007-optional-local-semantic-feature-index.md '**Revision-bound code and structural evidence**'
assert_contains DOC-SEMANTIC-ADR-CORPUS-DECISIONS docs/adr/0007-optional-local-semantic-feature-index.md '**Accepted decision and requirement evidence**'
assert_contains DOC-SEMANTIC-ADR-CORPUS-WORKING docs/adr/0007-optional-local-semantic-feature-index.md '**Working context**'
assert_contains DOC-SEMANTIC-ADR-TARGET docs/adr/0007-optional-local-semantic-feature-index.md '**TARGET** — admit one exact symbol, range, file, document, or passage. TARGET has no census prerequisite.'
assert_contains DOC-SEMANTIC-ADR-NEIGHBORHOOD docs/adr/0007-optional-local-semantic-feature-index.md '**NEIGHBORHOOD** — begin from explicit targets and perform only a declared, bounded structural or provenance expansion.'
assert_contains DOC-SEMANTIC-ADR-CENSUS docs/adr/0007-optional-local-semantic-feature-index.md '**CENSUS** — enumerate a declared closed repository or source scope'
assert_contains DOC-SEMANTIC-ADR-PARTIAL-NO-COMPLETE docs/adr/0007-optional-local-semantic-feature-index.md 'Partial indexes and bounded zero results never imply absence outside the admitted index, completeness of a repository/source, or completeness of an engineering domain.'
assert_contains DOC-SEMANTIC-ADR-DESCRIBE-INDEPENDENT docs/adr/0007-optional-local-semantic-feature-index.md 'Describe is independent of corpus size and acquisition mode.'
assert_contains DOC-SEMANTIC-ADR-CACHE-EXACT docs/adr/0007-optional-local-semantic-feature-index.md 'A cache hit is permitted only for exact identity equality'
assert_contains DOC-SEMANTIC-ADR-CROSS-CORPUS-AUTHORITY docs/adr/0007-optional-local-semantic-feature-index.md 'Similarity, rank, or grouping never equalizes authority'
assert_following_nonempty_line_equals DOC-SEMANTIC-ADR-DESCRIBE-OP-V1 docs/adr/0007-optional-local-semantic-feature-index.md '**Describe operation outcome v1:**' '`COMPLETE | MODEL_UNAVAILABLE | CANCELLED | TIMEOUT | RESOURCE_LIMIT | BACKEND_FAILURE | POLICY_MISMATCH`'
assert_following_nonempty_line_equals DOC-SEMANTIC-ADR-DESCRIBE-MEMBER-V1 docs/adr/0007-optional-local-semantic-feature-index.md '**Describe member outcome v1:**' '`COMPLETE | ABSTAINED | INVALID_INPUT | MODEL_UNAVAILABLE | CONTEXT_LIMIT | OUTPUT_INVALID | TIMEOUT | CANCELLED | RESOURCE_LIMIT | BACKEND_FAILURE | POLICY_MISMATCH | DUPLICATE_INPUT`'
assert_following_nonempty_line_equals DOC-SEMANTIC-ADR-EMBED-OP-V1 docs/adr/0007-optional-local-semantic-feature-index.md '**Embed operation outcome v1:**' '`COMPLETE | MODEL_UNAVAILABLE | CANCELLED | TIMEOUT | RESOURCE_LIMIT | BACKEND_FAILURE | POLICY_MISMATCH`'
assert_following_nonempty_line_equals DOC-SEMANTIC-ADR-EMBED-MEMBER-V1 docs/adr/0007-optional-local-semantic-feature-index.md '**Embed member outcome v1:**' '`COMPLETE | ABSTAINED | INVALID_INPUT | MODEL_UNAVAILABLE | CONTEXT_LIMIT | OUTPUT_INVALID | TIMEOUT | CANCELLED | RESOURCE_LIMIT | BACKEND_FAILURE | POLICY_MISMATCH | DUPLICATE_INPUT`'
assert_following_nonempty_line_equals DOC-SEMANTIC-ADR-INDEX-BUILD-OP-V1 docs/adr/0007-optional-local-semantic-feature-index.md '**Index-build operation outcome v1:**' '`COMPLETE | CANCELLED | TIMEOUT | RESOURCE_LIMIT | BACKEND_FAILURE | POLICY_MISMATCH`'
assert_following_nonempty_line_equals DOC-SEMANTIC-ADR-INDEX-BUILD-MEMBER-V1 docs/adr/0007-optional-local-semantic-feature-index.md '**Index-build member outcome v1:**' '`INCLUDED | INVALID_INPUT | EMBEDDING_UNAVAILABLE | CANCELLED | RESOURCE_LIMIT | BACKEND_FAILURE | POLICY_MISMATCH | DUPLICATE_INPUT`'
assert_following_nonempty_line_equals DOC-SEMANTIC-ADR-SEARCH-OP-V1 docs/adr/0007-optional-local-semantic-feature-index.md '**Search operation outcome v1:**' '`COMPLETE | INVALID_QUERY | INDEX_UNAVAILABLE | INDEX_MISMATCH | CANCELLED | TIMEOUT | RESOURCE_LIMIT | BACKEND_FAILURE | POLICY_MISMATCH`'
assert_following_nonempty_line_equals DOC-SEMANTIC-ADR-SEARCH-MEMBER-V1 docs/adr/0007-optional-local-semantic-feature-index.md '**Search result-member outcome v1:**' '`RETURNED | BELOW_THRESHOLD | FILTERED_BY_POLICY | DUPLICATE_MEMBER | INVALID_MEMBER`'
assert_following_nonempty_line_equals DOC-SEMANTIC-ADR-GROUP-OP-V1 docs/adr/0007-optional-local-semantic-feature-index.md '**Group operation outcome v1:**' '`COMPLETE | INDEX_UNAVAILABLE | INDEX_MISMATCH | CANCELLED | TIMEOUT | RESOURCE_LIMIT | BACKEND_FAILURE | POLICY_MISMATCH`'
assert_following_nonempty_line_equals DOC-SEMANTIC-ADR-GROUP-MEMBER-V1 docs/adr/0007-optional-local-semantic-feature-index.md '**Group member outcome v1:**' '`GROUPED | UNMATCHED | FILTERED_BY_POLICY | DUPLICATE_MEMBER | INVALID_MEMBER`'
assert_exactly_one_line DOC-SEMANTIC-ADR-SEARCH-IFF docs/adr/0007-optional-local-semantic-feature-index.md '**Search is `COMPLETE` if and only if one closed query record exists, the `search_index_members` denominator equation balances, and every index member has exactly one terminal search-member outcome.**'
assert_exactly_one_line DOC-SEMANTIC-ADR-GROUP-IFF docs/adr/0007-optional-local-semantic-feature-index.md '**Group is `COMPLETE` if and only if the `group_index_members` denominator equation balances and every index member has exactly one terminal group-member outcome, including `UNMATCHED`.**'
assert_contains DOC-SEMANTIC-ADR-NONCOMPLETE-ACCOUNTING docs/adr/0007-optional-local-semantic-feature-index.md 'A non-`COMPLETE` Search or Group retains the complete index denominator, evaluated-member count, and exactly one terminal member outcome for each member whose evaluation began; it cannot imply complete evaluation, complete coverage, or absence.'
assert_contains DOC-SEMANTIC-ADR-CONTEXT-DELTA docs/adr/0007-optional-local-semantic-feature-index.md 'the core engineering-context protocol uses neutral `context_state_delta`.'
assert_contains DOC-SEMANTIC-ADR-NO-SHIPMENT docs/adr/0007-optional-local-semantic-feature-index.md 'It does not authorize shipment, a public CLI or MCP surface, registry changes, or integration into core binaries.'
assert_contains DOC-SEMANTIC-ADR-NO-OP-REGISTRATION docs/adr/0007-optional-local-semantic-feature-index.md 'This ADR does not register or renumber operations 33–35'
assert_contains DOC-SEMANTIC-ADR-YZMA-NOT-INTEGRATED docs/adr/0007-optional-local-semantic-feature-index.md 'Yzma is not integrated into `lsp-trace`, its CLI, or `lsp-trace-mcp`.'
assert_contains DOC-SEMANTIC-ADR-WHOLE-OPERATION-ALGEBRA docs/adr/0007-optional-local-semantic-feature-index.md 'An Index-build whole-operation outcome is `COMPLETE` if and only if `index_build_admitted` balances'
assert_contains DOC-ADR0007-BUNDLE-LINK docs/adr/0007-optional-local-semantic-feature-index.md '[ADR 0007 prerequisite bundle scaffold](../pilot/adr0007/README.md)'
assert_contains DOC-ADR0007-BUNDLE-DRAFT docs/pilot/adr0007/README.md '**Bundle status:** `PREREQUISITES_DRAFT`'
assert_contains DOC-ADR0007-PILOT-DISABLED docs/pilot/adr0007/README.md '**Pilot status:** `PILOT_DISABLED`'
assert_contains DOC-ADR0007-SCHEMA-BOUNDARY docs/pilot/adr0007/README.md 'docs/pilot/adr0007/schemas/'
assert_contains DOC-ADR0007-APPROVED-BOUNDARY docs/pilot/adr0007/README.md 'qualification/adr0007/approved/'
assert_contains DOC-ADR0007-GATES docs/pilot/adr0007/prerequisite-gates.md 'All gates are conjunctive and fail closed.'
assert_contains DOC-ADR0007-ARTIFACTS docs/pilot/adr0007/artifact-inventory.md 'makes no secure-erasure claim'
assert_contains DOC-ADR0007-OWNERS-UNASSIGNED docs/pilot/adr0007/ownership-and-approvals.md '`UNASSIGNED`'
assert_not_contains DOC-ADR0007-NO-OWNER-ASSIGNED docs/pilot/adr0007/ownership-and-approvals.md 'OWNER_ASSIGNED'
assert_contains DOC-ADR0007-EVALUATION-FREEZES docs/pilot/adr0007/evaluation-plan.md '**F8 Report:**'
assert_contains DOC-ADR0007-PROTOCOL-AUTHORITY docs/pilot/adr0007/protocol-outline.md '`authority=0` and `accepted=false`'
assert_contains DOC-ADR0007-THREATS docs/pilot/adr0007/threat-model.md 'Process separation limits blast radius; it does not confer authority'
assert_contains DOC-FILTER-ADR docs/adr/0002-deterministic-seed-evidence-filtering.md '# ADR 0002: Add deterministic pairwise seed-evidence comparison'
assert_contains DOC-FILTER-README README.md '## Compare retained seed evidence'
assert_contains DOC-FILTER-SEMANTICS docs/SEMANTICS.md '## Pairwise seed-evidence filter operational contract'
assert_contains DOC-FILTER-COMMAND README.md 'lsp-trace filter evidence-inspection.json --compare-seeds LEFT_LABEL --compare-seeds RIGHT_LABEL --json'
assert_contains DOC-FILTER-TYPED docs/SEMANTICS.md 'ReferenceKey = (namespace, value)'
assert_contains DOC-INPUT README.md 'lsp-trace inspect SELECTOR_OR_ARTIFACT --seed LABEL --json'
assert_contains DOC-ALL-INPUT README.md 'lsp-trace inspect SELECTOR_OR_ARTIFACT --all-seeds --json'
assert_contains DOC-PRECEDENCE docs/SEMANTICS.md 'custody verification precedes structural validation, structural validation precedes semantic validation, and inspection follows successful admission'
assert_contains DOC-AUTHORITY docs/SEMANTICS.md '`NON_AUTHORITATIVE_DERIVED_VIEW`'
assert_contains DOC-RELEASE scripts/release-check.sh './scripts/check-docs.sh'
assert_contains DOC-PROFILE-README README.md '## Named server launch profiles'
assert_contains DOC-PROFILE-DISAMBIGUATION README.md '**MCP tool-advertisement profile** means only the set returned by `tools/list`'
assert_contains DOC-PROFILE-INDEPENDENCE cmd/lsp-trace/references/transport-routing.md 'It is unrelated to CLI `--profile NAME`.'
assert_contains DOC-PROFILE-SEMANTICS docs/SEMANTICS.md '## Named server profile resolution'
assert_contains DOC-PROFILE-SKILL cmd/lsp-trace/references/live-tracing.md '## Profiles and coordinates'
assert_contains DOC-PROFILE-SECRET README.md 'Graph invocation output records environment names/references, never values.'
assert_contains DOC-MCP-README README.md '## MCP offline evidence server'
assert_contains DOC-MCP-SKILL cmd/lsp-trace/references/transport-routing.md '## Choose CLI or MCP'
assert_contains DOC-MCP-STDIO README.md 'lsp-trace-mcp --publication-root'
assert_contains DOC-MCP-CAPABILITIES README.md 'lsp_trace_v1_capabilities'
assert_contains DOC-COORDINATES-CLI README.md 'CLI `--at PATH:LINE:COLUMN` positions are one-based.'
assert_contains DOC-COORDINATES-MCP README.md 'MCP `line` and `character` inputs, LSP requests and responses, and retained graph ranges are zero-based.'
assert_contains DOC-COORDINATES-ENCODING README.md '`utf-8` counts bytes, `utf-16` counts UTF-16 code units, and `utf-32` counts Unicode code points.'
assert_contains DOC-COORDINATES-OWNERSHIP cmd/lsp-trace/references/live-tracing.md 'The server owns the encoding; omission defaults to `utf-16`.'
assert_contains DOC-MCP-ALWAYS-LOCAL README.md 'local-development-only'
assert_contains DOC-MCP-COMPACT README.md 'prefer `lsp-trace-mcp --tool-profile compact`'
assert_contains DOC-MCP-FULL-COMPAT README.md 'compatibility `--tool-profile full` publishes exactly 41 canonical tools'
assert_contains DOC-MCP-MACHINE-LOCAL README.md 'This file is machine-local and ignored by this repository.'
assert_contains DOC-MCP-COMPACT-ADDITIVE README.md 'add only `"--tool-profile", "compact"` to the server'
assert_contains DOC-MCP-ROOT-IGNORED .gitignore '/.mcp.json'
assert_contains DOC-MCP-PROVIDERS README.md 'Host-provisioned relation providers'
assert_contains DOC-MCP-PROVIDER-CONTRACT docs/PROVIDERS.md 'lsp-trace.provider-collector-request.v1'
assert_contains DOC-MCP-PROVIDER-DECLARATION-SCHEMA schema/schemas/lsp-trace.bootstrap-provider.v1.schema.json 'lsp-trace.bootstrap-provider.v1'
assert_contains DOC-MCP-PROVIDER-REQUEST-SCHEMA schema/schemas/lsp-trace.provider-collector-request.v1.schema.json 'lsp-trace.provider-collector-request.v1'
assert_contains DOC-MCP-PROVIDER-OBSERVATION-SCHEMA schema/schemas/lsp-trace.provider-observations.v1.schema.json 'lsp-trace.provider-observations'
assert_contains DOC-MCP-INCOMING cmd/lsp-trace/SKILL.md '`incoming`: start from exact callee positions and trace callers upward.'
assert_contains DOC-MCP-SLICE-ENABLED cmd/lsp-trace/SKILL.md '`slice`: discover bounded outgoing nodes'
assert_contains DOC-MCP-WARNING README.md "developer's permissions"
assert_contains DOC-MCP-NO-SANDBOX README.md 'not sandboxed'
assert_contains DOC-MCP-LOCAL-ACCESS README.md 'local files and network'
assert_contains DOC-MCP-TRUST README.md 'must be trusted'
assert_contains DOC-MCP-ADR docs/adr/0003-always-local-stage2.md '# ADR 0003: Activate always-local Stage 2 lifecycle tools'
assert_contains DOC-MCP-HISTORICAL-ADR docs/adr/0003-persistent-mcp-language-server-sessions.md '**Status:** Superseded by [ADR 0003: Activate always-local Stage 2 lifecycle tools](0003-always-local-stage2.md)'
assert_contains DOC-PI-STANDARD-ADAPTER README.md 'pi install npm:pi-mcp-adapter'
assert_contains DOC-PI-PROJECT-CONFIG README.md 'Preferred project config: `.mcp.json`'
assert_contains DOC-PI-DIRECT-TOOLS README.md '"directTools": ['
assert_contains DOC-PI-COMPACT-ELEVEN README.md 'The recommended compact tool-advertisement profile advertises exactly eleven canonical MCP names'
assert_contains DOC-PI-EXECUTE README.md '"lsp_trace_v1_execute"'
assert_contains DOC-PI-SELF-CHECK README.md '/mcp reconnect lsp-trace'
assert_contains DOC-PI-NO-CUSTOM-EXTENSION README.md 'Do not add a repository-local Pi extension or a second MCP bridge.'
assert_contains DOC-PI-HOST-AUTHORITY README.md 'Only the host-authored `.mcp.json` command, arguments, and bootstrap file choose executable, environment, or working directory.'
assert_contains DOC-PI-CANCELLATION README.md 'Cancellation stops the caller observation; an already accepted lifecycle intent may continue.'
assert_contains DOC-PI-BOUNDED README.md 'Partial or truncated traversal results remain honest bounded outcomes.'
assert_contains DOC-PI-SEARCH-KEYWORDS README.md '"searchKeywords": {'
assert_contains DOC-PI-SEARCH-INTENTS README.md '"who calls this callee"'
assert_contains DOC-MCP-TRAVERSAL-COMPACT README.md '"detail":"compact","output_selector":"traces/callers.json"'
assert_contains DOC-MCP-LIFECYCLE-SUCCESS README.md 'Successful lifecycle guidance is categorical and comes only from the returned `result`'
assert_contains DOC-SKILL-TRAVERSAL-COMPACT cmd/lsp-trace/references/offline-evidence.md 'Compact traversal requires both `detail: "compact"` and `output_selector`;'
assert_contains DOC-ADR-THIRTEEN docs/adr/0003-always-local-stage2.md 'Unsupported platforms keep the same thirteen-tool discovery contract.'
assert_contains DOC-REPRESENTATIVE-MATRIX qualification/representative-preflight/matrix.v1.json '"source_state": "INTEGRATED"'
assert_contains DOC-REPRESENTATIVE-INSTALLED-EVIDENCE qualification/representative-preflight/installed-state.7a6a.v1.json '"custody": "OPERATOR_ASSERTED"'
assert_contains DOC-REPRESENTATIVE-SOURCE-INSTALLED qualification/FINAL-REPLAY-QUALIFICATION.md 'The report distinguishes repository source state from installed production state.'
assert_contains DOC-REPRESENTATIVE-VERSION qualification/FINAL-REPLAY-QUALIFICATION.md 'Executable presence is reported separately, and its version remains `VERSION_UNVERIFIED`'
assert_contains DOC-REPRESENTATIVE-SESSION qualification/FINAL-REPLAY-QUALIFICATION.md 'Environment flags are only `OPERATOR_ASSERTED` prerequisites'
assert_contains DOC-REPRESENTATIVE-EMBER qualification/FINAL-REPLAY-QUALIFICATION.md 'package version `1.0.3` is distinct from semantic protocol identity `ember-glint@1`'
assert_contains DOC-REPRESENTATIVE-SAFE-OUTPUT qualification/FINAL-REPLAY-QUALIFICATION.md 'publishes atomically without replacement at mode `0600`'

if [ "$failed" -ne 0 ]; then
  exit 1
fi
printf 'DOCUMENTATION CHECK PASS\n'
