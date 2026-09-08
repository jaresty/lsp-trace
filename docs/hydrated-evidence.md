# Internal bounded hydrated evidence core

**Internal-only FR21 increment. Not a public CLI/MCP feature, registry entry,
publication service, provider installation, or deployment claim.**

`internal/hydratedevidence` packages explicitly selected retained source context.
It does not acquire source, infer semantic relevance, discover containing
constructs, authenticate provider claims, or prove source/graph completeness.
No existing graph/provenance schema, identity, or artifact bytes are rewritten.

## Admission matrix

| Input | Admission | Source boundary | Authority ceiling |
|---|---|---|---|
| `lsp-trace.graph-provenance.v1` | Existing `graphprovenance.ValidateFor`, v1 | Existing validated bindings, supply and capture receipts | Native retained attribution; integrity only; analyzed version unverified |
| `lsp-trace.graph-provenance.v2` | Existing `graphprovenance.ValidateFor`, v2 | Existing graph/acquisition binding pointers, supplies and captures | Same ceiling; exact typed acquisition coordinate encoding retained |
| `lsp-trace.hydrated-sidecar.v1` | Owned explicit sidecar contract, bound to exact main artifact digest | Explicit asserted source receipts and source references | `CALLER_ASSERTED` / `NON_AUTHORITATIVE` only |
| Any other main or sidecar family/version | Explicit error | None | No implicit conversion or dropped contribution |

No dependency on retained-calls v2 or its pending review is introduced. Standalone
retained-calls exports, operational custody envelopes and provider packages are
not admitted by this increment. A provider's name or arbitrary opaque JSON does
not establish a source-anchor contract. No new provider source API is assumed.

Sidecars can accompany an admitted native artifact and refer to its native
receipt IDs, or retain their own asserted bytes/references. They cannot serve as
a self-declared native artifact. A sidecar record may call its opaque category
`CALLS`; its authority and qualification remain explicitly non-native. Neither
sharing a fragment nor supplying a relationship reference establishes independent
support for that relationship.

## Internal API

```go
policy := hydratedevidence.DefaultPolicy() // bodies disabled
catalog, err := hydratedevidence.Inspect(input, policy)
// Choose exact catalog record/source IDs, not inferred semantic relevance.
request := hydratedevidence.Request{
    Policy: policy,
    Selections: []hydratedevidence.Selection{/* explicit selections */},
}
bundle, err := hydratedevidence.Hydrate(input, request)
err = hydratedevidence.Validate(input, request, bundle)
err = hydratedevidence.ValidateJSON(input, request, serializedBundle)
text, err := hydratedevidence.Text(input, request, bundle)
snapshot, err := hydratedevidence.NewSnapshot(input, request, bundle)
page, err := snapshot.Page("") // subsequent calls use page.Next
reconstructed, err := hydratedevidence.Reassemble(input, request, allPages)
```

`Input.Artifact` and each `Input.Sidecars` element are **bytes, never filenames**.
All APIs are offline. They never call capture, open a workspace path, start a
provider, or fetch a reference. Changing/deleting an existing checkout cannot
change the result. A future acquisition operation would create new provenance,
not mutate an old artifact or retroactively complete its receipt.

`Inspect` returns a body-free catalog; `Hydrate` includes full selected bodies
only when `Policy.IncludeBodies` is true. The bundle contains the exact input
SHA-256 digests, request digest, effective policy, receipt/version metadata,
complete ordered origin ledger, canonical union spans, total counts and digest.
The input artifacts themselves are not embedded again: retain them separately
for validation/replay. A digest is an integrity join, not independent custody.

### IDs and references

* Native binding record: `native:<existing JSON pointer>`; v1 pointers address
  `GraphBytes`, while v2 pointers retain `/graph/...` or `/acquisition/...`.
* Native source: `native:<existing receipt ID>`.
* Sidecar record/source: `sidecar:<exact sidecar digest>:<caller ID>`.
* A synthetic **catalog selection handle**, `receipt:<source ID>`, selects a
  retained receipt explicitly. It is not a new native evidence record.

