## Artifact path

`/Users/schwa/dev/lsp-trace/.pi/evidence/adr0011-mechanical-plan-20260922T000000Z/MECHANICAL_EXECUTION_PLAN.md`

Read-only result: this path was not created. The parent must persist the content, compute its SHA-256 and byte length, and record them in the acceptance receipt.

## Acceptance boundary

This plan is not executable until the parent records an immutable:

### `ADR0011MechanicalPlanAcceptanceReceipt`

Required bindings:

- exact artifact path, SHA-256, and byte length;
- Git HEAD and content-addressed dirty-worktree patch;
- canonical worktree URI;
- accepted ADR 0011 implementation authorization;
- executor identity;
- exclusive write ownership;
- acceptance decision and timestamp.

For each assigned unit, also record an immutable `ADR0011UnitBaselineReceipt` containing:

- plan SHA-256;
- unit ID;
- Git HEAD and dirty-patch SHA-256;
- every write-set path;
- each path’s pre-state: absent, or exact SHA-256 and byte length;
- exclusive owner.

Acceptance may bind revisions, receipts, fingerprints, and owners. It may not add semantic content. A changed baseline requires parent re-acceptance, not executor interpretation.

Retained exact-session evidence fixes only this implementation seam:

`censusprogramc.Compose → programccompose.Compose → programcadmission.Admit → programc.ComputeComposite → programcrepresentative.Select`

`programc.computeProjection` is the generic PairWeight/Leiden seam. Existing composition, admission, representative selection, claim ceilings, and failures remain CALLS-specific.

## Executable-now units

No unit is executable in the current state because ADR 0011 implementation authorization and accepted baseline receipts are absent.

After those receipts exist, only the following units are mechanically specified by this plan.

### M1 — Closed private evidence vocabulary

**Write set**

- Create `internal/definitionreference/types.go`
- Create `internal/definitionreference/validate.go`
- Create `internal/definitionreference/validate_test.go`

**Exact transformation**

Define only:

- relations:
  - `REFERENCES_SYMBOL`
  - `RESOLVES_TO_DEFINITION`
- custody roles:
  - `QUERY_OCCURRENCE`
  - `REFERENCING_OCCURRENCE`
  - `REFERENCING_SYMBOL`
  - `REFERENCED_SYMBOL`
  - `DEFINITION_TARGET`
- request outcomes, in precedence order:
  1. `REVISION_MISMATCH`
  2. `POLICY_MISMATCH`
  3. `UNSUPPORTED`
  4. `CANCELLED`
  5. `TIMEOUT`
  6. `RESOURCE_LIMIT`
  7. `PROVIDER_FAILURE`
  8. `MALFORMED`
  9. `PARTIAL`
  10. `COMPLETE_EMPTY`
  11. `COMPLETE`
- member outcomes:
  - `ADMITTED`
  - `EMPTY`
  - `MALFORMED`
  - `EXCLUDED`
  - `LIMITED`
  - `FAILED`
- invariant values:
  - `authority=0`
  - `accepted=false`
  - `completeness="UNKNOWN"`

Validation order is fixed:

1. family/version;
2. required fields;
3. enum membership;
4. invariant constants;
5. digest syntax;
6. numeric bounds;
7. range validity;
8. relation/custody compatibility;
9. canonical ordering;
10. accounting equations.

`CALLS` is rejected as a relation kind. No existing Program C package may be imported.

**Exact command**

```sh
go test ./internal/definitionreference -run 'TestM1'
```

**Required assertions**

- `ASSERT_M1_RELATION_ENUM_EXACT`
- `ASSERT_M1_CALLS_RELATION_REJECTED`
- `ASSERT_M1_CUSTODY_ENUM_EXACT`
- `ASSERT_M1_REQUEST_OUTCOME_PRECEDENCE`
- `ASSERT_M1_COMPLETE_EMPTY_DISTINCT`
- `ASSERT_M1_CONSERVATIVE_CONSTANTS`
- `ASSERT_M1_VALIDATION_ORDER`

**Completion gate**

- Each assertion has an assertion-specific RED result and then passes.
- Only the three write-set files changed.
- No existing schema, Program C, CLI, MCP, or census file changed.

### M2 — Receipt-bound target identity and occurrence multiplicity

**Dependencies**

- Accepted M1 completion receipt.
- Exact M2 baseline receipt.

**Write set**

- Create `internal/definitionreference/identity.go`
- Create `internal/definitionreference/ledger.go`
- Create `internal/definitionreference/ledger_test.go`

**Exact domain separators**

```text
lsp-trace:definition-reference:query-target-receipt:v1
lsp-trace:definition-reference:method-receipt:v1
lsp-trace:definition-reference:occurrence:v1
lsp-trace:definition-reference:occurrence-ledger:v1
```

Identity formula:

```text
sha256(domain || 0x00 || canonical_json_without_identity || 0x0a)
```

For `REFERENCES_SYMBOL`:

1. Require a pre-existing referenced-symbol identity.
2. Require a query-target receipt binding the exact query occurrence, URI/range, encoding, document version/digest, session/generation, revision custody, method receipt, provider, and adapter.
3. Copy the target identity only from that receipt.
4. Never derive or replace it from references output.
5. Missing, ambiguous, unverifiable, or mismatched receipt returns `TARGET_IDENTITY_UNRESOLVED`.
6. The failed member contributes zero occurrences.

For `RESOLVES_TO_DEFINITION`:

- each returned definition target produces one occurrence;
- no preferred target is selected.

Repeated responses with equal endpoints remain distinct occurrences. Canonical sorting assigns ordinals but never deduplicates occurrences.

**Exact command**

```sh
go test ./internal/definitionreference -run 'TestM2'
```

**Required assertions**

