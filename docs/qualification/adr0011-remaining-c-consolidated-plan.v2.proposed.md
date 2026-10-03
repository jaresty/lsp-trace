# ADR0011 remaining C — consolidated execution plan v2 (PROPOSED)

## 1. Identity, current status and authority

This is a full successor plan, not an amendment of accepted bytes. Predecessor: `adr0011-remaining-c-consolidated-plan.proposed.md`, SHA-256 `b71e2e1947d2b33cc1fac5b37cd38176b80486b1da44a9367c11a63a30fd8b9e`. Exact new plan hash and all input hashes are in `.pi/evidence/adr0011-p1-decision-preparation-v2/MANIFEST.json`; no self-hash is embedded here.

**P0 ACCEPTED/CLOSED** at `fdff9da474033afa23fb13b0c1b72bbf6c29a936`: tracked-only portable fixtures and inert historical RED source are accepted. Final P0 review remains accepted, as confirmed by the parent handoff. Existing replay manifest: `.pi/evidence/adr0011-historical-red-replay-v1/SHA256SUMS.json`, SHA-256 `1a506915ed2f0fa2260156e7501d2a307fe07b43449cb37e7116648993d5d9e6` (11,249 bytes; hash rechecked read-only). Its limitation is **new replay against selected historical revision, not retroactive proof of original execution bytes**. No replay is run for this plan. Original plan observations about untracked tests and pending portability are historical observation-point facts, not present P0 blockers.

**P1 conditionally authorized subject to its gates; current work is preparation only, PROPOSED/UNFROZEN.** v1 received REVISE; separate successor mode is provisionally sound, not accepted. No runtime/testing/fixture freeze follows from this document. P2/P3, D, occurrence admission, indexing/path implementation and public enablement are not authorized. All evidence and derivatives retain authority=0, accepted=false, completeness=UNKNOWN and no producer authentication.

Historical accepted snapshot `98ec013b07f7f9e2931a155ba28e4cfa24b29ad1b86313b732c40c4505792431` accepted only bounded successful complete-frame cumulative behavior. Preserve wire.go `8f840e774562a942a11f1c49629805dc88fe3c3195a54be304f78a7f5aaf0822` and sessionruntime.go `9feeec2952a808055ebad5a8f8ba44ac44b7388b8db41ac6fbec37d28b568611`; historical successor test identity and post-P0 relocated identity are distinct. No retroactive snapshot rewrite.

## 2. Profile reconciliation and immutable dependencies

Basis: ADR0011:56 and the integration plan:5 make exact framed JSON-RPC capture and its 24-hour replay window **optional private diagnostics**, not universal occurrence-admission/Leiden prerequisites. Required canonical validated query, keyed manager transaction, exact available bounded payload and independently verified terminal ledger remain mandatory for their evidence-dependent claims. A digest alone cannot reconstruct ordinals. Privacy default-off is not replay availability; unavailable dependencies yield INCOMPLETE, deliberate privacy prohibition yields WITHHELD.

Historical GENERIC_LSP_REFERENCES/DEFINITION_EXACT_V5 remains unchanged: S/T/R/Df/L, A4/B4/C/D, complete capability/initialized originals, 64MiB mandatory retained inventory and owner-only 24-hour access still apply when claiming **that profile**. A successor payload-oriented integration profile needs separate accepted identity and a row-complete mapping; it cannot claim full V5 conformity after omitting historical framed retention. The P1 packet PROFILE-MAPPING.md maps each relevant obligation as historical, proposed enforcement, optional diagnostic, independent gate or unresolved. No immutable contract or accepted plan is weakened in place.

Selected T limits remain the proposed successor resource basis below. Older finite-limits proposals do not supply alternate buffer/work/retention numbers. Built-in local references policy (16 acquisition queries, its own result/object/publication limits), local privacy quotas and V5 limits are distinct profiles, not interchangeable defaults. Exact issuance-barrier implementation acceptance is snapshot-bound and does not qualify references or definitions.

## 3. Coverage inventory and ownership

Basis: predecessor PLAN §3, exact T/R/Df/M and retained/current managed source in the P1 packet. Implementation/source observations are bounded; no new execution is claimed. Shape is A4, capability/identity semantics B4, resource enforcement C, selected retention/readback separate. One checkpoint cannot discharge another.