Source IDs are local to their digest-bound catalog. The full source identity is
its input artifact digest plus receipt reference/digest, version reference,
content hash and acquisition classification—not its URI alone. Supply A and
post-traversal capture B at the same URI remain separate. This implementation
also retains separate spans for identical bytes under distinct receipts; it does
not implement optional cross-receipt content-addressed storage sharing.

Native node/relationship IDs and accompanying relationship references remain
attached to selected record metadata. Every selection retains its own ID and
record/source references even if repeated, nested, missing, excluded or invalid.
Unknown requested record/source IDs receive explicit origins, not disappearing
from the response. No all-record implicit selection is performed; callers can
use the catalog to construct an explicit artifact-focused list, including every
receipt. Non-source binding records remain catalogued and, when requested
without an available source join, retain an `UNKNOWN_SOURCE` disposition.

### Selection modes

| Mode | Meaning |
|---|---|
| `SPAN` | Caller-selected range and/or exact byte interval; coordinate authority is caller-asserted. If both are supplied they must agree exactly. |
| `RETAINED_RANGE` | Use the selected record's actually retained range. Missing range stays unknown; V2 invalid-anchor status cannot be upgraded by readable source. |
| `WHOLE_FILE` | Explicitly select the whole readable retained receipt. Byte offsets cover the exact content; original coordinates remain unavailable, not invented. |
| `BOUNDARY` | Explicit caller-asserted function/declaration/comment/template boundary, under a separate opt-in. |

`BOUNDARY` requires `AllowCallerBoundaries`, an exact source ID/content hash,
nonempty attribution reference, recognized construct kind, and
`CALLER_ASSERTED` / `NON_AUTHORITATIVE`. Its range is validated against those
exact bytes. This does not verify that a language construct really has that
boundary. Provider-authored automatic boundary discovery/admission is not
implemented; a claimed native boundary is rejected as unknown. An ordinary
CallHierarchyItem range never silently becomes a complete containing function.

A record's authority, its source receipt's authority, and the authority of the
selected coordinates are separate. `OriginalRange` means the original coordinates
of that explicit request or retained-range selection; the selected record also
retains its own range. `WHOLE_FILE` has no invented original range. Null revision
or content-hash metadata is unavailable, not inferred from a URI or repository.
V1 does not establish a position encoding here; supply an explicit encoding for
range conversion. V2 retains the validated acquisition request's encoding.

## Sidecar receipt contract

The owned schema is `internal/hydratedevidence/schema.json`; it is deliberately
not registered in the shared schema registry. Sidecar fields are:

* `schema_version`, `artifact_digest`, `authority`, `qualification`;
* `sources`: explicit ID, URI, receipt reference, version reference, optional
  asserted revision/hash, `source_encoding`, state and optional base64 content;
* `records`: caller ID, opaque claim kind, explicit source IDs, accompanying
  relationship references, optional range and declared position encoding.

Native source references use `native:<receipt ID>`; sidecar-local source
references use the local source ID. Sidecars cannot refer to other sidecars.
Duplicate sidecar byte inputs, duplicate local IDs, missing source foreign keys,
wrong artifact digests, unknown families and authority escalation fail admission.
Nullable sidecar input lists are accepted as empty; output tables are arrays.

`RETAINED_BYTES` and `TRUNCATED_INPUT` require explicit content (including an
explicit empty byte string) and matching SHA-256. The owned receipt consists of
the exact digest-bound asserted source fields; it is not a native source receipt
or a claim of independent authentication. Reference-only/missing entries cannot
carry a body. Asserted reference-only metadata remains an assertion when no
bytes are available to verify it. Native full bytes have already passed the
existing canonical receipt validators. Truncated sidecar input is not exported
as complete context, even when its retained fragment happens to be empty.

## Byte semantics and deterministic allocation

