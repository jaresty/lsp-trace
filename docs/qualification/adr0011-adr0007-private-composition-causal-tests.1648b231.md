# ADR 0011 → ADR 0007 private composition causal test specification

- **Schema:** `lsp-trace.adr0011-adr0007-private-composition-causal-tests.v1`
- **Contract:** `lsp-trace.adr0011-adr0007-private-composition-contract.v1`
- **Status:** frozen candidate for independent qualification
- **Test package:** `internal/adr0011adr0007composition`
- **Public wiring:** forbidden

Every test starts RED because the authorized private package does not yet exist. Tests must use real parsers and immutable artifacts from existing packages where accessible; otherwise a test-only builder may produce structurally valid exact carriers. A test builder cannot bypass production admission, mint production qualification, weaken trust custody, or become production code. Assertions include typed failure, zero effects, exact accounting, defensive copies, canonical bytes, and unchanged claim ceilings.

## A. Core admission and complete path

### C01 — CALLS-only complete composition

Given qualified stable complete CALLS Stage 7 predecessors, exact source contexts, an eligible representative, exact packet/replay custody, and deterministic private stage fixtures, composition reaches `CANDIDATE_COMMITTED`. Assert exact packet, stage, checkpoint, generation, and candidate identities; byte-for-byte canonical round trip; balanced accounting; source citations; and unchanged ceilings.

### C02 — exhaustive predecessor substitution matrix

For every required carrier role, replace exactly one valid carrier with a valid carrier from another generation while preserving all other fields. Also swap role order and duplicate one role. Each case returns `PREDECESSOR_SUBSTITUTED` or `IDENTITY_CONFLICT`, zero stage evaluations, zero checkpoints, zero candidates, and zero publication effects.

### C03 — source-bearing packet identity and custody

Mutate independently workspace/repository/revision/session/generation/position encoding; document version/revision/digest/length/bytes; occurrence/relation/community/target; every range class; representative/disclosure; coupling/instability; privacy/resource policy. Reseal each mutation. Admission rejects it. Equivalent input under permutations that are declared nonsemantic produces identical packet bytes.

## B. Required modes and outcomes

### C04 — D/R-only

Use complete REFERENCES_SYMBOL and RESOLVES_TO_DEFINITION evidence without CALLS. Assert no representative is manufactured, every community terminates `NO_CALLS_REPRESENTATIVE`, D/R evidence remains disclosed, publication is blocked without independent family/composition qualification, and accounting is complete.

### C05 — mixed

Use CALLS plus D/R evidence. Assert representative selection is identical to the CALLS-only projection, D/R cannot influence it, per-family coupling remains distinct, combined coupling is consumed only with valid independent qualification/composition/readiness custody, and any unavailable combined policy blocks candidate commit.

### C06 — zero-result

Use a fully terminal admitted request whose deterministic stages produce no candidates. Assert a complete canonical empty generation distinct from failure/incomplete, zero candidate count, balanced nonzero stage denominators where applicable, idempotent reuse, and no invented candidate.

### C07 — no eligible representative

Use a community with admitted evidence but no eligible CALLS representative. Assert `NO_CALLS_REPRESENTATIVE`, no Describe request for that community, explicit omission/candidate accounting, retained membership and source disclosures, and no D/R promotion.

### C08 — incomplete source context

For each role outcome `UNAVAILABLE`, `AMBIGUOUS`, `WITHHELD`, and `LIMIT_OMITTED`, assert exact source-bearing disclosure, no reacquisition, no context-complete Describe claim, `SOURCE_CONTEXT_INCOMPLETE` or policy-defined terminal omission, balanced accounting, and publication blocking when mandatory.

### C09 — unstable grouping

Supply actual successor result bytes whose exact pair comparison crosses the retained threshold or has failed/incomplete/missing runs. Assert `GROUPING_UNSTABLE`, exact run/pair accounting, no downstream stage evaluation, and no candidate/checkpoint beyond Stage 7 verification.

### C10 — unavailable combined policy

Exercise absent qualification, absent composition, absent readiness, stale epoch, wrong policy/input commitment, and unqualified family inventory. Assert `COMBINED_POLICY_UNAVAILABLE`, `QUALIFICATION_MISSING`, or `QUALIFICATION_STALE` as applicable; no fallback to CALLS or silent combined value.

### C11 — missing/stale qualification

