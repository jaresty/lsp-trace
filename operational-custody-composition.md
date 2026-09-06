# Operational custody composition

Baseline fe2d364. Owner: composition. Status: authorized additive source correction implemented; focused/vet, clean full suite and full race suite PASS. Production composition remains deferred for parent review. No previous claim existed in namespace; upstream historical blocked integration limits remain in sibling claims, unchanged.

## Scope before mutation
Own internal/execution/production.go and new operational production/config/test files; cmd/lsp-trace execution host wiring; cmd/lsp-trace-mcp/main.go host startup wiring; additive schema registration/validation for an operational evidence family; new integration tests and documentation. No upstream recorder/identity/trust API rewrite, no provider revision or ownership-test changes.

Goal: explicit opt-in bounded recorded-file operation, distinct from legacy supplied-source synthetic execution. Production reads actual source/configuration/declaration/mapping bytes, binds retained contributions, builds exact versioned identity, and checks independently provisioned host store before authenticated publication. All unobserved dependencies remain incomplete. Legacy schemas/response meanings remain unchanged.

Proposed API: OperationalInput nested in ProductionInput mutually exclusive with Source; NewProductionExecutorWithTrust(*schema.HostTrustStore) dependency injection; LoadHostTrustStore(path) privileged startup only; CLI/MCP --custody-trust-config startup flag. No input grants/context/config path allowed. Retained evidence includes InputEvidence, IdentityResult, trust admission, submitted receipt/evidence and explicit incomplete state, with semantic/schema validation before publication.

Dimensions: actual byte identity versus receipt digest; multi-class duplicate paths explicitly rejected; missing reads excluded but retained; exact SnapshotID and Policy; grant absence/substitution; URI/escape rejection; defensive copies; deterministic direct/CLI/MCP parity and publication integrity; nonauthoritative legacy compatibility.

Enforcement intended: real legacy execute baseline exited 0; retained baseline-execute.json and baseline-publication/ in this namespace. Mandated inner Bar build executed. Assertion-specific executable RED before production implementation; GREEN, negative perturbations, focused/full/race/CI/release/schema/custody/parity/omission/archive checks were planned, not all executed. External LSP/provider dependency instrumentation remains outside bounded seam.

## Concrete contract blocker and retained failure history

The required missing-input join cannot currently satisfy both upstream APIs without a fabricated content digest or omitted identity accounting:
- `internal/source/contributing_inputs.go:83-101` produces an UNREADABLE canonical receipt for an actual missing scoped file, with no content bytes and no ContentIdentity (`receipt.go:68-74`). Binding the failure to a retained contribution succeeds.
- `internal/source/manifest.go:67-70` unconditionally requires a syntactically valid SHA-256 Digest on every ManifestReceipt, including EXCLUDE entries. Empty digest is rejected before decisions are processed.
- `internal/source/manifest.go:86-88` requires every decision, including exclusion, to reference a supplied ManifestReceipt.
- `internal/source/identity_policy.go:64` delegates to that exact manifest contract.

Live assertion-specific RED: `go test ./internal/execution -run '^TestOperationalUnreadableIdentityContract$' -count=1 -v`. This compiled, performed a real failing ReadInput, bound its contribution, and evaluated both assertions. Retained complete output: `operational-contract-red.log`; executable Go test: `operational_contract_probe_test.go` in this namespace. Copy the probe to `internal/execution/` to reproduce, then remove it; it is intentionally failing and is not a standing passing regression test.

Exact observed assertion results:
- `ASSERT_FAILED_READ_EXPLICIT_EXCLUSION_WITHOUT_INVENTED_DIGEST: failed read cannot join identity: INVALID: malformed receipt "sha256:6b2e9f86a19f06c2d38bfef1582eba11071de07372379bba22b553db496c2f1c"`
- `ASSERT_FAILED_READ_DECISION_WITHOUT_CONTENT_RECEIPT: excluded outcome cannot join identity: ACCOUNTING: decision references unknown receipt "sha256:6b2e9f86a19f06c2d38bfef1582eba11071de07372379bba22b553db496c2f1c"`

No production mutation occurred. The temporary probe was removed after retaining its exact bytes outside the checkout. Post-removal `go test ./internal/source ./internal/schema ./internal/execution -count=1` returned `Go test: 157 passed in 3 packages`; `git status --short` was empty. No commit was created because there are no source changes to commit.

