# ADR0011 remaining C — consolidated execution plan (proposed)

**PLAN ONLY. Not implementation approval, C completion, D acceptance, occurrence admission, qualification, or public wiring.** This document is the only authorized write for this planning task. No tests were executed. All future changes and verification described below require subsequent authorization.

## 1. Evidence boundary and preserved baseline

**Basis:** the user accepted the bounded cumulative snapshot, not full C. Its manifest was independently hashed in this planning session and matched:

- Snapshot: `.pi/evidence/adr0011-c-cumulative-accepted-snapshot-v1/`
- `manifest.json` SHA-256: `98ec013b07f7f9e2931a155ba28e4cfa24b29ad1b86313b732c40c4505792431`
- Acceptance label: `ACCEPT_BOUNDED_CUMULATIVE_COMPLETE_FRAME_INCREMENT`
- Baseline HEAD recorded by the snapshot: `79b4edc972846e72790973d6ed48ca17a115bbf7`
- `SCOPE.txt:1–10`: successful complete-frame ledger only; excludes partial/failed/malformed precedence, full C, D, public acceptance, and portability.
- `verification.log:250–261,441` records five accepted pins, complete copied evidence trees and twelve frames; its concluding PASS is **retained mechanical verification**, not a new execution or producer authentication. The planning session read that bounded verification and recomputed the five current source pins below; it did not rerun the whole preservation procedure.

| Preserved item | Current SHA-256 rechecked against snapshot | Delta classification |
|---|---|---|
| `internal/lspwire/wire.go` | `8f840e774562a942a11f1c49629805dc88fe3c3195a54be304f78a7f5aaf0822` | Accepted cumulative runtime change on the per-frame baseline |
| `sessionruntime/sessionruntime.go` | `9feeec2952a808055ebad5a8f8ba44ac44b7388b8db41ac6fbec37d28b568611` | Accepted private-path runtime opt-in |
| `internal/adr0011methodresult/c_cumulative_complete_frame_successor_test.go` | `fd5045128d2ae9c59faa22de58c57a055ae3f037ea15ed812aa4ad96f409d586` | Accepted bounded successor verification; currently untracked |
| `internal/lspwire/cumulative_compatibility_test.go` | `0c17c89fdb0b09e7a751fb8afcfb9bb3b9601794ccd1d6370afa58c9881866ca` | Accepted compatibility verification; currently untracked |
| `internal/adr0011methodresult/c_cumulative_complete_frame_test.go` | `e6ef991bd2819325dbf411ddd23e08f3dd49e9b784255ab46ad83be632ff18af` | Immutable historical RED baseline, **not production delta**; currently untracked |

**Basis:** the snapshot separates `runtime.diff` from historical evidence. Preserve that separation in every later review: (a) per-frame predecessor, (b) accepted cumulative runtime delta, (c) successor/compatibility verification, (d) intentionally RED historical source, (e) new portability delta, and (f) each remaining-C behavioral delta. The snapshot's `runtime.diff` digest is `eb7fbdecd0f8e6c198fa17234cdfe487e452675e07a819e3663f11bdb59e1c23`. Never regenerate or rewrite an accepted snapshot to make new results look historical.

**Evidence class:** source bodies below are filesystem observations, not managed projected source or server-reported CALLS. Parent reports exact READY session `project`, generation `1`, key `sk1:949e027243da188035cc5c6532c4cb10df4210f2bdde80acc58b7b7b9af5f283`; this child could not call its MCP adapter. User explicitly permitted filesystem planning. Before implementation, re-list the managed session, verify exact workspace/generation, and resolve the structural gates in §5. No replacement server. No predecessor-agent conclusions or unverified predecessor numeric bounds are adopted.

## 2. Governing contracts, not obsolete candidate defaults

**Basis:** `docs/qualification/adr0011-generic-v5-implementation-only-decision.md:3–9` accepts contract coherence only, identifies the unchanged eleven-path V5 candidate manifest `b3bedbb010d0d46d29fff8f16a6aa2f91182f4bff47f284b38afff7644eaab5d`, and explicitly leaves C/D enforcement open. The current ownership matrix is accepted by that decision despite its historical PROPOSED heading. Its UNIMPLEMENTED cells describe its original observation point; the inventory below updates only what current source and bounded witnesses establish.

The following exact hashes were recomputed in this session:

| Alias | Exact file | SHA-256 |
|---|---|---|
| M | `docs/qualification/adr0011-generic-v5-enforcement-ownership.matrix.proposed.md` | `9c93213315644d30316f8f3893e7d1e9864023b8f38b5bc06ee5ca1d839f21d1` |
| T | `docs/qualification/originals/generic-lsp-exact-transport-v4.json` | `f3e25fab98cc4b8b96b76978cf56220ccc81f4ef41455ab0401041ea281d0bc8` |
| R | `docs/qualification/originals/generic-lsp-references-exact-v5.json` | `7c46893fecf0dda7a4942b8f33d92f0c2f49793d003c7eab4e8995aeddf8da36` |
| Df | `docs/qualification/originals/generic-lsp-definition-exact-v5.json` | `da62f5a161532fcb7c79dd93b0dca940a460c6e0c5fab83e8987de21c91b4dd3` |
| S | `docs/qualification/originals/adr0011-generic-envelope-v4.schema.json` | `df4908187030e3d3110bd3745290eb25d8a3934ffb2a85c5d47d8d22dc28f6f3` |

