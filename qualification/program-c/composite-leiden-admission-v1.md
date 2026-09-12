# Program C composite-to-Leiden admission v1

Status: internal typed computation seam; not a native capture, receipt, MCP input, or presentation schema.

## Decision

Exact composite bytes may enter the existing Program C Leiden computation only through `programcadmission.Admit`, which constructs an opaque `CompositeProjectionAdmission` consumed by `programc.ComputeComposite`. No exported constructor accepts `programc.Projection`. Native `programc.Project`, `programc.Compute`, and canonical presentation bytes/schema remain unchanged.

The separately identified admission artifact is `lsp-trace.private.program-c-composite-leiden-admission.v1`. It is not a Graph Provenance envelope and contains no native evidence receipt.

## Validation and projection

Admission calls `programccompose.Validate` on the exact composite bytes. Validation replays every retained exact constituent Graph Provenance V5 envelope, byte length and digest, identity, custody/source records, compatibility coordinates, conflict policy, canonical order, policy identity, composite identity, and canonical recomposition.

Nodes are the validated composite nodes in canonical ID order. Calls are exactly the validated composite server-reported CALLS edges and call sites. No relation is synthesized between constituents. All nodes are admitted independently of calls, preserving isolates. Duplicate typed identities deduplicate only when complete records are equivalent; conflicts fail during replay.

## Provenance and authority

The artifact retains canonical constituent identity, exact-envelope and graph digests/lengths, schema IDs/versions, session/generation/invocation identity, revision custody, and canonical supply, capture, and binding records.

`authority` is integer zero. `source_graph_complete` is `UNKNOWN`. Native receipt is absent by type. Admission identity is a domain-separated commitment distinct from composite identity. The composite claim ceiling is retained.

## Compatibility

The opaque seam invokes the same deterministic Program C partition implementation. It does not change native V5 admission, native computation, canonical community presentation schema, serializer, or bytes. Public composite presentation remains outside this contract.
