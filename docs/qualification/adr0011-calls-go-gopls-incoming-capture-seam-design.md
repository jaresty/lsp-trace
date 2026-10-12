# ADR 0011 Go/gopls incoming CALLS capture-seam design

**Profile:** `CALLS_GO_GOPLS_INCOMING_EXACT_V1`  
**Design state:** design-only; unimplemented; qualification-only; default disabled  
**Repository pin:** `/Users/schwa/dev/lsp-trace-adr0011-c17` at `e403b9c243c853a59a176949a52a712c893a5399`  
**Review gate:** `ADR0011_CALLS_INCOMING_CAPTURE_SEAM_DESIGN_GO` (not issued)  
**Companion gate:** `ADR0011_CALLS_INCOMING_TARGET_RESOLUTION_DESIGN_GO` (not issued)  
**Authority:** `provider_authority=UNAVAILABLE`, `authority=0`, `accepted=false`, `completeness=UNKNOWN`  
**Production admission:** `REFUSED_MISSING_EVIDENCE`

This document designs only the mechanical prepare/incoming capture seam. Target resolution is specified separately in `adr0011-calls-go-gopls-incoming-target-resolution-design.md`. The artifacts and authority are distinct and the designs must be reviewed together. Neither gate implies the other. This is not implementation, captured evidence, review, acquisition authorization, profile admission, or production authority. The accepted acquisition-v2 stop-before-effects evidence/review remains unchanged.

## 1. Frozen scope

The seam serves exactly one profile:

- language `go`;
- provider `gopls` under separately established managed process custody;
- direction `INCOMING` only;
- exactly one valid immutable `TargetPositionEnvelopeV1` from explicit `DIRECT_POSITION` or `DOCUMENT_SYMBOL_BOUND` target resolution;
- one `textDocument/prepareCallHierarchy` transaction followed, only after unique deterministic prepared-item selection, by one `callHierarchy/incomingCalls` transaction;
- no traversal, retry, probing, alternate position, fallback, target resolution, document-symbol request, outgoing request, normalization, relation creation, occurrence admission, grouping, publication, or production admission.

The seam never accepts loose URI/line/character arguments. It consumes the complete target-position envelope, verifies its digest/custody/mode provenance, and copies its exact chosen position into prepare unchanged. Invalid, stale, foreign, substituted, ambiguous, malformed, or over-limit envelopes refuse before prepare.

## 2. Selected seam and isolation

Select a qualification-only wrapper around the existing managed provider owner. It calls the same managed `RoundTrip` owner used by the real managed session, once for prepare and once for incoming, and consumes owner-supplied exact method-pair views. It performs only envelope verification, profile preflight, exact prepare assembly, raw-result structural scanning, deterministic prepared-item selection, unchanged-byte incoming assembly, bounded recording, and terminal classification.

The wrapper is not inserted into `incomingops.ResolvePreparedTarget`, acquisition runners, traversal, transient structural execution, CLI, MCP, registry, or production constructors. It never invokes target resolution itself. This avoids a second resolver policy and preserves separate artifacts and authority.

Future qualification code must be reachable only from `_test.go` or a dedicated `internal/...qualification` package imported solely by tests. No build tag, environment variable, config, exported production constructor, `init`, plugin, reflection, CLI/MCP operation, registry entry, capability advertisement, or runtime dispatch may activate it. Production packages must not import it.

## 3. Envelope intake and exact prepare handoff

Before sink reservation or provider dispatch, verify:

- closed envelope schema and canonical `envelope_sha256`;
- profile, explicit mode, URI, prepared version/source digest/length, workspace revision/custody, session/generation, provider identity reference, negotiated encoding, method, symbol ID/ordinal/kind/path/ranges, exact chosen position, selection policy, resolver evidence ID, capability/source/session custody evidence IDs, accounting, and limits;
- mode-specific provenance without fallback or substitution;
- exact managed owner metadata still equals the envelope's URI/version/source/session/generation/encoding/provider binding;
- chosen position is valid and contained in the envelope selection range/range.

The prepare logical params are exactly:

```json
{"textDocument":{"uri":"<envelope canonical_uri>"},"position":{"line":<envelope chosen_position.line>,"character":<envelope chosen_position.character>}}
```

The URI and position are copied mechanically without repair, conversion, clamping, normalization, resolver rerun, or remapping. Request construction retains:

- exact owner-observed params bytes, length, and SHA-256;
- method `textDocument/prepareCallHierarchy` and request ordinal `0`;
- envelope ID and full envelope SHA-256;
- an exact canonical hash link over `{canonical_uri, chosen_position}` from envelope to request;
- byte offsets of URI, line, and character values in the request where structurally available;
- equality assertions that decoded request URI and position equal the envelope values exactly;
- session/generation/version/source/encoding/provider binding, owner transaction key, limits, and duration.

