# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Disposable C15 Write-Set — Final Review V7

Status: **BLOCKED**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `3095a4465a691c80f3d0d1138113e1e5c3055523`
- Reviewed modified input: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-disposable-c15-writeset-final.md`
- Review mode: independent, documentation-only
- Mutation made by this review: this review document only
- Source, test, corpus, RED, prototype, generated artifact, canonical C15, disposable authority, production authority, and writer custody: **not issued**

## Disposition

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_DISPOSABLE_C15_WRITESET_FINAL_V4_BLOCKED
```

The final write-set is blocked because the pinned causal RED does not exercise the proposed W23 production path. In `sessionruntime/b4_callhierarchy_layer1_generic_v3_test.go`, `callHierarchyLayer1GenericV3Execute` decodes and applies the causal-outcome setter, then returns directly from the test harness when the class is `callHierarchyLayer1GenericCausalOutcomePreEffectRefusal` (current lines 1017–1018). That return occurs before:

1. `privateCallHierarchyLayer1WithPredecessors` (current lines 1020–1032);
2. Manager/transport construction (current lines 1034–1044);
3. `manager.RoundTrip(ctx, request)` (current line 1045); and
4. generic-owner lease acquisition and actual-observation assertions.

The corpus runner counts that harness-local return as `pre_effect_refused` and skips `callHierarchyLayer1GenericV3AssertActual` for it (current lines 2059–2073 and 2084–2091). Consequently, W23 does not causally require or verify the write-set’s proposed first-line `RoundTrip` branch, `finishCallHierarchyLayer1GenericRefusal`, immutable refusal-owner construction, Manager registry publication/refusal, outside-lock terminalization/join, or fail-closed acquisition behavior.

This is not a documentation wording issue. The proposed production branch could be absent, unreachable, incorrectly published, or implemented outside the stated lifecycle contract while W23 continued to satisfy the harness-local refusal result. The required claim “W23 pre-effect refusal has no effects through the existing refusal publication” is therefore not established by the pinned RED.

The same early return also violates the write-set’s claimed universal first executable attachment invocation order. W04/W05 reach `identity -> enabled -> causal outcome -> predecessors`; W23 reaches only `identity -> enabled -> causal outcome` and returns before predecessors. Thus the exact order cannot be confirmed for all three scoped occurrences as written.

## Mechanical review results

### Physical boundary

The proposed production boundary is exactly three existing files:

1. `sessionruntime/sessionruntime.go`
2. `sessionruntime/owned_method_pair.go`
3. `sessionruntime/b4_owned_method_pair_lease.go`

No fourth production file is named by the proposal. `sessionruntime/b4_definition_private.go` remains expressly forbidden. This physical boundary is internally closed, but closure does not cure the W23 causal gap.

### Causal-outcome shape

The proposed production shape is exact and internally consistent:

- `callHierarchyLayer1GenericCausalOutcomeClass uint8`;
- constants `Unavailable` (zero), `AcceptedTerminal`, and `PreEffectRefusal`;
- `callHierarchyLayer1GenericCausalOutcome` fields:
  - `class callHierarchyLayer1GenericCausalOutcomeClass`;
  - `terminal manageddiagnostic.Terminal`;
  - `refusalType callHierarchyLayer1GenericRefusalType`;
  - `refusalReason session.Failure`;
- accepted value: typed `manageddiagnostic.Terminal("successful-empty")`;
- refusal value: typed `callHierarchyLayer1GenericRefusalMethodUnavailable` plus `session.Failure("method unavailable")`;
- setter signature: `(callHierarchyLayer1RequestAttachment).privateCallHierarchyLayer1WithCausalOutcome(callHierarchyLayer1GenericCausalOutcome) callHierarchyLayer1RequestAttachment`;
- setter semantics: replace only `causalOutcome`, preserve all other fields and predecessor pointer, allocate nothing, parse nothing, and dispatch on nothing.

The test-only decoder is strict about missing, null, non-string, empty, case-shifted, whitespace-shifted, Unicode-shifted, unknown, duplicate, conflicting, and trailing forms. Production parsing is forbidden. The write-set accurately preserves that authority boundary.

### Placement and anti-dispatch

The pinned RED mechanically fixes the three scoped occurrences as:

- W04 / request ID 104: accepted terminal;
- W05 / request ID 105: accepted terminal;
- W23 / request ID 123: pre-effect refusal.

It requires `causal_outcome` to be absent on the other 82 occurrences. Its anti-dispatch mutations cover request ID, session/address, world label, method, expected terminal, response, error, frame, control, and external result while requiring decoded and setter values to remain unchanged. The write-set correspondingly forbids construction, selection, overwrite, normalization, or dispatch from ID, request key, world, method, expected/result/response/error/frame/terminal/control/external values, C controls, fixture/occurrence/pair names, registry identity, sibling actuals, and prototype output.

These placement and anti-dispatch guards are accepted as test-owned input constraints. They do not repair the W23 runtime/publication bypass.

### W04/W05

The proposed rule is appropriately narrow: W04/W05 consume the already typed `successful-empty` terminal only inside the admitted accepted finalizer after actual transport completion, without parsing result bytes or inspecting response/frame/RPC error/method/identity/world/expected values. Raw result custody remains independent. No production decoder is authorized.

The proposal’s W04/W05 rule is internally coherent and consistent with the typed test input. This review does not promote it to implementation authority because the overall write-set is blocked.

### A116, B/C exclusions, and lifecycle