| IDs | Selected obligations | Bounded status / remaining owner |
|---|---|---|
| C01/C02 | frame and capability frame2,097,152 bytes, header+separator+body | Accepted private-definition complete-frame guard; all-role/capability acquisition coverage open |
| C03 | partial header65,536 | Incremental newline-free/allocation-safe enforcement open, P1 |
| C04 | consumed cumulative8,388,608 | Accepted successful complete-frame V only; separate failed-prefix and attempt-wide C open, P1 |
| C05/C06 | inbound64; capability history16,387 | In-attempt all-role count and separate prior-history scope; exact counting/history allocation open, P2 with P1 ownership prerequisites |
| C07/C08 | request15,000ms; attempt60,000ms | One epoch/no resets; clock equality/interruptibility G4, P2 |
| C09/C10 | per-message remarshal: definition1,048,576; references1,572,864 | Current aggregate is not this unit; both methods require P2 witnesses |
| C11/C12 | exact raw result token524,288 each method | Token slicing/whitespace/custody and references path G6, P2 |
| C13/C14 | source256 docs;4,194,304 B/doc | Independent acquisition/copy accounting P3; legacy1MiB source bound preserved |
| C15 | logical owned33,554,432 | P1 ingress reservation is partial contribution; full aliases/copies/transfers/releases P3 |
| C16/C17 | objects4096; events8192 | Define units and charge before materialization/emission, P3 |
| C18 | ordered reached-stage failure precedence | P1 ingress/lifetime collisions now; later stage details gate P2/P3/retention only |

Six required work counters remain separately defined and measured: bytes_scanned, messages_processed, objects_materialized, events_emitted, documents_acquired, logical_buffer_bytes_reserved. No numeric aggregate work cap is selected. Outstanding buffers decrease on release; cumulative reservation work does not. Overflow fails before allocation.

Preserve independent ID bounds (raw4096/decoded1024/exponent100000, same-kind exact decimal), closed schemas/role denominators and chronology. Existing private four-slot/16MiB reservation, source1MiB and parser1000-item bounds are not selected-C equality evidence and must not be silently widened. Full V5 D retains67,108,864 mandatory bytes, owner elapsed<86,400,000ms/cross-process denial, optional536,870,912 executable snapshot and fresh independent readback. For successor integration these are profile-specific, never universal gates; required payload/issuance custody remains separately mandatory.

## 4. Decisions and managed gates

P1 packet DECISIONS.md contains exact unapproved tables, operation enumeration, lifetime transitions, history accounting, allocation safety and error arbitration; CASES.md contains corrected arithmetic/suppression specifications. It is the single decision packet, not a second implementation track.

- **G1:** one manager owner per declared attempt with supporting operations and target; per-request completion does not seal. Explicit terminal success/failure/cancel/retirement seals once, rejects later reads and releases or explicitly transfers every allocation after joins. Prior history has separate fixed-cut ownership/bounds; all consumed in-attempt frames count regardless of role.
- **G2:** distinguish acquired A, consumed C, validated V, normal capture K and owned Q. C may reach cap+one recorded sentinel only; no further normal capture/decode/admission/read after crossing. Acquisition/prefetch allowances are independent and numerically unresolved. Complete required body delivered with final n>0+EOF is not short-body failure.
- **G3:** **P1 prerequisite is only ingress/lifetime collision decisions and relevant error mappings**, including cancellation collisions and early safety reservations. Keep global stage precedence unchanged. Detailed P2/P3/source/object/event/retention rows remain unresolved and block their dependents, not P1 document preparation.
- **G4:** monotonic clock custody, equality/completion versus new blocking operations, earlier caller deadline, interruption and controlled schedules. No sleep-only proof.
- **G5:** reserve actual capacity before copy/growth; identify all backing owners, aliases, transfers, release and overflow. P1 covers ingress allocations only; P3 closes the rest.
- **G6:** separately identified references private ingress and exact raw-token span contract; no hidden reuse based on names/imports.

Managed evidence is required before later cross-file implementation. Current parent-acquired outgoing projections of RoundTripPrivateB4 and terminate are retained with exact request IDs, source hashes and omission counts. They show per-request reservation/copies, protocol/lifecycle exclusion, successful-teardown lease retirement and failed-teardown poisoning. They do not close constructor consumers, finishRoundTrip outgoing or consume/release incoming ownership. v1 and current instance differ despite equal session key/generation; preserve their provenance separately. Text is not server-reported CALLS; missing relationships remain unknown.

## 5. Sequential execution packages (not authorization)

**P0 — closed portability baseline.** Keep accepted tracked originals, relocation identity, inert historical source and reproduction instructions unchanged. Recheck exact bindings when later execution is authorized; do not rerun historical replay in P1 preparation. New revision-based replay never retroactively certifies original logs.

**P1 — decision approval then bounded ingress.** Before behavior changes: approve profile mapping, G1/G2, P1 subset of G3 and relevant G5; select independent acquisition/history bounds; close managed ownership gates. Candidate runtime ownership stays `internal/lspwire/{wire.go,read_frame.go}`, `sessionruntime/{sessionruntime.go,b4_definition_private.go}` plus narrowly authorized private observation/focused tests. Other projected helpers are read-only dependencies; stop if ownership scope is insufficient. Preserve frozen adapter and ordinary/per-frame/accepted successful-ledger behavior. Later freeze same-byte cases and expected counters before any run; use one writer and coherent packet review. No fixtures/tests are created now.

