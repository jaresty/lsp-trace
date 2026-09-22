# Accepted Mechanical Execution Plan: Document Preparation Failure Separation

Status: ACCEPTED FOR MECHANICAL EXECUTION

## Immutable inputs

- Luna draft: `sha256:b7ef14782dd26346d4f779a7dfcf38c9ba1c587894affc62aa675dcb387087f6`
- Bar derivation prompt V2: `BAR_CRAFT_PROMPT_V2.md`
- Persistent guard: `TestManagedPreparationExplicitLanguageIDIsNotMisclassifiedByURIAdmission`
- Guard source before execution: `sha256:83b212e3192f2e7c34fa7ebc8c14324ea7342b68959869c23be14c0725f248ad`
- RED record: `RED.txt`, containing `RED_BOUNDARY_CONFIRMED ASSERT_EXPLICIT_LANGUAGE_ID_NOT_MISCLASSIFIED`
- Live bounded observation: explicit routed language `go`; private diagnostic `LANGUAGE_ID_UNAVAILABLE attempted=10 succeeded=9 planned=14 failing_ordinal=9`.
- The live ordinal-9 subtype is `UNKNOWN` and must remain unknown until a later authorized observation distinguishes it.

## Accepted semantic decisions

### A. Runtime failure vocabulary

Add these private runtime failures:

```go
DocumentURIUnavailable    session.Failure = "DOCUMENT_URI_UNAVAILABLE"
DocumentOutsideWorkspace  session.Failure = "DOCUMENT_OUTSIDE_WORKSPACE"
DocumentSourceUnavailable session.Failure = "DOCUMENT_SOURCE_UNAVAILABLE"
```

Retain:

```go
LanguageIDUnavailable session.Failure = "LANGUAGE_ID_UNAVAILABLE"
```

Exact meanings:

- `DOCUMENT_URI_UNAVAILABLE`: URI parsing or managed-local URI admission fails because the scheme is not `file`, host/authority is forbidden, or usable path is absent.
- `DOCUMENT_OUTSIDE_WORKSPACE`: admitted local path is not lexically contained by the exact session generation's managed workspace.
- `DOCUMENT_SOURCE_UNAVAILABLE`: the URI, workspace, and effective language are valid, but the existing non-capture source read currently represented by `os.ReadFile` fails.
- `LANGUAGE_ID_UNAVAILABLE`: URI and scope are valid, but effective language is empty after existing authorized resolution.

No category carries a URI, path, source bytes, provider identity, language value, raw error, or filesystem detail.

### B. Public/private boundary

Add one-to-one internal values in `internal/liveprojection` and `internal/censuscontinuation` for the three new categories. Extend strict private diagnostic validation atomically.

Do not add or change a public CLI/MCP code, schema enum, checkpoint identity, request identity, or historical artifact. Public continuation classification remains the existing generic:

```text
failed_field=source_preparation
resource_category=availability
caller_action=RECONFIGURE_MANAGED_SOURCE_SUPPLY_THEN_RESUME
```

Historical `LANGUAGE_ID_UNAVAILABLE` records remain valid observations of their producing version and are not reinterpreted as proof of a truly absent language.

### C. Source-acquisition authority

This correction changes classification only.

- The existing `CaptureSupply=true` managed acquisition path and its `DOCUMENT_SUPPLY_UNAVAILABLE` behavior remain unchanged.
- `DOCUMENT_SOURCE_UNAVAILABLE` replaces only the current non-capture `os.ReadFile` failure that is incorrectly returned as `LANGUAGE_ID_UNAVAILABLE`.
- No read mechanism is added, removed, broadened, retried, or promoted.
- No alternate root, session, generation, profile, URI inference, extension inference, or filesystem fallback is introduced.

### D. Failure precedence

Preserve the current control-flow ordering. Do not implement Luna's proposed reorder in this correction.

The accepted order is:

1. already-cancelled/deadlined context;
2. URI parse/local-file admission;
3. exact session lookup and exact generation check;
4. lexical workspace containment;
5. effective language resolution;
6. current authorized source acquisition;
7. existing document/cache and lifecycle checks in their current locations;
8. existing notification serialization/write and supply publication.

Only the three conflated return values change. Moving URI checks behind session lookup, moving lifecycle checks before source acquisition, or changing cache/write ordering is outside this plan.

## Governing properties