Delete or mutate each qualification receipt, evidence record, trust anchor, reviewer commitment, validity epoch, readiness receipt, and policy-composition link. Assert atomic refusal before downstream execution and no production self-attestation path.

## C. Canonical integrity

### C12 — resealed semantic forgery

For request, packet, each stage result, checkpoint, replay request/result, and generation, mutate every semantic field class and recompute outer digest. Include algorithm, enum, scope, family inventory, availability, omissions, source bytes/ranges, partitions, accounting, outcomes, claims, limits, predecessor links, and candidate membership. Parsing or admission rejects every mutation and returns a zero object/effects.

### C13 — strict canonical parser matrix

Reject duplicate and unknown keys, trailing values, missing/extra LF, CRLF, noncanonical ordering/encoding where required, invalid UTF-8, integer overflow, invalid rational, zero untyped denominator, repeated identities, and oversized nested bytes. At-limit succeeds; limit-plus-one fails before allocation/effect.

### C14 — defensive-copy and aliasing isolation

Mutate all constructor inputs, returned byte slices, returned arrays/maps, checkpoint collections, and parsed results. Canonical bytes and identities remain unchanged under normal and race execution.

## D. Replay, idempotency, resume, concurrency

### C15 — canonical reconstruction

From only retained exact carriers and replay limits, reconstruct source packet, all stage requests/results, checkpoints, accounting, and generation without workspace/network/clock callbacks. Assert every reconstructed byte equals the original. Any mismatch returns exactly one `SEMANTIC_REPLAY_MISMATCH` and no partial object.

### C16 — complete-generation idempotent reuse

Call composition repeatedly with an equivalent request. Verify one generation identity and one canonical byte sequence; later calls verify and return it with zero new stage evaluations, checkpoints, candidates, ownership effects, or publication effects.

### C17 — interruption and exact-stage resume

Interrupt after each commit-eligible stage. Resume from its checkpoint and assert execution starts only at `next_stage`; completed stages are not called. Test no checkpoint before stage completion. Mutate, omit, reorder, skip, or splice each checkpoint and assert `CHECKPOINT_CONFLICT` with zero new effects.

### C18 — concurrent equivalent requests

Start many equivalent requests under the race detector with controlled barriers at ownership, stage completion, and commit. Assert one owner/evaluation per stage, exactly one complete generation, byte-identical results for all successful callers, deterministic candidate ordering, balanced ownership accounting, and no arrival-order identity effect.

### C19 — concurrent conflicts and owner loss

Race same claimed identity with different bytes/policy/receipt/checkpoint, and cancel the owner before and after a committed checkpoint. Assert `CONCURRENT_CONFLICT` for conflicting requests, no partial-stage visibility, and takeover only from the exact next stage after the last valid checkpoint.

## E. Accounting, ceilings, compatibility, isolation

### C20 — failure-before-publication matrix

Inject every typed failure at every stage before and after computation but before checkpoint commit. Assert no later stage, candidate generation, or externally visible publication; atomic state transition to `REFUSED`; exact expected/evaluated/terminal/missing/failed/omitted accounting; and checked overflow behavior.

### C21 — privacy and resource policies

Exercise every independent document/range/object/source-byte/request/output/work/canonical/aggregate limit and every privacy withholding rule. At-limit succeeds; plus-one fails or terminates explicitly. Withheld bytes never appear in packets, diagnostics, failures, checkpoints, replay, or candidate output.

### C22 — unchanged claim ceilings

For every success, empty, omission, refusal, resumed, replayed, reused, and concurrent result, assert exactly:

```text
authority=0
accepted=false
completeness=UNKNOWN
featureIdentity=UNRESOLVED
```

Qualification cannot raise these values. Forged higher claims are rejected even when resealed.

### C23 — historical compatibility

Run historical Stage 3–7, target-packet, checkpoint, replay, Describe/catalog, CALLS coupling/Ca/Ce/instability, parser, selector, default, and exit-behavior fixtures. Assert byte and reader compatibility. New composition parsers must not accept historical bytes under a new schema without explicit wrapping; old readers remain unchanged.

### C24 — public isolation and design-context boundary

Mechanically assert the implementation diff is confined to the exact contract write-set. Assert no registration/import/reference from `cmd/**`, `internal/mcp/**`, public schema registries, current census/catalog owners, help/completion/skills/capability advertisement, or ADR 0007 public operation wiring. Assert commit `4f5743ff` and string `lsp-trace census --catalog` appear only in qualification/design documentation, never implementation dispatch.

