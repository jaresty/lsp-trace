# Private raw-custody and 32 MiB directional measurement design

Status: `AUTHORED_NOT_GO`

## 1. Decision and exclusions

Retain `callHierarchyLayer1GenericOwner` as the sole generic terminal custody owner. Add, only in a separately authorized phase, a genuinely separate Manager-owned directional raw-custody reservation ledger. The new ledger is not a C15 account, child lease, backing owner, or charge against `privateB4ByteAccountV2`/`privateB4MaxOwnedBytes`. It governs whether one direction-specific qualification attempt may retain its bounded raw-custody set; C15 independently describes every backing allocation, alias, transfer, and release.

Layer 1 remains generic, opaque, and direction-neutral. Incoming and outgoing wrappers remain separate borrowers/consumers with separate attempt identities, scanners, assemblers, evidence, accounting records, semantic review, and GO tokens. Neither wrapper becomes a terminal owner.

Forbidden: public schema/API; CLI/MCP/registry/profile/config/build-tag activation; global limit expansion; duplicate generic owner or terminal; remarshal/reconstruction custody; digest-as-byte substitute; production result/graph mutation; fallback/retry; or ordinary-path entry.

## 2. Exact independent chains

Incoming:

```text
verified private target binding
→ reserve incoming slot and 33_554_432-byte maximum under Manager lock
→ prepare request exact frame/raw params
→ prepare response complete frame/raw result, or bounded incomplete scanner prefix
→ scan original result token and select zero-based ordinal/raw span
→ assemble {"item":<byte-identical selected span>}
→ incoming request/response exact custody, or bounded incomplete scanner prefix
→ private incoming observation
→ clear directional backing references and release reservation exactly once
→ generic terminal owner release/reuse
```

Outgoing is identical only through selection, then uses its own outgoing assembler, request, response, observation, and reservation. An incoming reservation, artifact, or review cannot satisfy outgoing, and vice versa.

Handoff requires byte equality, length equality, and matching SHA-256; hashes are integrity links only. Frames, lexical form, order, duplicate multiplicity, whitespace, key order, and token spelling remain unchanged.

## 3. Separate Manager-owned reservation ledger

Freeze these private v1 names/contracts:

```text
privateCallHierarchyDirectionalMeasurementMaxBytesV1 = 33554432
privateCallHierarchyDirectionalAggregateMaxBytesV1   = 33554432
privateCallHierarchyDirectionalMaxActiveSlotsV1      = 1
privateCallHierarchyDirectionalReservationV1
privateCallHierarchyDirectionalLedgerV1
```

One active slot is a qualification-version policy chosen to make aggregate custody equal one per-attempt ceiling and to avoid unqualified concurrency multiplication. It is not inferred from observed workload, server behavior, or production demand. A source requirement disproving one-slot operation requires a new reviewed version; v1 must not silently increase it.

The Manager owns the ledger and mutates it only while holding existing Manager synchronization. Admission is one atomic checked transaction before provider write, scanner retention, capture-cap increase, or allocation: validate private/default-off eligibility; require `active_slots == 0`; checked-add requested bytes to live bytes; require requested and resulting live bytes `<= 33554432`; then commit slot, attempt identity, reserved bytes, and counters together. Failure leaves all fields unchanged.

Per-attempt ceiling and aggregate live ceiling are both exactly `33554432`. Exact limit is accepted when every independent guard admits. `33554433`, checked-add overflow, or a second active attempt is rejected before avoidable effect. Release clears all directional references/bytes first, then atomically decrements live bytes and slot exactly once. Duplicate release is false/no-op. Reuse receives a fresh attempt/generation identity; stale leases cannot view reused backing.

Required ledger fields: direction; attempt identity; allowed/requested/admitted bytes; live/aggregate/slot before and after; cumulative admitted/released; component entries; release identity; refusal stage/reason. Cumulative counters do not grant capacity.

## 4. Exhaustive backing-to-ledger mapping and non-duplication

Every retained backing reachable from the directional attempt has exactly one mapping row:

| Backing role | C15 classification | Directional ledger component |
|---|---|---|
| prepare request frame/params | existing or new enumerated owner/copy/alias/transfer row | exact retained capacity once |
| prepare response frame/result token | existing or new enumerated row | exact retained capacity once |
| failed-read scanner prefix | new scanner-prefix backing row | exact prefix capacity once |
| selected raw item | alias if bounded owner view; otherwise separately charged C15 copy | zero for alias; exact copy capacity once |
| assembled directional request/frame | separately charged copy unless transport ownership proves transfer | exact retained capacity once |
| directional response frame/result | existing backing reuse, transfer, or separately charged copy after census | exact distinct retained capacity once |
| observation/record backing | enumerated owner/copy rows | exact distinct retained capacity once |

Non-duplication invariant:

```text
for each live backing identity B and attempt A:
  exactly one C15 ownership classification exists for B
  at most one directional component (A,B) exists
  directional_sum(A) = sum(distinct mapped backing capacities for A)
  no alias contributes bytes
  no transfer creates a second charge
  no C15 charge is debited from or nested beneath the directional ledger
```

Relationship/delta census joins the two scopes by stable attempt ID, backing ID, source fingerprint, and operation ID. It reports C15 ownership deltas and directional reservation deltas in separate tables, then a relationship table proves complete mapping and non-duplication. It never merges totals or treats a reservation as a C15 owner.

## 5. Failed-read custody at the transport scanner

Current `internal/lspwire/read_frame.go` withholds all bytes when `ReadWithExactFrame` or `ReadWithFrameIfWithin` fails. V1 therefore requires an explicit private scanner result at that exact transport boundary; current source does not satisfy this requirement.

Proposed `ReadWithBoundedIncompleteFrameV1` (unexported or otherwise private without public surface) owns a pre-reserved bounded capture buffer before reading. On malformed decode/framing, timeout/cancellation, short EOF, or other partial read it returns the bytes actually consumed up to the reservation as `INCOMPLETE_OPAQUE_PREFIX`, with actual length, digest, limit, terminal cause, and `complete=false`. It never reconstructs missing header/body bytes, never calls the prefix a frame/result/message, never decodes it into semantic success, and never salvages members/ranges. Zero bytes read is distinct from a present empty prefix.

The Manager transfers or copies that prefix into a census-enumerated immutable backing already mapped to the active directional reservation. The sole generic terminal owner retains the terminal and opaque prefix custody; directional wrappers only borrow or use a separately mapped copy. Prefix mapping IDs are newly namespaced directional correlation metadata only: they cannot be predecessor IDs, history identities, history entry IDs, lineage identities, or inputs to any predecessor/history digest, ordering, classification, or lifecycle decision. Prefix lifetime ends with exact-once terminal release; clearing precedes slot/byte reuse. If even bounded prefix custody cannot be reserved before read, refuse before provider write. If transport fails after write, retain only actually read bounded bytes and emit zero semantics.

## 6. Frozen proposed private symbols/files

Current exact scanner path:

- `internal/lspwire/read_frame.go`: `framedReadCapture.add`, `Reader.ReadWithExactFrame`, `Reader.ReadWithFrameIfWithin`.
- `internal/lspwire/successor_ingress.go`: `SuccessorIngressReader.ReadFrame`, `readMore`, `consumeCopy`, `topLevelResultSpan`.
- `sessionruntime/sessionruntime.go`: `Manager.roundTripWithPrivateTransaction`, `readResult` and the real transport loop.

Future exact private seams (names frozen for v1 review):

- `internal/lspwire/read_frame.go`: `boundedIncompleteFrameV1`, `Reader.readWithBoundedIncompleteFrameV1`.
- `sessionruntime/call_hierarchy_directional_raw_v1_private.go`: `reserveCallHierarchyDirectionalV1Locked`, `releaseCallHierarchyDirectionalV1Locked`, `scanCallHierarchyRawArrayV1`, `selectCallHierarchyPreparedOrdinalV1`, `assembleIncomingCallHierarchyParamsV1`, `assembleOutgoingCallHierarchyParamsV1`, `handoffCallHierarchySelectedRawV1`.
- `sessionruntime/b4_owned_method_pair_lease.go`: generic frame/result/prefix publication through the existing sole owner only.

