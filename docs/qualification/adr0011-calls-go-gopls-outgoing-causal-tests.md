# ADR 0011 Go/gopls outgoing CALLS causal-test specification

- **Exact profile:** `CALLS_GO_GOPLS_OUTGOING_EXACT_V1`
- **State:** planned and unexecuted
- **Plan-review gate:** `ADR0011_CALLS_OUTGOING_PLAN_GO` (not issued)
- **Profile GO:** not defined or issued
- **Production admission:** `REFUSED_MISSING_EVIDENCE`

These persistent semantic contracts are plans, not tests run or evidence produced. Passing future tests cannot itself create evidence, review, authority, admission, or GO.

## Transaction and caller selection

### O01 — distinct prepare request/result custody
Retain prepare request and result as separate records with independent identity, bytes/bounded capture, ordinals, limits, terminal outcomes, and completeness. Removing either, deriving one from the other, or combining them refuses before outgoing dispatch.

### O02 — deterministic prepared caller selection
Exercise one and multiple prepared items. Bind documentSymbol-assisted candidates by URI/name/kind/range/selectionRange and direct-position candidates by URI plus position containment in range and selectionRange. Test valid explicit ordinal, out-of-range ordinal, nonmatching ordinal, unique implicit match, no match, and ambiguous matches. Only a valid target-bound selection dispatches outgoingCalls.

### O03 — selected caller passed unchanged
Mutate name, kind, URI, range, selectionRange, detail, tags, or opaque data between prepare result and outgoing request. Expect substitution refusal and zero dispatch/effects. Lossy remarshal cannot prove equality.

### O04 — optional documentSymbol firewall
Retain documentSymbol request/result and ordering separately when used. A documentSymbol-only item, changed selected symbol, hidden failed prepare probe, or documentSymbol response shaped as a call item admits zero CALLS.

## Provider and workspace custody

### O05 — initialization and capabilities
Vary initialize request/result, client call-hierarchy capability, server `callHierarchyProvider`, position encoding, options, and terminal initialization state independently. Missing, stale, foreign, unsupported, or substituted custody refuses.

### O06 — exact gopls/toolchain/environment
Vary gopls bytes/digest/length/version/argv/workdir/environment and Go toolchain/version/`go env`/flags/tags/module/workspace bytes one field at a time. Equal version with unequal bytes and equal bytes under changed behavior inputs cannot silently retain identity.

### O07 — repository, file, session, generation
Vary commit, dirty-state proof, workspace root, target bytes/digest/length/URI/language, session, generation, and encoding. Stale, foreign, or substituted custody admits zero occurrences.

### O08 — shared artifact references do not inherit authority
Reference a gopls/toolchain/env/repo/workspace artifact also named by incoming evidence. Require independent verification of immutable identity, relevance, freshness, and outgoing binding. Remove one check or substitute the incoming receipt/review/GO; outgoing role remains missing.

## Outgoing direction and occurrence semantics

### O09 — caller→callee method orientation
For queried prepared caller `A` and returned `to=B`, require method and canonical orientation `A -> B`. Reverse to `B -> A`, relabel an incoming `from` item as `to`, or infer direction from source text; expect refusal.

### O10 — returned callee identity
Retain every field of returned `to`, including URI, range, selectionRange, kind, optional detail/tags/data, and member ordinal. Mutate each independently; expect changed identity or refusal, never endpoint repair.

### O11 — fromRanges belong to queried caller
Use caller `A` and callee `B` in different documents. Assert each `fromRanges` value resolves in `A`'s document/encoding and is retained as the caller-side call site. Interpreting it in `B`, moving it to the callee, or fabricating a callee range refuses.

### O12 — one occurrence per range and zero-site member
Use one member with two ranges: require two occurrences. Use `fromRanges: []`: retain one zero-site response member and admit zero occurrences. Missing/null/scalar/object `fromRanges` is malformed, not empty.

### O13 — duplicate ranges and repeated members
Return duplicate equal ranges and duplicate equal members at distinct ordinals. Require every occurrence and original member/range ordinal to survive. Canonical ordering cannot collapse multiplicity.

### O14 — parallel edges and self-loops
Return parallel `A -> B` members and `A -> A`. Require explicit occurrences. Any later pair union, deduplication, or self-loop removal is a separately identified transform and is outside this profile.

### O15 — ordinal preservation under reorder
Permute prepare items, outgoing response members, and range arrays. Require changed source ordinals and identities under the frozen policy; a producer that sorts before retaining original order fails.

## Terminal and refusal semantics

### O16 — prepare terminal matrix
Exercise missing, null, empty, one/many items, no/ambiguous match, invalid ordinal, malformed first/later item, truncated capture, reorder, substitution, stale/foreign input, server error, cancellation, timeout, unsupported, and ceiling refusal. Every nonqualified outcome has zero outgoing dispatch and effects.

