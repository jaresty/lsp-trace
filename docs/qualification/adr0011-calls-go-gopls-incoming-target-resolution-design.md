# ADR 0011 Go/gopls incoming CALLS target-resolution design

**Profile:** `CALLS_GO_GOPLS_INCOMING_EXACT_V1`  
**Design state:** design-only; unimplemented; qualification-only; default disabled  
**Repository pin:** `/Users/schwa/dev/lsp-trace-adr0011-c17` at `e403b9c243c853a59a176949a52a712c893a5399`  
**Review gate:** `ADR0011_CALLS_INCOMING_TARGET_RESOLUTION_DESIGN_GO` (not issued)  
**Authority:** `provider_authority=UNAVAILABLE`, `authority=0`, `accepted=false`, `completeness=UNKNOWN`  
**Production admission:** `REFUSED_MISSING_EVIDENCE`

This document designs target resolution only. It is not implementation, captured evidence, review, acquisition authorization, profile admission, or production authority. It is a distinct artifact from the capture-seam design and the two must be reviewed together. The accepted acquisition-v2 stop-before-effects evidence/review remains unchanged.

## 1. Decision and boundary

Exactly two explicit modes produce the same closed target-position-envelope schema:

- `DIRECT_POSITION`: the caller supplies the exact zero-based position. This mode is retained for replay and advanced qualification.
- `DOCUMENT_SYMBOL_BOUND`: a complete bounded `textDocument/documentSymbol` result is used to select exactly one symbol and derive the exact start position.

Mode is mandatory. There is no default, fallback, retry into the other mode, alternate locator, name lookup, regex lookup, prepare-driven repair, or best-effort selection. Each mode has distinct provenance and resolver evidence. Neither mode may relabel its artifact as the other.

Target resolution ends after emitting one immutable target-position envelope or one refusal. It does not dispatch `textDocument/prepareCallHierarchy` or `callHierarchy/incomingCalls`.

## 2. Common target-position-envelope schema

Both modes emit the same schema and required fields:

```text
TargetPositionEnvelopeV1 = {
  schema,
  profile_id,
  resolution_mode,
  canonical_uri,
  prepared_document_version,
  prepared_source_byte_length,
  prepared_source_sha256,
  workspace_revision,
  workspace_revision_custody,
  session_id,
  generation,
  provider_identity_ref,
  negotiated_position_encoding,
  resolver_method,
  canonical_symbol_id,
  selected_symbol_ordinal,
  symbol_kind,
  hierarchy_path,
  range,
  selection_range,
  chosen_position,
  selection_policy_id,
  resolver_evidence_id,
  capability_evidence_id,
  source_custody_evidence_id,
  session_custody_evidence_id,
  resolver_result_sha256,
  accounting,
  limits,
  provenance,
  envelope_sha256
}
```

The schema is closed: unknown fields refuse. URI, version, source digest and byte length, workspace revision/custody, session/generation, encoding, method, symbol ID and ordinal, kind, hierarchy path, range, selection range, exact chosen position, selection policy, and resolver evidence ID are mandatory in both modes. Fields inapplicable to direct input use explicit mode-defined values, never omission or `null`.

`envelope_sha256` is the SHA-256 of frozen canonical envelope bytes excluding only the `envelope_sha256` field, under domain `lsp-trace.adr0011.calls.go.gopls.incoming.target-position-envelope.v1`. Canonicalization is sorted-key UTF-8 JSON, no insignificant whitespace, duplicate keys forbidden, integers only.

### 2.1 Shared validity

- `canonical_uri` equals the managed prepared document URI.
- `prepared_document_version`, source bytes/digest, session, and generation are identical at resolver start and completion.
- `chosen_position` is valid under the negotiated encoding and lies in `selection_range` and `range` under the frozen half-open convention.
- `selection_range` lies within `range`.
- Capability, workspace, source, session, and generation custody are frozen before the first resolver effect and hash-linked into the envelope.
- A stale, foreign, substituted, mismatched, incomplete, or over-limit input produces no envelope.

## 3. `DIRECT_POSITION`

`DIRECT_POSITION` consumes an already frozen caller position plus a caller-selected, independently identified symbol binding record. It performs no LSP resolver request. The binding record supplies the mandatory canonical symbol ID, ordinal, kind, hierarchy path, range, and selection range and must independently bind the same URI/version/source/session/generation/encoding.

Frozen provenance:

```text
resolution_mode = DIRECT_POSITION
resolver_method = DIRECT_POSITION
selected_symbol_ordinal = ordinal from the direct binding record
chosen_position = caller position, unchanged
selection_policy_id = DIRECT_POSITION_EXACT_V1
resolver_evidence_id = direct binding evidence ID
resolver_result_sha256 = digest of the complete direct binding record
provenance.authority = CALLER_AND_BOUND_RECORD_ONLY
```

The caller position is neither moved to a symbol start nor repaired. Missing or inconsistent symbol binding refuses. This mode is for exact replay and advanced qualification; it does not claim that a document-symbol request occurred.

