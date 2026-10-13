# Architecture and qualification design

## 1. Non-negotiable invariants

- `sessionruntime.Manager` and its private Generic Layer 1 owner are the **sole** terminal and byte-custody authority.
- Layer 1 captures exact framed request/response bytes, copies them into manager-owned storage, accounts before allocation, and owns refusal, terminal transition, callback drain, zeroing, release, STOP/RESTART retirement, and C15 reconciliation.
- Incoming/outgoing adapters never retain an alias into the owner. They receive a bounded immutable copied view while a Layer 1 callback lease is active, produce a direction-specific immutable transfer, and return before callback completion.
- A directional transfer cannot release, extend, mutate, reacquire, or terminalize Layer 1 custody. It cannot contain owner pointers, mutable slices/maps, callbacks, or a direction union.
- No directional semantic effect occurs until the complete predecessor chain and exact selected-item handoff are joined. Refusal is before effects.
- All new machinery is private and default-off. No CLI, MCP schema, environment, config, registry, capability, build tag, exported API, or production constructor enables it.
- Omitted selector behavior and selected `CALLS_ONLY` behavior must preserve legacy request bytes, response bytes, transaction order, and emitted graph bytes exactly. Capture metadata is out-of-band and cannot reorder or remarshal the legacy path.

## 2. Generic Layer 1 ownership model

### Allocation and admission

Layer 1 reserves checked logical capacity before each owned copy. Exact equality with the selected live cap is admitted; plus one returns `RESOURCE_EXHAUSTED` before allocation or publication. Aliases add references but not bytes; copies reserve their actual capacity. Transfers move an already charged ownership identity without double charge. Any failed multi-reservation rolls back atomically.

### Lifetime and release

The owner state is `(open, active_callbacks, release_started, released)`. Lease admission requires `open && !released`. Terminal request closes admission, waits for `active_callbacks==0`, zeroes the observation, releases all C15 charges exactly once, and broadcasts completion. Explicit lease release competes through the same exact-once release operation. STOP detaches all matching-generation owners; RESTART detaches only the retired generation; both join release before deletion or successor use.

### C15 implications

The directional design reuses the C15 `privateB4ByteAccountV2` ownership algebra but receives no separate C15 authority. Every retained frame/span/view copy must be classified as owner, alias, copy, transfer, or release and added to the existing private ownership census before implementation review. The selected live-owned cap is 33,554,432 bytes. Outstanding ownership decreases on release; cumulative reservation work does not. Directional transfer bytes remain charged to the Generic Layer 1 transaction until callback return and must not escape. A direction-specific artifact requires a new separately charged owner after immutable copy; failure to reserve it refuses before artifact creation.

## 3. Incoming model (independently authored)

Exact chain:

`documentSymbol transaction → prepareCallHierarchy transaction → selected prepared callee item → incomingCalls transaction → incoming member ledger`

Incoming identity is a closed record containing direction=`INCOMING`, its own profile ID, provider ID, provider binary identity, initialize identity, workspace/session/generation/revision, target URI/source identity, three transaction keys and method ordinals, selected prepared-item ordinal and exact original span digest, unchanged handoff digest, limits identity, and terminal identity. An outgoing predecessor/profile/provider can never satisfy it.

For response member ordinal `r`:

- returned `from` is the caller; queried prepared target is the callee;
- each `fromRanges[s]` is caller-side source occurrence `(r,s)`;
- duplicates, repeated members, self-loops, and parallel pairs remain distinct;
- non-null `fromRanges:[]` yields one **zero-site member record**, `ranges=0`, `occurrences=0`, `edges=0`; it is not repaired, inferred, or discarded;
- missing, null, scalar, object, malformed range, or partial member makes the complete incoming transaction malformed with no prefix admission.

Accounting identity:

`members_seen = members_with_sites + zero_site_members`; `ranges_seen = sum(valid range array lengths)`; `occurrences_admitted = ranges_seen` only after whole-transaction validation; `edges_emitted = occurrences_admitted` before any separately named pair projection. Refused transactions have `occurrences_admitted=0` and `edges_emitted=0` while retaining observed/refused counters.

## 4. Outgoing model (independently authored)

Exact production chain:

`traceExecutor.Execute → ResolveSession/Metadata/PrepareDocument/resolveSymbol → explicitSeedSpec → acquisitionorchestration.ExecuteExplicitTrace[Prepared] → outgoing traversal → outgoing member ledger`

Exact qualification chain:

`documentSymbol transaction → prepareCallHierarchy transaction → selected prepared caller item → outgoingCalls transaction → outgoing member ledger`

Outgoing identity is a separate closed record containing direction=`OUTGOING`, a distinct profile ID and provider ID, its own binary/initialize/workspace/session/generation/revision custody, transaction keys/ordinals, selected caller span and handoff, limits, and terminal. Incoming evidence cannot populate any field.

For response member ordinal `r`, returned `to` is the callee, selected prepared item is the caller, and `fromRanges[s]` are caller-side sites. Empty-array policy is independently reviewed even if its mechanical accounting matches incoming. No outgoing assembler may call or import the incoming assembler or its semantic types.

## 5. Availability state machine and precedence

States:

1. `DISABLED` — private selector absent; no recorder allocation or behavior change.
2. `REQUESTED` — direction-specific private request exists but no authority inferred.
3. `PRECHECK_REFUSED` — identity/capability/profile/revision/limit prerequisite failed before wire effects.
4. `L1_ACTIVE` — Generic Layer 1 owns admitted transaction and exact frame custody.
5. `L1_TERMINAL_UNAVAILABLE` — transaction terminal exists but complete immutable view cannot be supplied.
6. `TRANSFER_AVAILABLE` — complete immutable direction-neutral Layer 1 view supplied during callback.
7. `DIRECTION_REFUSED` — directional identity, selection, handoff, grammar, or accounting failed; no semantic effects.
8. `DIRECTION_COMPLETE_EMPTY` — complete valid top-level `[]`/qualified null policy, or only zero-site members; no occurrences.
9. `DIRECTION_COMPLETE_NONEMPTY` — complete occurrence ledger produced, still authority zero.
10. `RELEASED` — callback returned; Layer 1 drained and zeroed; no later access.

Precedence is earliest causally reached:

`disabled → identity/profile/revision/capability preflight → deadline → header/frame → cumulative wire → message count → decoded message → raw result → source/buffer → object/event → retention → readback → directional grammar/selection/handoff → terminal release`.

An earlier reached failure cannot be relabeled by a later condition. Server protocol error precedes result grammar because there is no successful result token to interpret. Cancellation/timeout classification follows the exact Layer 1 transport terminal. Lifecycle terminalization prevents later directional success.

## 6. Numeric qualification hard-stop

No limit is selected by this package. The exact-transport originals are explicitly `PROPOSED` and are pinned only as informative dependencies; they cannot justify architecture numbers. `numeric-bound-audit.log` audits every previously listed candidate with retained source/measurement, observed maximum or governing requirement, explicit margin, unit/counter owner, exact-limit witness, plus-one typed refusal, exact shadowing guard, and disposition.

Only the existing 33,554,432-byte C15 baseline has a current governing implementation and general equality/+1 evidence; it still lacks the required directional delta census and therefore cannot authorize this design. All other candidate rows lack accepted authority or retained measurement and are `BLOCKED_NOT_RED_UNJUSTIFIED`. The architecture hard-stops rather than inventing margins, treating invocation defaults as requirements, or using proposed policy as authority.

## 7. Exact minimal future write-sets (not authorized)

### Shared Generic Layer 1

- `sessionruntime/b4_owned_method_pair_lease.go`: extend private neutral owner/view/accounting and immutable callback transfer only.
- `sessionruntime/sessionruntime.go`: attach capture to the existing transaction finalizer/refusal/lifecycle joins only.
- one existing private sessionruntime test file selected at implementation review, or one new `_private_test.go` only if existing ownership prevents a coherent causal RED.

No other shared file is in the minimal set. In particular, no exported surface or public schema.

### Incoming only

- `incomingops/executor.go`: private request plumbing and incoming-only transfer consumption at the existing `SessionClient`/`traverse.Incoming` seam.
- `internal/traverse/incoming.go` (exact filename must be confirmed structurally before implementation): incoming-only member/range occurrence ledger and empty-range semantics.
- incoming-owned private tests adjacent to those files.

Must not modify `trace_executor.go`, outgoing traversal, shared semantic types, or public input schema.

### Outgoing only — exact HEAD census

- `cmd/lsp-trace-mcp/trace_executor.go`, `(*traceExecutor).Execute` (lines 92–178): private qualification plumbing at the existing dispatch to `acquisitionorchestration.ExecuteExplicitTracePrepared` / `ExecuteExplicitTrace`, without changing public `input`.
- `internal/slicer/discover.go`, `slicer.Client`, `DiscoverPrepared`, and `Discover`; exact outgoing effect site line 214 calls `client.OutgoingCalls(ctx, q.item)`. `Discover` owns BFS queue/expanded state, validation, ranges, nodes, edges, terminals, diagnostics, completeness, and truncation; the managed runtime owns wire transactions.
- mutation candidates only if separately authorized: `cmd/lsp-trace-mcp/trace_executor_test.go`, `internal/slicer/discover_test.go`, and `sliceops/provenance_wire_test.go`. `sliceops/retained_calls_wire_test.go` is verification-only unless a causal failure proves mutation necessary.

The exact census and hashes are retained in `outgoing-structural-census.log`. Must not modify `incomingops/executor.go`, public schemas/surfaces, incoming traversal, or incoming semantic types.

Any broader write-set requires a new design review.

## 8. Public-surface identification

Existing public surfaces potentially traversed are MCP `incoming`/slice/trace operations and their graph artifacts. This package adds **none**. The qualification selector, owners, views, profiles, provider identities, ledgers, and review tokens remain package-private and `json:"-"`. Any proposed public field, operation, schema member, registry entry, environment variable, config key, capability advertisement, or exported Go symbol is a design violation and requires a new versioned package.

## 9. Rejected alternatives

- Directional owners terminalizing or releasing custody: rejected as duplicate authority.
- Shared incoming/outgoing enum or occurrence assembler: rejected because it permits identity and authority crossover.
- Reversing outgoing evidence to claim incoming (or vice versa): rejected; method semantics and provider chains differ.
- Treating empty `fromRanges` as one inferred edge: rejected because no call site exists.
- Dropping empty members: rejected because it loses provider evidence and breaks accounting.
- Treating missing/null as empty: rejected as repair.
- Remarshalling request/result for identity: rejected because byte custody is lost.
- Enabling via current public selectors: rejected; qualification remains private/default-off.
- Deriving provider authority from hashes, same-process status, source inspection, or one live result: rejected.
- Reusing old conflicting numeric proposals: rejected in favor of selected exact-transport originals.
- Broad source/test edits “for convenience”: rejected; write-sets are directionally minimal.
