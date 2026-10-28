# Private retained-partial instability

`ComputeInstabilityPrivate(ctx, sealedProjection, orderedSeeds)` is an offline consumer, not a workflow, acquisition operation, ordinary Program C profile, or replay. It computes exactly one partition per requested seed. Requests require 2..6 distinct seeds; all unordered pairs are retained (six runs means fifteen pairs). Seed order is part of the request digest. Invalid seed sets are refused before math. There is no retry, seed substitution, resource increase, or isolate filtering. Independent source evidence count remains one.

The distinct profile is `DERIVED_RETAINED_PARTIAL_INSTABILITY_PRIVATE_V1`. Exact policy bytes are `InstabilityPolicyBytesPrivate()`; their SHA256 is `InstabilityPolicyDigestPrivate()`. This new, not publicly admitted format binds the private input and native graph hashes, private projection profile/digest, ordered seeds, per-run algorithm/resolution/logical digest and communities. Resolution remains 1. Authority is 0; accepted, canonical-complete and replay remain false; partial is true, source completeness UNKNOWN, feature identity UNRESOLVED.

The source-neutral helper uses existing exact rational maximum matching, node reassignment, canonical VI, logical partition digest, Jaccard and median helpers without constructing ordinary Outcomes, BoundaryArtifacts, receipts or requests. Historical files and pins remain unchanged. Matching retains m<63 and n*2^m<=1,000,000. The private guard checks width before shifting and uses n>limit/states before multiplication; it also checks representability before reporting blocked work. Blocked metrics retain community counts, state count when representable, guard and work representability. Four communities against 62 singleton communities are refused in either orientation without entering the matcher. Fixed thresholds remain reassignment/unmatched rates <=0.05, VI <=0.10 bits, minimum Jaccard >=0.80. Any blocked pair gives RESOURCE_INCOMPLETE, never stability.

## Terminal and incomplete accounting

Every declared run and pair has a row, even when not attempted. Run statuses are COMPLETE, EMPTY, FAILED and NOT_ATTEMPTED. Pair statuses are COMPLETE, BLOCKED_MATCHING_RESOURCE, NOT_APPLICABLE_EMPTY and NOT_ATTEMPTED. Separate summary counts reconcile exactly to declared run/pair denominators.

EMPTY requires a zero-occurrence sealed projection and all runs returning EMPTY with complete canonical node coverage. It preserves singleton bookkeeping and source ceilings, reports EMPTY, and labels all pairs NOT_APPLICABLE_EMPTY with no fabricated numerical stability result. Mixed EMPTY/COMPLETE runs and zeroed/missing communities are rejected. Empty is no occurrence evidence, not positive stability evidence.

Nil context is rejected explicitly before Err is called. Once a valid sealed request exists, cancellation or computation failure returns a detached INCOMPLETE assessment plus the original error. A failed run is FAILED; later runs/pairs remain NOT_ATTEMPTED. Cancellation after run one retains one completed run and five unattempted runs; cancellation during pairing retains completed/blocked metrics and all remaining unattempted rows. Partial rows cannot make the overall assessment STABLE. Such diagnostics are unsealed and cannot validate as successful metric reports.

`ValidateInstabilityPrivate` requires the sealed completed assessment and its sealed projection. It rejects unsealed/decoded/altered assessments, checks the retained byte seal, checks canonical partition coverage and logical digests, recomputes pair metrics from retained partitions, and requires exact canonical bytes (16 MiB input cap). Unknown fields, duplicate fields, alternate accounting, caller thresholds, or replacement source/profile/run metadata cannot match those bytes. Validation never reruns Leiden. `CloneInstabilityPrivate` detaches nested partitions, seeds, pair metrics and seal bytes. Source accessors detach raw, native and manifest bytes.

Bounds are per operation, not persistent quotas. Node/occurrence caps and the existing Leiden panic guard are inherited unchanged. Context is checked before/after each run and pair; it cannot interrupt an individual Leiden or matching operation and does not establish hard CPU/heap guarantees. There are no public callbacks or global test hooks; an unexported runner dependency supports deterministic failure tests only.

## Verification and evidence boundaries

Behavioral RED: with the old multiply-first predicate restored only in the new private guard, `TestPrivateMatchingGuardBoundaries` failed: `n=4 m=62 limit=1000000 guard= want=MATCHING_WORK_LIMIT`. Only the scalar guard ran, so this witness never invoked the unsafe allocation. Restoring division-before-multiplication made the boundary test and both full 62-node partition orientations GREEN. Scalar tests cover m=0,1,62,63 and exact 1,000,000 versus 1,000,001 work without allocation.

Pure size fixtures cover node counts 1,16,100,1000, one-community stability and varying singleton counts. Large singleton partitions are resource-blocked. These are observed dispositions, not timing or performance promises.

The existing fresh eight-record exact-25 fixture tests derivation after borrowed roots close, projection, six seeds, validation and mutations, in default/nondefault source contexts. Its 27 singleton communities produce fifteen blocked pairs: RESOURCE_INCOMPLETE is expected. A dedicated `TestOwnedPrivateInstabilityFixture` reuses that producer setup without the broad qualification mutation matrix. It checks closed-root ownership, nil context, cancellation before math/after one run/after one pair, exact row accounting and no new fixture traffic.

Consumer-unit fixtures additionally test EMPTY, stable/unstable partitions, original computation-error identity, no subsequent computation, pair interruption, cloning, exact six invocations, pre-math rejection of seven seeds, logical/seed/source/status/threshold mutations and decoded-DTO rejection. These internal numerical states do not constitute new producer qualification. Attempts to obtain an EMPTY producer fixture under the existing fixed partial qualifier were refused (complete empty scope, zero-range CALLS, malformed response or incomplete discovery); no qualification or CALLS rule was relaxed. Thus actual-producer EMPTY coverage remains a fixture gap, not an asserted result.

Final targeted checks:
- Normal affected test families: 206 tests passed across three packages.
- Focused private math/consumer race tests: 15 passed.
- Dedicated owned producer plus requested `TestRetainedPartialQualificationPrivate/derived-private-native-input` race selection: 13 passed.
- Targeted vet, gofmt and diff check passed.

Prior two 200-second race timeouts are not deadlock evidence; the narrower final race gates completed. The immutable dirty-tree Stage7C gate was neither edited nor excluded and was not run in this unit. No full-suite confidence is claimed.

No historical slice was computed or written, no live/server/replay operation occurred, and nothing was committed or pushed. Independent parent review remains required before use or integration. Connecting a reviewed assessment to a later consumer is separate work; no workflow, Describe integration, historical assessment, or public admission is claimed here.