T/R/Df are single-line exact originals; references below use literal member names rather than fabricated line ranges. M:58–68 enumerates C; M:70–77 enumerates D; M:17 fixes checkpoint ownership. A4 shape and B4 semantics cannot discharge C; C cannot discharge D.

**Conflicting-number rule:** the earlier `adr0011-generic-lsp-finite-limits.packet.proposed.md:13–33` is background, not the selected numeric authority. In particular, do **not** import its 16,777,216 allocation cap, 1,000,000 work ceiling, 16,777,216 executable cap, 33,554,432 retained cap, or inclusive 24-hour access boundary. T instead specifies logical buffers 33,554,432; mandatory retention 67,108,864; optional executable snapshot 536,870,912; owner access strictly less than 86,400,000 ms; and named work counters without a numeric work-total cap. No work threshold or new failure type may be invented from that older packet.

## 3. Authoritative bounded inventory

### Reading the status columns

**Basis:** test names are locators, not coverage. `Implemented` below means a guard is visible at the identified owner, not proven reachability across every path. `Partial` means a different unit, scope or subset. `Missing at inspected seam` does not assert an exhaustive repository-wide negative; elsewhere remains unknown. `Verified bounded` means assertion bodies and retained result logs were inspected, not rerun. `Unknown` means no attributable exact-boundary execution was established here. `Deferred D` is an ownership boundary, never a waiver of downstream admission.

Source abbreviations:

- W = `internal/lspwire/wire.go:134–289` (reader, bounds, decode and successful-byte commit).
- F = `internal/lspwire/read_frame.go:22–119` (capture/hashing and independent returned copies).
- RT = `sessionruntime/sessionruntime.go:749–788,877–1011` (deadline, private limits, inbound loop).
- PB = `sessionruntime/b4_definition_private.go:35–40,66–121,139–176` (definition-only reservation, lease and lifecycle).
- BR = `internal/adr0011methodresult/b4_definition_bridge.go:44–86,135–154` (private synthetic parser/limits).
- PA = `internal/adr0011methodresult/b4_definition_private_adapter.go:28–96` (manager-selected capture comparisons).
- PR = `internal/adr0011methodresult/parser.go:55–136` (whole-result grammar and caller-selected candidate ceiling).
- SD = `sessionruntime/sessionruntime.go:283–285,326–425` (document supply acquisition).
- MT = `internal/adr0011methodtransport/transport.go:21–36,130–170` (older provisional transport).
- PUB = `internal/adr0011methodresult/candidate_retention.go:18–61` (candidate-only publisher/readback).

Witnesses inspected:

- **Vframe:** `internal/adr0011methodresult/c_complete_frame_boundary_test.go:24–47,84–175` pins exact 2,097,152/2,097,153-byte frames; checks at-cap transport/lease/B4 continuation and over-cap RESOURCE_EXHAUSTED with zero decode, raw-retain, B4 and private-publish entries. Retained `evidence/adr0011-c-cumulative-successor-v2/perframe.compatibility.log:1–20` inside the accepted snapshot reports those subcases passing. This is narrow private-definition-path compatibility, not every capability/reference frame.
- **Vcum:** successor test `:20–100` checks six complete deliveries, accepted total 8,388,608, rejected 8,388,609, and sixth-frame entry markers. Retained `successor.red.log:6–16` shows +1 reached decode/retention; `successor.green.log:6–16` shows sixth decode=0, retained=0, messages=5, RESOURCE_EXHAUSTED, no lease/B4/publication. Both logs are in the same snapshot evidence directory; frozen successor hash is the pin in §1. The publication is **private synthetic candidate publication**, not D.
- **Vcompat-source:** `internal/lspwire/cumulative_compatibility_test.go:11–79` asserts disabled-cumulative capture precedes body observer on complete/partial reads, and failed/partial cumulative reads leave the successful-byte ledger at zero. Its source is pinned by §1; it is evidence of the deliberately narrow contract, not a witness satisfying full consumed-byte C. No fresh run is claimed.

### 3.1 C admission/resource obligations (M:62–68)