### O17 — successful top-level null is successful-empty
Exercise successful protocol results `[]` and `null`, missing payload, transport/protocol error, malformed top level, and partial/truncated response as separate causal vectors. A terminally successful, complete `callHierarchy/outgoingCalls` response whose present result token is exactly JSON `null` is successful-empty, exactly matching acquisition semantics: count one begun and successful expansion, retain the null result bytes/token and response ordinal, record zero returned members, zero ranges, zero occurrences, zero rejected members/ranges, and zero production-visible effects. `[]` has the same canonical empty CALLS representation and accounting but retains distinct source bytes/token identity; canonical equivalence never aliases custody. Missing payload is evidence-missing; transport/protocol error is failed; malformed and partial/truncated responses are atomic typed refusals. None may normalize to successful-empty.

### O18 — malformed and partial are atomic refusal
Exercise malformed top level; malformed first/later member; invalid `to`; invalid URI/range/kind/selectionRange; missing/null/scalar/object ranges; trailing JSON; and truncated capture after a valid prefix. Admit zero qualified occurrences and do not salvage the prefix. A complete successful top-level `null` is the sole null success covered by O17; member-level nulls and incomplete captures remain malformed/refused.

### O19 — reordered, substituted, stale, and foreign
Substitute profile/domain/provider, binary/toolchain/env, root/revision/file, capability, session/generation, caller item, request/result, callee, range, ordinal, limit, or admission envelope independently. Expect changed identity or typed refusal with zero effects.

### O20 — no laundering or fallback
Try incoming direction flipping, copied/resealed evidence, digest-only payload claims, synthetic fixtures, self-review, runtime flags/config/build tags, family aliases, and plan-GO-shaped strings. Missing roles remain missing; no fallback or production effect occurs.

## Accounting, ceilings, replay, and compatibility

### O21 — finite independent outgoing ceilings
For each initialization/documentSymbol/prepare/outgoing expansion, message, byte, request, node, depth, member, range, occurrence, time, work, and allocation bound, freeze exact-limit behavior and plus-one refusal. No bound borrows from another.

### O22 — closed outgoing accounting
Assert declared = begun + unstarted; begun = successful + failed + cancelled + timed_out + unsupported + ceiling_refused; members = with_sites + zero_site + rejected; ranges = admitted + rejected; every occurrence references exactly one retained location. Overflow, underflow, duplicate ordinal, and impossible totals refuse.

### O23 — replay has no reacquisition
Replay retained records with filesystem, process, workspace, clock, and LSP callbacks forbidden. Require the same terminal result and canonical bytes. Missing/reordered/stale/substituted inputs produce one refusal, no retry, and no ambient read.

### O24 — historical and direction isolation
Omitted selector remains exact historical `CALLS_ONLY`. Missing/empty/case-varied/alias/incoming/bidirectional selectors cannot select this profile. Incoming qualification and equal-byte shared artifacts imply no outgoing role or authority.

### O25 — no bidirectional or Stage 3–7 work
Present separately qualified directions or downstream grouping/readiness/source/replay/coupling artifacts. This four-file plan neither validates nor composes them and creates no shared Stage 3–7 receipt.

### O26 — privacy and diagnostics
Withhold selected environment/source/diagnostic values under a future frozen policy. Require prohibited bytes/reversible encodings absent from outputs and diagnostics while custody and accounting remain explicit.

### O27 — plan gate confinement
An independent reviewer may assess only these four files and issue `ADR0011_CALLS_OUTGOING_PLAN_GO` or REVISE/REJECT. The token cannot make any manifest role PRESENT, authorize evidence production or implementation, qualify the profile, activate production, or act as profile GO.

### O28 — future semantic-vector order and oracle independence
Assert the only permitted future order is frozen evidence, independently authored expected vectors, independent exact-byte vector review, handwritten checks, pinned same-adapter semantic-vector execution, exact comparison, then bounded validation receipt. Reorder any pair, derive expectations from evidence/tool output, or omit oracle provenance; expect refusal.

### O29 — same-adapter controlled contrast
For a canonical world and one-variable mutations, execute baseline and mutations through the same pinned adapter/invocation. Compare exact output, failure, effects, accounting, and classification. Changed adapter identity, unequal invocation, uncontrolled mutation, or exceeded adapter ceiling blocks the receipt.

### O30 — disagreement blocks and tool authority is bounded
Force disagreement between expected bytes, handwritten checks, semantic-vector output, classification, or effects. Require blocked validation and qualification. semantic-vector may report canonical worlds, controlled mutations, same-adapter contrasts, exact comparisons, oracle provenance, and its finite ceiling; it cannot establish gopls evidence correctness, outgoing normalization, qualification policy, authority, admission, review, or GO.

## Required handoff

```text
freeze four author files
→ independent four-file plan review
→ ADR0011_CALLS_OUTGOING_PLAN_GO or REVISE/REJECT
→ stop
```

Any next phase requires new explicit authorization. Current result is **NO GO**; no test has run and no evidence role is PRESENT.