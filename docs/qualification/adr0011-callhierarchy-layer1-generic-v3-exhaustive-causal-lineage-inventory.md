# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Exhaustive Causal-Lineage Inventory

Status: **REVIEW_REQUIRED / NO MUTATION / PRODUCTION BLOCKED**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Pinned inventory HEAD: `4add2ac3f9cf556789869067b35170dd2c9f6276`
- Current additional inputs:
  - untracked proposal `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-attachment-enable-causal-input-amendment.md`;
  - untracked independent review `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-attachment-enable-causal-input-amendment-review.md`, status **BLOCKED**.
- Scope: documentation-only causal-lineage inventory.
- Authority not issued: corpus, RED, test, source, prototype, generated artifact, C15 execution, implementation, writer, commit, activation, production, semantic commitment validity.

This document inventories carriers and derivations. It does **not** decide whether any opaque commitment is valid or meaningful. `available`, identity, length, digest, and bytes establish only the stated finite custody/equality observations.

## 1. Evidence and authority legend

| Mark | Meaning |
|---|---|
| `CURRENT` | Present in retained, reviewed v3 qualification evidence at the pinned lineage. |
| `PROPOSED` | Described by the current untracked attachment-enable amendment; not frozen or accepted. Its independent review is BLOCKED. |
| `BLOCKED` | No accepted production carrier/seam currently exists, the required outcome lacks an independently frozen carrier, or reviewed evidence identifies an unresolved contract contradiction. |
| `A` | Production-owner/view observation: compare an actual finite observation to expected. |
| `B` | RED-only derivation from actual execution; production seam is `NONE`. |
| `C` | Fixture/control-only classification; production seam is `NONE`. |

The accepted expected schema is 145 paths. Ordinary ownership is `A=116, B=20, C=9`. Only `root/24` is occurrence-sensitive: its 35 ledger-dimension paths move from A to B, yielding `A=81, B=55, C=9`. The same 35 path names remain A for every other occurrence, including all 16 World-24 workers.

### 1.1 Finite production transfer chain and current authority

The path tables use these exact seam abbreviations.

| Seam | Exact symbol/file chain | State |
|---|---|---|
| `S-REQ` | `RoundTripRequest.callHierarchyLayer1`; `callHierarchyLayer1RequestAttachment`; value-returning private attachment setters in `sessionruntime/owned_method_pair.go` and `sessionruntime/b4_owned_method_pair_lease.go` | Prior disposable boundary only; production not accepted. |
| `S-IDFRAME` | `(*Manager).privateCallHierarchyRequestIdentity`, `newCallHierarchyOriginalRequestFrameObservation`, `newCallHierarchyOriginalResponseFrameObservation`, and both `copyFrameBytes` methods in `sessionruntime/owned_method_pair.go` | Prior disposable authorization/evidence only; production not accepted. |
| `S-OWNER` | `newCallHierarchyLayer1GenericOwner`, `callHierarchyLayer1GenericOwner`, and immutable observation storage in `sessionruntime/b4_owned_method_pair_lease.go` | Prior disposable authorization/evidence only; production not accepted. |
| `S-RESULT` | `(*Manager).RoundTrip`, optional existing `(*Manager).finishRoundTrip` transfer, and private `RoundTripResult.callHierarchyLayer1Generic` in `sessionruntime/sessionruntime.go` | Prior disposable authorization/evidence only; production not accepted. |
| `S-LEASE` | `(RoundTripResult).privateCallHierarchyLayer1GenericLease() (any, bool)`, `callHierarchyLayer1GenericLease.View`, `.Callback`, `.Release`, owner `.requestTerminal`, and `.waitReleased` in `sessionruntime/b4_owned_method_pair_lease.go` | Prior disposable authorization/evidence only; production not accepted. |
| `S-PRED` | `callHierarchyOpaqueCommitmentList`, `callHierarchyLayer1RequestAttachment.predecessors`, and `privateCallHierarchyLayer1WithPredecessors` in `sessionruntime/owned_method_pair.go` / `sessionruntime/b4_owned_method_pair_lease.go` | Prior disposable-reviewed v3 addition; production not accepted. |
| `S-ENABLE` | **PROPOSED:** `callHierarchyLayer1RequestAttachment.enabled bool` in `sessionruntime/b4_owned_method_pair_lease.go`; `privateCallHierarchyLayer1WithEnabled(bool)` in `sessionruntime/owned_method_pair.go`; read at `(*Manager).RoundTrip` in `sessionruntime/sessionruntime.go`; propagate only through `newCallHierarchyLayer1GenericOwner` | `PENDING / NO GO`; not frozen; production blocked. |
| `NONE` | No production transfer seam: RED computation or fixture-only structure. | Required for every B/C row. |