| ID; exact contract obligation | Implementation status and inspected basis | Verification status / current witness | Remaining execution obligation |
|---|---|---|---|
| C01 T.frame_bytes = **2,097,152**, complete original header + separator + body | **Implemented narrow private-definition opt-in:** W:195–237 checks declared full length before body allocation/decode/raw capture; RT:883–888 enables it only for private reservation | **Verified bounded Vframe**; all-role coverage unknown | Apply/verify C at every selected held frame ingress, not merely selected response; preserve ordinary-reader behavior |
| C02 T.capability_exchange_frame_bytes = **2,097,152**; complete request AND response and initialized notification required | **Partial/missing acquisition binding:** C01 covers inbound frames seen by its reader, not independently supplied B4 capability originals or outgoing history. `id.go:111–127` still permits body plus header overhead | **Unknown** integrated capability/initialized-frame C witness; M:62 identifies obligation | Gate acquisition and exact-frame admission for initialize, initialized, register/unregister, pending/error/unmatched records; no claimant-only admission |
| C03 T.partial_header_bytes = **65,536** | **Partial, not preallocation-safe:** W:183 reads a whole line before W:192 checks; read error returns before header cap check | **Unknown** incremental boundary; visible source gap | Bound newline-free incremental reads; cap/equality/+1/EOF tests, before unbounded allocation; resolve crossing-byte convention |
| C04 T.cumulative_wire_bytes = **8,388,608**, complete and partial consumed original bytes including headers, notifications/unmatched messages (M:64) | **Implemented successful-complete ledger only:** W:248–258 checks after full body acquisition; W:285–287 commits only after valid decode; RT enables per newly created reader | **Verified bounded Vcum**, explicitly NOT partial/failed or transaction-wide history coverage | Add authoritative consumed ledger without reinterpreting accepted successful ledger; failed read, truncation, prefetch and transaction scope gates in §4 |
| C05 T.inbound_messages = **64** per transaction | **Partial:** RT uses caller `MaxMessages`, defaults 1, counts successful decoded reads; PB requires positive but does not enforce 64; MT has provisional 64 | **Unknown** selected C 64/65 execution; Vcum establishes six only | At-cap fixture must include terminal response as message 64; attempted 65th rejects without admission. Decide failed/attempted counting and transaction scope |
| C06 T.capability_exchange_frames_max = **16,387** | **Missing at inspected acquisition seam;** M:64 distinguishes held history from schema shape | **Unknown** 16,387/16,388 causal boundary | Define history ownership versus target 64-message budget; isolate count below byte/event limits; no schema-only closure |
| C07 T.request_ms = **15,000 ms** per request | **Partial:** RT has caller deadline/context cancellation and REQUEST_TIMEOUT, not fixed selected maximum | **Unknown** exact monotonic equality/+1 and pre-block boundary | Selected request budget, controlled clock, completed-at-boundary vs new blocking operation policy gate |
| C08 T.transaction_ms = **60,000 ms** preflight through terminal | **Missing at inspected round-trip seam:** RT start is one request; PB owner label is not a continuous transaction clock | **Unknown** across multiple bounded operations | One transaction lifetime/clock, cancellation/STOP/RESTART behavior, no reset at each request; see G1/G4 |
| C09 Df.decoded_message_remarshal_bytes = **1,048,576**, each inbound message | **Missing selected per-message guard:** RT:934–940 sums marshal lengths into a caller aggregate, including notification/unmatched messages | **Unknown** per-message equality/+1 | Measure complete re-marshaled JSON-RPC message before continuation; unrelated aggregate guards must not shadow witness |
| C10 R.decoded_message_remarshal_bytes = **1,572,864**, each inbound message | **Missing selected references owner:** PB rejects references; RT aggregate and MT 1 MiB provisional cap are not this per-message policy | **Unknown** integrated references equality/+1 | Separately authorize references private ingress; preserve old transport bounds rather than widening them by accident |
| C11 Df.raw_result_token_bytes = **524,288**, exact selected original result token | **Missing selected ceiling:** RT copies RawMessage; BR permits up to 1 MiB and bounds whole response body at 1 MiB; these are different units | **Unknown** exact token equality/+1 | Bind original token span to accepted complete frame; freeze whitespace/offset semantics; reject before candidate materialization |
| C12 R.raw_result_token_bytes = **524,288**, exact selected original result token | **Unknown selected runtime enforcement:** inspected MT is older private transport, not a proved V5 C references connection | **Unknown** selected equality/+1 | Identify managed references acquisition seam; reuse slicing only after exact-original/custody verification |
| C13 T.source_documents = **256** per transaction | **Missing transaction ledger at inspected SD:** per-document preparation and BR caller map do not count acquired documents across transaction | **Unknown** 256/257 | Define query/target/notebook parent identities and reacquisition semantics; charge before acquisition; distinguish SOURCE-role graph shape |
| C14 T.source_document_bytes = **4,194,304** per document | **Partial/different bound:** SD capture uses 1 MiB; ordinary path uses ReadFile; BR supplied target bytes are not bounded acquisition proof | **Unknown** 4 MiB/+1 under selected custody path; earlier 1 MiB may shadow | Add separately scoped supplier/admission seam; do not loosen historical supply contract; verify acquisition bound before copy/parse |
| C15 T.logical_owned_buffer_bytes = **33,554,432** | **Partial/different resource:** PB reserves up to 16 MiB across four slots; F and RT make multiple copies; no selected transaction-wide ownership ledger established | **Unknown** exact reservation/release boundary | Define owners, aliases, capacity vs length and handoffs; reserve before allocations/copies; checked arithmetic and release-on-all-terminal-paths |
| C16 T.objects = **4,096** | **Partial:** PR checks caller max before each array member, BR selects 1000; neither establishes all transaction materializations | **Unknown** 4096/4097 causal C witness | Define object unit and charge before materialization; isolate at ledger when older parser ceilings shadow, then prove integrated routing |
| C17 T.events = **8,192** | **Missing selected aggregate at inspected seams;** M:67 and schema event shape are not admission | **Unknown** 8192/8193 causal witness | Define which acquisition/capability/target events count, charge before append/emission, and prevent partial event admission |
| C18 T.failure_precedence = **preflight → deadline → header_and_frame → cumulative_wire → message_count → decoded_message → raw_result → source_and_buffer → object_and_event → retention → readback** | **Partial:** RT maps original/cumulative frame errors to RESOURCE_EXHAUSTED; W checks cumulative before JSON syntax yet commits after it; no full staged taxonomy established | **Unknown** full collisions; Vcum proves only valid complete-frame crossing | Freeze reached-stage rules and typed failures; malformed/error precedence decision is mandatory, not resolved by copying old proposed taxonomy |