Source content is UTF-8. Position encoding is separately declared as `utf-8`,
`utf-16` or `utf-32`; unsupported/missing encodings never trigger guessing. BOM
is retained and counts as one code point (three UTF-8 bytes). CR, LF and CRLF
terminate lines; coordinate positions address line content/endpoints, not line
terminator interiors. Cross-line spans preserve all original terminator bytes.
UTF-8 code-point splits and UTF-16 surrogate splits are invalid. Explicit byte
intervals must also end/start on UTF-8 boundaries. Empty readable content and
empty valid intervals are supported.

Within each exact source ID and selected position encoding, the producer sorts
intervals by start/end and deterministically unions overlaps, nesting, duplicate
and touching intervals. Different encodings remain separate. Whole-file
selections use the `UNAVAILABLE` original-position-encoding group. Every selected
origin maps to its entire exported union; no origin is removed by deduplication.

Work allocation follows request order. Union output allocation follows source ID,
encoding, start/end order. Work is conservatively charged `3*sourceBytes+1` per
resolvable selection, before scanning its bytes. Body/span/page allocation is
performed on whole unions, not reset per origin or page. Oversized unions get an
omission, never a truncated fragment or fictional immutable reference.

### Dispositions and failure policy

Origins distinguish `EXPORTED`, `UNKNOWN_RECORD`, `UNKNOWN_SOURCE`,
`PRIVACY_EXCLUDED`, `REFERENCE_ONLY`, `MISSING_BYTES`, `TRUNCATED_INPUT`,
`UNKNOWN_BOUNDARY`, `INVALID_COORDINATES`, `WORK_BUDGET`, `PAGE_BUDGET`,
`SPAN_BUDGET`, and `BODY_BUDGET`. The source's original input status remains
separate (for example, a failed bounded native read is missing bytes, not a
retained truncated body). Resolution precedence is record/source join, privacy,
source availability, boundary validity, then work/coordinate evaluation; source
metadata remains visible even when privacy is the selected origin disposition.

Malformed/conflicting selectors, duplicate selection IDs, invalid policies,
unsupported/malformed artifacts, excessive input/selection counts, and final
serialized-output overflow fail with no bundle. Zero readable selections do not
claim complete context. `Complete` means only that every nonempty set of requested
spans was wholly exported; it never means analyzed-version or source completeness.

## Limits

| Resource | Default | Hard ceiling / behavior |
|---|---:|---|
| Exact input bytes, combined | 64 MiB | 192 MiB, plus stricter existing family limits |
| Serialized bundle bytes | 16 MiB | 64 MiB; overflow is an error |
| Included body bytes | 4 MiB | 16 MiB; whole-union omissions |
| Origins | 4,096 | 10,000; oversized request is an error |
| Exported union spans | 4,096 | 10,000; whole-union omissions |
| Coordinate work units | 64 MiB | 512 MiB; deterministic per-origin omissions |
| Page bytes | 64 KiB | 4 KiB minimum, 1 MiB maximum |
| Pages | 4,096 | 10,000; overflow is an error |

Additional fixed bounds: 64 sidecar envelopes; 10,000 combined source/record
entries per sidecar; 100,000 catalog sources/records; 4 MiB serialized selection
request; 64 JSON container levels. The nesting bound applies before recursive
native validation, including opaque `data` JSON, but source contents remain
base64 scalar bytes and are never recursively interpreted. Input-size/family
bounds govern parsing and admission; the recorded work counter describes only
coordinate resolution, not total CPU time. No wall-clock timeout is claimed.
Typed Go inputs are caller-allocated; JSON entry points check byte bounds before
parsing. Limits do not promise a fixed process RSS independent of Go serialization.
No zero-valued limit silently becomes unlimited.

## Immutable paging and validation

`NewSnapshot` validates the bundle and privately owns serialized pages. Pages
contain ordered, indivisible header/source/origin/span entries. They bind the
same result digest (which includes input, request and policy), exact ordinal,
continuation, total pages and total origin/span counts. Re-requesting a page is
stable. Mutating returned page values cannot mutate the snapshot.