No seam above is represented here as accepted production. The attachment-enable proposal is **unresolved**, not accepted: its review confirms that `S-ENABLE` is finite and the offline `11 false / 74 true` mapping is independent of expected data, but issues `ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_ATTACHMENT_ENABLE_CAUSAL_INPUT_BLOCKED`. The proposal requires correction and re-review before any disposable prototype/C15 delta or production write-set gate can confer authority.

## 2. Independently frozen runtime inputs and observable sources

| Source/carrier | Status | Exact role and unavailable semantics | Anti-feed boundary |
|---|---|---|---|
| `driver.request` excluding `attachment_enabled` | `CURRENT` retained self-contained driver | Request identity, params, method/deadline and request construction inputs; exact nil/empty/present distinctions must survive. | Expected, world/name, pair labels, and C controls cannot construct it. |
| `driver.request.attachment_enabled` | `PROPOSED`, independently review-confirmed as a lawful mapping but amendment **BLOCKED** | Required non-null bool; proposed `false` for 11 `w25-pair/*/disabled`, `true` for other 74. This carrier is independently derivable without any C value, but is not frozen or accepted. | Must be authored offline from occurrence kind and materialized directly; runtime may not infer it from occurrence text, commitment content, expected/control, transport, or result. Optional C agreement checking is downstream validation only and cannot be a prerequisite for deriving the bit. |
| `driver.commitment_inputs.{selected,handoff,predecessors[*]}` | `CURRENT` | Explicit bytes plus metadata; unavailable is `false`, `bytes_hex=null`, identity empty, length zero, digest 64 zeroes; predecessor cardinality/order literal. | Expected commitment metadata, fixture names, hidden registry/runtime state, and sibling/pair members cannot feed execution. |
| ordered `driver.transport.events[*]` | `CURRENT` | Exact response-frame or control-event script, preserving order and event bytes/timing/class/message. | World/name/expected cannot select an event. |
| actual `Manager.RoundTrip` request/result and fake-transport observations | Required runtime source, production carrier `BLOCKED` | Original request frame, successfully emitted original response frames including malformed frames, result/error/failure, terminal, transport/accounting/external values. No-frame is unavailable frame plus available-empty response sequence after attempted execution. | Existing expected values, prototype output, or fabricated frames/errors cannot be actual operands. |
| actual owner/view/lease/callback/release events | Required runtime source, production carrier `BLOCKED` | Acquisition, view, callback return/error/panic containment, terminal closure, drain, and first/second release outcomes. | C controls and expected lifecycle values cannot manufacture these events. |
| World-24 16 worker actual A ledgers | `CURRENT` formula; production execution still blocked | Sole operands for root/24 aggregate B values, in worker order. Any unavailable required worker makes aggregate unavailable; no zero substitution. | Root expected aggregate and another worker's expected values cannot feed computation. |
| World-25 two independently executed member actuals | `CURRENT` formula; production execution still blocked | Sole operands for transport/external equality; each member retains its independent request and commitments. | Pair label, shared expected values, or inherited sibling actuals cannot manufacture equality. |

## 3. Exhaustive retained-path inventory

Each row is a rigorously enumerated family. The member column is the explicit complete member set and the cardinality is part of the inventory. All family sets are disjoint.

### 3.1 Category A — ordinary occurrences, 116 paths