**Basis:** C01/C04 acceptance is real but bounded by opt-in scope and test construction. No required C row above is waived. Unknown verification must become an attributable witness or remain an explicit blocker; it must not be converted to “verified” merely because a similarly named test exists.

### 3.2 Required work accounting (T.work_counters, M:67)

**Basis:** T enumerates counters, not a numeric aggregate work cap. Current RT diagnostics and PB reservation do not establish an authoritative cross-transaction counter ledger. Each row therefore has missing selected implementation at the inspected seam and unknown full verification. Define increments/overflow behavior; do not invent a 1,000,000-unit guard.

| Counter | Implementation status | Verification status / missing witness | Required definition gate |
|---|---|---|---|
| `bytes_scanned` | Missing selected ledger; W observes header/body bytes, not every scan | Unknown; instrument scans of exact small input | Count actual repeated scans versus one ingress pass; keep separate from wire bytes |
| `messages_processed` | Partial RT.Messages counts valid decoded reads only | Unknown; mix notification/request/unmatched/matched/invalid | Which attempted/validated stage increments; relation to 64 bound |
| `objects_materialized` | Partial PR candidate counting, no universal object ledger | Unknown; tiny-object isolated allocator witness | Object kinds and pre-materialization reservation |
| `events_emitted` | Missing selected accounting | Unknown; pre-emission observer at 8192/8193 | Scope of capability/target/diagnostic events; no double charge |
| `documents_acquired` | Missing selected transaction aggregation | Unknown; repeated versus distinct acquisition | Counting identity and retries; relation to 256 SOURCE documents |
| `logical_buffer_bytes_reserved` | Partial PB conservative reservation, different scope | Unknown; reserve/transfer/release history | Cumulative reservation-work counter versus outstanding-byte gauge; aliases/copies |

### 3.3 Adjacent finite rules: preserve checkpoint ownership

**Basis:** M:30–35,45 separates shape/B4 checks from C resources. Include these dependencies so “all C limits” does not silently mean all A4/B4/D obligations are done.

| Rule / bound | Implementation status | Verification status | Ownership/action |
|---|---|---|---|
| One JSON message per frame | W:266–275 decodes one Message and requires EOF on second decode | Source-established guard; fresh/replayed boundary execution unknown here | Preserve strict framing; concatenated value is not an extra admitted message |
| ID raw token **4096 B**, decoded string UTF-8 **1024 B**, absolute exponent **100000**, exact decimal without float/expansion | `internal/adr0011genericv5proposal/id.go:16–108` implements bounded offline typed parsing | Offline source inspected; full managed chronology integration unknown here | B4, not C discharge; preserve original tokens in complete frames and matching same-kind semantics |
| Capability events ≤8192, observed frames ≤16387, inbound frames ≤64, result token length ≤524288, target ordinals array ≤4096 | M:31–33 records S shape bounds | Shape is not a consumed/acquired/materialized witness | A4 validates shape; C must independently charge actual input; ordinal value has no schema maximum |
| SOURCE ≤256; TARGET_EVENTS predecessors 1–256; TERMINAL ≤265; READBACK ≤267; TRANSACTION 11–268 | M:35 records exact S role-graph bounds | No new graph enforcement verification performed | A4/B4/D obligations; avoid mistaking graph predecessor count for acquisition count |
| Smaller historical ceilings: MT 64 messages/1 MiB/one minute; PB four slots/16 MiB reservation; SD 1 MiB supply; BR 1000 candidates/1 MiB body | Visible implementation at cited sources | Not selected-C equality witnesses | Preserve compatibility; expose isolated successor seams if they shadow selected caps, never silently weaken guards |

### 3.4 D explicitly deferred from C, mandatory before admission

| Exact T/S obligation | Implementation status / inspected basis | Verification status | D prerequisite |
|---|---|---|---|
| Mandatory retained originals **67,108,864 B**, capability request/response and initialized frames included; over-cap `RETENTION_QUOTA_EXCEEDED` | **Deferred D; no generic aggregate established.** PUB expressly does not require/retain exact LSP frames | Unknown full D equality/+1, rollback and readback | Atomic all-originals inventory/accounting, including metadata under an approved counting rule; reject without selector publication |
| Owner-process access elapsed **<86,400,000 ms**; cross-process access false | **Deferred D;** PUB explicit owner removal is not timed access control | Unknown 86,399,999/86,400,000 and restart/cross-process denial | Continuous owner monotonic epoch; no unauthenticated wall-time reconstruction or reset on restart |
| Optional private executable snapshot **536,870,912 B**, does not gate admission | **Deferred D optional feature;** no selected enforcement proved | Unknown optional cap/+1 and absence-is-nongating witness | Keep optional bytes out of mandatory-admissibility decisions and producer-authentication claims |
| Terminal counters/authority/completeness and independently reopened originals/readback correspondence | **Deferred D;** PUB verifies only canonical candidate consistency; M:77 requires fresh independent final readback | Unknown generic D originals-to-terminal correspondence | No COMMITTED_VERIFIED/MATCH from claimant fields, in-memory copies or candidate publisher alone |

## 4. Charging and unresolved contract decisions

**Basis:** W:167–175 documents the accepted successful-frame semantics, and F:22–23 excludes prefetched later bytes from frame capture. Full C needs distinct quantities; collapsing them would both overclaim acceptance and obscure resource consumption.

