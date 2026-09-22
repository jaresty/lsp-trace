## Protocols

- No further tools, research, execution, mutation, or delegation performed.
- Prior plans remain rejected and non-authorizing; this response uses only retained evidence.

## Artifact Path

`/Users/schwa/dev/lsp-trace/.pi/evidence/adr0010-mechanical-plan-20260922T000000Z/MECHANICAL_EXECUTION_PLAN.md`

This response provides plan content only. The artifact has not been written, hashed, or accepted.

## Acceptance Boundary

The plan becomes executable only after the parent:

1. Persists this plan at the exact path above and records its SHA-256.
2. Binds it to an exact Git commit and clean worktree.
3. Confirms:
   - `internal/featureattribution/contracts.go`:
     `sha256:4a5831b04b35070226a627140b5dc0c50a20ab4b25f2cb25a343caa290924758`, 13,594 bytes.
   - `internal/featureattribution/contracts_test.go`:
     `sha256:072dd3788f0b7a7ede3dc006d989ea4a62ed206bb8462c8db6d5e685cd0b6bce`, 4,449 bytes.
4. Attaches every immutable decision receipt required by the unit being authorized.
5. Assigns one integration owner for shared files and final verification.

Acceptance may bind revision, receipts, owners, and order. It may not add contract semantics. DTO fields, schema identities, membership semantics, failure codes, envelope shapes, CLI flags, and output shapes must already be fixed by the named receipts.

Global invariants:

- Exact-only `GRAPH_SUBJECT`, fully contained `SOURCE_RANGE`, and exact `RELATION_OCCURRENCE` endpoint mapping.
- No LLM, embeddings, names, descriptions, metadata, ranking, graph proximity, CALLS traversal, or first-match selection.
- Live locators freeze into immutable receipts before offline attribution.
- Ambiguous locator resolution selects no anchor.
- Authority remains `0`; accepted remains `false`; completeness remains `UNKNOWN`.
- Offline attribution has no ambient workspace, session, LSP, process, network, or “latest” fallback.
- ADR 0007 artifacts remain immutable.

## Executable-Now Units

### E0 — Persist and Hash the Plan

**Owner:** Parent only.

**Write set:**

- `.pi/evidence/adr0010-mechanical-plan-20260922T000000Z/MECHANICAL_EXECUTION_PLAN.md`
- `.pi/evidence/adr0010-mechanical-plan-20260922T000000Z/MECHANICAL_EXECUTION_PLAN.sha256`

**Transformation:**

1. Write the accepted plan byte-for-byte using LF line endings and one terminal newline.
2. Compute SHA-256 over the Markdown bytes.
3. Write:

```text
<64 lowercase hex>  MECHANICAL_EXECUTION_PLAN.md
```

to the `.sha256` file.

**Gate:**

```bash
cd /Users/schwa/dev/lsp-trace/.pi/evidence/adr0010-mechanical-plan-20260922T000000Z
shasum -a 256 -c MECHANICAL_EXECUTION_PLAN.sha256
```

Required output:

```text
MECHANICAL_EXECUTION_PLAN.md: OK
```

### E1 — Freeze the Source Revision

**Dependency:** Accepted E0 artifact.

**Write set:** One `SOURCE_REVISION_RECEIPT` under the same evidence directory; no source files.

**Required observations:**

```bash
cd /Users/schwa/dev/lsp-trace
git diff --quiet
git diff --cached --quiet
git rev-parse --verify HEAD
shasum -a 256 internal/featureattribution/contracts.go
wc -c < internal/featureattribution/contracts.go
shasum -a 256 internal/featureattribution/contracts_test.go
wc -c < internal/featureattribution/contracts_test.go
```

**Gate:**

- Clean worktree.
- Forty-character commit identity.
- Both retained digests and byte lengths match exactly.
- Receipt binds the plan digest, commit, paths, digests, and lengths.

A mismatch blocks every source-writing unit and requires re-planning.

No other implementation unit is currently choice-free.

## Blocked Units and Missing Receipts

### B1 — Foundation Contracts and Schemas

**Blocked on:** `FOUNDATION_CONTRACT_RECEIPT`