Whole spans reserve 2 KiB of page overhead during global allocation. A span that
cannot fit gets `PAGE_BUDGET` before paging. Exact page packing checks every
serialized page again. An indivisible metadata entry too large for a page fails
snapshot creation; metadata is never dropped. There is no publication store,
implicit disk write or immutable-whole-span-reference fallback. Each page's
transport overhead does not reset the global body/span/work budgets.

`Reassemble` requires the complete canonical ordered sequence and rejects
missing/duplicate/reordered pages, tampered entries/hashes, noncanonical packing,
wrong snapshot/continuation and inconsistent totals. The reconstructed bundle is
validated against the original input and request. Output semantic validation does
not call `Hydrate`: it re-admits the input, checks source/record/selection joins,
coordinate dispositions and exact bytes, and uses an independent endpoint-event
sweep to check the producer's interval unions and exhaustive origin mappings.
Shared origin-resolution rules are explicitly not a second independent parser;
separate hard-coded Unicode offsets and a bitmap interval oracle test them.
Validation establishes consistency with supplied artifacts, not authenticity of
their original acquisition claims.

## Verification and remaining integration

Tests cover native v1 and v2, mixed native/sidecar shared fragments, same-URI
supply/capture isolation, deletion and newer-checkout replay, empty/ref-only/
missing/truncated content, caller-boundary opt-in, coordinate authority, BOM/
CRLF/non-BMP coordinates, independent interval unions, privacy, global exact
budget edges, immutable paging, unknown families, and coherent output tampering.

FR20/FR21 package, document and schema ownership is explicitly registered in the
integrated-conformance dirty-change guard; unrelated paths remain rejected.
`TestFR20HydrationIntegration` feeds original public CLI graph-provenance/v2 bytes
from the local wire fixture (and installed gopls positional fixture when available)
to this core, deletes the checkout, and checks exact shared native/caller-sidecar
bodies, omissions, authority separation and complete paged replay. This is a
bounded integration proof, not public hydration. Public schema/operation
registration, CLI/MCP selectors, body opt-in, publication and capability discovery
remain deferred. Public parity, publication custody, real-provider boundary
support and deployed availability are **not qualified by this proof**.

## Internal focused-selection checkpoint

This additive Go API implements the selection boundary of FR21 points 10/11,
not their public CLI/MCP surfaces or full AC17 qualification:

```go
focus := hydratedevidence.DefaultFocusRequest()
focus.NodeIDs = []string{/* exact native graph node IDs */}
focus.RelationIDs = []string{/* exact native graph edge relation IDs */}
focus.SidecarRecordIDs = []string{/* exact admitted sidecar catalog record IDs */}
// All three default false; each is an independent explicit choice.
focus.IncludeBodies = true
focus.WholeFile = false
focus.EndpointContext = false
result, err := hydratedevidence.HydrateFocused(input, focus)
err = hydratedevidence.ValidateFocused(input, focus, result)
// Existing core interfaces remain directly usable, without a new paging algorithm.
err = hydratedevidence.Validate(input, result.Request, result.Bundle)
text, err := hydratedevidence.Text(input, result.Request, result.Bundle)
```

`FocusRequest.CorePolicy` supplies mandatory bounded core limits; use
`DefaultFocusRequest` rather than an all-zero policy. Focus `IncludeBodies`
overrides the core policy's body bit, so the latter cannot bypass the focus
opt-in. Optional `PositionEncoding` supplies an explicit conversion encoding
(e.g. for V1); conflicting V2 encoding receives the core's invalid-coordinate
outcome. It does not convert caller `SPAN` coordinates into native coordinates.
No boundary inference or new provider adapter is added.

Selection policy is deliberately narrow and recorded in the manifest:

- `RETAINED_NODE_RANGE`: exact typed native `nodes[i].id` selects the binding at
  `nodes[i]/range`, not all string/pointer-prefix matches or an inferred complete
  function. V2 pointers retain `/graph`. Identifier-only ranges stay identifier-only.
