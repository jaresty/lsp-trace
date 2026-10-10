# Independent review — ADR 0011 Go/gopls incoming CALLS target resolution design

- **Review type:** independent exact-source joint design review, Track A
- **Repository:** `/Users/schwa/dev/lsp-trace-adr0011-c17`
- **Reviewed HEAD:** `e403b9c243c853a59a176949a52a712c893a5399`
- **Reviewed design:** `docs/qualification/adr0011-calls-go-gopls-incoming-target-resolution-design.md`
- **Design SHA-256:** `1385f84a218dea17648be0bc253d1aa87d81e824ff9fd3184263f9f857d245cc`
- **Companion capture design SHA-256:** `79547b57a598b3700699fc4e909db01ed35a2205a798ff0980a7732753e25527`
- **Decision:** `ADR0011_CALLS_INCOMING_TARGET_RESOLUTION_DESIGN_GO`
- **Authority ceiling:** `provider_authority=UNAVAILABLE`, `authority=0`, `accepted=false`, `completeness=UNKNOWN`
- **Production admission:** `REFUSED_MISSING_EVIDENCE`

## Review boundary

This review decides only whether the exact pinned target-resolution design is mechanically precise enough to freeze as a design. It is not implementation, evidence production, acquisition authorization, profile qualification, production admission, provider authority, a commit authorization, or `ADR0011_QUALIFIED_CALLS_INCOMING_PROFILE_GO`.

The companion capture-seam design was reviewed jointly at the envelope/prepare interface, but its artifact, authority, proposed write-set, and decision remain distinct. Neither decision implies the other.

## Evidence and historical pins

The review preserves these predecessor boundaries:

- the corrected qualification plan was accepted only as `ADR0011_CALLS_INCOMING_PLAN_GO` in `adr0011-calls-go-gopls-incoming-review-v2.md`; it supplied no evidence or profile GO;
- acquisition v1 independently accepted only four authority-zero custody roles as `PRESENT`: repository commit/dirty-state custody, workspace-root identity, target-file bytes/digest/URI/language, and managed session/generation custody;
- acquisition v1 did not retain exact JSON-RPC wire identity, initialize/capability custody, document-symbol custody, or complete prepared-item identity;
- acquisition v2 at this HEAD was correctly stopped before semantic effects, with zero workspace derivations and zero `documentSymbol`, prepare, incoming, normalization, and occurrence dispatches;
- acquisition v2 retained the manifest unchanged at SHA-256 `334a6a1f3c26b0408023d24f8bd08f74bfbcd0125629ee1cf04fdd425fa6ce87`, with four `PRESENT`, ten `PLANNED`, and thirty-one `MISSING` roles;
- the accepted v2 stop/review remains `BLOCKED` for acquisition and leaves production admission refused.

No predecessor evidence, manifest, design, source, or review is amended by this artifact.

## Findings

### 1. Modes, common schema, and provenance are closed

`DIRECT_POSITION` and `DOCUMENT_SYMBOL_BOUND` are mandatory explicit modes. They emit the same closed `TargetPositionEnvelopeV1` schema while retaining distinct mode, method, policy, evidence, result-digest, and authority provenance. Inapplicable direct-mode fields receive explicit mode-defined values rather than omission. Unknown fields refuse.

There is no default mode, cross-mode fallback, retry, alternate locator, name or regex lookup, prepare-based repair, normalization, clamping, or best-effort selection. A refusal in one mode cannot select or relabel the other mode.

### 2. Document-symbol request and custody are exact and bounded

The document-symbol mode permits exactly one `textDocument/documentSymbol` request whose params contain the exact managed document URI. The design requires the complete hierarchical `DocumentSymbol[]` result in original nested server order and retains exact request/result bytes, lengths, digests, transaction identity, method, document/session/generation binding, negotiated encoding, limits, duration, and capability evidence.

Flat `SymbolInformation[]`, mixed shapes, partial/progress results, omitted children, reordered or reconstructed results, malformed JSON, duplicate keys, `null`, and truncated bytes refuse. Complete-result storage is reserved before dispatch; an unretainable complete result cannot emit an envelope.

