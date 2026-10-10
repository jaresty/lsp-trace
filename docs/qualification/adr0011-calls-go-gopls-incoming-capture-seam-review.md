# Independent review — ADR 0011 Go/gopls incoming CALLS capture seam design

- **Review type:** independent exact-source joint design review, Track A
- **Repository:** `/Users/schwa/dev/lsp-trace-adr0011-c17`
- **Reviewed HEAD:** `e403b9c243c853a59a176949a52a712c893a5399`
- **Reviewed design:** `docs/qualification/adr0011-calls-go-gopls-incoming-capture-seam-design.md`
- **Design SHA-256:** `79547b57a598b3700699fc4e909db01ed35a2205a798ff0980a7732753e25527`
- **Companion target-resolution design SHA-256:** `1385f84a218dea17648be0bc253d1aa87d81e824ff9fd3184263f9f857d245cc`
- **Decision:** `ADR0011_CALLS_INCOMING_CAPTURE_SEAM_DESIGN_GO`
- **Authority ceiling:** `provider_authority=UNAVAILABLE`, `authority=0`, `accepted=false`, `completeness=UNKNOWN`
- **Production admission:** `REFUSED_MISSING_EVIDENCE`

## Review boundary

This review decides only whether the exact pinned capture-seam design is mechanically precise enough to freeze as a design. It is not implementation, evidence production, acquisition authorization, profile qualification, production admission, provider authority, a commit authorization, or `ADR0011_QUALIFIED_CALLS_INCOMING_PROFILE_GO`.

The target-resolution design was reviewed jointly at the envelope/prepare interface, but its artifact, authority, proposed write-set, and decision remain distinct. Neither decision implies the other.

## Evidence and historical pins

The review preserves these predecessor boundaries:

- the corrected qualification plan was accepted only as a coherent plan freeze and supplied no evidence or profile GO;
- acquisition v1 accepted exactly four authority-zero custody roles and explicitly lacked original prepare/incoming frames, raw order/cardinality, null-versus-array form, complete prepared-item identity, document-symbol custody, initialize/capability custody, and managed provider/toolchain custody;
- acquisition v2 was correctly stopped before effects at reviewed HEAD `e403b9c243c853a59a176949a52a712c893a5399`;
- v2 recorded zero workspace derivations and zero prepare, incoming, document-symbol, normalization, occurrence, and production-visible effects;
- v2 preserved the evidence manifest at SHA-256 `334a6a1f3c26b0408023d24f8bd08f74bfbcd0125629ee1cf04fdd425fa6ce87`, with four `PRESENT`, ten `PLANNED`, and thirty-one `MISSING` roles;
- the accepted v2 stop evidence remains acquisition-`BLOCKED`, `authority=0`, `accepted=false`, `completeness=UNKNOWN`, and `REFUSED_MISSING_EVIDENCE`.

No predecessor evidence, manifest, design, source, or review is amended by this artifact.

## Findings

### 1. The prepare/incoming chain is exact and original-byte preserving

The seam consumes exactly one valid immutable `TargetPositionEnvelopeV1`; loose coordinates cannot reach prepare. It verifies the closed schema, canonical envelope digest, mode provenance, predecessor evidence, profile, document/workspace/session/provider custody, encoding, symbol binding, ranges, position, accounting, and limits before reservation or dispatch.

Prepare params contain exactly the envelope URI and chosen position. They are copied without conversion, repair, clamping, normalization, remapping, or resolver rerun. The design retains owner-observed params bytes, method, request ordinal, transaction key, envelope ID/digest, exact URI-position hash link, decoded equality proof, and the complete session/version/source/encoding/provider binding.

The prepare result is scanned once as a complete original-byte JSON array. Raw array order and every top-level item span are preserved. Canonical bytes identify the selected prepared item, while exact raw bytes prove original order and custody; canonical bytes are never sent to gopls.

The incoming request is mechanically assembled from a fixed prefix, the exact selected raw item bytes, and a fixed suffix. The item cannot be remarshal-derived, reordered, whitespace-normalized, substituted, or repaired. Equality and digest proofs bind the incoming item subspan to the selected prepare span.

### 2. Prepared-item selection and handoff are deterministic

Every prepare member is structurally checked. Candidates must match the envelope URI, canonical symbol constraints, kind, range/selectionRange binding, and frozen containment predicate. Duplicate identities do not collapse ordinals. Exactly one candidate must match; zero matches refuse as missing and multiple matches refuse as ambiguous before incoming.

The seam retains raw ordinal, exact selected span and digest, containing-result digest, canonical item bytes/digest, canonical prepared-item identity, and predecessor links. This satisfies the corrected plan’s requirement for exact ordinal-plus-item identity and unchanged selected-item handoff.

The capture seam never performs target resolution or dispatches `textDocument/documentSymbol`. There is no second resolver policy or fallback path.

### 3. Terminals and accounting are exhaustive

