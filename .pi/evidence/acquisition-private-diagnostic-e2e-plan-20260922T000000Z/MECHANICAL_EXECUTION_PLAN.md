# Authorizing Mechanical Execution Plan

## Authorization boundary

This is a plan only. The implementation agent must not modify any production, test, schema, or configuration file until this plan is approved. The only current-session write-set is this plan and its SHA-256 sidecar.

Evidence basis: `.pi/evidence/mixed-scope-v6-qualification-20260922T030500Z/`. The observed failure is a post-restart exact census request that passed preflight and ended in public `ACQUISITION_FAILED` during acquisition, with no publication, trace, ledger, managed-preparation diagnostic, worker, or model.

## Intended production path

The guard must execute in-process through:

`lsp_trace_v1_execute -> lsp_trace_v1_census -> censusExecutor -> censusacquisition.Core -> censusBatchAcquirer -> deterministic injected failure -> private acquisition diagnostic persistence -> generic public envelope`.

The injected failure is a package-private deterministic seam, never a live gopls crash.

## Likely implementation write set and ownership

1. `cmd/lsp-trace/census_batch_acquirer.go` — owner: census acquisition implementer. Preserve `censusBatchFailure` codes and `operation.NormalizeFailure` at `acquireBatch` lines 177–180. Add only a typed/private diagnostic handoff carrying phase, session identity, ordinal, normalized operation failure code/category, and request fingerprint; never pass raw error text or request payload to persistence.
2. The file containing the MCP census orchestration and `censusAcquisitionFailure()` — owner: MCP census implementer. Locate exact symbol before editing. Convert the internal failure to the existing generic public envelope while invoking the private recorder as best effort. Recorder errors must be ignored for the primary result.
3. `internal/censusacquisition/` — owner: acquisition-contract implementer. Add/extend the private record contract and deterministic failure injection option at the Core boundary. Keep existing public schemas and managed-preparation diagnostics separate.
4. A new package-private/internal recorder file adjacent to the census orchestration (exact path to be confirmed after symbol search) — owner: custody implementer. Implement absolute clean path validation, parent mode 0700, regular file mode 0600, append-only NDJSON, bounded record/line/total sizes, and exclusive/serialized append.
5. A new package-private/internal readback/validator file adjacent to the recorder — owner: custody implementer. Strictly decode one record per line, reject unknown fields, duplicate keys, malformed variants, corruption, overflow, and custody drift; validate taxonomy and required-field combinations.
6. Existing census E2E harness location (exact test file to be confirmed from current package layout) — owner: E2E implementer. Add one production-shaped in-process guard and focused unit tests. No live server crash and no test-only replacement of the public dispatch chain.

No schema/config/public artifact files are in the write set unless implementation-time inspection proves an existing private contract requires one; escalate before adding any.

## RED phase

1. Add the deterministic injected acquisition failure seam and a failing in-process E2E guard. Inject a normalized operation failure from the session execution seam used by `censusBatchAcquirer.execute`; assert the full dispatch path is reached, not a direct Core call.
2. Assert the pre-change/RED condition: the guard must fail because no private record is persisted (or because the typed diagnostic is lost before orchestration). Capture the failure output as implementation evidence; do not weaken the assertion to accept only the public envelope.
3. Add RED unit cases for the private contract: each taxonomy value (`INVALID_INPUT`, `SESSION_DRIFT`, `ACQUISITION`, `ADMISSION`, `CANCELLED`), optional batch ordinal, normalized operation code/category, phase, exact session/generation, and request fingerprint. Reject raw URI/path/source/seed/LSP/error/selector/public-artifact fields by construction and readback.
4. Add RED custody/readback cases: absent parent, relative path, symlink, wrong parent mode, wrong file mode/type, oversized line/record, malformed JSON, duplicate key, unknown field, invalid enum, missing required field, integer overflow, appended corruption, and changed permissions/ownership between write and read.

## GREEN implementation order

1. Define a private record with an allowlisted JSON field set only: taxonomy, batch ordinal when known, normalized operation code, stable diagnostic category, phase, session ID, generation, request fingerprint. Use exact session/generation from the request/session identity. Fingerprint only canonical bounded request identity; do not hash or persist prohibited raw material as a substitute for a forbidden field.
2. Define a private typed diagnostic interface from Core/acquirer to orchestration. Map `censusBatchFailure.Code` to the closed taxonomy. Map operation failure through `operation.NormalizeFailure`; persist stable code/category only. Preserve acquisition phase and ordinal. Unknown internal failures must map to `ACQUISITION`, not create a new public or private taxonomy value.
3. Inject the recorder at orchestration construction, with a no-op/default only where production configuration already supplies the private evidence root. The E2E harness supplies a temporary absolute clean root and deterministic recorder implementation through an internal seam; it must not bypass `lsp_trace_v1_execute`.
4. On acquisition failure, attempt one private append. If recording fails, retain the original primary failure and continue generic public mapping. Never expose the recorder error, raw cause, or private path in the public envelope.
5. Implement custody-safe append: reject non-absolute/unclean paths; create/check parent as 0700; create/check regular file as 0600; use append-only semantics, bounded NDJSON line size, and serialization that cannot interleave records. Define whether fsync is required and escalate if not already dictated by the existing custody contract.
6. Implement strict readback with a duplicate-key-detecting decoder or equivalent token pass before typed decode; reject unknown fields and all malformed/corrupt/overflow/custody-drift cases. Readback must return typed records, never raw JSON or raw error strings.
7. Keep the existing public `ACQUISITION_FAILED` envelope byte/schema compatible. Do not add private fields to it. Keep historical public schemas and existing managed-preparation diagnostics unchanged.
8. Ensure failure exits before publication/continuation creation and before worker/model creation. The E2E guard must assert zero publication/continuation objects, zero workers/models, and no trace/ledger side effects.

