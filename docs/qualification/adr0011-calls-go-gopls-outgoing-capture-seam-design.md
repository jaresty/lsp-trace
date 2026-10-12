# ADR 0011 — test-only outgoing CALLS capture seam design

- **Profile:** `CALLS_GO_GOPLS_OUTGOING_EXACT_V1`
- **Track:** B only
- **Status:** design only; unimplemented
- **Design gate:** `ADR0011_CALLS_OUTGOING_CAPTURE_SEAM_DESIGN_GO` is the only decision this document may receive
- **Evidence/profile GO:** not defined or issued here
- **Authority:** `0`
- **Accepted:** `false`
- **Completeness:** `UNKNOWN`
- **Production admission:** `REFUSED_MISSING_EVIDENCE`
- **Baseline:** exact accepted stop-review commit `9b98e2c7d71114be48e9a7485546fbcaf5f61e36`

## Decision boundary

Assumption: the accepted stop packet and its independent review establish only a source-visible observability gap, not live gopls evidence. Basis: they record zero semantic requests and identify manager-internal framed observations that are unavailable at the acquisition boundary. Therefore this ADR designs a seam but produces no implementation, test result, evidence, manifest transition, copy, profile GO, production enablement, semantic-vector execution, or qualification authority.

The design gate is exactly:

```text
ADR0011_CALLS_OUTGOING_CAPTURE_SEAM_DESIGN_GO
```

That token means only that this design is coherent enough for a separately authorized implementation proposal. It cannot authorize implementation, change a manifest role, make evidence `PRESENT`, qualify the profile, admit production, or substitute for an independent outgoing evidence review or profile GO. Production remains `REFUSED_MISSING_EVIDENCE` before and after this design.

## Required observable contract

Assumption: decoded Go values and deterministic re-encoding cannot establish original-wire identity. Basis: the accepted stop review identifies original manager-owned frames as the unavailable custody and the coordinator's `boundedJSON` records as normalized semantic records. Therefore the seam must expose immutable, bounded capture from the manager-owned transaction, before normalization or semantic effects.

For each opted-in transaction the test-only recorder returns one immutable record containing:

1. exact original framed request bytes and exact original framed response bytes, byte-for-byte as written/read;
2. exact raw JSON request params and raw JSON result token slices into, or mechanically copied from, those retained frames without re-encoding;
3. framing identity: content length, frame byte length, and the manager's completed write/read observations;
4. transaction identity: exact session ID, generation, request key/JSON-RPC ID, method, manager owner sequence, request ordinal, response ordinal, and expansion/probe ordinal supplied by the test-only caller;
5. terminal transport/protocol metadata sufficient to classify one outcome without consulting ambient state;
6. declared recorder limits and closed recorder accounting, separate from acquisition and qualification accounting; and
7. SHA-256 digests over every retained byte field, used as integrity links rather than replacements for bytes.

The recorder is generic only at this transaction-custody level. It may be shared by separately authorized incoming work, but it has no relation direction, profile name, caller/callee interpretation, occurrence semantics, qualification role, authority, review result, or GO token.

## Outgoing qualification wrapper

Assumption: the manager owns wire truth while the outgoing profile owns prepared-caller selection and occurrence meaning. Basis: `RoundTripResult` already carries manager observations, whereas the accepted outgoing plan defines target binding, caller-to-callee orientation, and `fromRanges`. Therefore qualification-specific assembly belongs in a private test-only outgoing wrapper immediately outside managed round trips, not in the generic recorder and not inside production normalization.

The wrapper accepts only the exact profile binding and method chain:

```text
CALLS_GO_GOPLS_OUTGOING_EXACT_V1
  textDocument/prepareCallHierarchy
  -> selected prepared caller
  -> callHierarchy/outgoingCalls
  -> caller-to-callee outgoing occurrence ledger
```

It retains:

- the complete ordered prepare raw result, including every item and source ordinal;
- the selected prepared-caller ordinal;
- the selected caller's complete canonical identity: `name`, `kind`, canonical `uri`, `range`, `selectionRange`, optional `detail`, optional `tags`, and opaque `data`, with field presence and exact raw item bytes preserved;
- a mechanical equality handoff proving that the unchanged selected item bytes are the exact `item` member used in `CallHierarchyOutgoingCallsParams`;
- the full outgoing raw result token and response bytes, including successful top-level `null` distinctly from `[]`;
- every outgoing response member's unchanged raw bytes and member ordinal;
- every `fromRanges` member's unchanged raw bytes and range ordinal; and
- each emitted `outgoingCalls` occurrence linked mechanically to the retained member and range bytes by SHA-256 plus their byte spans and ordinals.

A digest never stands in for missing bytes. The wrapper may compute canonical identity and CALLS occurrences only after all applicable capture and qualification limits pass. It does not rewrite, sort, deduplicate, salvage, or replace source bytes. Canonical output order is a derived view and retains all source ordinals.

### Mechanical item-to-occurrence linkage

For selected caller bytes `S`, outgoing member bytes `Mᵢ`, and caller-side range bytes `Rᵢⱼ`, an admitted occurrence carries:

```text
profile = CALLS_GO_GOPLS_OUTGOING_EXACT_V1
method_chain = [textDocument/prepareCallHierarchy, callHierarchy/outgoingCalls]
caller = complete canonical identity selected from S
callee = complete canonical identity of Mᵢ.to
call_site = decoded Rᵢⱼ in the caller document and negotiated encoding
selected_prepare_ordinal
outgoing_member_ordinal = i
from_range_ordinal = j
sha256(selected_item_bytes = S)
sha256(outgoing_request_item_bytes = S)
sha256(outgoing_member_bytes = Mᵢ)
sha256(from_range_bytes = Rᵢⱼ)
source byte spans for S, Mᵢ, and Rᵢⱼ
```

The two selected-item hashes must match and the byte sequences must be equal; hashes alone are insufficient. One occurrence is emitted per retained range. Duplicate equal ranges, repeated equal members, parallel edges, and self-loops remain distinct by ordinal. A non-null member with `fromRanges: []` remains one zero-site member and emits no occurrence. Missing or null member `fromRanges` is `MALFORMED`.

## Exclusive terminal outcomes

Assumption: terminal classification must not be inferred after partial normalization. Basis: the accepted plan requires atomic refusal and distinguishes successful top-level `null` from missing, failed, malformed, and partial payloads. Therefore exactly one of the following outcomes is assigned before semantic effects:

| Outcome | Exclusive condition | Qualified occurrences/effects |
| --- | --- | --- |
| `COMPLETE` | Both transactions and selected handoff are complete and valid; outgoing result is a complete non-null array; all members/ranges and capture/accounting checks pass. | Emit the complete occurrence ledger, including zero occurrences for `[]` or only zero-site members. |
| `SUCCESSFUL_EMPTY` | The outgoing transaction is terminally successful and complete and its present top-level raw result token is exactly JSON `null`. | Retain exact `null` bytes/token and successful accounting; emit zero members, ranges, occurrences, rejected rows, and production-visible effects. |
| `TRANSPORT_ERROR` | Request write or response read fails at the transport boundary, including connection loss or timeout represented as transport failure. | Zero qualified occurrences and zero semantic/production effects. |
| `PROTOCOL_ERROR` | A complete matching JSON-RPC error response or framing/ID/method protocol violation is retained. | Zero qualified occurrences and zero semantic/production effects. |
| `MISSING_PAYLOAD` | Required request/result/frame bytes are absent although no more specific transport or protocol terminal evidence accounts for the absence. | Zero qualified occurrences and zero semantic/production effects. |
| `MALFORMED` | Complete retained bytes violate required JSON/LSP/profile shape, including malformed prepared items, `to`, or member-level `fromRanges`. | Zero qualified occurrences; no prefix salvage. |
| `PARTIAL_CAPTURE` | Any required frame, raw token, ordered result, selected item, member, range, span, digest linkage, or accounting record is truncated or incomplete. | Zero qualified occurrences; no prefix salvage. |
| `LIMIT_REFUSED` | Any independent declared limit would be exceeded or arithmetic/accounting cannot be closed. | Refuse before normalization, occurrence emission, or other semantic effects. |
| `CANCELLED` | Cancellation wins the terminal race and its keyed manager observation is retained; it is not recast as transport failure or partial success. | Zero qualified occurrences and zero semantic/production effects. |