| Path key/family and explicit members (cardinality) | 1. Independently frozen driver input or observable runtime source | 2. Exact production transfer seam | 3. Resulting classification/observation | 4. Unavailable/disabled/nil-empty semantics | 5. Anti-feed proof |
|---|---|---|---|---|---|
| `accounting.{bytes,diagnostics,objects,work}.{available,known_zero,value}` (12) | Actual Manager private accounting snapshot caused by request, transport, lifecycle and terminal execution. | `S-OWNER` via `S-RESULT`; candidate must reuse existing Manager accounting primitives. | A: exact triples retained in owner/view. | Unavailable is not zero; `known_zero = available && value==0`. | No expected accounting, world/name, commitment content, or hidden side table may populate actuals. |
| `commitments.selected.{available,identity,length,sha256}`; `commitments.handoff.{available,identity,length,sha256}`; `commitments.predecessors[*].{available,identity,length,sha256}` (12 schema paths) | Exact explicit `driver.commitment_inputs`; predecessor sequence literal and ordered. | `S-REQ`; predecessors additionally `S-PRED`; then `S-OWNER`/view. | A: opaque metadata only; no `bytes_hex` in expected/view. | Unavailable normalization is false/empty/0/zero-digest; zero predecessors is an empty ordered sequence, not one unavailable predecessor. | Expected commitments, semantic validity, names, pair labels and registry lookup cannot feed runtime attachment. Defensive copy prevents caller mutation. |
| `error.{available,class,code,data_base64,data_form,message,present,serialized_hex}` (8) | Actual generic server-error observation and runtime failure class. | `(*Manager).RoundTrip` in `S-RESULT` to `S-OWNER`. | A: exact actual error observation. | No server error frame: `available=false`, `present=false`, nullable details null, `data_form="unavailable"`, serialized null; runtime class may remain known. Available-empty must not be collapsed into unavailable. | Control event class cannot fabricate a JSON-RPC error payload; expected error bytes cannot become actual. |
| `external.{available,failure,notifications_hex,responses_hex}` (4) | Actual external execution result/messages/failure and exact ordered emitted bytes. | `(*Manager).RoundTrip` in `S-RESULT` to `S-OWNER`. | A: generic external observation. | After attempted execution, observation may be available with empty response bytes `""`; unavailable is distinct/null as schema requires. | `external.execution_expected` is C and cannot feed these fields; names and expected responses are forbidden. |
| `frames.auxiliary`; `frames.request.{available,frame_hex,frame_length,frame_sha256}`; `frames.response.{available,frame_hex,frame_length,frame_sha256}` (9) | Complete actual encoded request frame; ordered successfully emitted response-frame event bytes; actual auxiliary frame collection. | `S-IDFRAME` → `S-OWNER`. | A: exact originals, lengths and SHA-256; malformed emitted response retained raw. | No response frame: false/null/0/64-zero digest. Malformed frame remains available. Empty control-event-only script contributes no frame. | Parsed bodies/results, errors, terminals, expected frames and normalized renderings cannot feed original-frame custody. Defensive byte copies required. |
| `identity.binding.{available,value}`; `identity.generation.{available,value}`; `identity.method.{available,value}`; `identity.profile.{available,value}`; `identity.request_identity.{generation,request_key,session,workspace_nonce_hex}`; `identity.transaction.{available,local_hex}` (14) | Actual Manager request identity and transaction allocation. | `(*Manager).privateCallHierarchyRequestIdentity` in `S-IDFRAME`, then `S-OWNER`. | A: exact request/transaction identity observations. | Each availability bit governs only its sibling; unavailable uses private zero value and is not available-empty. | Fixture name, occurrence/world, pair labels, expected identities and hidden lookup cannot generate actual identity. |
| `refusal.{reason,refused}` (2) | Actual generic admission/refusal result from execution. | `(*Manager).RoundTrip` admission → `S-OWNER`. | A: private generic refusal observation. | `refused=false` requires null/no reason under contract; disabled publication is not automatically semantic refusal unless actual contract says so. | Commitment availability/content and inherited scenario labels are not refusal policy; expected control reason cannot feed production. |
| `result.{available,bytes_hex}` (2) | Actual `RoundTripResult.Result` bytes. | `S-RESULT` → `S-OWNER`, defensive copy. | A: raw result availability and bytes. | Preserve unavailable/null versus available-empty bytes. `result.form` remains absent. | Expected result, parsed response-result fields and fixture labels cannot feed actual result. |
| Ordinary-only key `{occurrence!=root/24}::round_trip_ledger.dimensions.{callbacks,diagnostics,external_bytes,leases,reservations,transport_bytes,transport_messages}.{before,after,delta,available,known_zero}` (35) | Actual occurrence-local Manager before/after transaction ledger snapshots. | Existing Manager ledger primitives through `S-OWNER`/`S-RESULT`. | A for all 84 ordinary occurrences, including all 16 W24 workers. | Unavailable is not zero; when available, `delta=after-before`, `known_zero=(delta==0)`. | Expected arithmetic, another occurrence, aggregate labels, or hidden state cannot supply actual snapshots. |
| `round_trip_ledger.terminal.{before,after}.{available,value}` (4) | Actual terminal snapshots before and after execution. | `(*callHierarchyLayer1GenericOwner).requestTerminal` / `S-RESULT` / `S-OWNER`. | A: exact terminal ledger records. | Unavailable value is null/zero private value, not empty terminal text. | Expected terminal, error serialization and world labels cannot feed terminal snapshots. |
| `terminal.{available,value}` (2) | Actual Manager terminal disposition. | owner `.requestTerminal`, `.waitReleased`, `S-RESULT`. | A: exact terminal outcome. | Unavailable is distinct from an empty terminal string; terminal closes admission and precedes drain completion as specified. | Error/response bytes, expected terminal, or fixture reason cannot be substituted. |
| `transport.{bytes,messages,request_bytes,request_messages}.{available,known_zero,value}` (12) | Actual request/response transport byte and message accounting. | `(*Manager).RoundTrip` transport accounting → `S-OWNER`. | A: four exact accounting triples. | Available-known-zero differs from unavailable; no-frame response may be available-known-zero after an attempt. | Expected frame sizes, fabricated errors, names and controls cannot provide actual transport counts. |