## Main E2E assertions

- The request crosses every named production-shaped layer.
- Deterministic injected failure is classified privately as `ACQUISITION` with phase acquisition, normalized operation code/category, exact session ID/generation, request fingerprint, and batch ordinal.
- Public response remains exactly the existing generic `ACQUISITION_FAILED` envelope; compare bytes/schema against a pre-existing generic fixture or golden contract without exposing private fields.
- Private NDJSON has one valid record, no prohibited field names or values, bounded size, and valid custody.
- Recorder failure is simulated and cannot change the primary public result.
- No base/continuation publication, trace, ledger, managed-preparation diagnostic, worker, or model exists after the failure.
- Re-running the same deterministic request produces deterministic classification and does not leak a raw cause.

## Perturbations / falsification matrix

1. Change injected operation code and raw error text: public bytes stay compatible; private code/category changes only as normalized; raw string never appears.
2. Omit ordinal: record remains valid with absent ordinal; known ordinal is retained.
3. Mutate session ID or generation before execute: classify `SESSION_DRIFT`; no acquisition record may falsely claim the old identity.
4. Cancel before and during acquisition: classify `CANCELLED`; no publication or worker/model side effects.
5. Force invalid request and admission failure through existing seams: classify `INVALID_INPUT` and `ADMISSION` respectively, proving taxonomy boundaries.
6. Delete/replace/chmod/chown the private root/file between append and readback: strict readback rejects custody drift.
7. Inject recorder `ENOSPC`, permission, short-write, and serialization errors: public result remains the original generic acquisition failure; recorder error is not surfaced.
8. Add unknown JSON field, duplicate key, trailing garbage, huge integer, truncated line, or invalid enum: readback rejects the record.
9. Insert URI, absolute path, selector, seed bytes, raw LSP payload, raw error, or public artifact identity into a test input: private output must contain none of them, including as JSON keys.
10. Use a live gopls crash: test must refuse/skip this mode; the main guard remains deterministic and in-process.

## Verification tiers

- Tier 0: `gofmt`/static checks for changed Go files; inspect `git diff --check`; verify only authorized files changed.
- Tier 1: focused package tests for private record mapping, duplicate-key/unknown-field rejection, custody checks, bounded writes, and recorder-secondary behavior.
- Tier 2: the single production-shaped in-process E2E guard through `lsp_trace_v1_execute`, plus side-effect absence assertions.
- Tier 3: affected package suite and repository Go tests using the repository’s tiered Go verification guidance; do not require an exhaustive suite before Tier 2 is green.
- Evidence capture: record test command, commit/worktree identity, public response bytes/schema result, private file mode/path properties, readback result, side-effect counts, and perturbation outcomes under a new evidence directory. Never commit generated evidence into production paths.

## Stop conditions and ambiguity escalation

Stop immediately if exact census orchestration symbol/file cannot be located, if the existing custody contract conflicts with the proposed 0700/0600 behavior, if a public schema would need changing, if fingerprint input cannot be bounded without prohibited data, if duplicate-key rejection cannot be guaranteed by the selected decoder, or if a recorder failure can race/change the primary result. Escalate these as decisions; do not guess.

Required semantic decisions before implementation approval: (1) exact private evidence root/configuration injection seam and whether fsync is mandatory; (2) canonical bounded input used for request fingerprint; (3) stable diagnostic category vocabulary and operation-code normalization contract; (4) whether `SESSION_DRIFT` and `CANCELLED` records are written for failures before the acquisition call or only after Core returns; (5) exact public generic envelope fixture/byte comparison method; (6) ownership of the E2E harness file and acceptable temporary-root lifecycle.

## Completion gate

Approve only when the RED guard fails for the intended missing private diagnostic, GREEN passes all main assertions and perturbations, the public envelope is unchanged, strict readback/custody tests pass, no side effects are created, `git diff` contains no unapproved production/test/schema/config changes, and all six semantic decisions are resolved or explicitly accepted as documented exceptions.