- `ASSERT_M2_REFERENCE_TARGET_PREESTABLISHED`
- `ASSERT_M2_QUERY_RECEIPT_EXACT_MATCH`
- `ASSERT_M2_TARGET_IDENTITY_UNRESOLVED_ZERO_OCCURRENCES`
- `ASSERT_M2_MULTI_DEFINITION_ONE_OCCURRENCE_PER_TARGET`
- `ASSERT_M2_EQUAL_ENDPOINT_OCCURRENCES_PRESERVED`
- `ASSERT_M2_CANONICAL_ORDER_AND_ORDINAL`
- `ASSERT_M2_DOMAIN_SEPARATED_IDENTITIES`
- `ASSERT_M2_LEDGER_ACCOUNTING_BALANCED`

**Completion gate**

- All assertions have valid RED/GREEN observations.
- Input permutation produces identical canonical ledger bytes.
- Only the three M2 files changed.

## Blocked units and exact missing receipts

No executor may be assigned to these units.

### C3 — Bounded acquisition

Missing: `ADR0011AcquisitionContractReceipt`

It must fix every request field/type/tag, all numeric bounds, method-policy vocabulary, provider/adapter selection, LSP union normalization, accounting equations, failure precedence, revision-custody vocabulary, canonical identities, and fixture identities.

### C4 — Schemas

Missing: `ADR0011SchemaContractReceipt`

It must supply exact filenames, `$id` values, schema draft, complete schema bytes or deterministic generator input, required fields, closed unions, bounds, Go/schema mirror policy, registration scope, and expected schema digests.

### C5 — Grouping input

Missing: `ADR0011ReferenceGroupingInputContractReceipt`

It must fix exact types, fields, tags, receipt selectors, ledger selector, custody references, accounting equations, canonical order, domain separators, validation order, and failures.

### C6 — Admission

Missing: `ADR0011ReferenceAdmissionContractReceipt`

It must fix the opaque admission API, source binding, defensive-copy boundaries, validation order, caps, failure vocabulary, and exact relation to C5 bytes. It must prohibit edits to existing `programcadmission` and `programccompose` contracts.

### C7 — Policy and qualification

Missing: `ADR0011GroupingPolicyContractReceipt`

It must fix policy and qualification schemas, action semantics, corpus manifests, reviewer requirements, acceptance criteria representation, denominator equations, exact decision derivation, receipt matching, and test-fixture rules.

### C8 — PairWeight projection

Missing: `ADR0011PairWeightProjectionContractReceipt`

It must select one direction/projection architecture and fix action order, numeric representation, accumulation order, exceptional-number behavior, contribution records, composition identity, and zero-projection behavior when unqualified.

### C9 — Leiden-kernel architecture

Missing: `ADR0011ProgramCKernelArchitectureReceipt`

It must select exactly one architecture and prescribe destination package, API, edit hunks or AST transformation, iteration order, byte-pin fixtures, allowed shared-file writes, and compatibility commands. Executors may not choose between an adapter and shared package.

### C10 — Reference Program C result

Missing:

- accepted ADR 0010 integration receipt;
- accepted C5–C9 receipts;
- `ADR0011ReferenceProgramCResultContractReceipt`.

The latter must fix the result fields, private profile, claim ceiling, identity domain and field order, environment bindings, terminal outcomes, and fixture digests.

### C11 — Artifacts and replay

Missing: `ADR0011ReplayArtifactContractReceipt`

It must fix snapshot, packet, checkpoint, replay request/result, typed failures, predecessor graph, canonical bytes, environment matching, and the mechanism proving zero live LSP/workspace access.

### C12 — Public activation

Missing: `ADR0011PublicActivationContractReceipt`

It must bind:

- accepted ADR 0010 integration;
- qualified ADR 0007 inventory receipt;
- exact immutable matching `QUALIFIED` grouping-policy receipt;
- accepted C3–C11 receipts;
- frozen public schema, CLI, and MCP names;
- exact capability tuple;
- Go/CUE qualification results;
- adversarial-corpus results;
- maintainer authorization.

C12 has no executable write set. A successor Craft-derived plan must define it after the receipt exists.

## Exact gates

1. **Baseline gate:** plan and write-set fingerprints match.
2. **Authorization gate:** ADR 0011 implementation authorization exists.
3. **Ownership gate:** one exclusive writer per write set.
4. **RED gate:** named assertion executes and fails for the intended present-but-wrong fixture; compile failure does not qualify.
5. **GREEN gate:** the same assertion passes after the prescribed transformation.
6. **Compatibility gate:** no existing CALLS file or canonical artifact changes in M1–M2.
7. **Dependency gate:** every predecessor completion or decision receipt exists.
8. **ADR-order gate:** C10 cannot begin before accepted ADR 0010 integration.
9. **Policy gate:** no PairWeight projection or Leiden invocation without an immutable exact-match `QUALIFIED` receipt.
10. **Activation gate:** C12 remains blocked until every named receipt matches.

## Stop rules

Stop and return to the parent if:

- any receipt is missing, mutable, stale, or mismatched;
- a file in the write set differs from its baseline;
- another writer owns or changes the same path;
- implementation requires a field, enum, bound, identity rule, architecture, or failure not fixed here;
- a RED test does not reach its named assertion;
- any file outside the unit write set changes;
- references are coerced into CALLS;
- target identity would be inferred from provider output;
- existing CALLS bytes or omission semantics change;
- representative or outward-consumer logic consumes references;
- replay requires live session/workspace access;
- public exposure or policy activation is proposed before C12 is unblocked.

No executor may diagnose, redesign, weaken a gate, expand a write set, or choose among alternatives. Such a need invalidates mechanical execution and requires a new decision receipt and successor plan.