The receipt must provide accepted, content-addressed file bytes or an exact patch fixing:

- all Go DTO names and complete field definitions;
- JSON keys and required/optional status;
- closed-union discriminators and payload shapes;
- complete enum values and numeric bounds;
- canonical ordering rules;
- identity domain separators;
- validation order;
- exact schema IDs and repository paths;
- exact Go/schema parity tests.

**Intended write set:**

- `internal/featureattribution/contracts.go`
- `internal/featureattribution/contracts_test.go`
- exact schema paths named by the receipt under:
  - `internal/schema/schemas/`
  - `internal/mcpcontract/testdata/schemas/`

**Gate:**

```bash
go test ./internal/featureattribution -count=1
go test ./internal/mcpcontract -run '^TestFeatureAttribution' -count=1
```

`Attribute` must remain explicitly `NOT_READY`.

### B2 — Immutable Artifact Admission

**Blocked on:** `ARTIFACT_ADMISSION_CONTRACT_RECEIPT`

The receipt must fix:

- loader interface and method signatures;
- admission function signatures;
- exact typed failure codes;
- diagnostic policy;
- byte limits;
- validation order;
- exact file bytes or patch.

Required validation order:

1. selector syntax;
2. selected-byte load;
3. unavailable check;
4. byte bound;
5. byte-length equality;
6. SHA-256 equality;
7. strict decode;
8. schema identity;
9. semantic validation;
10. immutable copied output.

**Write set:**

- `internal/featureattribution/artifacts.go`
- `internal/featureattribution/artifacts_test.go`

**Gate:**

```bash
go test ./internal/featureattribution -run '^TestArtifactAdmission' -count=1
```

### B3 — ADR 0007 Inventory and Membership Admission

**Blocked on:** `ADR0007_ATTRIBUTION_INPUT_CONTRACT_RECEIPT`

The receipt must bind exact selectors, schema IDs, digests, byte lengths, and field paths for:

- inventory candidate IDs and correction lineage;
- candidate-to-constituent membership records;
- constituent IDs and kinds;
- graph-subject IDs;
- logical URIs;
- revision/custody identities;
- position encodings and display ranges;
- relation-occurrence IDs and endpoint identities;
- membership record IDs and canonical ordering;
- graph/source/relation custody artifacts;
- executable compatibility predicate.

It must retain a passing compatibility result for those exact artifacts.

No placeholder or provisional candidate identity satisfies this gate.

### B4 — Exact Offline Mapper

**Blocked on:**

- accepted B1;
- accepted B2;
- `ADR0007_ATTRIBUTION_INPUT_CONTRACT_RECEIPT`;
- `MAPPER_PATCH_RECEIPT`.

`MAPPER_PATCH_RECEIPT` must provide exact mapper/test bytes and the exact permitted `Attribute` patch.

**Write set:**

- `internal/featureattribution/mapper.go`
- `internal/featureattribution/mapper_test.go`
- receipt-identified `Attribute` hunk in `contracts.go`

**Gate:**

```bash
ADR0010_INVENTORY_FIXTURE='<receipt-bound path>' \
ADR0010_MEMBERSHIP_FIXTURE='<receipt-bound path>' \
ADR0010_CUSTODY_FIXTURE='<receipt-bound path>' \
go test ./internal/featureattribution -run '^TestMapper' -count=1
```

Required assertions include exact equality, range containment, custody mismatch, independent relation endpoints, one-to-many mapping, witness deduplication, limits, accounting, ceilings, and byte-identical replay.

### B5 — Locator Resolver

**Blocked on:**

- accepted B1;
- `RESOLVER_PATCH_RECEIPT`.

The receipt must provide exact resolver/test bytes and exact candidate-accounting fields.

Fixed disposition rule:

- zero candidates → `UNRESOLVED`, no anchor;
- one candidate → `RESOLVED`, exact anchor;
- two or more → `AMBIGUOUS`, no anchor.

**Write set:**

- `internal/featureattribution/resolver.go`
- `internal/featureattribution/resolver_test.go`

**Gate:**

```bash
go test ./internal/featureattribution -run '^TestResolver' -count=1
```