P1 counterexamples cover newline-free/header separator at/+1, exact frame at/+1, accepted six-frame cumulative originals, partial n+error, complete n+EOF, malformed collisions, independent prefetch, two requests/one owner, all-role history charging, explicit sealing, cancellation, STOP/RESTART, abandoned lease and reserve-before-copy. Zero capture entries require deferred capture, not cleanup; per-frame K and attempt totals are distinct. Setup failures are BLOCKED_NOT_RED, already-GREEN is bounded evidence, and a runtime delta—not changed assertions—must cause future GREEN.

**P2 — messages/time/decoded/result.** Requires accepted P1 and applicable G4/G6 decisions. Freeze64/65 inbound including terminal at64;16,387/16,388 history; controlled15,000/15,001 and60,000/60,001 schedules plus equality; both methods' remarshal/raw token boundaries. Earlier lower guards require honest reachability calculations and valid isolated+integrated witnesses, not relaxed historical limits. Review both methods in one coherent packet.

**P3 — source/ownership/objects/events/work.** Requires accepted P1/P2 and G5. Freeze256/257 docs,4MiB/+1,32MiB/+1,4096/4097 objects,8192/8193 events; aliases/copies/transfers/all terminal releases and small exact work-counter traces. No partial candidates/events/selectors survive failure. Full C resource closure cannot be claimed from the P1 allocator contribution.

**P4 — selected C closure and separate retention handoff.** Verify every selected C/work row, exact source/test/input identities, portable checkout evidence, same-byte causal witnesses, unknown-rule/zero-test rejection and compatibility together. Scope must name successor profile, not full V5 if D/framed clauses are unfulfilled. Independent consolidated acceptance required. Handoff lists remaining A4/B4/profile/retention/admission obligations; no D/admission implementation follows automatically.

## 6. Integration gates beyond C

Basis: ADR0011 and INTEGRATION:7–29. This ordering is a plan, not current work authorization:

1. Accept exact successor method transaction/payload/privacy lifecycle; implement/verify selected C and relevant method custody.
2. **Separately reviewed ACCEPTANCE of COMMON NORMALIZED OCCURRENCE MODEL.** Preserve typed orientation, exact original ordinal/multiplicity, selectors, source/revision custody and authority; equal endpoints do not deduplicate evidence.
3. **Separately reviewed ACCEPTANCE of HISTORICAL CALLS COMPATIBILITY ADAPTER.** Project accepted CALLS only; retain original selector/caller/callee/call-site/ordinal; no historical artifact rewrite/reissue/migration.
4. Complete acquisition/publication/occurrence admission and qualification **independently for references and definitions**. References qualification need not await definitions. Candidate publication, optional-frame D success and C success confer no admission; preserve `ErrMethodEvidenceUnadmitted` until the relevant successor gate passes.
5. Only then **COMMON INDEXING** of eligible admitted families, with independently approved construction/node/occurrence/memory/work budgets. Forward/reverse adjacency preserve original typed occurrences, not inferred inverse LSP results.
6. One bounded **private admitted-path example** is an intermediate milestone, not the end goal. Explicit kind/depth/frontier/work/cancel limits; no hidden acquisition/hydration/rebuild, no partial accepted witness. `retainedpath.Search` currently rebuilds and is CALLS-shaped; no automatic reuse qualification.
7. Continue toward **search → inspect evidence paths → hydrate relevant bodies → verify external links** under separate consumer gates. Search text cannot create semantic edges or establish external/runtime identity. Grouping still needs frozen policies, independent held-out acceptance, CUE provider/corpus, no-CALLS and mixed delivery, and NO_CALLS_REPRESENTATIVE behavior; no thresholds chosen here.

**Independent assessment lane:** assess/exercise existing search, FULL_DEFINITION projection and retained hydration before proposing replacements. This lane does **not** wait for C completion or new relation/value-flow implementation. Distinguish advertised capability, bounded observed output, custody, truncation and genuine gap; preserve existing integration plan's no-implementation boundaries. This document does not execute that lane.

## 7. Exit, open choices and change log

P1 preparation exits with a coherent reviewable unfrozen packet, not an approval. Runtime entry remains blocked by explicit decision choices, bounded acquisition/history values and managed ownership closure. Later unknowns block only their dependent packages. Required payload replay/privacy, method qualification, common model/adapter, index/path and grouping gates remain independent.

Changes from predecessor: current P0 accepted/revision-based status; historical untracked/portability observations relabeled; optional-frame profile reconciliation with immutable V5 preservation; attempt rather than request sealing; independent acquisition and history accounting; exact EOF/capture/reservation corrections; narrowed G3 prerequisite; explicit common-model and CALLS-adapter acceptance before indexing; separate relation qualification; admitted example as intermediate workflow milestone; independent search/hydration assessment lane. No runtime or historical bytes changed.

## Model interpretation

This successor is an unaccepted plan with exact predecessor binding. Verification priorities are profile-specific claims, missing ownership evidence, and separately frozen counterexamples. Better framing: preserve accepted increments while closing only the next package's decisions, without turning diagnostic retention or later feature work into universal gates.