## F. Deterministic ADR 0007 stage vectors

### C25 — Describe vector

Freeze exact source-bearing packet, policy, backend/profile, prompt/instruction, response, citations, and outcomes. Prove request/result identities, abstention behavior, preparation failures, and source-support accounting. No model call is required for the causal contract test; immutable captured result bytes are used.

### C26 — Search vector

Freeze corpus/query/policy bytes and exact member outcomes. Prove every member is terminal, scoring uses exact representation, order is canonical, omissions remain explicit, and no mutable index or ambient corpus is consulted.

### C27 — provisional Group vector

Freeze Search result and grouping policy. Prove label-independent deterministic memberships, grouped/unmatched accounting, canonical order, and labels excluded from feature identity authority.

### C28 — Location vector

Freeze exact source-location custody. Prove deterministic carriers and unavailable/ambiguous/withheld/limit outcomes; no source selection or relation repair.

### C29 — Source Text Search vector

Freeze document bytes, pattern/query policy, limits, and match ordinals/ranges. Prove exact UTF-16/byte custody as applicable, multiplicity, no ambient file access, explicit omissions, and inability to repair earlier context.

### C30 — candidate identity and complete chain

Mutate each constituent, predecessor identity, stage policy, source/location/text-search evidence, disclosure, omission, accounting, claim, privacy, resource, or contract identity. Each mutation changes candidate/generation identity or is rejected. Reordering nonsemantic sets does not change identity; changing semantic ordinals does.

## Qualification matrix

Each required authorization case maps as follows:

| Required case | Causal tests |
|---|---|
| CALLS-only | C01, C25–C30 |
| D/R-only | C04 |
| mixed | C05 |
| zero-result | C06 |
| no eligible representative | C07 |
| incomplete source context | C08 |
| unstable grouping | C09 |
| unavailable combined policy | C10 |
| missing/stale qualification | C11 |
| predecessor substitution | C02, C03 |
| resealed semantic forgery | C12, C13 |
| interruption/resume | C17 |
| concurrent equivalent requests | C18, C19 |
| historical compatibility | C23 |

Required behaviors map as follows:

| Behavior | Causal tests |
|---|---|
| exact Stage 7 admission/substitution rejection | C02, C03, C12 |
| source-bearing packet identity | C03, C08 |
| missing/incomplete/unstable/unavailable outcomes | C08–C11, C20 |
| representative/NO_CALLS behavior | C04, C05, C07 |
| deterministic ADR 0007 inputs | C25–C29 |
| candidate identity/predecessor chain | C30 |
| canonical replay/reconstruction | C15 |
| idempotent complete reuse | C16 |
| exact-stage resume | C17 |
| concurrent equivalent arbitration | C18, C19 |
| failure-before-publication/accounting | C20, C21 |
| unchanged ceilings | C22 |

## Required implementation qualification sequence

```text
causal RED for C01–C30 with package absent
→ independent RED review
→ one private production writer constrained to exact write-set
→ focused normal tests
→ race tests for replay/store/checkpoint/arbitration
→ Stage 7 compatibility and historical fixtures
→ ADR 0007 Describe/catalog compatibility
→ public-isolation diff gate
→ independent whole-diff review
→ standalone implementation commit
→ post-commit closure
```

Minimum commands after implementation:

```bash
go test ./internal/adr0011adr0007composition -run 'TestCompositionCausal' -count=1
go test -race ./internal/adr0011adr0007composition -count=1
go test -race ./internal/programc ./internal/programcrepresentative ./internal/adr0011adr0007composition -count=1
go test ./internal/programc ./internal/programcrepresentative ./internal/programcadmission ./internal/referencegrouping ./internal/graphkernel ./internal/targetpacket ./internal/censuscontinuation ./internal/retainedreplay ./internal/describerequest ./internal/describeworker ./internal/provisionalfeaturecatalog -count=1
go vet ./internal/adr0011adr0007composition ./internal/programc ./internal/programcrepresentative
git diff --check
```

The isolation gate must prove every changed implementation/test path matches the exact write-set and that no public surface changed. Repository-wide baseline failures remain separately disclosed and cannot be converted into a false repository-wide GREEN claim.