| Quantity | Current observable behavior | Required successor rule or explicit decision |
|---|---|---|
| Acquired bytes | `bufio.Reader` may prefetch; capture excludes subsequent-message prefetch; body allocation occurs before ReadFull | **G2:** define underlying I/O acquisition versus logical frame consumption; prefetched storage must enter buffer accounting even if wire charge is assigned later |
| Consumed original wire bytes | W's local header count and ReadFull `n` observe consumed prefixes, but cumulative success field does not include failed prefixes | T/M require complete plus partial consumed bytes, including separator and irrelevant messages. Incremental accounting must not roll back consumed bytes on malformed/EOF/cancel. **G2:** choose bounded observation of crossing byte, without reading arbitrarily beyond remaining budget |
| Validated complete-frame bytes | W commits `used + length` only after JSON/version/kind validation; +1 is checked before decode | Preserve this historical meaning and witness; new consumed accounting must be separately named or a versioned successor with explicit compatibility boundary |
| Retained raw-copy bytes | F appends capture then returns a second copy; RT may copy again; successful malformed-frame capture can occur before validation fails | Count owned buffers, not merely the capture limit. **G5:** decide precisely which temporary copies/reservations are charged, when ownership transfers, and which failed prefixes D must retain |
| Decoded message bytes | RT uses `len(json.Marshal(message))`, currently summed | R/Df select this whole-message per-message unit, not original wire whitespace or decoded character counts. Check marshal error handling and reserve output ownership |
| Selected raw result bytes | BR/RT use RawMessage; exact scalar/array bytes can differ from a re-marshal | **G6:** freeze token start/end and surrounding whitespace conventions against exact originals; no reserialization to satisfy the cap |
| Source bytes/documents | SD supply and BR target map have different custody and size behavior | **G5:** independently held source identity, duplicate acquisition/retry charging, per-document bytes and transaction document count must be separately bound |
| Logical owned buffers and objects/events | PB has a conservative private reservation; no C-wide object/event ownership proof | **G5:** define outstanding gauge, work counters, reserve-before-copy/materialization, transfer/release, and overflow; do not infer Go heap use from policy bytes |

**G1 — transaction lifetime and scope (blocks multi-operation C).** Basis: RT creates a reader per round trip while T requires transaction-wide bytes/counts/60 seconds; PB.Transaction is a label. Decide one lifecycle owner binding workspace/session/generation/transaction and its start/end, capability history cut, request suboperations and terminal release. Decide whether prior initialized/capability frames are charged once on acquisition, on transaction attachment, or by separately scoped history accounting. Reconcile 64 inbound messages with 16,387 capability frames without inventing a contradiction or an unlimited exception. Failed/preflight attempts, reentry, cancellation, completed STOP/RESTART, and no lease consumption must have explicit lifetime outcomes. No budget reset by spawning a new reader or relabeling a request.

**G2 — partial/failed reads and prefetch (blocks byte-ledger replacement).** Basis: accepted failure/partial noncharging is intentionally narrower than M:64. Define resource charging at actual logical consumption, tracking I/O acquisition/prefetch separately. Exhausted input, partial CRLF, missing newline, short body, read error with nonzero `n`, cancellation during body and buffered next-frame bytes need exact counter traces. Decide whether the cap+1 sentinel byte is charged as attempted/consumed and whether it is retained; either choice must have a reviewed contract, bounded read behavior and no partial admission.

**G3 — malformed precedence and typed failures (blocks collision implementation).** Basis: T fixes stage order but M:68 says C failure types are unspecified; W may return a cumulative failure before discovering malformed JSON. Approve a decision table for invalid/duplicate/missing Content-Length, invalid JSON, wrong version/kind, truncated header/body, concurrent timeout/cancel, frame/cumulative/count collisions, decoded/raw/source/buffer/object/event failures. Distinguish actual reached failures from predicted downstream failures. Preserve accepted RESOURCE_EXHAUSTED mapping in its exact scope. Do not silently import INVALID_FRAME/TRUNCATED_FRAME/TRANSACTION_TIMEOUT from the older proposed packet. If selected immutable clauses genuinely cannot be reconciled, stop for a separate contract decision rather than editing their bytes.

**G4 — deadline equality and clock custody (blocks deadline claims).** Basis: T gives numeric request/transaction limits; RT uses caller wall-deadline/context mechanics and source filesystem reads are synchronous (SD:326–330). Approve monotonic epoch, start point, equality for an already completed operation, prohibition on beginning a new blocking operation at exhaustion, earlier caller-deadline precedence, and controllable clock/blocked-I/O seams. A timeout-only test using sleep cannot establish exact equality. Uninterruptible acquisition must be resolved, bounded or explicitly block C exit, not hidden by before/after checks.

**G5 — resource ownership/units (blocks ledger closure).** Basis: selected logical buffers differ from legacy reservations. Define every allocation/copy/retained original/source/object/event owner, count/release points, aliased versus independent copies, deduplication, error unwinding and checked arithmetic. The six work counters need increments and overflow rules, not invented numeric ceilings. Retained mandatory-original counting belongs to a separate D ledger with a reviewed handoff.

**G6 — references and exact token admission (blocks generic C claims).** Basis: PB is definition-only and BR is a synthetic candidate bridge. Select a private references path explicitly; verify exact raw token extraction and both methods' per-message budgets before shared continuation. Reuse of older method transport or production references chains is conditional, not established by names or imports.

## 5. Reuse candidates and managed-LSP pre-implementation gates