A cardinality check: `12+12+8+4+9+14+2+2+35+4+2+12 = 116`.

### 3.2 Category B — ordinary occurrences, 20 paths

For every row below the production transfer seam is exactly `NONE`.

| Exact path (cardinality) | 1. Independently frozen driver input or observable runtime source | 2. Seam | 3. Resulting classification/derivation | 4. Unavailable/disabled/nil-empty semantics | 5. Anti-feed proof |
|---|---|---|---|---|---|
| `lifecycle.acquired` (1) | Actual acquisition call return. | `NONE` | B: true iff actual acquisition succeeded. | Disabled/no-owner normally yields false; no fabricated acquisition. | Expected/C availability cannot feed return. |
| `lifecycle.completed` (1) | Actual admitted callback normal return or contained panic completion. | `NONE` | B: actual completion event. | No admitted callback is false; panic counts completed only after containment returns the frozen callback error. | Expected completion cannot trigger callback. |
| `lifecycle.released` (1) | First actual `Release()` return. | `NONE` | B: true iff first release won. | No lease/disabled is false under actual execution. | Expected release cannot become a call result. |
| `lifecycle.sequence[*]` (1 schema path) | Independently recorded actual acquisition, completion/containment, and first successful release events. | `NONE` | B: ordered subsequence of `acquired`, `completed`, `released`. | Empty differs from nil/nonempty; order exact. | Expected sequence cannot seed recorder. |
| `derived.control.actual_attachment_available` (1) | Actual `RoundTripResult` private owner attachment discovery. | `NONE` | B: non-nil discoverable owner. | Disabled/missing owner false. | `control.attachment_available` and proposed enable bit expected agreement cannot supply actual. |
| `derived.control.actual_reason` (1) | Actual inspection failures: `field`, `owner:nil`, `acquire:false`, `view:false`, signature failure; empty only on success. | `NONE` | B: exact observed prerequisite reason. | Empty success differs from missing/unavailable. | `control.reason` cannot feed result. |
| `derived.external.{result_bytes,server_error_bytes,notifications_bytes,responses_bytes,failure,messages,bytes,request_messages,request_bytes}` (9) | Actual `RoundTripResult` bytes/messages/failure and actual frozen error serialization. | `NONE` | B: defensive copies, exact ordered concatenations, and actual counters. | Result/error bytes null iff unavailable; available empty response/notification sequence is empty bytes/string, not null; failure preserves nil versus present. | Expected A fields may be comparison targets but never actual operands; no synthesized response/error. |
| `derived.lifecycle.{observation_available,callback_completed,release_first,release_second}` (4) | Actual acquire+View, callback return/contained panic, first and second release returns. | `NONE` | B: direct actual formulas; second false proves one winner. | Disabled/no lease yields false actual outcomes; callback error remains transparent and panic contained. | Expected lifecycle and C controls cannot drive operations or returns. |
| `derived.identity.transaction_request_equals_request_identity` (1) | Equality over actual A transaction/request identity, including availability. | `NONE` | B: actual-to-actual identity equality. | Availability participates; unavailable is not silently equal to available-zero. | No expected identity operand. |

B cardinality: `1+1+1+1+1+1+9+4+1 = 20`.

Additional B assertions introduce no schema paths: World-25 `transport_equal` and `external_equal` are computed after both actual member executions; World-24 aggregate formulas are covered in §3.4.

