# Independent review: ADR 0011 incoming Go/gopls acquisition v2

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `e403b9c243c853a59a176949a52a712c893a5399`
- Decision: `BLOCKED`
- Acquisition gate: `PENDING`
- Authority: `0`
- Accepted: `false`
- Completeness: `UNKNOWN`
- Production admission: `REFUSED_MISSING_EVIDENCE`

## Accepted stop-before-effects evidence

The packet consistently records that no exact-workspace session existed and that the only READY session was rooted at `/Users/schwa/dev/lsp-trace`, not the reviewed worktree. It also consistently records that the managed capability surface did not expose the required pre-execution gopls binary/version/digest/launch custody, Go toolchain/environment custody, initialize request/result and negotiated call-hierarchy capability custody, or original prepare/incoming JSON-RPC frames.

The stop was therefore correctly taken before effects. The packet records zero workspace derivations, prepare dispatches, incoming dispatches, document-symbol dispatches, normalization outputs, occurrences, semantic graph operations, and production-visible effects. It does not infer, recreate, copy, or promote historical prepare/incoming payloads.

## Verified boundaries

- `HEAD` remains exactly `e403b9c243c853a59a176949a52a712c893a5399`.
- Before this review file, the only worktree changes were the six untracked v2 evidence files.
- The tracked evidence manifest is unchanged at SHA-256 `334a6a1f3c26b0408023d24f8bd08f74bfbcd0125629ee1cf04fdd425fa6ce87`, length `10433` bytes.
- The manifest still has exactly four `PRESENT` roles, ten `PLANNED` roles, and thirty-one `MISSING` roles.
- Every entry in `SHA256SUMS` matched both digest and byte length.
- Every pre-boundary evidence entry in `BOUNDARIES.json` matched both digest and byte length.
- `BOUNDARIES.json` correctly describes the packet before this independently authored review; this review is intentionally not retrofitted into the author-owned hash set.

## Blocking defects in the proposed test-only capture seam

The stop evidence is accepted, but the proposed seam is not yet a minimal sufficient prospective-capture seam.

1. **Target-selection custody is omitted.** The proposal names initialize, `prepareCallHierarchy`, and `incomingCalls` frames, but the frozen profile requires separately retained `documentSymbol` request/result, ordering, selected-symbol identity, ranges, ambiguity/absence/malformed outcomes, and attempted prepare sequence whenever document-symbol-assisted selection is used. The proposal neither captures that transaction nor explicitly restricts future acquisition to a direct-position path.

2. **Selection and unchanged-handoff custody is unspecified.** Capturing raw frames alone does not define retention of prepare raw result order, selected ordinal, canonical full prepared-item identity, or proof that the exact selected item is passed unchanged to `incomingCalls` without substitution or lossy remarshal.

3. **Transaction and terminal accounting is underspecified.** Monotonic frame ordinals and cumulative byte/message counts do not by themselves supply request ordinal, expansion ordinal, operation limits, attempted/unattempted and captured/uncaptured accounting, terminal outcome, cancellation/timeout/error distinctions, or fail-closed treatment of partial and malformed frames.

4. **The test-only/non-production boundary is asserted, not designed.** The proposal does not name an activation mechanism, dependency-injection boundary, build constraint, ownership point, or proof that the recorder cannot be enabled by production configuration. It also does not show that observing the real managed process path leaves launch, initialization, framing, timing, buffering, and semantic request behavior unchanged.

5. **Minimality is not demonstrated.** One proposed recorder combines executable copying, version discovery, argv/cwd/environment custody, Go toolchain and workspace inventory, sink reservation, and wire recording. The packet does not identify the existing owner seams or show why one combined process-boundary recorder is smaller than narrowly injected test-only custody and transport observers.

6. **Provider authority remains unavailable.** A self-observing test recorder can retain bytes and process facts, but the proposal does not identify an independent authority source for the manifest role `provider_authority_for_exact_gopls_profile`. It therefore cannot by itself make the future acquisition package sufficient for qualification.

## Required correction before implementation consideration

Freeze a mechanically precise test-only design that names its owner/write set and activation barrier; proves production unreachability and semantic transparency; either captures `documentSymbol` custody or explicitly selects the direct-position-only path; records exact method frames plus selection, unchanged handoff, expansion, limits, and terminal accounting; identifies independent provider authority; and retains fail-closed behavior before semantic dispatch when any pre-execution custody or sink reservation is unavailable.

No implementation, manifest/source/plan/Track B edit, semantic-vector use, GO, or commit is authorized by this review.
