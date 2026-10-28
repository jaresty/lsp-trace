# Derived retained partial native input — private V1

`DeriveRetainedPartialNativePrivate` is a new **producer**, not an additional
inspection report. Its only source is the opaque successful result of
`QualifyRetainedPartialNativePrivate`, with exact qualification pins and an explicit
invocation-local byte/work budget. The full paired eight-record and retained-input
qualification happens first. Qualification now keeps detached immutable record
bytes in a private self-bound state; it keeps no reader, root FD, mutable graph,
Manager guard, runtime client, or seed-authority capability. Copying the opaque
qualification shares that immutable state. Zero/decoded qualifications and mixed
pins cannot produce input. Repeated pure derivation is allowed.

The output is `DERIVED_RETAINED_PARTIAL_PRIVATE_V1`, containing:

- Actual canonical native graph.v5 bytes from the existing typed `graph.Result`
  producer, through the explicitly private `graph.ProjectRetainedV3ToV5Private`
  typed-document projection. It retains the original document's typed
  `ProcessContext` (environment-name identities, digests, variable count, state,
  and redaction metadata) exactly. The standard canonical projection supplies
  other fields with schema, workspace URI, observed server command, topmost=false
  and NOT_REQUESTED enrichment. A new semantic receipt commits the final typed
  document. No manual JSON field patch, source receipt, sibling candidate, or
  authenticated seed grant is invented.
- A typed `VALIDATED_PRIVATE_INPUT_MANIFEST` binding the exact qualification ID,
  qualification-report hash, pins hash, eight record IDs/lengths, retained input
  observation hash, original acquisition graph hash, **new** canonical native graph
  hash, private source-carrier hash, first-class frozen six-member source POLICY
  and its hash, canonical seed bytes/hash, observed provider/session/generation,
  caller-asserted original code revision, verifier source-pin proposal, source
  read budget, disposition counts, withheld count, graph/accounting counts, and
  qualification read/work accounting.
- Exact record bytes, including genuine acquisition request/element references,
  source receipts and withheld/outside dispositions, raw lexical request bindings,
  seed/target ordering and retained inspection/first-pair comparison results.

`ParseDerivedRetainedPartialNativePrivate` is a closed consistency parser. An
8 MiB envelope cap and lexical depth/token/container bounds precede typed decode.
Canonical byte equality rejects unknown, duplicate, case-alias and missing keys.
It recomputes record pins, qualification identity, policy/seed/graph hashes, the
original request binding, canonical discovery batch, claim/output references,
inspection/filter projections, and outcome bindings; it reuses the private source
parser, `acquisition.ValidateResult`, and native graph schema/semantic validation.
It compares the entire recomputed typed manifest and exact native serialization.
Parsing returns detached data, **never** the opaque producer result or authority.
It does not independently authenticate a producer or re-open the full retained
snapshot; actual producer authority within this private contract comes only from
its successful sealed qualification.

## Ceilings and excluded claims

Origin is `DERIVED_RETAINED_INSPECTION`. Original acquisition bytes and the newly
serialized graph have separate digests. This is **not** the original captured
NativeV5 identity, its ordinary replay-input manifest, or replay admission.
`OriginalNativeV5IdentityClaimed=false`, `ReplayAdmitted=false`, authority=0,
accepted=false, completeness=UNKNOWN, feature identity=UNRESOLVED, partial=true,
canonical_complete=false, and `NOT_PROGRAM_C_ADMITTED` remain explicit.
The source carrier stays private; the ordinary V5 VersionV2/PolicyV2 source gate
and withheld-receipt exclusions are unchanged. Observed provider identity is
original provenance, not a live lease. Original code revision and verifier source
pin are separate caller/review assertions, not authenticated compiled identity.

Derivation does no filesystem/source IO, Manager checks, acquisition, claim
recovery, replay, publication, Program C execution, or real retained writes.
Byte/work accounting is conservative and invocation-local, not a global CPU/heap
quota; cancellation is checked at entry/exit, not a promise of asynchronous
interruption inside existing pure validators. Bounds are private and do not
expand any live/public budget. No output publication or Census receipt is added.

## Retained process-context correction

