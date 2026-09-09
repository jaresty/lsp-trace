# Final local analytics repair

## Scope and base

The repair was authored from exact commit `90c8261118f5fe2ee30dee0dd8c52c639de75957`. It remains package-local and synthetic: no Program B admission is required by `EvaluateLocal`, no local descriptor has a protocol name, and no CLI, MCP, schema-family, or public registry entry was added. Historical normative v2 constants, exported APIs, schema alias, descriptors, and Program B admission behavior remain unchanged.

## B1 — closed operation-specific result validation

`ValidateLocalResultJSON` now rejects noncanonical JSON, duplicate/unknown/trailing members, invalid operation/status, malformed SHA-256 values, noncanonical relation filters, inconsistent accounting, invalid COMPLETE/LIMIT payload combinations, wrong payload schema IDs, malformed findings, metric counts/density/omissions, ranking scores/order/tie-break text, and any result that cannot be deterministically replayed from exactly one retained byte slice. Recomputing the unkeyed digest does not bypass semantic validation.

COMPLETE has exactly one payload matching `Operation`; LIMIT has no payload and must report `Units > Limit`. `Units` equals the exact sum of decoder, node, edge, selection, and kernel components.

Four package-owned JSON Schema artifacts document the retained input and three operation-specific outputs. They are embedded by the repository's existing wildcard but intentionally absent from schema-family and MCP/public registration maps.

## B2 — canonical retained relations

The closed vocabulary is exactly:

- `CALLS`
- `BINDS_ARGUMENT`
- `PASSES_CALLBACK`
- `INVOKES_TASK`
- `TRIGGERS_RELOAD`
- `UPDATES_STATE`
- `RENDERS_FROM`

The invented `SUPPORTS` relation is rejected. Distinct edge records preserve multiplicity; structural relation counts alone deduplicate `(from,to,relation)`.

## B3 — independent support identity

Support contributes only when the edge records `provenance_authority=VALIDATED_AUTHORITY`, `provenance_custody=PROVIDER_VERIFIED`, and nonempty `provenance_id` and `support_group`. Metrics identify support by `(validated authority, custody, provenance ID, support group)`. Ranking adds target node to that identity. Repeated aliases for one qualified support collapse; distinct provenance remains distinct; bare labels and caller-asserted, unknown, or unvalidated provenance contribute no independent support.

This qualification affects counting only and does not authenticate local execution or create Program B authority.

## B4 — exact incremental work bounds

`MaxWork` is a positive integer. Work is charged in this order:

1. exact retained byte length before UTF-8 or JSON decoding;
2. one unit per admitted node;
3. one edge unit and one selection unit per edge;
4. one kernel unit per node and selected edge.

Each incremental charge stops immediately when total work first exceeds the limit. The byte precharge is indivisible, so a limit below input length returns LIMIT without parsing and honestly reports the full byte charge. Later phases stop at `Limit + 1`. Totals are never clamped, and the component sum always equals `Units`.

The byte ceiling is 2 MiB so the separately declared 8192-edge maximum is reachable by a valid retained document; node, edge, string, and byte exact maxima and one-over cases are covered.

## B5 — adversarial guards

Focused tests cover nested duplicate/unknown members, omitted/wrong and re-digested semantic result mutations, operation/payload/schema/status substitution, SHA-256 lexical form, inconsistent accounting, raw noncanonical node/edge order, all seven relation values plus invented/unknown rejection, multiplicity and provenance collisions, target-scoped support, ranking/tie/density semantics, exact byte/node/edge/string boundaries, MaxWork minus/exact/plus one, and semantic-builder permutation versus canonical-byte admission.

## Focused verification

The following package-local checks pass on the repaired tree:

- `go test ./internal/normativeanalytics -count=1` — 53 tests passed.
- `go test -race ./internal/normativeanalytics -count=1` — 53 tests passed.
- `go vet ./internal/normativeanalytics` — no diagnostics.
- `go build ./internal/normativeanalytics` — pass.
- `go test ./internal/schema ./internal/normativeanalytics -count=1` — 135 tests passed.
- Four local normative JSON schemas parse successfully and remain unregistered.
- `git diff --check` — pass.

No full suite, full race, network, install, deployment, product/liveD01 execution, Program B authority action, or public registry action was performed.