**Basis:** the following actual bodies were inspected, so they are concrete candidates rather than guessed packages. Their relationship descriptions are textual source observations or proposed data flow; none is a server-reported call graph.

| Candidate | What inspected source supports | What must not be assumed / structural gate |
|---|---|---|
| W Reader and F capture | Per-frame/cumulative-success guard; consumed-byte hashing; capture/decode observer entry | Resolve all consumers and source-bearing definitions under exact READY session; ordinary-reader compatibility and partial-read ownership before modifying reader internals |
| RT + PB | Private definition opt-in, reservation, exact selected-read lease, one-use consumption, generation checks | Resolve runtime incoming/slice evidence for `RoundTripPrivateB4`, `roundTripWithPrivate`, `ConsumePrivateB4Definition`, finish/retirement paths; transaction owner and references extension remain decisions |
| PA + BR + PR | Manager-selected exact-byte comparisons; B4 result grammar; whole-result failure without item prefix | Resolve projected bodies and actual continuation paths; candidates stay authority=0, accepted=false, completeness=UNKNOWN; BR 1000 is not C objects=4096 |
| SD prepared supply | Bounded opt-in source read, no-follow alternative, distinct legacy read | Resolve source acquisition/identity ownership and which paths reach transaction; do not widen legacy supply cap to fit synthetic witness |
| PUB + `internal/publication` APIs | PUB calls no-replace bound-file publication and verified readback for candidate bytes | Candidate publisher lacks exact-frame custody and is not D. Inspect publication owner before any D reuse; do not treat its success as occurrence admission |
| `internal/programcadmission/typed_boundary.go:8–18,20–21,55–68` | Explicit relation-kind vocabulary and deliberate CALLS-only admission; nonempty MethodCandidates rejects | Preserve this rejection. New D/R occurrence admission needs its own approved contract/version; never convert MethodCandidates into accepted input |
| `internal/retainedpath/path.go:15–26,37–84,88–122` | Bounded search over opaque already-admitted directed edges; index construction and search mechanics | Edge fields are CALLS-shaped; `Search` rebuilds indexes and its budget excludes index construction. Not a ready typed immutable-index consumer. Inspect reuse only after typed admission; separately bound construction and retain relation kind/ordinal |

**Managed evidence gate:** re-list (do not assume generation 1 remains READY), target exact workspace, request source-bearing structural context and bounded incoming/slice evidence for the above owners. Store/return operation identity and scope in the later execution record. Text establishes declarations and visible operations only; server-reported relationships establish CALLS only within their returned scope. If unavailable, the executing owner must obtain explicit authorization for the particular degraded implementation decision; this plan does not preauthorize all cross-file wiring.

## 6. Bounded one-writer execution packages

**Basis:** the user requires one writer, frozen counterexamples and coherent reviews, not a new agent handoff at every tiny edit. Use one implementation owner across these packages. Reviewers may inspect read-only coherent packets at the stated boundaries; they do not become competing writers. No delegation or review execution occurs in this planning session.

### Package P0 — portable accepted baseline and historical archive

**Prerequisites:** authorize implementation separately; verify §1 identity and current worktree before any source change. Existing tracked A/B fixtures and per-frame originals were confirmed by `git ls-files`; cumulative successor source explicitly says its ignored originals are not portable (`:17–19`), and historical helper uses `../../.pi/evidence/adr0011-c-cumulative-wire-v1` (`:24–27`). Therefore repair portability before accumulating further C work.

**Owned changes:** `internal/adr0011methodresult/testdata/` additive cumulative fixture tree and inert historical archive; successor/helper test files; compatibility test packaging; a bounded reproduction instruction file in the later authorized package. Do not alter snapshot, prior contracts or per-frame original bytes.

**Prescribed outcome:**

1. Copy the accepted twelve exact frames and necessary manifests into an ordinary tracked testdata location, preserving each byte length, SHA and ordered aggregate. A relocated manifest/loader is a **new identity**, separately binding its immutable source manifest `b44515d1f806842069824b1c8e7039edc9e080a020a64dcbfc7352acdeaad61e`; never rewrite that historical original in place. Track ordinary files without `git add -f` or broad ignore-rule exceptions.
2. Preserve the complete historical RED source unchanged as an inert byte artifact (for example `.go.txt` in testdata) and its exact SHA from §1. After separately authorized packaging, exclude its active `_test.go` version from default discovery. Extract/adapt required helpers into a newly identified active helper; do not mutate the historical artifact or call its intentionally failing test from default tests. The successor must not rely on compiling the historical source.
3. Freeze the relocated active test/helper identity and unchanged wire bytes **before** any new behavior runs. Distinguish portability changes from runtime changes: the accepted runtime pins remain unchanged in P0.
4. In a genuinely clean fresh checkout of the later recorded revision (not a worktree inheriting ignored originals), prove the named fixture tests execute with nonzero expected case counts using only tracked files. No `.pi/evidence` dependency, absolute developer paths, network or local notebook required. Declare supported platform/toolchain rather than claiming all-platform portability. Report unavailable prerequisites as BLOCKED_NOT_RED, never SKIP-as-pass.
5. Reproduce historical failure only in an explicit disposable archival procedure with matching historical runtime/test identities, not the default suite. Require default package checks to avoid intentional RED poisoning.

**Review boundary:** one portability packet containing tracked-file inventory, byte correspondence, archive isolation, active test identities and clean-checkout execution. Review archive/reproduction and accepted-runtime invariance together, not per copied frame. This package cannot claim any additional C resource coverage.