The scanner and selector are shared direction-neutral mechanics; incoming/outgoing assembler functions and wrapper state remain separate.

## 7. Real 16 MiB path and private 32 MiB reachability

The real Manager path is `RoundTrip` → `roundTripWithPrivate` → `roundTripWithPrivateTransaction` in `sessionruntime/sessionruntime.go`, which creates the managed transport reader and enforces request `MaxBytes`. The current exact/capture readers ultimately use `internal/lspwire` limits; `DefaultLimits().MaxBodyBytes` is `16 << 20`. Private B4 admission additionally rejects above `sessionruntime/privateB4MaxBytes = 16 << 20`. Provider provisioning caps `MaxProviderLimits.ResponseBytes` at `16 << 20`, but that is a provisioning/configuration ceiling rather than proof that every Manager response traverses provider package code. Therefore the causal claim is limited to the demonstrated Manager/private-B4 and `lspwire` guards; provider is an independent configuration constraint, not asserted as the first runtime guard.

A response above 16 MiB cannot reach complete directional custody through the current managed private path. Current revision is `BLOCKED_NOT_RED` for a >16 MiB behavioral RED.

A future reachable path must remain private and default-off: atomically reserve the one slot/32 MiB ledger first; attach a private transaction-scoped effective body/capture bound to that reservation; pass it through Manager to the exact scanner; leave `DefaultLimits`, public request schemas, provider public/global limits, and nil/disabled behavior unchanged. The override expires with that reservation. Any required public/global change is `REVISE`. Direct owner/scanner injection, altered global constants, or post-guard fixture bytes are non-causal.

## 8. Typed states and compatibility

`null`, `[]`, nonempty arrays, protocol error/`-32601`, malformed complete bytes, timeout, cancellation, incomplete prefix, absent bytes, pre-effect refusal, and complete success remain distinct. Incomplete prefixes are custody evidence only and always produce zero semantic effects.

Disabled and `CALLS_ONLY` paths perform no new allocation, hashing, scheduling, diagnostic, or branch-visible work and remain byte/value identical. Their readiness-history entry count, exact entry bytes, digests/cuts, directions, order, history identity, acquired/outstanding bytes, receipt-owner and borrower counts, retirement state, predecessor commitment list/bytes/order, and generic attempt-lineage fields must equal an otherwise identical baseline execution. Required future compatibility tests snapshot those values before and after disabled and `CALLS_ONLY` controls and require byte/value equality plus zero prefix-mapping IDs.

**No-history-change invariant:** this design may observe existing opaque predecessor commitments but cannot append, remove, replace, reorder, reinterpret, hash into, or otherwise alter readiness history, immutable-history metadata, predecessor semantics, generic attempt lineage, generation identity, or lifecycle transitions. `sessionruntime/readiness_history_private.go` and `sessionruntime/owned_method_pair.go` are pinned dependencies and not write targets. If the proposed generic-owner extension cannot preserve this invariant without touching history behavior, stop `REVISE`.

Authority remains `0`; source-graph completeness remains `UNKNOWN`; accepted remains false.

## 9. Hard stops and unresolved evidence

Stop with `REVISE` if a private seam is impossible, a public/global change is required, ownership duplicates, a second terminal appears, raw bytes are reconstructed, one-slot policy is bypassed, the ledgers cannot be separately and exhaustively reconciled, any history/lineage dependency must change behavior, or any predecessor/history/lineage byte, value, identity, order, ownership count, lifecycle transition, or semantic meaning differs.

Managed neighborhood evidence did not establish CALLS (`TRUNCATED`, then `INVALID_SERVER_RESPONSE/TRAVERSAL_FAILED`). Target-only projection confirmed the generic source at authority 0 and unknown completeness. No runtime evidence yet proves private 32 MiB reachability, failed-read prefix custody, release/reuse at that size, or accepted C15/directional deltas.