1. A non-empty effective language cannot yield `LANGUAGE_ID_UNAVAILABLE` because URI admission, workspace scope, or source acquisition failed.
2. Each newly separated cause remains private and maps outward to the unchanged generic availability result.
3. Exact session/generation, source custody, cache/version behavior, and protocol sequencing remain unchanged.
4. Preparation remains a request-wide barrier: no partial returned document set, later attempt, resolver phase, worker, or model follows first failure.
5. The implementation does not identify or encode the live ordinal-9 subtype.

## Mechanical units

### Unit U1 — runtime taxonomy

Owner: one write-capable worker.

Write set:

- `sessionruntime/sessionruntime.go`
- new focused `sessionruntime/document_preparation_failure_test.go`, or existing `sessionruntime/document_supply_behavior_test.go`, but not both unless required by existing fixture ownership.

Inputs:

- this plan identity;
- existing `PrepareDocument` implementation;
- existing runtime fixtures.

Prescribed transformation:

1. Add the three exact constants from Decision A.
2. Replace the URI-admission `LanguageIDUnavailable` return with `DocumentURIUnavailable`.
3. Replace the workspace-containment `LanguageIDUnavailable` return with `DocumentOutsideWorkspace`.
4. Leave the true empty-language return as `LanguageIDUnavailable`.
5. Replace only the non-capture `os.ReadFile` failure with `DocumentSourceUnavailable`.
6. Do not reorder code or change any other return.
7. Add table-driven tests proving all four categories, explicit `go`, zero protocol writes, nil supply, and unchanged true-empty-language behavior.

Expected output:

- runtime package compiles;
- each cause has one exact failure;
- existing supply/cache/lifecycle tests remain GREEN.

Objective gates:

```bash
go test ./sessionruntime -run 'Test.*(DocumentPreparation|DocumentSupply|Language)' -count=1
go test -race ./sessionruntime -run 'Test.*DocumentPreparation' -count=1
go vet ./sessionruntime
```

Prohibited:

- edits outside U1 write set;
- control-flow reordering;
- new source mechanisms or fallback;
- profile/extension/default language inference;
- public transport/schema changes.

Stop and escalate if:

- a new category requires changing lifecycle or lock ordering;
- existing tests establish a published meaning conflicting with the prescribed category;
- a cause cannot be separated by replacing only the specified returns.

### Unit U2 — live-projection translation

Dependency: U1 accepted and its focused gates pass.

Owner: one write-capable worker.

Write set:

- `internal/liveprojection/prepare.go`
- `internal/liveprojection/prepare_test.go`

Prescribed transformation:

1. Add exact additive values:
   - `DocumentPreparationURIUnavailable = "DOCUMENT_URI_UNAVAILABLE"`
   - `DocumentPreparationOutsideWorkspace = "DOCUMENT_OUTSIDE_WORKSPACE"`
   - `DocumentPreparationSourceUnavailable = "DOCUMENT_SOURCE_UNAVAILABLE"`
2. Map the three U1 failures one-to-one in `normalizeDocumentPreparationFailure`.
3. Preserve every existing mapping and unknown fallback.
4. Add table rows proving the three new mappings and continued distinction from supply and language failures.

Objective gates:

```bash
go test ./internal/liveprojection -run 'Test.*Preparation' -count=1
go vet ./internal/liveprojection
```

Prohibited:

- edits outside U2 write set;
- path/URI inspection in translation;
- public classifications;
- collapse into supply or language categories.

Stop and escalate if:

- a strict external schema enumerates these internal values;
- mapping requires source/path detail;
- U1 identity or names differ from this plan.

### Unit U3 — continuation diagnostics and integration GREEN

Dependency: U2 accepted and its focused gates pass.

Owner: one write-capable worker. This owner exclusively owns the existing integration RED file.

Write set:

- `internal/censuscontinuation/capture.go`
- `internal/censuscontinuation/managed_preparation_diagnostic.go`
- `internal/censuscontinuation/capture_managed_test.go`
- `internal/censuscontinuation/managed_preparation_diagnostic_test.go` only if strict-validation coverage belongs there.

Prescribed transformation:

1. Add exact private values:
   - `ManagedDocumentPreparationURIUnavailable = "DOCUMENT_URI_UNAVAILABLE"`
   - `ManagedDocumentPreparationOutsideWorkspace = "DOCUMENT_OUTSIDE_WORKSPACE"`
   - `ManagedDocumentPreparationSourceUnavailable = "DOCUMENT_SOURCE_UNAVAILABLE"`