### 3.3 Category C — retained nine-path ownership inventory, with reviewed W24 shape contradiction

For every row below the production transfer seam is exactly `NONE`. The retained ownership accounting classifies nine C schema paths, but the attachment-enable review establishes a material physical-shape fact: all 16 World-24 worker envelopes omit `expected.control` entirely. Therefore the proposal's universal all-85 cross-rule over the three `control.*` values below is unsatisfiable as written. This inventory does not manufacture those fields, inherit them from `root/24`, weaken the rule silently, or change the `116/20/9` / `81/55/9` accounting. The occurrence-sensitive C validation rule is unresolved and **BLOCKED** pending reviewed correction.

| Exact path (cardinality) | 1. Independently frozen fixture source | 2. Seam | 3. Resulting classification | 4. Unavailable/disabled/nil-empty semantics | 5. Anti-feed proof |
|---|---|---|---|---|---|
| `comparison.available` (1) | Strict fixture structure. | `NONE` | C: gates comparison container traversal/cardinality only. | False means no comparison organization, not no production capability. | Cannot select request or execution. |
| `concurrency.available` (1) | Strict fixture structure. | `NONE` | C: gates worker container traversal/cardinality only. | False means no concurrent fixture organization. | Cannot select owner/lifecycle. |
| `control.attachment_available` (1) | Fixture classification where physically present. The proposal's all-85 cross-rule is **BLOCKED** because this path is absent from all 16 W24 workers. | `NONE` | C only; at most an optional downstream agreement check under a future exact occurrence-sensitive rule. | Disabled member classified unavailable where present; absence in W24 must remain explicit and cannot imply an actual. | Never request input or actual/B operand; never borrow `root/24` control. |
| `control.enabled` (1) | Fixture pair label where physically present. The proposal's all-85 cross-rule is **BLOCKED** because this path is absent from all 16 W24 workers. | `NONE` | C only; not execution carrier. Any future cross-check is optional validation after independent bit derivation and must state its exact occurrence domain. | False labels disabled fixture member where present; W24 absence is not false. | Must not choose enablement; proposed `driver.request.attachment_enabled` is the sole lawful independent request operand if later accepted. |
| `control.observation_available` (1) | Fixture classification where physically present. The proposal's all-85 cross-rule is **BLOCKED** because this path is absent from all 16 W24 workers. | `NONE` | C only; at most an optional downstream agreement check under a future exact occurrence-sensitive rule. | Disabled member classified unavailable where present; W24 absence is not unavailable/false. | Never fabricates view/acquisition and never borrows root control. |
| `control.paired_world` (1) | Fixture pairing metadata. | `NONE` | C: pair closure/order only. | Missing/invalid pair is fixture RED. | Cannot dispatch sibling or share its actual. |
| `control.reason` (1) | Fixture explanation. | `NONE` | C only. | Empty/present is fixture data, never refusal. | Cannot feed `refusal.reason` or B actual reason. |
| `external.execution_expected` (1) | Fixed fixture declaration that harness must conduct represented execution. | `NONE` | C: require a real attempted execution; moved from predecessor A inventory. | False/missing is fixture invalid under current fixed contract, not a production outcome. | Cannot populate `external.available` or result/error. |
| `name` (1) | Human/subtest/manifest label. | `NONE` | C: organization only. | Renaming must leave request and actual execution identical. | No name/world/scenario dispatch or lookup. |

C cardinality: `1+1+1+1+1+1+1+1+1 = 9`.

Containers (`comparison.executions`, `comparison.pairs`, pair `disabled`/`enabled`, relationship declarations/reasons, `concurrency.workers`, manifest paths/names, traversal positions) are non-path C structure. They do not alter the 145 denominator.

### 3.4 Occurrence-specific World-24 root migration — 35 paths

Occurrence key: `root/24::<path>`. Exact members are:

```text
round_trip_ledger.dimensions.callbacks.{before,after,delta,available,known_zero}
round_trip_ledger.dimensions.diagnostics.{before,after,delta,available,known_zero}
round_trip_ledger.dimensions.external_bytes.{before,after,delta,available,known_zero}
round_trip_ledger.dimensions.leases.{before,after,delta,available,known_zero}
round_trip_ledger.dimensions.reservations.{before,after,delta,available,known_zero}
round_trip_ledger.dimensions.transport_bytes.{before,after,delta,available,known_zero}
round_trip_ledger.dimensions.transport_messages.{before,after,delta,available,known_zero}
```