Provider-order permutations must produce identical canonical receipts.

### B6 — MCP Operations

**Blocked on:**

- accepted B4 and B5;
- `MCP_SURFACE_CONTRACT_RECEIPT`.

The receipt must fix:

- schema and envelope IDs;
- envelope ownership;
- executor families;
- operation descriptions;
- manifest count delta;
- profile visibility;
- availability state;
- direct/gateway output shapes;
- exact patches.

Canonical operation names:

- `lsp_trace_v1_resolve_feature_locators`
- `lsp_trace_v1_feature_map`
- `lsp_trace_v1_resolve_and_feature_map`

**Write set:** exact receipt-declared changes limited to:

- `internal/mcpcontract/contract.go`
- `internal/mcpcontract/testdata/stage1-manifest.v1.json`
- `internal/mcpcontract/feature_attribution.go`
- `internal/mcp/registry.go`
- `internal/operation/types.go`
- `cmd/lsp-trace-mcp/main.go`
- declared schemas/tests

**Gate:**

```bash
go test ./internal/mcpcontract -run '^TestFeatureAttribution' -count=1
go test ./internal/mcp -run '^TestFeatureAttribution' -count=1
go test ./cmd/lsp-trace-mcp -run '^TestFeatureAttribution' -count=1
```

The mapper must remain offline-only. Operations remain hidden/reserved until qualification.

### B7 — CLI Commands

**Blocked on:**

- accepted B4 and B5;
- `CLI_SURFACE_CONTRACT_RECEIPT`.

The receipt must fix every flag, default, required status, exit code, help/error text, stdout shape, and stderr policy for:

- `feature-map`
- `feature-map-resolve`

**Write set:**

- exact receipt-declared hunk in `cmd/lsp-trace/main.go`
- `cmd/lsp-trace/feature_map_command.go`
- `cmd/lsp-trace/feature_map_command_test.go`

**Gate:**

```bash
go test ./cmd/lsp-trace -run '^TestFeatureMap' -count=1
```

Raw locators must be rejected by `feature-map`; ambiguous resolution must not invoke attribution.

### B8 — Qualification and Public Enablement

**Blocked on:**

- accepted B4–B7;
- `QUALIFICATION_ENABLEMENT_RECEIPT`.

The receipt must retain passing evidence for all ADR 0010 qualification cases, including UTF-8/UTF-16 ranges, relation endpoints, shared mappings, zero results, mixed outcomes, identity substitution, bounds, privacy, replay, and unchanged ADR 0007 bytes.

**Gate:**

```bash
ADR0010_INVENTORY_FIXTURE='<receipt-bound path>' \
ADR0010_MEMBERSHIP_FIXTURE='<receipt-bound path>' \
ADR0010_CUSTODY_FIXTURE='<receipt-bound path>' \
go test ./internal/featureattribution ./internal/mcpcontract ./internal/mcp ./cmd/lsp-trace ./cmd/lsp-trace-mcp \
  -run 'FeatureAttribution|FeatureMap|Resolver|Mapper|ArtifactAdmission' \
  -count=1
```

Only a passing qualification receipt may authorize advertisement or ADR status changes.

## Exact Stop Rules

Stop immediately if:

- any digest, byte length, commit, or patch preimage differs;
- a required receipt is absent, invalid, mutable, or incomplete;
- a worker must choose a field, schema, enum, bound, failure code, envelope, flag, or output shape;
- an unexpected build/test failure occurs;
- a test cannot produce assertion-specific failure and pass results;
- a write would leave the unit’s declared set;
- write sets overlap without the named integration owner;
- any proposed change weakens strict parsing, identity binding, accounting, authority ceilings, ambiguity handling, or Gate A;
- mapping would depend on anything other than exact admitted identities;
- historical ADR 0007 bytes would change.

The executor must return only:

```text
ADR0010_MECHANICAL_STOP
unit: <unit ID>
reason_code: <stable code>
expected: <receipt value or invariant>
observed: <exact observed value>
write_set_touched: <none or exact paths>
last_passing_gate: <gate ID or none>
```

No repair, redesign, or continuation is authorized.