The partition is exhaustive and pairwise exclusive. Precedence is evidence-based: `CANCELLED` when keyed cancellation is terminal; otherwise `TRANSPORT_ERROR` for failed transport; otherwise `PROTOCOL_ERROR` for a complete protocol error/violation; otherwise `MISSING_PAYLOAD` for absent required payload; otherwise `PARTIAL_CAPTURE` for present but incomplete custody; otherwise `LIMIT_REFUSED` for pre-effect limit refusal; otherwise `MALFORMED` for complete invalid payload; otherwise `SUCCESSFUL_EMPTY` only for exact successful complete top-level `null`; otherwise `COMPLETE`. If facts do not support exactly one class, the classifier returns `PARTIAL_CAPTURE` and no effects rather than guessing.

## Independent limits and refusal point

Assumption: a combined budget permits one dimension to conceal exhaustion in another. Basis: the accepted profile requires finite independent ceilings and forbids borrowing. Therefore the test-only recorder and outgoing wrapper each freeze and report independent limits for:

- request-frame bytes, response-frame bytes, raw params bytes, raw result bytes, aggregate retained bytes;
- prepare items, outgoing members, ranges, and admitted occurrences;
- source span count and bytes;
- request, response, owner-sequence, probe, and expansion ordinals;
- elapsed acquisition duration and recorder callback duration separately;
- logical work units and hash operations;
- diagnostics count, per-diagnostic bytes, and aggregate diagnostic bytes; and
- enforceable allocation/storage bytes.

Every dimension has exact-limit success and plus-one refusal. No dimension borrows from another. All addition and multiplication use checked arithmetic. Limit declaration, use, remaining amount, terminal disposition, and exact refusal reason are retained.

Limit checks happen before parsing/normalization where byte/member framing permits and before any semantic effect in all cases. If a member/range count can only be known by bounded scanning, scanning is recorder work and produces no normalized item or occurrence until every independent limit passes. A refusal clears provisional semantic values and returns only bounded test diagnostics and accounting.

Recorder effects are separately bounded and accounted: immutable-copy bytes, hash operations, span records, diagnostics, callback duration, and allocation are not charged as acquisition occurrences and cannot be hidden inside production request accounting. Recorder exhaustion yields `LIMIT_REFUSED` or `PARTIAL_CAPTURE` according to whether refusal occurred before or after required custody began; neither changes the underlying production round-trip result.

## Transparency and production unreachability

Assumption: test observation must not become a production behavior switch. Basis: the existing hook is package-private and not configured by production hosts. Therefore the proposed facility remains structurally unreachable from production:

- implementation files, constructors, interfaces, and enabling options use `_test.go` or an `internal` test-only owner reachable only from test compilation;
- no CLI, MCP schema, environment variable, config, build/profile selector, manifest field, exported production constructor, or runtime default can enable it;
- ordinary `sessionclient.New`, `acquisition.Client`, `WireClient`, coordinator, and production hosts retain their existing signatures and behavior;
- default/disabled execution performs no recorder allocation, callback, hashing, diagnostics, or branch-visible semantic change; and
- no test-only record is accepted by production publication, replay, admission, or qualification paths.

**Enabled/disabled semantic transparency:** for identical managed inputs and transport observations, enabling the recorder must leave the production `RoundTripResult`, errors, lifecycle transitions, request ordering, cancellation behavior, acquisition values, normalized graph, diagnostics, and production accounting byte-for-byte or value-for-value identical to disabled execution.

