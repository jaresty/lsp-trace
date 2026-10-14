# V5 — four independent causal witnesses

Binding: **ADR0011_V5_REFRAME**. Verdict ceiling: **AUTHORED_NOT_GO**, never GO.

## Exact14 scope

The connected production implementation remains: production executor SessionClient dispatch, neutral acquisition adapter, private Manager ingress, shared transaction, reserved original request/response transport, lexical aliases, provider ordinals through traverse/slicer, immutable noncopy views, actual SHA256 observations, separate 32 MiB raw/general partitions with combined 64 MiB authority, and generic/V2/C15 terminal convergence. Exactly six carrier types remain. Ordinary `Runtime.RoundTrip` is byte-for-byte unchanged; no CLI/MCP/schema activation is added.

`union.patch` contains exactly the 14 manifest paths: 13 connected production paths and one four-witness test file. This correction changes only two production paths (private Manager allocation/failure observation and shared transaction missing-ownership guard) plus that test file, relative to the previous union. Metadata and post-application hashes are regenerated. No additional production or test path is introduced.

## The four witnesses

1. **EXACT32:** production SessionClient prepare acquisition reserves request + original response capacity exactly 32 MiB. The sole retained raw `make` is in `allocateBackingLocked`; immediately before it, a snapshot captures live/cumulative charge, allocation count, ownership-object count, occupied state, component and length. The fixture independently supplies expected ordered request/response lengths. Both snapshots must show full prior reservation and ownership, not merely correct final totals. Every acquired fixture packet must correspond to exactly one Manager frame observation, and vice versa. Physical provider ordinal **1**, exact frame/element commitments and release remain checked.
2. **PLUS_ONE:** ordinal-1 source acquisition/release precedes a prospective incoming request frame exactly 32 MiB+1. Quota refusal is **proposed, not acquired**, before retained raw allocation and WRITE. An independent Manager stdin probe measures Write calls and offered raw bytes before provider decoding, plus actually emitted bytes; all deltas must be zero. A small valid incoming request then succeeds on the **same READY Manager, client, session ID and generation**, with positive raw transport bytes matching its decoded request. General ownership-table bookkeeping remains separately reported.
3. **PARTIAL_EXTERNAL:** a real incoming transaction consumes notification, unmatched response and truncated response. Acquired prefixes are independently hashed. The exact typed Manager terminal is `session.SessionPoisoned` (`SESSION_POISONED`), and the client error and all incoming-frame observation labels must agree. This is explicitly distinct from `session.ResourceExhausted` (`RESOURCE_EXHAUSTED`) quota refusal. Ownership releases normally.
4. **EARLY_TERMINAL:** a real incoming null response takes the production client's early return. Generic release wins once, invalidates the source view, and releases every created object.

Each witness starts a real Manager, completes its initialize/readiness exchange over fixture pipes, and enters production `incomingops.SessionClient.call` with that Manager. The controlled protocol provider is not production-server, authentication, or semantic-completeness evidence. Fixture-side bytes form an independent length/digest oracle. No manually constructed generic owner is substituted.

## Independent Phase2 counterfactuals

The retained matrix preserves all prior gates, including request/response allocation bypass, charge omission, digests, provider ordinal, aliases, release, nonpoisoning refusal, proposed commitment and incomplete-prefix-write detection. The prior `manager-admission` gate is corrected: instead of injecting an unconditional `ResourceExhausted`, it skips the actual production admission call. An unowned directional transaction then deterministically returns before pending/I/O via the missing-ownership guard. Its production-entry assertion fails without timeout, panic, or compilation failure.

Added independent gates:

- Move the actual request allocation before quota/ownership checks, retaining that same backing: `ASSERT_C4_REQUEST_ALLOCATION_BOUNDARY` fails.
- Independently move the actual response allocation before those checks: `ASSERT_C4_RESPONSE_ALLOCATION_BOUNDARY` fails.
- Drop only request-frame observation: `ASSERT_C4_EXACT32_COMPLETE_CORRESPONDENCE` fails.
- Relabel the actual acquired partial/external terminal as quota refusal: `ASSERT_C4_PARTIAL_EXTERNAL_TYPED_FAILURE` fails.
- Independently relabel only observation failure strings, leaving typed Manager failure intact: the same typed-failure assertion fails.

Snapshots are private qualification receipts, not acceptance authority or a process-wide allocation audit. Moving the physical allocation and its immediately adjacent observation together is the counterfactual; a detached final-accounting sample is not substituted. The direct frame-release loop is retained but is not claimed independently critical because the inherited account terminal also releases remaining C15 objects.

## Reproduction and authority limits

Run `python3 verify.py --source /private/tmp/adr0011-incoming-calls-v1-6yiY4m`.

The campaign may remain at `49334ce23de37c0094f50d21d57ab251a46130ca` with the old union applied. The verifier does not reset, clean, apply to, or otherwise mutate it. It records HEAD, status, tracked binary diff digest and every untracked-file digest before/after and requires exact equality. A source at clean patch base is also supported. Disposable clones are checked out at patch base `04a4ca85ffbfd3916010897fb6fcc193ba47252d`, base hashes checked, and the corrected union freshly applied.

Only the four named witnesses run, each in its own test-binary process with recorded PID. Each critical-edge mutation is applied alone and must compile and fail its named assertion (not timeout/panic). Each edit is restored, post-hashes rechecked, then all four fresh-process witnesses pass again. Broader lifecycle, race, sorting, arithmetic-helper, C15 and semantic campaign scenarios are deferred. No delegation, commit, or campaign-source mutation occurs.

`PATH_MANIFEST.json`, `WITNESS_CONTRACT.json`, `CRITICAL_EDGE_MUTATIONS.json` and `verify.py` describe the exact boundary. `SELF_HASHES.json` / `.sha256` seal those files plus this README and the union, not authenticity. `verification/RESULT.json` / `.sha256` bind actual logs, receipts, source state and mutation outcomes. No production GO is authorized.