| Occurrence+path family (cardinality) | 1. Source | 2. Seam | 3. Result | 4. Unavailable semantics | 5. Anti-feed proof |
|---|---|---|---|---|---|
| `root/24::round_trip_ledger.dimensions.<7 dimensions>.<5 members>` (35) | The 16 `w24-worker/01..16` independently executed actual A ledgers, in fixed worker order. | `NONE` | B, not root production state: `available=AND workers`; `before=0`; if available, `delta=SUM worker.actual.delta`, `after=before+delta`, `known_zero=available && delta==0`. | Any unavailable required worker makes aggregate unavailable under existing normalization; never substitute zero or expected. | Root expected aggregate, worker expected values, world/name, and production root aggregate storage are forbidden. A production root aggregate field is ownership RED even if equal. |

All other 110 `root/24` paths retain their ordinary categories: 81 A, 20 ordinary B, 9 C. Each W24 worker remains ordinary `116/20/9`.

## 4. Conditional-behavior branch decision table

Every production-owned conditional outcome below must have an independent carrier. `BLOCKED` means the current documents do not freeze an accepted production carrier, even where a finite candidate seam is proposed.

| Decision/outcome | Independently frozen carrier or actual source | Production-owned result / classification | Coverage and nil/empty/order rule | Carrier status |
|---|---|---|---|---|
| Attachment available versus absent | Lawful independent mapping: **PROPOSED** `driver.request.attachment_enabled`, authored offline as 74 true / 11 false without reading C; finite `S-ENABLE`. | Enabled may publish owner; disabled publishes no owner/observation while transport still executes. | Required/non-null if accepted; no inferred default. C controls may only provide an optional downstream cross-check on an explicitly lawful occurrence domain. They cannot derive the bit. | **BLOCKED** — independent review confirms mapping/seam plausibility but blocks the amendment because all 16 W24 workers omit `expected.control`; carrier is not frozen and occurrence-sensitive C validation is unresolved. |
| Selected commitment available/unavailable | Explicit selected commitment input. | A selected metadata. | False→empty identity, zero length/digest, null bytes; availability is not validity. | Input current; production owner carrier blocked. |
| Handoff available/unavailable | Explicit handoff input. | A handoff metadata. | Same normalization; independent of enablement. | Input current; production owner carrier blocked. |
| Predecessor count/order/availability | Literal ordered predecessor input array. | A ordered metadata; defensive copies. | 0/1/2 cardinality as materialized; unavailable element retained in place; nil/empty setter input canonicalizes no predecessors, not one unavailable predecessor. | `S-PRED` disposable-reviewed; production blocked. |
| Request `Params` nil / empty / present | Exact `driver.request` value and encoded actual request frame. | A request-frame custody; external transport. | Preserve null, zero-length/empty, and present bytes distinctly; no removed request body/payload projection. | Runtime seam blocked. |
| Original request frame | Actual complete encoded outbound frame. | A `frames.request.*`. | Unavailable is false/null/0/zero digest; bytes copied defensively. | `S-IDFRAME` only disposable; production blocked. |
| Original response frame: valid | Successfully emitted ordered `response-frame` bytes. | A `frames.response.*`; transport/external actuals. | Concatenate exact complete frames in order. | Runtime seam blocked. |
| Original response frame: malformed | Successfully emitted malformed frame bytes. | A frame remains available/raw; separate actual error/failure/terminal. | Never parse/normalize before custody. | Runtime seam blocked. |
| Original response frame: no frame | No successfully emitted response-frame event, including cancel/deadline/shutdown/unavailable. | A false/null/0/zero digest; response byte/message accounting available-known-zero after attempted execution. | Available empty external response sequence `""` differs from unavailable/null. | Runtime seam blocked. |
| Result unavailable / available-empty / present | Actual `RoundTripResult.Result`. | A `result.*`; B `derived.external.result_bytes`. | Null iff unavailable; empty bytes remain available-empty. | Runtime seam blocked. |
| Error unavailable / present | Actual generic server-error observation. | A `error.*`; B serialized error bytes. | No frame/control event does not fabricate error payload; nullable details stay null. | Runtime seam blocked. |
| Refusal false/true | Actual generic admission result. | A `refusal.*`. | Commitment absence/zero content is not refusal policy; proposed disabled publication is not inferred refusal. | Runtime seam blocked. |
| Terminal unavailable/value | Actual Manager terminal disposition. | A terminal and terminal-ledger snapshots. | Unavailable distinct from empty; closure prevents new callbacks. | Owner/lifecycle seam blocked. |
| Accounting/transport/external/ledger availability and zero | Actual Manager snapshots and transport/external observations. | A local observations; B formulas/equalities/aggregates. | Unavailable ≠ zero; `known_zero` only when available and actual value/delta is zero. | Runtime seam blocked. |
| Owner construction/publication | Proposed explicit enable bit plus otherwise admissible actual request. | Private owner in `RoundTripResult`. | Disabled: none; enabled: eligible even if every commitment unavailable/zero-normalized. | **BLOCKED** pending `S-ENABLE` acceptance and production gate. |
| View acquisition | Actual private lease acquisition and `View()` returns. | B acquired/observation availability/reason. | No owner or post-terminal acquisition fails closed. | `S-LEASE` only disposable; production blocked. |
| Callback success | Actual admitted callback returns nil. | B completed/callback_completed and lifecycle order. | Completion recorded before release. | `S-LEASE` only disposable; production blocked. |
| Callback error | Actual admitted callback returns error. | B completion plus transparent same error. | Error does not prevent containment/drain accounting. | `S-LEASE` only disposable; production blocked. |
| Callback panic | Actual admitted callback panics and is contained. | B completed only after containment returns frozen callback error. | Panic cannot escape or skip drain. | `S-LEASE` only disposable; production blocked. |
| Concurrent terminal/release/drain | Actual admitted callback set, terminal request, condition-variable wait, release calls. | B sequence, release-first/second; terminal A. | Terminal closes admission; admitted callbacks drain; exactly one release true; subsequent operations fail closed. | `S-LEASE` only disposable; production blocked. |
| Defensive copying | Caller mutation probes and repeated view/callback observations. | A byte/slice stability plus causal negative assertion. | No caller, holder, request, view, callback, or sibling alias; predecessor equality value-based, never pointer-based. | Proposed/disposable design; production blocked. |
| World-24 aggregates | Sixteen worker actual A ledgers. | 35 `root/24` B values. | Fixed order; AND availability; no unavailable→zero substitution. | Formula current; executions/production blocked. |
| World-25 disabled/enabled pair | **PROPOSED** independently materialized false/true request bits; independent commitments and executions. | Disabled no private publication; enabled eligible; external behavior compared. | Eleven ordered pairs, both members executed; no inherited driver/actual. | **BLOCKED** pending attachment carrier. |
| World-25 transport equality | Two members' actual complete request/response frames, messages, request messages, bytes and request bytes. | Additional B assertion, no schema path. | Deep equality after both executions, preserving nil/empty. | Formula current; execution blocked. |
| World-25 external equality | Two members' independently captured actual generic external observations. | Additional B assertion, no schema path. | Equality computed actual-to-actual only. | Formula current; execution blocked. |