### Package P1 — approved transaction/charging decision table and bounded ingress

**Prerequisites:** close G1–G3 and managed reader/runtime ownership gates before behavior changes; close relevant buffer rules in G5 for ingress allocations. If decisions are unresolved, stop at the decision table—do not choose hidden defaults.

**Owned candidate files:** W/F, RT/PB, private observation support and focused tests. Any new ledger type/package must have a narrow transaction-owned interface justified by the managed seam evidence; no general resource framework.

**Counterexamples to freeze:** newline-free header at/+1; valid header with separator crossing; partial EOF/error with bytes; declared frame at/+1; complete cumulative at/+1 from accepted originals; partial prefix crossing cumulative; malformed-under-budget and malformed-over-budget; prefetch containing subsequent frame; two requests sharing a transaction; new generation; cancellation/STOP/RESTART and abandoned lease. Record expected typed outcome, reached stage, consumed/acquired/validated/retained counts and zero downstream entries on failure.

**Execution:** one writer performs focused same-byte RED→GREEN for each independently observable guard, without external handoff between each change. Preserve accepted success-ledger behavior or introduce an explicitly reviewed successor mode; keep ordinary disabled-cumulative and per-frame-only compatibility. Freeze tests/originals/expected table first; a fixture/deadline/setup failure is BLOCKED_NOT_RED, not the counterexample. A runtime mutation must be the cause of GREEN, not a modified assertion or cheaper fixture.

**Review boundary:** one coherent ingress/lifetime packet with the decision table, exact source/test/original pins, causal failure records, restoration to final intended state, and compatibility evidence. Do not claim message/deadline/resources closed merely because byte ingress is green.

### Package P2 — counts, deadlines and per-message/result limits

**Prerequisites:** P1 accepted; G4/G6 closed; references ingress selected and structurally inspected; capability-history scope from G1 approved. No temporary widening of production legacy caps just to reach a test boundary.

**Owned candidate files:** selected runtime/private method entry, exact token slicing owner, capability-original admission owner identified by structural evidence, and focused tests. Keep B4 chronology semantics separate from C accounting.

**Frozen witnesses:** 64th versus 65th target inbound message with terminal selected at cap; notification, server request, unmatched/error responses counted according to approved table; 16,387/16,388 held capability frames with other budgets demonstrably slack; 15,000/15,001 and 60,000/60,001 controlled-clock cases plus equality/new-blocking distinction; references/definition per-message marshal limits at/+1 using decoded content rather than wire whitespace; 524,288/524,289 exact result tokens independently for both methods, including null/empty and preserved whitespace. Include same-input limit collisions and late cancellation.

**Reachability rule:** a lower historical guard or a genuinely incompatible joint ceiling makes a proposed equality witness unobservable. Show that calculation, isolate the exact owner with test-only injection if semantically valid, and also supply a real integrated path witness; isolated ledger success alone does not prove runtime wiring. Missing references path or ambiguous 64/history scope blocks the affected rows and C exit.

**Review boundary:** one method/count/time packet covering both units and integration, with exact nonzero executed case inventory. Reuse retained byte fixtures but do not recertify earlier increments as newly authored work.

### Package P3 — source, logical ownership, objects/events and counters

**Prerequisites:** G5 approved; structural evidence identifies acquisition and release owners; P1/P2 accepted. All six work counters are defined without an invented total cap.

**Owned candidate files:** transaction ledger, selected source acquisition interface, method materialization/event owners and tests. Legacy supply/parser bounds remain unchanged unless separately approved; private supplier/allocator/event-sink seams must be bounded and non-authorizing.

**Frozen witnesses:** 256/257 documents and 4,194,304/4,194,305 bytes per document; 33,554,432/33,554,433 outstanding logical owned bytes; 4096/4097 objects; 8192/8193 events; alias versus copy; reservation denial before allocation; release/transfer on success, malformed result, cancellation, STOP/RESTART and readback/publication failure. Tiny fixtures at injected owners isolate otherwise shadowed caps, paired with integrated evidence that the selected path charges that owner. Every counter's expected small-input trace and overflow behavior is asserted. No partial candidates/events/selectors survive failure.

**Review boundary:** one resource-ownership packet with all ownership/release paths and isolated plus integrated evidence. Add a row-by-row C closure report; stale or unknown rows are blockers, not “covered by resources tests.”

### Package P4 — C closure and D handoff, not D implementation

**Prerequisites:** all selected C obligations have implementation and verification closure, selected contract hashes unchanged, portable fresh checkout evidence available. Recheck ordinary-reader/per-frame/B4 compatibility together once at this boundary; use focused/package verification rather than an exhaustive suite after every small edit.

**Deliverable:** accepted C-only report with exact supported rule IDs, concrete test cases and logs, every input/runtime/test hash, charge/failure tables, covered entry paths, and remaining A4/B4/D obligations. A named checker must fail on unknown rules/zero selected tests and return coverage explicitly (M:89); a passing wildcard or zero-match run is not evidence.

**Review boundary:** one independent consolidated C exit review. D work remains a separately authorized phase; no index or path demonstration can consume private candidates simply because C closed.

## 7. C exit criteria and controlled continuation to the private example

### C exit (all required)

**Basis:** M assigns each checkpoint an independent responsibility. C may be accepted only when:

- C01–C18 and each work-counter row have a concrete selected owner, implemented rule, reached-stage witness, and observed at-cap/+1 (or explicit equivalent finite-boundary test for nonnumeric rules). No required C implementation/verification cell remains missing or unknown. A waiver changes scope and cannot be labeled full C.
- Transaction lifetime, capability-history scope, partial/failed charging, malformed precedence, deadline equality and resource ownership decisions are approved and executable without hidden defaults.
- Exact acquired/consumed/validated/retained distinctions are reflected in counters, not only prose; overflow and all failure cleanup paths are covered; earlier guards never masquerade as the intended witness.
- Same-byte frozen focused RED→GREEN records identify the failing assertion, exact runtime delta, result stream and test/original hashes. Setup failures remain BLOCKED_NOT_RED. Earlier accepted evidence is cited, not overwritten.
- Tracked fixtures reproduce from a clean fresh checkout; intentionally RED history is byte-preserved and inert; default tests neither require ignored evidence nor discover intentional failures.
- Independent review accepts coherent package boundaries and the consolidated coverage report. Public defaults/old readers/B4 identity and CALLS-only admission remain compatible; no C success asserts D, public acceptance or producer authentication.

### D prerequisites and the non-bypass chain

**Basis:** PUB says it is not a method receipt and does not retain exact LSP frames; the typed input boundary rejects D/R candidates. Consequently the desired chain is conditional:

1. **Private acquired typed evidence:** approved A4/B4 selection, exact managed request/source/generation custody, completed C and bounded typed method outcomes. A caller-provided candidate/map is not acquired authority.
2. **D retention/readback:** separately authorize and implement all mandatory originals, aggregate quota/rollback, owner access/expiry, terminal correspondence and independent final reopening. Resolve how independent readback occurs under owner-only/cross-process-denied access; do not weaken either requirement by assumption. Optional executable snapshot never gates admission.
3. **Occurrence admission:** independently bind method receipt, source/revision/endpoint identity, exact original kind/direction/ordinal/multiplicity and tombstone status. Close applicable method qualification/endpoint-policy gates; preserve `ErrMethodEvidenceUnadmitted` until the successor admission contract is accepted. D success alone does not confer occurrence admission.
4. **Bounded private index:** only already-admitted typed occurrences enter one immutable in-memory example index under explicit finite node/occurrence/build-memory/work budgets. Choose and approve these example budgets later; C caps are not automatic graph budgets. Forward/reverse adjacency share occurrence identities; reverse adjacency is not an inferred inverse LSP result. No generic persistent index/store, refresh service, public selector or Leiden/grouping expansion.
5. **Bounded path example:** consume that admitted handle with approved search/depth/frontier/work/cancellation limits; no hidden acquisition, source resolution, hydration, replay or rebuild during the walk. Preserve type and original direction in path witnesses, including equal-endpoint multiplicity and original ordinals. Limit/cancel yields an explicitly incomplete result without partial accepted witness. Missing evidence is unavailable/UNKNOWN, not a fabricated empty graph.

**Conditional reuse warning:** `retainedpath.Search` presently rebuilds adjacency, uses caller/callee-shaped edges, and excludes index construction from its search budget. A future small adapter or search-over-prebuilt-index seam requires separate semantic review; this plan does not authorize relabeling definition/reference edges as CALLS. `adr0011-admitted-relation-index-architecture-b.proposed.md:3–9` is an unaccepted broader design with qualification gates, not permission to skip them for a demo. If the private example needs a narrower qualification contract, obtain that explicitly; do not call a candidate-only walk an admitted example.

**Relation boundary:** method dispatch, name references, definition resolution and value flow are distinct. Textual dispatch syntax or a REFERENCES_SYMBOL/RESOLVES_TO_DEFINITION edge is not a CALLS edge and does not establish value propagation. Keep source observations, server-reported call hierarchy, typed method results, admission facts and design proposals separately labeled. The bounded example does not prove executable behavior or whole-workspace completeness.

## 8. Open-decision summary

1. **G1:** transaction start/end, shared owner/clock, history attachment and 64 versus 16,387 scope.
2. **G2/G3:** partial/failed/acquired/consumed accounting, crossing byte/prefetch, malformed precedence and bounded typed failures.
3. **G4:** monotonic deadline equality and interruptibility of acquisition.
4. **G5:** buffer/document/object/event units, copies/transfers/releases and six work-counter increments; no invented work cap.
5. **G6:** private references ingress and exact token-span semantics; isolate historical ceilings without weakening compatibility.
6. **Managed evidence gate:** exact current READY session plus projected owners and bounded server-reported dependencies before cross-file implementation decisions.
7. **Downstream gates:** D custody/readback under owner-only access; independently accepted typed occurrence admission and endpoint/qualification policy; then narrowly approved private example budgets and path/index adapter.

## Model interpretation

- This is a filesystem-backed plan with bounded retained verification, not a live-semantic architecture proof or fresh test result.
- Selected exact T/R/Df/M contracts supply limits; obsolete proposals and unrelated legacy ceilings do not supply defaults.
- Future packages are sequential single-writer units with coherent review boundaries, not authorization to begin implementation.
- Verification priorities: clean-checkout fixture independence; exact-boundary causal accounting; C/D/admission separation before any typed path example.

Improved framing: preserve the accepted successful-frame increment, close the remaining transaction-wide resource obligations with explicit contract gates, and only then pursue independently accepted D custody and typed admission for one bounded private path example.