## Authorized bounded correction (subsequent user instruction)

User explicitly authorizes a NEW additive source identity entry point/policy/tests/docs for failed observations, on clean fe2d364. This correction is a separate commit; production composition remains deferred for parent review. Own only new `internal/source/observed_identity.go`, `internal/source/observed_identity_test.go`, `docs/observed-input-identity.md`, and the committed mirror of this single claim `operational-custody-composition.md`. No edits to manifest.go, BuildIdentity, historical vectors, recorder, trust store, providers or production transports.

Proposed exact API: `BuildObservedIdentity(ObservedIdentityRequest) (ObservedIdentityResult, error)`; request fields Evidence InputEvidence, Acquisition AcquisitionContext, Revision *RevisionAttestation. Result fields Policy, SourceID, SnapshotID, CollectionID, Manifest []byte, Acquisition, Revision. New policy `lsp-trace.observed-input-identity.v1`; manifest version `lsp-trace.observed-input-manifest.v1`. Source content covers only readable path/acquired-byte digest pairs. Snapshot commits canonical observed records including failed path/class/status/failure receipt hash and failure details, contribution references, and incomplete reasons. Collection commits snapshot plus acquisition context. Optional revision remains nonauthoritative metadata outside identity. Host grants later bind exact returned Policy/SnapshotID, not source/legacy IDs.

Enforcement: adapt retained actual-read probe into persistent source test using a present rejecting API stub, obtain assertion-specific RED, then smallest additive implementation. Validate canonical receipt/status/class/path/hash/content consistency, duplicate/conflicting paths and contribution references, explicit incompleteness, ordering, mutation boundaries, all-failed/empty/mixed cases; isolate destructive perturbations; old identity/source/graph/schema tests, full and race checks; clean scoped commit. Prior RED/blocker history below remains intact.

## Required upstream decision (historical blocker, now correction authorized)

Identity/receipt owners must approve one explicit contract before composition resumes: preferably an additive identity input representation for failed observations/exclusions that preserves path and canonical failure-receipt identity without asserting an acquired-byte digest. Alternatively explicitly narrow the operation so failed reads terminate before identity and define how retained failure evidence is transported without a snapshot. The latter changes the requested failed-read composition behavior and must not be silently chosen here. Do not weaken historical manifest validation, substitute receipt JSON digest for content, invent an empty-byte read, or silently omit failures.

## Derivation (historical blocked composition phase)

Goal: join actual bounded read receipts, exact versioned identity, independently provisioned host admission and real CLI/MCP publication while retaining incomplete outcomes.

Dimensions: strict explicit input; immutable actual bytes versus canonical receipt IDs; failed-read exclusion and unknown dependency accounting; exact Policy/SnapshotID; privileged host configuration; additive validated publication; deterministic transport parity and legacy preservation.

Enforcement performed: read all four coordination contracts; claimed intended boundaries; ran and retained the real legacy execute baseline; executed literal `bar build make witness ground gate falsify atomic` bound to this frame; constructed an actual-read composition probe; obtained two assertion-specific runtime failures, not setup/compile failures; retained probe/output; restored clean fe2d364 checkout; focused source/schema/execution suite passed (157 tests).

Terminal: concrete upstream contract blocker, not implemented operational composition. No assertion-specific GREEN for the requested failed-read join exists. No production code, host startup configuration, schema registration, transport integration, destructive implementation perturbations, full/race/CI/release/archive reruns, deployment or program acceptance is claimed. Their execution is deferred until the failure-observation identity contract is resolved. The blocker concerns the required missing-input case, not proof that a readable-only opt-in executor is impossible.

## Additive correction evidence

Implemented exactly four new scoped files: `internal/source/observed_identity.go`, `internal/source/observed_identity_test.go`, `docs/observed-input-identity.md`, and this claim's committed mirror `operational-custody-composition.md`. No existing source file changed. The initial blocker is resolved by a new manifest/domain rather than weakened historical validation. Production composition is intentionally not part of this commit.