Any failed equality or hash link refuses before write. The owner-returned request bytes, not reconstructed bytes, are authoritative for capture. The envelope remains a distinct predecessor artifact; its authority is not merged into the prepare pair.

## 4. Mechanical transaction chain

The retained chain is:

```text
target-position envelope bytes + digest
→ exact unchanged envelope position hash-linked into prepare request bytes
→ complete ordered prepare raw-result bytes
→ selected zero-based prepared-item ordinal
→ exact selected raw item bytes + canonical prepared-item identity
→ incoming request bytes containing those exact unchanged item bytes
→ complete ordered incoming raw-result bytes
→ exactly one terminal
```

Each arrow has predecessor/successor artifact IDs and SHA-256 links. Missing links never produce success.

### 4.1 Complete prepare result and selection

The prepare result must be a complete JSON array. A bounded scanner walks original result bytes once, preserving array order and locating every top-level item as an exact byte span. It does not unmarshal/remarshal for custody.

Selection is deterministic:

1. every member is structurally checked as a complete `CallHierarchyItem`;
2. every candidate must have valid URI, name, kind, range, and selectionRange;
3. candidates must match the envelope URI, canonical symbol identity constraints, kind, and selected range/selectionRange binding defined by the profile;
4. the unchanged envelope chosen position must satisfy the frozen containment predicate;
5. duplicate identities do not collapse ordinals;
6. exactly one candidate must match; its zero-based raw array ordinal is selected.

Zero matches, multiple matches, malformed members, unscannable boundaries, stale/substituted symbol fields, or any bound breach refuse before incoming. No lexical sorting, normalization, alternate target, fallback, or resolver request participates.

Retain selected ordinal, exact raw item span, byte length/digest, containing-result digest, frozen canonical item bytes/digest, and canonical prepared-item identity under domain `lsp-trace.adr0011.calls.go.gopls.incoming.exact.v1/prepared-item/v1`. Canonical bytes identify; exact raw bytes prove source order and unchanged handoff. Canonical bytes are never sent to gopls.

### 4.2 Exact unchanged incoming handoff

Assemble incoming params as fixed ASCII prefix `{"item":`, exact selected raw item bytes, and `}`. The item bytes may be copied into a reserved owner buffer but may not be parsed, substituted, reordered, whitespace-normalized, or remarshal-derived.

Retain method `callHierarchy/incomingCalls`, request ordinal `1`, exact owner-observed params bytes/length/digest, selected item digest and canonical identity, item subspan offset/length, equality proof, envelope predecessor ID, prepare pair ID, and owner transaction key. Require:

```text
incoming_item_bytes = selected_raw_item_bytes
sha256(incoming_item_bytes) = selected_raw_item_sha256
```

Inequality, missing pair, hash mismatch, or inability to reserve the complete incoming chain refuses before incoming write. A written request not completely captured is `PARTIAL_CAPTURE`, never success.

### 4.3 Complete incoming result

Retain exact managed-owner result bytes, length, digest, response order, member boundaries, caller items, and `fromRanges` accounting. Successful non-empty results must be one complete JSON array. Exact `[]` is successful empty. `null`, partial arrays, malformed members/ranges, omitted members, or omitted ranges are not rewritten.

No caller item, range, CALLS occurrence, edge, caller/callee interpretation, grouping, runtime assertion, feature identity, or production claim is normalized or admitted. The raw result is qualification evidence only. Target-resolution authority does not establish any of these facts.

## 5. Terminal algebra

Every attempt emits exactly one terminal:

| Terminal | Condition | Success |
|---|---|---:|
| `COMPLETE` | Both transactions/captures complete; incoming result is valid non-empty array; all links, custody, limits, and accounting reconcile. | yes |
| `SUCCESSFUL_EMPTY` | Both transactions/captures complete; incoming result is exactly `[]`; all links, custody, limits, and accounting reconcile. | yes |
| `TARGET_ENVELOPE_REFUSED` | Envelope schema, provenance, custody, digest, mode, position, symbol binding, or predecessor evidence is invalid/stale/foreign/substituted. | no |
| `TRANSPORT_ERROR` | Managed write/read/process failure with no more specific terminal. | no |
| `PROTOCOL_ERROR` | JSON-RPC envelope, ID, method/result correspondence, frame, or server error. | no |
| `MISSING_PAYLOAD` | Required request/result/item/pair payload is absent. | no |
| `MALFORMED` | Present payload is invalid JSON, wrong shape, duplicate-key-invalid, or contains malformed required item/member/range. | no |
| `AMBIGUOUS_SELECTION` | More than one prepared item satisfies the frozen predicate. | no |
| `PARTIAL_CAPTURE` | Any effect or response is retained only partly, or attempted/captured accounting differs. | no |
| `LIMIT_REFUSED` | Any independent finite bound cannot be reserved or would be exceeded before the next avoidable effect. | no |
| `CANCELLED` | Authoritative parent/deadline cancellation wins before complete success. | no |