The proposal preserves exactly the A116 ordinary-occurrence projection and states unchanged corpus accounting of A=9,825, B=1,735, C=765, union=12,325 across 85×145 paths. The causal-outcome carrier is driver input, not a new expected A/B/C path. W24 aggregation remains B-only; W25 equality remains RED-only; all nine C fixture/control/organization paths have production seam `NONE`.

The owner/carrier/registry/retirement/lifecycle model is closed as written:

- one request-carried attempt pointer and one transient result-carried attempt pointer;
- immutable owner distinct from the transient carrier;
- unchanged `RoundTrip`, `roundTrip`, `roundTripWithPrivate`, `roundTripWithPrivateTransaction`, and `finishRoundTrip` signatures;
- one Manager owner slice and one exact `(sessionID string, generation uint64)` retired-key map;
- publication, retirement marking, and detach serialized under `m.mu`;
- STOP marks before all-generation session detach;
- RESTART marks before exact-old-generation detach;
- Shutdown transfers the whole slice, nils it, and clears the retired set;
- terminal request and wait occur outside `m.mu`;
- explicit release and teardown share the sole `releaseLocked` transition;
- ordinary successful publication remains open/acquirable;
- closed/retired publication returns only after outside-lock terminalization/join;
- no alternate session registry, registry-entry type, second index, owner back-pointer, goroutine, channel, parser, serializer, or parallel lifecycle machinery.

The detach allocation contract is exact: one unconditional detached-slice allocation at `detachCallHierarchyLayer1GenericOwnersLocked`, retained-pointer compaction in place, stable order, and nilled vacated tail elements. No retained-slice backing allocation or lifecycle action under `m.mu` is admitted.

These are internally consistent proposal constraints, not verified runtime outcomes; no disposable patch or canonical C15 exists.

## Closed C15 inventory reconciliation

The modified write-set’s lists mechanically reconcile to exactly:

| Candidate class | Exact total | Review result |
|---|---:|---|
| Field candidates | 12 | reconciled |
| Type candidates | 25 | reconciled |
| Existing declarations expected to change | 17 | reconciled |
| New function/method candidates | 22 | reconciled |
| Allocation/copy-site rows | 16 | reconciled |

The lists contain no duplicate candidate names. No list item is marked optional, conditional, implementation-selected, or open. Conditional runtime allocation behavior is described within fixed allocation-site rows and does not create conditional candidate membership.

Classification boundaries are correctly preserved:

- the generation key is one type candidate, not an anonymous field or map-key candidate;
- the thirteen refusal constants remain members of one refusal-type candidate;
- the three causal-outcome constants remain members of one class-type candidate;
- contained struct fields in the closed type declarations are not separately inflated into dynamically discovered field candidates;
- body-only edits to `RoundTrip`, `roundTripWithPrivateTransaction`, `finishRoundTrip`, `runLifecycle`, and `Shutdown` remain changed existing declarations;
- body-only causal additions to the already proposed constructor and accepted/final round-trip finalizers do not create duplicate function candidates;
- `roundTrip` and `roundTripWithPrivate` are not misclassified as changed declarations;
- pointer copies through by-value request flow and the one result initialization are not allocation candidates;
- the identity and causal-outcome setters/copies are comparable value copies and add no allocation candidate;
- no bridge, alternate carrier, signature change, direct helper-test edit, or fourth-file declaration appears in the candidate set.

Every candidate remains `UNKNOWN`. The list is a proposed exact expected set only. It is not canonical C15, does not classify a candidate, and grants no mutation or reuse authority.

## Negative-patch boundary

The document correctly treats commit `9e92fc2a6cdcb44715dc9038c4a87c6227c4295d` as negative evidence only. It inherits no source, test result, carrier shape, causal conclusion, authority, custody, or reusable implementation. The two earlier blocked disposable paths likewise produced no canonical C15. This review does not convert any negative patch, blocked proposal, or noncanonical list into authority.

## Required correction before re-review

A successor immutable RED must route W23’s typed `PreEffectRefusal` through the complete attachment sequence and `Manager.RoundTrip`, then independently assert:

1. the exact `identity -> enabled -> causal outcome -> predecessors` setter order;
2. refusal before pending-key, writer, transport, admission, reservation, worker, lease, callback, frame, result, diagnostic, terminal, accounting, and parse effects;
3. exactly one refusal owner through `finishCallHierarchyLayer1GenericRefusal` and the existing publication/registry lifecycle;
4. the exact typed method-unavailable enum and reason;
5. unavailable admitted/terminal/result/frame state;
6. available-known-zero only for independently untouched effect dimensions;
7. no admitted `finishRoundTrip` finalization;
8. open acquisition when publication succeeds, or completed outside-lock terminalization/join and fail-closed acquisition when publication is refused by closure/retirement; and
9. no source, corpus, expected-output, or harness-local shortcut that can satisfy W23 without exercising production.

That correction requires a new review input and fresh independent review. This review does not authorize editing the current immutable RED, source, corpus, prototype, or C15 artifacts.

## Final result

The three-file boundary and closed candidate accounting are mechanically coherent, and the proposed typed W04/W05 and lifecycle rules contain no inventory expansion. Nevertheless, W23 is currently intercepted by test code before predecessors and before `RoundTrip`, so the central pre-effect refusal/publication claim is not causally tested. The requested GO cannot be issued.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_DISPOSABLE_C15_WRITESET_FINAL_V4_BLOCKED
reason: W23 is a harness-local pre-effect return before predecessors and Manager.RoundTrip; existing refusal publication, registry, lifecycle, and acquisition are not exercised
canonical_C15: NONE
candidate_classification: ALL UNKNOWN
negative_patch_authority: NONE
disposable_authority: NOT ISSUED
production_authority: NOT ISSUED
```