**Recorder-failure transparency:** recorder panic, allocation refusal, hashing failure, callback timeout, invalid test configuration, or recorder-limit exhaustion is observable only through the bounded test-only recorder result. It cannot replace, delay beyond the recorder's own bound, cancel, retry, mutate, or suppress the production round trip or its result. Panic is contained. A failed recorder produces no qualification success.

## Minimality analysis

Assumption: minimality is measured by the smallest authority-preserving change that can expose exact manager-owned bytes and bind them to outgoing semantics. Basis: exact frames exist in `sessionruntime`, while selection and normalization live above the current `sessionclient` result-only boundary.

| Candidate | Benefit | Rejected cost or retained role |
| --- | --- | --- |
| Extend the existing manager-owned recorder/pair machinery | Captures original frames, keys, write/read observations, and raw tokens at their owner; reuses defensive-copy and panic-containment patterns; can remain private and test-only. | **Selected for transaction custody only.** It must not learn outgoing profile identity, caller selection, occurrence semantics, evidence roles, or qualification authority. |
| Put a qualification wrapper around the managed acquisition call chain | Can bind ordered prepare results, selected ordinal/full identity, unchanged selected-item handoff, full outgoing result, and occurrence hashes without changing production normalization. | **Selected for outgoing-specific assembly.** It consumes test-only recorder output and remains separately bounded; it does not broaden the generic production acquisition client. |
| Capture inside outgoing acquisition/normalization | Direct access to method-specific values and occurrence construction. | Rejected: too late for original framed identity, couples test qualification to production semantics, risks effects before refusal, and would make recorder failure behavior-visible. |

Decision: use a split seam: a generic private test-only transaction recorder at the manager owner plus an outgoing-only test qualification wrapper at the managed handoff. This is smaller than a general response-frame accessor or production `acquisition.Client` change and safer than recording inside outgoing normalization.

A future incoming implementation may reuse only the generic recorder contract and mechanics. There is explicitly **no shared qualification authority**. Outgoing independently owns and must independently review:

- exact profile binding `CALLS_GO_GOPLS_OUTGOING_EXACT_V1`;
- the prepare → selected caller → outgoing method chain;
- selected-handoff equality and complete caller identity;
- caller-to-callee orientation and caller-side `fromRanges` occurrence semantics;
- outgoing evidence roles, provenance, limits, accounting, and refusal outcomes; and
- every outgoing review token and any future outgoing evidence/profile GO.

Incoming evidence, receipts, manifests, reviews, or GO tokens satisfy none of those outgoing obligations, even when generic recorder code or transaction bytes are equal.

## Proposed narrowly scoped implementation write-set

This is a proposal, not authorization. A future separately approved implementation should be confined to:

1. `sessionruntime/*_test.go`: test-only generic transaction-recorder contract, immutable byte copies, framing/key/owner-sequence binding, independent recorder limits/accounting, panic/failure containment, and focused unit tests;
2. `internal/acquisition/sessionclient/*_test.go`: test-only adapter/wrapper that requests recorder capture for exactly `textDocument/prepareCallHierarchy` and `callHierarchy/outgoingCalls` and returns the production value unchanged alongside test-only custody;
3. `docs/qualification/testonly/...` or a new test-only Go package under `internal/`: outgoing-specific classifier and assembler tests for ordered prepare custody, selected ordinal/full identity, unchanged handoff, raw outgoing result, hash-linked member/range bytes, outcomes, limits, and transparency; and
4. only if package visibility makes `_test.go` composition impossible, one narrowly scoped unexported `sessionruntime` hook file whose sole caller is test code and whose production default is nil/no-op, subject to an explicit independent unreachability test.

Excluded from the proposed write-set: CLI/MCP schemas, manifests, evidence copies, production acquisition interfaces, public APIs, profile registries, selector behavior, graph normalization, publication/replay/admission, semantic-vector, incoming qualification code, and production enablement. Any need to touch an excluded area invalidates this minimal write-set and requires a new design review; it is not an implementation convenience.

## Required RED matrix