- A native `edges[i].relation_id` selects **every** `edges[i]/call_sites[j]` with
  the exact caller node's URI and retained range. Endpoint declaration ranges
  are added only with `EndpointContext`. No acquisition bookkeeping is mined.
- `ALL_EXACT_BOUND_RECEIPTS`: all receipt IDs bound to each selected carrier are
  included in sorted source-ID order, including distinct supply and post-capture
  receipts at the same URI. This is not a latest/analyzed-version decision.
- `PRESERVE_OCCURRENCES`: input order is node IDs, relation IDs, then sidecar IDs;
  order and duplicate occurrences within each list are preserved. Generated core
  origin IDs distinguish each occurrence/site/receipt. Empty IDs remain unknown.
- Only actual records in an admitted `hydrated-sidecar.v1` envelope are accepted
  by the sidecar selector; receipt handles/native pointers are not sidecar records.
  Sidecar kinds and relationship references remain opaque asserted claims.

Every requested occurrence remains in `Manifest.Origins`. `MAPPED` means an
identity join, **not** successful body delivery or complete context. Each site's
`OriginIDs` indexes `Bundle.Origins` for the exact privacy, availability,
coordinate, and budget disposition. Unknown/ambiguous IDs, unsupported record
selectors, no-call-site groups, absent bindings/sources/ranges and invalid
retained anchors have explicit manifest dispositions. A zero-site edge retains
its requested relation and endpoints with `NO_CALL_SITES`, even when endpoint
context is requested; it never claims call-site context from endpoints alone.

All `NON_SOURCE` catalog records are counted as excluded bookkeeping, not
selected with empty source IDs and not reported as missing-source warnings.
The **full** catalog remains available through `Inspect`, and the **full source
catalog remains in the core bundle** because core semantic validation requires
it. It is a validation dependency, not an instruction to capture/render every
source. The manifest determines focused selection; a future public renderer can
project it without trimming the validated bundle. Existing core `Text` still
prints the complete source catalog and only the selected origins/spans.

The manifest digest binds exact input digests, exact focus-request/policy digest,
declared policies, ordered requested occurrences and all site/origin mappings.
Validation re-admits separately supplied original bytes and focus parameters,
reconstructs the expected plan, separately audits native array-based coverage
and receipt/origin foreign keys, then calls existing core `Validate` (never
`Hydrate`). The native admission and typed projection are shared, not a second
native parser or independent custody attestation. Rehashing an omitted requested
ID, omitted call-site, changed authority or substituted policy cannot repair it.

The core origin cap also bounds requested focus occurrences, expanded sites and
expanded receipt selections, including sites with no receipts. Focus input is
bounded to 4 MiB and 1024 bytes per requested ID; combined result JSON is bounded
by `MaxOutputBytes`. There is no wall-clock/RSS promise, new paging/publication
format, shared-registry schema, public wiring, or automatic source acquisition.

### Original FR20 fixture and measurement convention

`testdata/focused-fr20.v2.json` is an unchanged copy of the original public FR20
fake-wire artifact retained by the earlier integration run, SHA-256
`8913b3d062312be15531728f801e2f677f4a65852fd5fbeaf4ef1007ae1f83cf`.
The source checkout was deleted by that original run. The test reads artifact
bytes only, selects its actual two native edge IDs, and reuses core hydration,
validation and Text; it neither rebuilds a graph nor runs a provider.

`TestFocusedActualFR20Measurement` reports UTF-8 bytes of core Text for the two
selected relationships versus a baseline selecting every catalog record/receipt
pair (including an empty source ID for source-less records), both in retained-range
mode with body inclusion. It separately reports focused JSON bytes, distinct
selected span-content bytes and distinct full retained source-content bytes
(SHA-256 deduplication for measurement only, not receipt identity merging).
These are this pinned fixture's measurements, not the unprovided D01 trial, not
public renderer measurements, and not a universal reduction ratio. D01 replay,
CLI/MCP parity, deployment and full FR21 acceptance remain deferred.
