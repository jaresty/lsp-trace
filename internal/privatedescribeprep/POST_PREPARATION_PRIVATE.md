# Private post-preparation mechanical accounting

This prospective typed profile is **not** ordinary Describe, native/workflow admission, a generic workflow framework, a semantic qualification, or source-policy expansion. Historical ad-hoc 25-description JSON is not imported or silently coerced. The historical four nominations/six supplies and existing sealed source policy are unchanged. Synthetic 1/4/25/64 mechanical cases do not qualify producers or establish repository scalability.

## Exact API

```go
NewPostPreparation(context.Context, []Input, Limits, PostPreparationLimits) (PostPreparation, error)
(PostPreparation).PreparedBytes() ([]byte, error)
(PostPreparation).PreparationSHA256() (string, error)
(PostPreparation).Manifest() ([]PacketManifest, error)
(PostPreparation).Reconcile(context.Context, []CallerSubmission, []CallerReview) (StageLedger, error)
```

The constructor calls existing `Prepare` directly with sealed projection/assessment pairs. It checks complete selected-index coverage, constituent accounting, exact packet/part bindings, ordered part hashes, part byte/count budgets and evidence counts before retaining private immutable state. It does not counterfeit qualified `targetpacket`, `retainedprojection`, or `describerequest` inputs. There is no exported constructor from `Result`, JSON or arbitrary bytes. A zero handle refuses operations. Internal `postFromPrepared` exists solely as the constructor's mechanical validation seam; package-local DTO fixtures are not producer qualification or public admission.

The handle is read-only computational state, not spent execution authority or a replay handle. It accepts no callback, provider, source path, Manager, partition runner, publication or IO capability. `Reconcile` never mutates state; DTOs, getters, manifests, coverage selections, raw gap rows and returned outcomes are detached. Callers must not mutate inputs concurrently.

### CallerSubmission

Fields: `PreparationSHA256`, `InputSHA256`, `Target`, `PacketID`, `OrderedPartSHA256`, `AuthorTag`, `Disposition`, `WorkingLabel`, `TechnicalDescription`, `Claims`, `Limitations`, `Alternatives`, `RejectionCondition`, and embedded `PrivateCeilings`. Claims contain `Text` and `Citations`; citations contain `Handle` and optional `SourceQuote`.

`DESCRIBED` requires nonblank bounded label, description, claims with citations, alternatives and rejection condition. `ABSTAINED` requires explicit limitations and zero claims. Exact constructor-retained preparation and manifest identities are required. Hash comparisons are byte-for-byte, including prefix, length and every hex character; no normalization or alternate identity scheme exists. All hashes use `pilot.Digest` (`sha256:` prefix).

Citations can refer only to that packet's handles. A quote is at most 240 UTF-8 **bytes**, must cite an own `RETAINED_SOURCE_AST_DECLARATION`, and must occur literally in its `Body`. A matching quote does not entail the claim. Prepare legitimately reuses some endpoint handles across START/FRONTIER rows: all rows remain in the manifest, that handle remains own-packet evidence, and a multi-kind handle cannot authorize a source quote. Source code and comments are inert text, including instruction-like strings.

`PrivateCeilings` must be explicit: Authority=0, Accepted=false, FeatureIdentity=UNRESOLVED, SourceCompleteness=UNKNOWN, OrdinaryNative/Describe/Workflow=NOT_ATTEMPTED. No accepted verdict exists. Invalid caller DTOs produce a rejection reason and digest, not a retained DTO with escalated authority fields.

### CallerReview

Fields: `PreparationSHA256`, `InputSHA256`, `Target`, `SubmissionSHA256`, `ReviewerAuthorTag`, `Verdict`, `Findings`, and embedded `PrivateCeilings`. Obtain the canonical `SubmissionSHA256` from a first `Reconcile` output; supply it in a subsequent call along with the same submission. Verdict is `SUPPORT_WITH_LIMITS`, `CHANGES_REQUIRED`, or `ABSTAIN`, with nonblank bounded findings. A stale pin, missing/invalid submission, bad ceilings or same producer/reviewer author tag yields REJECTED and cannot upgrade a submission. Different tags record only `CALLER_ASSERTED_INDEPENDENCE_NOT_AUTHENTICATED`; they are not authentication or semantic independence proof.

## Ledger and provisional leads

Every selected original index is preserved in preparation order, independent of caller submission/review ordering. Identities are exact `(InputSHA256, Target)` struct map keys, never delimiter concatenation. Preparation states remain PREPARED/SOURCE_GAP/RESOURCE_GAP. Prepared items have PENDING/SUBMITTED/ABSTAINED/REJECTED submission status and PENDING/RECORDED_SUPPORT_WITH_LIMITS/CHANGES_REQUIRED/ABSTAIN/REJECTED review status. Gap rows have no submission state and cannot receive submissions or reviews. Unknown/foreign identities, duplicate submissions and duplicate reviews reject the whole call, rather than attributing a failure to another namespace. Known prepared targets with invalid bindings/citations/quotes are individually REJECTED.