The historical `DecodeNativeV3` returns a narrower runtime model and cannot
recover omitted effective-environment or working-directory secrets. Serializing
that model alone regenerates empty-default process metadata; original/derived
lineage hashes do not make those embedded assertions true. The private helper
therefore decodes the available full typed native document, validates the source,
uses the standard canonical producer for graph fields, preserves the original
**typed metadata** projection before exposing any output, and recomputes and
validates the final V5 semantic commitment. It never reads environment, cwd,
source URI or filename. Historical decoder/marshal defaults and engine behavior
are unchanged.

The existing native semantic contract permits `IDENTIFIED_NOT_EMBEDDED`, not an
UNKNOWN context state. Missing, UNKNOWN, or invalid process-context metadata is
therefore rejected, not replaced with made-up zeros/digests. Preserving metadata
is not recovering process secrets or independently authenticating old assertions.
Tests use synthetic non-default retained metadata and file-backed trace metadata
whose path is never opened. Full native semantic-field comparisons cover exact
context, nodes, CALLS endpoints/ordered canonical call sites, boundaries with
messages/provenance, seeds, memberships, source/trace metadata and receipts (apart
from explicit V5 schema/receipt coordinates). The actual qualified producer also
compares full source projections; only execution-bundle references may change
with declared workspace/provider enrichment.

## Private carrier codec correction and full-path witness

The source descriptor is the private EvidenceV2-shaped carrier, but its
`GraphBytes` are native **graph.v3**, not graph.v2. The ordinary EvidenceV2 codec
remains unchanged and intentionally still rejects a decoded non-default context
when its lossy runtime projection is reserialized. The private format now has
an explicit typed wire DTO with the exact original field order/shape. It compares
the runtime graph using `MarshalWithRetainedProcessContextPrivate`, preserving
metadata from the separately validated original native document before emitting
its unchanged original bytes into both graph carriers. The original graph digest
continues to bind those exact bytes; it is not recalculated from defaults.

Private capture retains the native document, then decodes its runtime projection
for existing deterministic acquisition and census validation. The private
`checkResultV2Profile` uses the private encoder for preflight and that decoded
runtime model for `acquisition.ValidateResult`; row limits, replay/accounting,
source receipts, supplies, census, bindings, graph references and policy checks
still execute. Ordinary checkResult/V2 paths use the original encoder. No process
secrets are inserted into Result, recovered, or inferred from the verifier.

`TestRetainedPartialQualificationNonDefaultContextPrivate` runs the actual owned
capture → private parser → paired eight-record retention → qualifier → derived
producer → new parser path with fixed synthetic metadata: two effective variables,
one explicit environment-name identity and `/fixture/original-cwd`. The same full
field/raw-binding assertions run for default and non-default cases. An explicit
legacy-parser RED run produced `V2 typed graph differs from captured bytes` on
this path before restoring the private encoder. The default fixture stays
byte-compatible with legacy encoding. Additional codec guards reject acquisition
context substitution and typed graph mutation, and demonstrate that ordinary V2
still rejects the private profile and retains its historical non-default codec
failure. Test-only metadata setup runs before actual capture and never observes
ambient environment/cwd or changes authorization.

## Qualification and remaining seam

The owned synthetic eight-record fixture exercises actual qualification → actual
derivation → private parse/validation. Its measured output is approximately
2.03 MB, including a 101,148-byte graph: 25 targets, 100 requests, 27 nodes,
3 CALLS, 3 call sites and 1 withheld capture. These are **synthetic fixture** counts,
not assumed historical capture counts. Tests also check exact policy/source/seed
bindings, preserved target/seed order and occurrences, substitutions, strict wire
rejection, budgets, detached accessors, repeated deterministic output, old V5
rejection, and derivation after all borrowed readers have been closed. Existing
fixture traffic counters must remain unchanged.

A compatible private Program C consumer / source seam still needs separate
qualification. Stage-2 broader admission, composition, Describe and full-pipeline
claims remain pending. This producer is not registered in ordinary native V5,
public MCP/CLI dispatch, or Program C admission. Independent review and parent
integration are still required; no commit or publication is performed here.