## 5. Mechanical occurrence coverage — exactly once

### 5.1 Occurrence-class census

| Occurrence class | Explicit members | Count | Ownership per member | Retained path assertions |
|---|---|---:|---:|---:|
| Roots except World-24 | `root/01..23`, `root/25` | 24 | `116 A + 20 B + 9 C = 145` | 3,480 |
| World-24 root | `root/24` | 1 | `81 A + 55 B + 9 C = 145` | 145 |
| World-24 workers | `w24-worker/01..16` | 16 | `116 A + 20 B + 9 C = 145` | 2,320 |
| World-25 executions | `w25-execution/01..22` | 22 | `116 A + 20 B + 9 C = 145` | 3,190 |
| World-25 pair members | `w25-pair/01/disabled`, `/enabled` through pair 11, in disabled-then-enabled order | 22 | `116 A + 20 B + 9 C = 145` | 3,190 |
| **Total** | exact structural records above | **85** | occurrence-sensitive exact-once ownership | **12,325** |

### 5.2 Category totals over occurrence+path keys

| Category | Mechanical calculation | Exact total |
|---|---:|---:|
| A | `84 ordinary × 116 + root/24 × 81` | **9,825** |
| B | `84 ordinary × 20 + root/24 × 55` | **1,735** |
| C | `85 × 9` | **765** |
| **Union** | `9,825 + 1,735 + 765` | **12,325 = 85 × 145** |
| Duplicate occurrence+path keys | required | **0** |
| Missing occurrence+path keys | required | **0** |
| Removed/deferred paths admitted | `85 × 17` checked absent | **0 present** |