Precedence: authoritative `CANCELLED`; then `PARTIAL_CAPTURE` after an effect; then pre-effect `TARGET_ENVELOPE_REFUSED` or `LIMIT_REFUSED`; then `TRANSPORT_ERROR`, `PROTOCOL_ERROR`, `MISSING_PAYLOAD`, `MALFORMED`, `AMBIGUOUS_SELECTION`; then successful empty or complete. Subordinate diagnostics are retained but only one terminal is emitted.

Prepare `[]` is `MISSING_PAYLOAD` with `NO_EXACT_PREPARED_SELECTION`, not successful empty. Timeout is a cancellation cause, not another terminal. There is no fallback after any terminal condition.

## 6. Finite limits and accounting

All values are positive, finite, caller-declared within hard maxima, frozen before dispatch, overflow-checked, and independently report `{allowed, observed, admitted, refused}`:

- envelope bytes and predecessor objects;
- prepare and incoming request bytes separately;
- prepare and incoming response bytes separately;
- prepared items;
- incoming members;
- `fromRanges` per member and total;
- JSON tokens and nesting depth;
- whole-attempt and per-transaction duration;
- diagnostics count, per-diagnostic bytes, and cumulative bytes;
- recorder bytes and objects.

No sharing or borrowing occurs. Limits cannot be increased from response contents. Recorder storage is separate from provider request/result budgets and cannot change bytes sent or accepted. Reserve complete bounded attempt storage before prepare and complete remaining chain before incoming. Pre-effect reservation failure refuses; post-effect failure is `PARTIAL_CAPTURE`. No normalization, graph mutation, publication, or external effect occurs after an unsatisfied bound is known.

## 7. Semantic transparency and failure isolation

For the same valid envelope, fake managed owner, and scripted frames, recorder-enabled and recorder-disabled executions have identical ordered provider request bytes, transaction keys, selected ordinal/raw item, incoming bytes, responses, request count, cancellation, deadlines, and provider-visible ordering. The only delta is separately accounted recorder-owned allocations and the returned qualification artifact.

Recorder failures are fail-closed for qualification but cannot cancel, retry, rewrite, suppress, replace, or reschedule managed owner behavior. Panic may not cross the owner boundary. Cleanup releases leases and bounded objects. Captured artifacts never enter production result types or parsers.

## 8. Checkable API sketch

```text
QualificationCapture.Run(context, TargetPositionEnvelopeV1,
                         CaptureLimits, ManagedPairRuntime, ReservedSink)
  -> CaptureAttempt

EnvelopeVerifier.Verify(envelope, exact managed metadata)
  -> verified immutable predecessor OR refusal

PrepareAssembler.AssembleVerifiedPosition(envelope)
  -> exact params + envelope-position equality/hash proof

ManagedPairRuntime.RoundTrip(method, exact params bytes, finite bounds,
                             capture-owned-pair=true, expected binding)
  -> owner-held exact params/result view + status

RawArrayScanner.Scan(result bytes, independent bounds)
  -> ordered byte spans + complete accounting

PreparedSelector.Select(envelope, ordered spans)
  -> unique ordinal/raw span/canonical identity OR refusal

IncomingAssembler.Assemble(selected raw span)
  -> exact {"item":<unchanged bytes>} + subspan/hash proof

ChainRecorder.Reserve / AppendEnvelope / AppendPrepare /
                    AppendSelection / AppendIncoming / Finish
```

Required invariants:

```text
count(terminals) = 1
success iff terminal in {COMPLETE, SUCCESSFUL_EMPTY}
incoming_dispatched implies unique_selection
prepare.request.position = envelope.chosen_position
prepare.position_link = sha256(canonical(envelope URI + chosen_position))
incoming_item_bytes = selected_raw_item_bytes
documentSymbol_dispatches_by_capture = 0
target_resolution_artifact_id != capture_artifact_id
provider_authority = UNAVAILABLE
authority = 0
```

## 9. Proposed implementation write-set

No implementation is authorized. If and only if both design gates are independently issued, the proposed capture implementation write-set is limited to new qualification-only files and is separate from the target-resolution write-set:

1. `internal/adr0011incomingqualification/capture_test.go` — envelope-consuming orchestration, terminal algebra, bounds, recorder.
2. `internal/adr0011incomingqualification/raw_array_test.go` — bounded original-byte prepare/incoming scanner and prepared-item identity.
3. `internal/adr0011incomingqualification/capture_red_test.go` — capture RED matrix and isolation/equivalence tests.
4. `internal/adr0011incomingqualification/testdata/capture/` — frozen fake-owner inputs and expected qualification artifacts.

No existing production file, manifest, evidence, review, target resolver, plan, source package, `incomingops`, session runtime, acquisition, CLI, MCP, registry, traversal, semantic vector, Track B, or production schema is included. If exact owner pair custody requires a production hook, this write-set is invalid and the design returns to revision.

## 10. Capture RED matrix

| ID | RED assertion |
|---|---|
| C01 | Loose direct coordinates are rejected; only a valid complete target-position envelope can reach prepare. |
| C02 | Both resolution modes are accepted through the identical envelope schema while retaining distinct provenance. |
| C03 | Invalid/stale/foreign/substituted envelope or predecessor evidence dispatches nothing and never triggers fallback/resolution. |
| C04 | Prepare URI/position equal envelope URI/chosen position exactly; conversion, clamping, remapping, or perturbation fails before write. |
| C05 | Envelope digest and exact URI/position hash link verify mechanically against owner-observed prepare bytes. |
| C06 | Capture never dispatches `textDocument/documentSymbol`; resolver and capture artifacts remain distinct. |
| C07 | Ordered prepare raw bytes and member ordinals survive; duplicate identities do not collapse order. |
| C08 | Zero prepared matches refuse as missing; multiple matches refuse as ambiguous before incoming. |
| C09 | Selected raw span, canonical prepared-item identity, containing-result hash, ordinal, and envelope symbol binding all verify. |
| C10 | Incoming item subspan is byte-identical to selected raw bytes; remarshal/key-order/whitespace substitution fails before write. |
| C11 | Prepare and incoming transaction keys/methods/ordinals are distinct and hash-linked into one chain. |
| C12 | Valid non-empty array yields only `COMPLETE`; exact `[]` yields only `SUCCESSFUL_EMPTY`. |
| C13 | Transport, protocol, missing, malformed, ambiguity, cancellation, envelope refusal, and each limit map to one terminal. |
| C14 | Truncation, omitted member/range, sink short write, or post-effect recorder failure yields only `PARTIAL_CAPTURE`. |
| C15 | Every independent limit refuses at its frozen boundary with complete accounting and no borrowing. |
| C16 | Per-member and total `fromRanges` reconcile at zero, boundary, and overflow cases. |
| C17 | Enabled/disabled provider requests, selection, incoming bytes, responses, cancellation, and scheduling are identical. |
| C18 | Static reachability proves no production constructor, CLI, MCP, registry, runtime, build-tag, environment, or config activation. |
| C19 | Target-resolution evidence cannot be admitted as CALLS occurrence/caller-callee/grouping/runtime/feature/production authority. |
| C20 | Every generated artifact remains authority zero, provider authority unavailable, and production-inadmissible. |

## 11. Review and verification boundary

The target-resolution and capture-seam documents require separate proposed implementation write-sets and RED matrices, but reviewers must evaluate them together at their interface. A target-resolution design approval alone cannot authorize capture work; a capture design approval alone cannot authorize resolver work. Neither authorizes acquisition, evidence promotion, manifest edits, production hooks, CLI/MCP/runtime registration, semantic-vector use, Track B changes, commit, profile admission, or authority increase.

After any later authorized implementation: retain causal RED output; independently review RED causality; implement only the relevant new qualification files; run focused qualification tests and static reachability checks; verify enabled/disabled equivalence and recorder faults; run bounded existing compatibility tests; then verify exact worktree status, evidence-v2 hashes, manifest state, `git diff --check`, and absence of generated evidence or issued admission tokens.

Passing proves only mechanical envelope consumption, isolation, and capture behavior. It does not establish document-symbol semantic truth, CALLS truth, source completeness, provider correctness, runtime behavior, feature identity, production suitability, or authority.

## 12. Current handoff

Current disposition is no authorization:

- `ADR0011_CALLS_INCOMING_CAPTURE_SEAM_DESIGN_GO` is not issued;
- `ADR0011_CALLS_INCOMING_TARGET_RESOLUTION_DESIGN_GO` is not issued;
- no implementation, acquisition, manifest change, evidence promotion, production hook, or profile admission is authorized;
- `provider_authority=UNAVAILABLE`, `authority=0`, and production admission remains `REFUSED_MISSING_EVIDENCE`;
- acquisition-v2 evidence/review and its hashes remain unchanged.