Every attempt emits exactly one of `COMPLETE`, `SUCCESSFUL_EMPTY`, `TARGET_ENVELOPE_REFUSED`, `TRANSPORT_ERROR`, `PROTOCOL_ERROR`, `MISSING_PAYLOAD`, `MALFORMED`, `AMBIGUOUS_SELECTION`, `PARTIAL_CAPTURE`, `LIMIT_REFUSED`, or `CANCELLED` under an explicit precedence.

Exact incoming `[]` is successful empty; prepare `[]` is missing payload because no exact prepared selection exists. Null, malformed, partial, omitted-member/range, cancellation, timeout cause, protocol/server error, ambiguity, and recorder short-write behavior remain distinguishable and fail closed. No prefix admission or fallback follows a terminal condition.

Envelope, prepare request/response, incoming request/response, items, members, per-member and total `fromRanges`, tokens/depth, durations, diagnostics, and recorder objects/bytes each have independent positive finite frozen bounds and `{allowed, observed, admitted, refused}` accounting. No budget borrowing or response-driven increase is allowed. Complete storage is reserved before avoidable effects; pre-effect failure is `LIMIT_REFUSED`, while incomplete retention after an effect is `PARTIAL_CAPTURE`.

### 4. Enabled, disabled, and failure behavior is semantically transparent

For equal envelope, fake managed owner, and scripted frames, recorder-enabled and recorder-disabled executions must have identical ordered provider request bytes, transaction keys, selected ordinal/raw item, incoming bytes, responses, request counts, cancellation, deadlines, scheduling, and provider-visible order. Only separately accounted recorder allocations and the qualification artifact may differ.

Recorder failure is qualification-fatal but cannot cancel, retry, rewrite, suppress, replace, or reschedule managed-owner behavior. Panic cannot cross the owner boundary. Qualification artifacts cannot enter production result types or parsers.

These obligations make observer transparency testable rather than asserted.

### 5. The selected seam is minimal among the three material placements

Three placements are material to the v2 blocker:

1. **Managed process-boundary combined recorder.** The v2 proposal combined executable copying, toolchain/environment discovery, initialize and method-frame recording, and sink reservation. This is broader than the semantic pair being qualified, mixes custody acquisition with pair capture, and still leaves prepared selection and unchanged-item handoff unspecified.
2. **Managed-owner pair wrapper selected by this design.** The qualification-only wrapper uses the existing managed owner’s exact `RoundTrip` and method-pair views, adds only envelope verification, pair preflight, original-byte scanning, deterministic selection, unchanged-item assembly, bounded recording, and terminal classification. It is the narrowest point that still sees both authoritative owner bytes and the selection/handoff relation.
3. **Post-decode, acquisition, traversal, or normalization seam.** This is too late: original response bytes, raw order, duplicate ordinals, selected raw span, unchanged incoming item bytes, and pre-normalization failure accounting may already be lost or transformed.

The middle seam is therefore minimal for the stated proof obligations. It neither broadens into process/toolchain custody nor depends on lossy downstream normalization.

### 6. Test-only isolation prevents a second production semantic path

The wrapper is excluded from `incomingops.ResolvePreparedTarget`, acquisition runners, traversal, transient structural execution, CLI, MCP, registry, and production constructors. It may be reachable only from `_test.go` or a dedicated internal qualification package imported solely by tests. Production packages may not import it.

No build tag, environment variable, config, exported production constructor, `init`, plugin, reflection, CLI/MCP operation, registry entry, capability advertisement, or runtime dispatch may activate it. Static reachability is a required RED assertion. If exact owner-pair custody requires a production hook, the proposed write-set is invalid and the design returns to review.

The seam does not implement an alternate production resolver, client, transport, parser, traversal, or graph path. It invokes the same managed owner for a bounded qualification attempt and emits no production relation or result. Thus it creates no second production semantic execution path.

### 7. Authority and artifacts remain distinct

The target envelope, prepare pair, selected-item record, incoming pair, terminal/accounting record, and capture attempt have distinct IDs, schemas, hashes, and authority. Hash links establish lineage without merging authority.

Incoming raw results are retained as qualification evidence only. The seam does not normalize or admit caller items, `fromRanges`, CALLS occurrences, edges, direction, grouping, runtime assertions, feature identity, or production claims. Target-resolution authority cannot establish those facts.

Provider authority remains unresolved. A transparent recorder can preserve bytes and custody but cannot supply independent `provider_authority_for_exact_gopls_profile`. This unresolved role continues to block acquisition sufficiency, exact-profile qualification, and production admission; it does not block freezing this mechanical design.

## Decision

The pinned capture-seam design corrects the acquisition-v2 seam defects: it includes the exact prepare/incoming chain, target-envelope predecessor, deterministic ordinal/item selection, byte-identical handoff, exhaustive terminals and accounting, independent limits, pre-normalization refusal, test-only production unreachability, enabled/disabled and recorder-failure transparency, and a minimal owner-adjacent placement. It preserves separate target/capture artifacts and does not create a second production semantic path.

This decision freezes the design only. It authorizes no implementation now, no acquisition, no evidence or manifest change, no source change, no profile qualification, no production admission, no provider-authority claim, no commit, and no GO other than the exact design token below.

ADR0011_CALLS_INCOMING_CAPTURE_SEAM_DESIGN_GO
