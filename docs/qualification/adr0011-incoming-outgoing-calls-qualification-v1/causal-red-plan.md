# Causal RED plans through real paths

No test or implementation is created by this package. Each RED must first fail at the named missing production seam, not at a fake replacement path. Passing before implementation is a blocker.

## Shared Layer 1 RED sequence

1. Drive `Manager.RoundTrip` with the private, default-off qualification request through the real framed child transport.
2. Assert exact request/response frame bytes, transaction key, method, workspace/session/generation, terminal, and C15 snapshots are unavailable today at the immutable transfer seam (`RED: transfer absent`), while ordinary result bytes remain unchanged.
3. Add only the neutral Layer 1 transfer; rerun and require the RED to move to the directional consumer boundary.
4. Exercise success, protocol error, malformed frame, short read/write, cancellation, timeout, session crash, pre-key refusal, post-key refusal, STOP, RESTART, explicit release, callback panic, and callback/lifecycle race.
5. For each: one terminal, callback drain before zero, exact-once C15 release, no use after return, and disabled parity.

## Incoming RED sequence

Real path: `incomingops.Executor.Execute → ResolveTarget → SessionClient.call → Manager.RoundTrip → traverse.Incoming`.

1. Script real managed frames for documentSymbol, prepareCallHierarchy, and incomingCalls. Do not call an assembler directly.
2. First RED requires an incoming immutable transfer joined to exact three-transaction predecessors and unchanged selected-item handoff. Current path lacks this qualified chain.
3. After transfer exists, RED requires direction-specific member ledger before `traverse.Incoming` emits graph occurrences.
4. Cases:
   - one member with one range;
   - one member with `fromRanges:[]` ⇒ members=1, zero_site=1, ranges=occurrences=edges=0, complete-empty directional terminal;
   - missing/null/scalar/object `fromRanges` ⇒ malformed, no prefix;
   - duplicate ranges/members, parallel pairs, self-loop ⇒ ordinals and multiplicity preserved;
   - stale generation, foreign workspace, swapped prepare, outgoing profile/provider, changed selected span/handoff ⇒ pre-effect refusal;
   - top-level `[]`, allowed exact null only if exact-profile contract independently accepts it, server error, malformed/trailing bytes, cancellation, timeout.
5. Compare selector omitted versus private CALLS_ONLY selected: provider write bytes, response bytes, request count/order, transaction keys, cancellation/deadline behavior, and legacy graph artifact bytes must be byte-identical.

## Outgoing RED sequence

Real production path: `traceExecutor.Execute → PrepareDocument/resolveSymbol → explicitSeedSpec → acquisitionorchestration.ExecuteExplicitTrace[Prepared] → outgoing traversal`.

1. Drive the public trace/slice executor with a managed fake child; do not invoke outgoing traversal directly as the only witness.
2. First RED requires the outgoing-specific immutable transfer at the exact outgoing provider request path. Incoming types/constants/helpers are forbidden imports.
3. Require the independently joined documentSymbol → prepare → outgoingCalls chain and unchanged selected caller item.
4. Exercise outgoing `to` identity and caller-side `fromRanges` with duplicates, zero-site members, parallel pairs, self-loops, malformed arrays, server error, cancellation, timeout, stale/foreign/substituted identities.
5. Compare omitted selector and private CALLS_ONLY selection for byte-identical provider and legacy artifact behavior.
6. Assert outgoing failure cannot alter incoming accounting, evidence, provider identity, review status, or terminal.

## Numeric RED gate — currently blocked

No numeric RED may be authored from the candidate values. `numeric-bound-audit.log` shows that at least twelve values lack accepted non-proposed authority or retained measurements. A future candidate must first supply, per row, the governing requirement or measurement, observed maximum, explicit arithmetic margin, unit/counter owner, and exact shadowing order. Only then may its exact equality require continuation and plus one require typed pre-effect refusal with unchanged snapshots. An unreachable side remains `BLOCKED_NOT_RED` naming the exact earlier guard; limits may not be altered to manufacture reachability.

## Authority REDs

Incoming and outgoing each require an independent exact-profile fixture binding:

- language-server full executable bytes/digest/length/version;
- argv, cwd, behavior-relevant environment and toolchain/module inputs;
- initialize request/result and initialized notification exact frames;
- negotiated capabilities and position encoding;
- exact workspace revision/dirty state and target source bytes;
- request/response originals and transaction custody;
- independently signed/accepted provider semantics and review identity.

Substitute one field at a time and require changed identity or typed refusal. Cross-direction provider IDs, evidence roles, review signatures, and GO tokens must fail closed.

## Acceptance sequence

`RED witnessed → minimal implementation separately authorized → RED green through real path → disabled/CALLS_ONLY byte-parity green → C15 census/release review → incoming authority review → outgoing authority review → exact-profile runtime recapture → package checksums/readback → separate production-admission decision`.

No earlier step implies a later one.