2. Map U2 categories one-to-one.
3. Extend `validManagedPreparationFailure` with exactly those values.
4. Keep recorder shape, limits, permissions, and public `classifyCaptureFailure` unchanged.
5. Turn `TestManagedPreparationExplicitLanguageIDIsNotMisclassifiedByURIAdmission` GREEN by expecting the outside-workspace private category while retaining explicit `go`, no partial documents, no later attempt, one private record, and generic public availability.
6. Add a 14-document table with failures at zero-based ordinal 9 for URI, scope, source, and language categories. Each row must assert attempted `10`, succeeded `9`, planned `14`, failing ordinal `9`, exactly ten calls, exact session/generation/language on every call, and no eleventh call.
7. Add strict round-trip/unknown-value rejection tests for the additive private values without changing historical value acceptance.

Objective gates:

```bash
go test ./internal/censuscontinuation -run 'TestManagedPreparationExplicitLanguageIDIsNotMisclassifiedByURIAdmission|Test.*ManagedPreparation' -count=1
go test ./internal/censuscontinuation -count=1
go vet ./internal/censuscontinuation
```

Prohibited:

- edits outside U3 write set;
- public schema or transport changes;
- exposing URI/path/source/raw errors;
- retry, fallback, resolver, worker, or model behavior changes;
- claiming the synthetic outside-workspace RED is the live subtype.

Stop and escalate if:

- strict private parsing requires a version change rather than additive accepted values;
- historical fixtures fail to parse;
- public output changes;
- the persistent RED passes without the prescribed taxonomy split.

### Unit U4 — read-only compatibility review

May run in parallel with each completed writer gate, never against files actively being mutated.

Owner: read-only investigator.

Read set:

- callers and exhaustive switches for runtime, liveprojection, and managed diagnostic failures;
- public MCP/CLI schemas and historical artifacts.

Output:

- bounded list of missed internal switches, incompatible caller assumptions, schema exposure, or historical parse risks;
- no edits.

Gate:

- no confirmed missed exhaustive switch or public enum expansion;
- any finding returns to parent for a new routed decision, not worker improvisation.

## Orchestration

Writer dependency graph:

```text
U1 -> U2 -> U3
```

Writers must execute sequentially because later package compile gates depend on earlier constants and mappings. Do not run U1/U2/U3 writers concurrently.

U4 read-only review may run after each unit's writer stops and before the next unit is accepted. Independent read-only reviewers may run in parallel with parent verification of a completed, stable unit.

Every delegated worker must verify:

- this plan's SHA-256 supplied by the parent;
- its exact unit ID;
- its exact write set;
- dependencies' accepted gate results.

A worker may not amend this plan. Ambiguity, mismatched identity, overlapping writes, unexpected failure, or required out-of-set edit causes immediate stop and escalation.

## Parent integration gates

After U1-U3 are accepted:

```bash
go test ./sessionruntime ./internal/liveprojection ./internal/censuscontinuation -count=1
go test -race ./sessionruntime ./internal/liveprojection ./internal/censuscontinuation -run 'Test.*(DocumentPreparation|ManagedPreparation)' -count=1
go vet ./sessionruntime ./internal/liveprojection ./internal/censuscontinuation
test -z "$(gofmt -l sessionruntime internal/liveprojection internal/censuscontinuation)"
git diff --check
```

Repository formatting remains governed by the known unrelated `.pi/evidence` exception; touched production/test files must be clean.

## Falsification and acceptance

Existing violating observation:

```text
ASSERT_EXPLICIT_LANGUAGE_ID_NOT_MISCLASSIFIED
language_id=go
diagnostic=failure=LANGUAGE_ID_UNAVAILABLE
```

Required satisfying observation after U3:

- the same committed named test reports PASS;
- the assertion remains bound to explicit `go` and the synthetic outside-workspace state;
- the private failure is `DOCUMENT_OUTSIDE_WORKSPACE`;
- public availability and barrier assertions remain GREEN.

The current plan has falsification coverage incomplete until that satisfying observation exists. No worker or parent may report the correction accepted before the same guard has both retained RED provenance and a GREEN result.

Operational validation is a separate authorized phase after code acceptance. It may issue one fresh census and may observe any narrower private subtype or success; it must not assume the subtype in advance or expose the failing document identity.

## Global prohibited actions

- no hosted API, network, model, worker, or hidden download;
- no alternate session/generation/root;
- no profile, extension, MIME, or filename language inference;
- no direct-read fallback added by this correction;
- no public schema, checkpoint, request, or artifact identity change;
- no historical artifact rewrite or reinterpretation;
- no URI, path, source, raw error, or provider leakage;
- no stash, reset, revert, clean, or unrelated-work modification;
- no live census until parent integration acceptance and separate authorization.