API: `BuildObservedIdentity(ObservedIdentityRequest) (ObservedIdentityResult,error)`. Request Evidence/Acquisition/Revision; result Policy/SourceID/SnapshotID/CollectionID/Manifest/Acquisition/Revision. Policy `lsp-trace.observed-input-identity.v1`; observed manifest `lsp-trace.observed-input-manifest.v1`. Readable source entries are canonical path/acquired-byte digest pairs. Snapshot commits source plus normalized observations, canonical receipt bytes/hash, failure records with no content digest, contributions, and incomplete reasons. Collection adds acquisition context. Revision metadata is detached and outside identity. Parent must bind returned Policy and SnapshotID in independently provisioned host trust.

RED: retained `observed-identity-red.log` records the persistent actual-recorder assertion `ASSERT_FAILED_READ_EXPLICIT_EXCLUSION_WITHOUT_INVENTED_DIGEST: observed identity not implemented` for mixed and all-failed cases against a present rejecting stub. All tests compiled and executed. `observed-identity-utf8-red.log` independently records lossy UTF-8 path/metadata acceptance before its narrow validation correction.

GREEN: `observed-identity-green.log` is the initial focused GREEN; final `observed-final-focused.log` contains all 67 observed-identity parent/subtest PASS records, plus unchanged source/graph/schema tests. Full source/graph/schema race run passed (`observed-race.log`); `go vet ./internal/source ./internal/graph ./internal/schema` passed. Independent Python preimage calculation is retained as `observed-vector.py`; its source/snapshot/collection vectors are pinned in TestObservedIdentityFixedVectors.

Six independent temporary production reductions, each restored before the next, produced these exact failures (retained `observed-perturb-*.log`):
- snapshot: `ASSERT_OBSERVED_FAILURE_path: failed observation must change snapshot/collection only` (also class/reason fail; status passes).
- content: `ASSERT_OBSERVED_CONTENT: changed actual bytes lost`.
- context: `ASSERT_OBSERVED_CONTEXT: acquisition must affect only collection`.
- receipt-hash: `ASSERT_OBSERVED_REJECT_receipt-id: inconsistent evidence accepted`.
- canonical: `ASSERT_OBSERVED_REJECT_canonical-bytes: inconsistent evidence accepted`.
- ownership: `ASSERT_OBSERVED_OWNERSHIP: request mutation escaped into result`.
No production perturbation switches or altered fixtures remain. These six reductions are bounded falsification evidence, not an assertion that every negative subcase received its own isolated reduction.

Counterexamples include actual missing vs directory read errors at one path, failed vs genuinely empty readable file, changed readable bytes, all four classes, same-path conflicting versions/classes/status, coherent forged receipt IDs/canonical bytes with repaired contribution references, coherent out-of-scope paths, absent incomplete reasons, unobserved/duplicate contribution references, replay ordering, and detached request/result data. Invalid UTF-8 rejects rather than aliasing JSON replacement characters.

## Derivation

Governing goal: additive observation identity that retains failed acquisition custody without fabricated source content, preserving all historical identity byte contracts.

Dimensions: real readable content versus failed observations; independent new policy/source/snapshot/collection domains; strict canonical receipt/path/class/status/content/hash and accounting validation; explicit permanent dependency incompleteness; order invariance and detached data; original identity/manifest compatibility.

Enforcement: user authorized the precise upstream correction after the witnessed composition blocker; persistent actual-recorder RED preceded the implementation; explicit failed-record manifest and new framed hash domains supplied minimal correction; canonical validation and coherently forged negative tests distinguish admission from blind hashing; new fixed vectors, focused/race/vet and six restored destructive reductions pass. Existing source/graph/schema files and old vectors have zero diff against fe2d364. Clean source commit qualification passed: `go test ./... -count=1` (`observed-full.log`) and `go test -race ./... -count=1` (`observed-full-race.log`), all packages, including integratedconformance's unchanged dirty-tree ownership checks. A claim-only amendment records these results; no executable code changes follow the passing full/race runs.

Limits and handoff: consistency of public InputEvidence values is not proof of real acquisition, authenticity, or dependency completeness. One observation per path is a deliberate hard rejection of multiversion/multiclass conflicts, not evidence dropping. SourceID covers all observed readable classes even without contribution binding. Manifest commits readable digests and original canonical receipt bytes, not another copy of readable Content; retain original InputEvidence in later published evidence. The new observed manifest is an internal identity serialization, not yet a registered publication schema. Parent review must precede actual production reads, trusted host startup, exact new Policy/SnapshotID admission, failure-preserving public schema/publication and real CLI/MCP parity. No deployment, consumer changes, or Program A/B acceptance.