The 17 absent paths remain the four request-body, four request-payload, four response-body, four response-result paths, and `result.form`. They are not aliases, hidden carriers, or members of the 145.

### 5.3 Exact-once key rule

The assertion key is `(occurrence_id, retained_path)`, never `retained_path` alone. For every key:

1. choose the ordinary partition unless `occurrence_id == root/24` and the path is one of the explicit 35 aggregate members;
2. for that exact exception choose B instead of A;
3. assert/derive/validate once under the chosen category;
4. prohibit skip lists, wildcard exemptions, duplicate assertions, fallback-to-expected, record/update mode, and golden rewrite.

## 6. Anti-feed proof obligations

A review must establish all of these statically and, only under later authority, causally at runtime:

1. Final envelopes are self-contained; registry IDs and occurrence IDs are offline authoring joins only.
2. `expected`, including commitments and `expected.control.*`, never constructs a request, attachment, transport event, owner, view, actual observation, or B operand.
3. `name`, world number, pair ordinal/member label, comparison/concurrency containers, and fixture reasons never dispatch production or choose driver values.
4. Commitment availability/content never enables, admits, refuses, classifies, or selects result/error/terminal behavior.
5. The proposed enable bit is not inferred from attachment zero value, commitment content, expected controls, transport contents, result contents, or hidden state. Its lawful independent mapping is separate from any optional C agreement check.
6. No all-85 C cross-check may pretend the three `expected.control.*` paths exist in the 16 W24 workers; no root borrowing, manufactured controls, or silent “where present” weakening is permitted.
7. Production retains observations only; RED computes B. In particular production must expose no World-24 root aggregate.
8. World-25 equality is actual-to-actual after two executions, not expected-to-expected or sibling inheritance.
9. Raw response custody comes only from successfully emitted frame bytes; malformed remains raw and control events emit no response frame.
10. Defensive copies eliminate mutable aliasing; predecessor holder pointer identity has no equality meaning.
11. No semantic commitment validity, direction, target, relation, qualification, completeness, authenticity, publication authority, or production use is inferred.

## 7. Current blockers and required review disposition

| Item | Current evidence | Required disposition |
|---|---|---|
| 145-path schema and 85 occurrence structure | `CURRENT`; exact finite inventories and reviewed corpus evidence exist. | Preserve; no mutation authorized here. |
| Ordinary and World-24 occurrence ownership | `CURRENT`; exact `116/20/9` and `81/55/9` rules exist. | Review this transcription and exact-set arithmetic. |
| `driver.request.attachment_enabled` | `PROPOSED`; review confirms the independent 11-false/74-true mapping needs no expected value, but the amendment is BLOCKED and the field is absent from accepted frozen corpus evidence. | **BLOCKED** until the proposal is corrected, independently re-reviewed/frozen, and separately authorized rematerialization exists. |
| Optional C agreement cross-check | Proposal incorrectly requires all three `expected.control.*` values for all 85 occurrences; all 16 W24 workers omit `expected.control`. Adding controls, borrowing root controls, or silently using “where present” is unauthorized. | **BLOCKED** pending an exact occurrence-sensitive rule preserving absences, or a separate exact path/value/ownership amendment. This cross-check must remain separate from independent bit derivation. |
| Finite enable production seam | `PROPOSED` `S-ENABLE`; review finds it finite/reviewable but issues BLOCKED because the containing amendment is inconsistent. No accepted prototype/C15 delta or production gate exists. | **BLOCKED** pending corrected design review, then any separately authorized disposable evidence, finite delta review, disposal, final write-set and named writer. |
| Other owner/view/lease production carriers | Prior disposable design/review evidence, but no production authority established by the inputs used here. | **BLOCKED**; do not describe as implemented or accepted production. |
| Runtime causal verification | Explicitly paused by this task. | Not performed; no test/prototype/source/corpus execution or mutation. |
| Semantic commitment validity | Out of scope and forbidden inference. | No disposition; opaque custody/equality only. |

## 8. Final status

```text
REVIEW_REQUIRED
NO MUTATION
PRODUCTION BLOCKED
```

This inventory accounts mechanically for every retained path and occurrence-specific ownership key, but it grants no corpus, RED, source, prototype, test, production, semantic, writer, commit, or activation authority.