## 4. `DOCUMENT_SYMBOL_BOUND`

The complete mandatory layer is:

```text
exact request
→ complete nested result in raw server order
→ deterministic flattening and selection
→ selected raw ordinal + canonical symbol identity
→ exact chosen position
→ immutable target-position envelope
→ later prepare consumes that position unchanged
```

### 4.1 Exact request and complete result

The only resolver method is `textDocument/documentSymbol`. Params contain exactly the envelope URI. The managed owner retains exact request params bytes and complete result bytes, lengths, SHA-256 digests, transaction key, method, session/generation, document version/source digest, encoding, capability evidence ID, limits, and monotonic duration.

The result must be one complete bounded hierarchical `DocumentSymbol[]`. Flat `SymbolInformation[]`, partial results, progress fragments, omitted children, reconstructed order, malformed JSON, duplicate keys, `null`, mixed shapes, or truncated bytes refuse. The exact nested raw-order result is retained as a distinct resolver artifact; it is not the target-position envelope.

### 4.2 Deterministic flattening

Flatten depth-first pre-order, preserving each raw sibling order. The zero-based flattened ordinal is assigned before filtering. `hierarchy_path` is the zero-based child-index vector from the top-level array to the selected symbol. No lexical sorting, deduplication, name preference, kind preference, provider-specific normalization, range-size tie-breaker, or response-dependent policy change is allowed.

Each visited symbol must have a valid name, kind, range, selectionRange, and children shape; selectionRange must lie within range. The scanner accounts for every raw symbol, nesting level, JSON token, and byte before selection.

### 4.3 Selection and identity

The caller supplies an exact query position only as selection input. The frozen policy is `DOCUMENT_SYMBOL_UNIQUE_STRICT_MOST_SPECIFIC_CONTAINMENT_V1`:

1. retain symbols whose half-open `selectionRange` contains the query position;
2. retain only candidates whose selection range is strictly contained by every broader retained candidate on its hierarchy path;
3. choose the unique deepest retained candidate;
4. if zero candidates, tied deepest candidates, crossing/non-nested candidates, duplicate paths, or any malformed candidate exist, refuse;
5. derive `chosen_position` as the exact start of the selected symbol's `selectionRange` under the negotiated encoding.

The selected flattened ordinal and hierarchy path must address the same raw symbol. Its canonical identity is:

```text
sha256(
  "lsp-trace.adr0011.calls.go.gopls.incoming.document-symbol.v1" || 0x00 ||
  canonical({uri, version, source_sha256, session_id, generation, encoding,
             method, ordinal, hierarchy_path, name, detail, kind,
             range, selection_range, resolver_result_sha256,
             selection_policy_id})
)
```

The exact result and flattening ledger remain separate artifacts from the canonical identity and envelope.

### 4.4 Authority boundary

`textDocument/documentSymbol` authority is limited to selecting a symbol and deriving its start position. It cannot establish CALLS occurrences, caller/callee relationships, grouping, runtime behavior, feature identity, source completeness, provider correctness, or production authority. It cannot repair, corroborate, or substitute prepare/incoming evidence.

## 5. Frozen capability and custody

Before any resolver effect, freeze and later recheck:

- exact managed provider identity and `textDocument.documentSymbol` capability for `DOCUMENT_SYMBOL_BOUND`;
- exact call-hierarchy capability needed by the later capture attempt, without treating it as exercised here;
- workspace URI, clean revision identity, and custody status;
- canonical document URI, complete prepared source bytes, byte length, digest, and exact document version;
- managed session ID, generation, owner lease, initialize result reference, and negotiated position encoding;
- resolver policy/schema digests and all finite limits.

`DIRECT_POSITION` requires the same source/session/workspace custody but records document-symbol capability as `NOT_EXERCISED_BY_MODE`; absence cannot trigger mode substitution. `DOCUMENT_SYMBOL_BOUND` requires capability present and exactly pinned. Any change between freeze and completion is `STALE_OR_SUBSTITUTED` and yields no envelope.

## 6. Complete accounting and limits

Every attempt records `{allowed, observed, admitted, refused}` independently for:

- request bytes;
- result bytes;
- JSON tokens and depth;
- top-level symbols;
- total flattened symbols;
- children per symbol;
- hierarchy depth;
- symbol name/detail bytes;
- source bytes;
- diagnostics count and bytes;
- recorder objects and bytes;
- transaction and whole-attempt duration.

All limits are positive, finite, caller-declared within frozen hard maxima, overflow-checked, and fixed before effects. Capacity cannot be borrowed between dimensions. Complete-result reservation occurs before request dispatch; any inability to reserve refuses before write. An effect whose complete result cannot be retained is a failed attempt and cannot emit an envelope.

## 7. Refusal algebra

Target resolution emits either exactly one valid envelope or exactly one refusal:

- `MODE_ABSENT_OR_UNKNOWN`
- `CAPABILITY_UNAVAILABLE`
- `WORKSPACE_CUSTODY_UNPROVEN`
- `SOURCE_CUSTODY_UNPROVEN`
- `SESSION_CUSTODY_UNPROVEN`
- `FOREIGN_DOCUMENT_OR_SESSION`
- `STALE_OR_SUBSTITUTED`
- `TRANSPORT_OR_PROTOCOL_ERROR`
- `MISSING_RESULT`
- `MALFORMED_RESULT`
- `SYMBOL_ABSENT`
- `SYMBOL_AMBIGUOUS`
- `DIRECT_BINDING_INVALID`
- `POSITION_INVALID`
- `LIMIT_REFUSED`
- `PARTIAL_CAPTURE`
- `CANCELLED`

Ambiguity, absence, malformed data, stale/foreign/substituted custody, and any limit breach are refusals, not empty success. No refusal may dispatch prepare or select the other mode. Diagnostics are bounded and cannot contain unaccounted payload bytes.

## 8. Artifact separation and review

The following are distinct immutable artifacts with distinct IDs and authority:

1. capability/workspace/source/session custody bundle;
2. direct binding record or document-symbol request/result pair;
3. flattening/selection ledger;
4. target-position envelope;
5. capture attempt consuming that envelope.

Hash links do not merge authority. The target-resolution and capture-seam designs must be reviewed together because the latter accepts only this envelope; each keeps its own review gate and proposed write-set.

## 9. Proposed implementation write-set

No implementation is authorized. If and only if `ADR0011_CALLS_INCOMING_TARGET_RESOLUTION_DESIGN_GO` is independently issued, the proposed implementation write-set is limited to new qualification-only files:

1. `internal/adr0011incomingqualification/target_envelope_test.go` — closed envelope, canonical bytes, digest, shared custody checks.
2. `internal/adr0011incomingqualification/direct_target_test.go` — direct-position resolver and bound-record validation.
3. `internal/adr0011incomingqualification/document_symbol_target_test.go` — bounded raw hierarchical scan, flattening, selection, and identity.
4. `internal/adr0011incomingqualification/target_resolution_red_test.go` — target-resolution RED matrix.
5. `internal/adr0011incomingqualification/testdata/target-resolution/` — frozen fake-owner inputs and expected qualification artifacts.

No existing production file, schema, manifest, evidence, review, CLI, MCP, registry, runtime, provider, acquisition, traversal, or admission file is included. If a production hook is required, this write-set is invalid and the design returns to revision.

## 10. RED matrix

| ID | RED assertion |
|---|---|
| T01 | Mode is mandatory; unknown/absent mode has no resolver or prepare effect. |
| T02 | Both modes emit the identical closed envelope schema and distinct mode provenance. |
| T03 | No error, absence, ambiguity, or capability condition falls back between modes. |
| T04 | Direct mode preserves the exact caller position and requires a fully matching bound symbol record. |
| T05 | Document-symbol mode emits exactly one request and retains one complete nested raw-order result. |
| T06 | Flat, mixed, partial, truncated, malformed, duplicate-key, or omitted-child results refuse. |
| T07 | Depth-first pre-order flattening preserves sibling order, ordinal, and hierarchy path under permutations. |
| T08 | Unique strict-most-specific containment selects one symbol; absent, tied, crossing, or malformed candidates refuse. |
| T09 | Chosen position equals the exact selected `selectionRange.start`; no normalization or repair occurs. |
| T10 | Canonical symbol identity changes for URI/version/source/session/generation/encoding/method/ordinal/path/kind/range/selectionRange/result/policy substitutions. |
| T11 | Envelope binds every mandatory custody, symbol, position, policy, and evidence field; omission or unknown fields refuse. |
| T12 | Capability, workspace/source/session custody, stale, foreign, and substituted inputs fail before envelope emission. |
| T13 | Every independent bound reports complete accounting and refuses without borrowing. |
| T14 | Recorder short write or incomplete response yields no envelope and only `PARTIAL_CAPTURE`. |
| T15 | Document-symbol artifacts cannot satisfy CALLS occurrence, caller/callee, grouping, runtime, feature, or production-authority assertions. |
| T16 | Target-resolution artifacts and capture artifacts retain distinct IDs, schemas, hashes, and authority. |
| T17 | Static reachability proves no production constructor, CLI, MCP, registry, runtime, build-tag, environment, or config activation. |
| T18 | Every generated artifact remains authority zero and production-inadmissible. |

## 11. Current handoff

Current disposition is no authorization:

- `ADR0011_CALLS_INCOMING_TARGET_RESOLUTION_DESIGN_GO` is not issued;
- no implementation, acquisition, manifest change, evidence promotion, production hook, or profile admission is authorized;
- `provider_authority=UNAVAILABLE` and production admission remains `REFUSED_MISSING_EVIDENCE`;
- the acquisition-v2 evidence/review and its hashes remain unchanged.