Capability, workspace revision/custody, source bytes/version/digest/length, provider identity, session/generation/lease, initialize reference, position encoding, policies, schemas, and limits are frozen before the first resolver effect and rechecked afterward. Direct mode records document-symbol capability as not exercised and cannot use capability absence to substitute modes.

### 3. Flattening, selection, ordinal, and identity are deterministic

Depth-first pre-order flattening preserves raw sibling order. The flattened zero-based ordinal is assigned before filtering, and the hierarchy path is the exact zero-based child-index vector. The design prohibits lexical sorting, deduplication, name/kind preference, range-size tie-breaking, provider-specific normalization, and response-dependent policy changes.

Selection uses one frozen strict-most-specific containment policy. Zero candidates, tied deepest candidates, crossing or non-nested candidates, duplicate paths, and malformed candidates refuse. The selected ordinal and hierarchy path must address the same raw symbol. The canonical symbol identity binds URI, version, source digest, session/generation, encoding, method, ordinal, path, name/detail/kind, ranges, complete result digest, and selection policy under a fixed domain.

Raw response, flattening ledger, canonical symbol identity, and target envelope remain separate immutable artifacts.

### 4. Position derivation and prepare handoff are exact

For document-symbol mode, `chosen_position` is exactly `selectionRange.start` under the negotiated encoding. Direct mode preserves the exact caller position. Neither mode repairs or relocates the position.

The envelope hash binds the closed canonical envelope. The companion capture design accepts only that complete envelope, verifies its digest and custody, and mechanically copies `{canonical_uri, chosen_position}` into prepare. It retains an explicit hash link and equality proof against owner-observed prepare params. This is a sufficient unchanged-handoff contract without merging resolver and capture authority.

### 5. Refusal behavior is complete and fail-closed

The algebra covers absent/unknown mode, unavailable capability, unproved workspace/source/session custody, foreign document/session, stale or substituted state, transport/protocol failure, missing or malformed result, symbol absence or ambiguity, invalid direct binding, invalid position, limit refusal, partial capture, and cancellation.

Ambiguity, absence, malformed data, stale/foreign/substituted custody, and limit failure are refusals rather than empty success. No refusal emits an envelope, dispatches prepare, or changes modes. Accounting is independent across request/result/source/diagnostic/recorder/time and structural dimensions, with positive finite frozen bounds, checked overflow, no borrowing, and pre-dispatch complete-result reservation.

### 6. Authority is strictly target selection

Document-symbol evidence may establish only bounded target selection and exact start-position derivation. It cannot establish CALLS occurrences, caller/callee relationships, direction, grouping, runtime behavior, feature identity, source completeness, provider correctness, or production authority. It cannot substitute for, repair, or corroborate prepare/incoming evidence.

Direct binding evidence has only caller-and-bound-record authority. Hash links preserve lineage but do not merge artifact identity or authority.

### 7. Joint interface and production isolation are coherent

The resolver ends after one envelope or one refusal and never dispatches prepare or incoming. The capture seam never performs document-symbol resolution. The two designs therefore interlock through one immutable envelope while retaining distinct IDs, schemas, evidence, authority, review gates, write-sets, and RED matrices.

The proposed resolver implementation is limited to new qualification-only files. Static reachability must prove no production constructor, CLI, MCP, registry, runtime, build-tag, environment, config, or source path can activate it. Any required production hook invalidates the proposed write-set and returns the design to review.

## Decision

The pinned target-resolution design closes the acquisition-v2 target-selection defect with deterministic, bounded, provenance-preserving mechanics and a strict authority ceiling. Its interface with capture is exact without conflating artifacts or CALLS authority.

This decision freezes the design only. It authorizes no implementation now, no acquisition, no evidence or manifest change, no source change, no profile qualification, no production admission, no provider-authority claim, no commit, and no GO other than the exact design token below.

ADR0011_CALLS_INCOMING_TARGET_RESOLUTION_DESIGN_GO