For each constituent:

- requested = prepared + source gap + resource gap
- prepared = pending + submitted + abstained + rejected

GroupingPrecondition is always `UNRESOLVED_NOT_CHECKED`; NominationCensus is always `NOT_ADMITTED`. Discovery never implies grouping readiness.

Only valid DESCRIBED submissions with exactly matching caller SUPPORT_WITH_LIMITS reviews yield `PROVISIONAL` technical leads. Their ID hashes a structured exact input namespace and target, never the working label or community. Leads inherit the canonical submission, claims/references, alternatives, limitations, falsifier, review and exact own-packet GAP/SOURCE/CALL_ENDPOINT_SOURCE_AVAILABILITY records. All emitted outcomes preserve zero authority, nonacceptance, unresolved feature identity and unknown source completeness. Support is labeled `CALLER_REVIEW_RECORDED_NOT_VERIFIED_SEMANTIC_SUPPORT`. This is mechanical validity separated from caller-recorded meaning, not qualified semantic admission.

## Required finite budgets

All `PostPreparationLimits` values are required and positive; no public defaults:

| Field | Private hard ceiling |
|---|---:|
| MaxConstituents | 64 |
| MaxLedgerTargets | 4096 |
| MaxSubmissions, MaxReviews | 256 each |
| MaxClaimsPerItem | 16 |
| MaxCitationsPerClaim | 32 |
| MaxStringBytes | 64 KiB |
| MaxItemBytes | 1 MiB |
| MaxInputBytes, MaxOutputBytes | 16 MiB each |
| MaxEvidenceRecords | 262144 |
| MaxWork | 10000000 |

MaxLedgerTargets is separate from Prepare's 64 target cap: untouched resource gaps may still require an index beyond 64. Caller counts and all strings are checked before mapping, copying or encoding caller DTOs. UTF-8 scanning follows byte-length rejection. Conservative allowance is six bytes per string byte, plus fixed key/container/scalar overhead; MaxItemBytes and MaxInputBytes constrain this estimate, not merely final JSON length. Complete-index output reservation includes twice retained preparation bytes, twice caller DTO allowance, index and coverage overhead before building/marshaling outcomes. A conservative estimate can refuse output that would actually encode smaller. No clipping, partial authority, dropped records or borrowed per-item budgets occurs. Exact serialized output is checked again.

Work uses subtraction-safe counters with cancellation checks in bounded loops; source quote searches charge the full retained Body length before searching. Evidence records are counted before evidence maps. Constructor input parsing/AST/partition evidence validation remains governed by existing `Prepare` budgets; post-processing does not rerun partitioning. These are bounded computational guards, **not** hard process heap or CPU guarantees. The allocation regression checks early rejection of a preconstructed 1 MiB text, not a universal allocation bound.

## Verification and remaining boundaries

Pure mechanical tests cover variable 1/4/25/64 prepared targets, exact/one-over string/input/output/item/work budgets, count overruns, rejected citations/quotes/pins/authority, pending/abstention/review states, deterministic caller reordering and aliasing. Owned tests reuse `preparationOwnedInput(t)` after its roots are closed: selected 1/4/25 retain actual one prepared and 0/3/24 source gaps. Two independent nil-selection inputs retain 50 indexes, two prepared and 48 source gaps; identical root labels/source bytes do not merge. Foreign citations/review pins and mixed sealed assessments fail. Traffic counters must remain unchanged across post calls.

Observed focused checks in this writer thread: `go test ./internal/privatedescribeprep -count=1 -timeout=240s` (83 tests), the two new owned integration tests (2 tests), and focused `go test -race` across both packages (63 tests) passed. No broad repository CI was run. An initial owned test rejected legitimate START/FRONTIER handle reuse; preserving all own manifest rows while refusing multi-kind quote authority corrected that integration mismatch. A temporary counterfactual disabling handle membership caused the no-quote foreign-handle guard to fail; restoring it returned the guard and complete focused checks to green. This is one explicit perturbation witness, not exhaustive mutation coverage.

Independent read-only review found no blocking, high, or medium correctness defect. Parent executable checks passed: all 83 preparation tests, both new owned integration tests, 63 focused race tests across both packages, and 19 retained-partial-computation regression tests. Formatting and tracked-diff whitespace checks passed. No broad repository CI or exhaustive mutation adequacy is claimed. MaxWork is not a comprehensive CPU meter; cancellation can race the final bounded marshal, which is not interruptible.

This increment introduces no push, live acquisition, new provider, public operation, policy relaxation or durable publication. Source-scope generalization, actual semantic verification, authentication, feature acceptance, grouping readiness, history import, merge/split/revision APIs, durability and publication remain future separately governed boundaries.