Every row must fail before implementation for the stated reason and later pass only within a separately authorized implementation phase. Passing these tests would establish seam behavior only, never evidence truth, qualification, authority, admission, review, or GO.

| RED ID | Controlled setup | Required assertion |
| --- | --- | --- |
| R01 | Exact successful prepare and outgoing array. | Original framed request/response and raw params/result bytes are retained with matching key/session/generation/method/owner sequence; decoded re-encoding cannot satisfy the assertion. |
| R02 | Prepare returns multiple ordered items; select a nonzero target-bound ordinal. | Complete order and every item survive; selected ordinal and all canonical/optional/opaque fields match selected raw item bytes. |
| R03 | Mutate one byte/field between selected prepared item and outgoing request item. | Refuse before outgoing qualification effects; digest equality alone cannot bypass byte equality. |
| R04 | One outgoing member with two ranges, duplicate ranges, repeated member, parallel edge, and self-loop. | One occurrence per range; member/range ordinals and raw-byte hash links preserve multiplicity without deduplication. |
| R05 | Successful complete outgoing top-level `null`, then `[]`. | `null` is exclusively `SUCCESSFUL_EMPTY`; `[]` is `COMPLETE` with zero occurrences; canonical emptiness may match but custody bytes never alias. |
| R06 | Remove payload with otherwise completed request metadata. | Exclusively `MISSING_PAYLOAD`; zero normalization/effects. |
| R07 | Transport failure, complete JSON-RPC error, and cancellation as three vectors. | Exclusively `TRANSPORT_ERROR`, `PROTOCOL_ERROR`, and `CANCELLED`; no cross-classification or prefix success. |
| R08 | Complete malformed member/range and separately truncated response after a valid prefix. | Exclusively `MALFORMED` and `PARTIAL_CAPTURE`; no salvaged occurrence. |
| R09 | Exercise exact limit and plus one independently for every byte/member/range/duration/diagnostic/work/allocation dimension. | Exact limit succeeds where otherwise valid; plus one is exclusively `LIMIT_REFUSED`; no borrowing and no semantic effects. |
| R10 | Overflow, duplicate ordinal, reordered/mismatched key, stale/foreign generation, or missing frame. | Refuse with closed bounded accounting; zero qualified occurrences/effects. |
| R11 | Run identical production round trip with recorder disabled and enabled. | Production result, error, lifecycle, ordering, cancellation, graph/semantic values, diagnostics, and production accounting are identical. |
| R12 | Force recorder panic, timeout, hash/allocation failure, and recorder-limit exhaustion. | Production round trip remains identical and completes normally; only bounded test-only recorder status changes; no qualification success. |
| R13 | Search/build production packages and enumerate CLI/MCP/config/profile surfaces. | No production-reachable enablement, exported constructor, environment/config switch, schema field, or default recorder work exists. |
| R14 | Feed an incoming recorder record/review/GO or equal shared bytes to outgoing assembly. | Generic custody may be mechanically reusable, but outgoing profile binding, method chain, handoff, semantics, evidence roles, review, and GO remain independently missing. |
| R15 | Attempt capture after production acquisition has normalized or emitted an occurrence. | Test fails because custody/refusal must precede normalization and effects; inside-acquisition capture is not an admissible seam. |
| R16 | Attempt digest-only record with omitted source bytes/spans. | `PARTIAL_CAPTURE`; hashes cannot replace unchanged bytes. |

## Review and stop

Assumption: design review and implementation authority are distinct. Basis: all prior outgoing gates are explicitly narrow. Therefore an independent reviewer may inspect only this ADR and its one-file ancestry/hash and issue either `ADR0011_CALLS_OUTGOING_CAPTURE_SEAM_DESIGN_GO`, `REVISE`, or `REJECT`.

Stop after that decision. Even `ADR0011_CALLS_OUTGOING_CAPTURE_SEAM_DESIGN_GO` leaves implementation unauthorized, all evidence roles unchanged, authority at zero, completeness unknown, no profile GO defined or issued, and production `REFUSED_MISSING_EVIDENCE`.
