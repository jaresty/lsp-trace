# Program B v2 independent review — repaired candidate

## Status

**synthetic fixture-qualified, package-private, unshipped**

This review withdraws any prior claim that the repository has achieved real-world `PROGRAM_B_ADMITTED`, provider qualification, public registration, or independently anchored production admission. The package-private admission fixtures prove only their synthetic mechanism. They are not permission to execute local analytics and are not evidence of production/provider qualification.

## Repaired implementation assessment

The dormant `internal/normativeanalytics` package now performs three distinct local computations over an explicit retained graph:

- **Analysis** emits deterministic ROOT/LEAF structural findings and retained edge-ID witnesses.
- **Metrics** emits node/edge counts, per-node in/out degree, directed distinct-nonloop density with the exact `n(n-1)` denominator, and an explicit omission when fewer than two nodes make that denominator undefined.
- **Ranking** emits integer incoming-edge scores ordered by score descending and node ID ascending.

Each operation owns its work budget and stops while traversing when the next measured unit would exceed policy. A limited result contains accounting but no partial analytical evidence. Relations are retained exactly as supplied; the implementation never infers or relabels `CALLS`.

Results use distinct schemas (`lsp-trace.normative-analysis.v2`, `.metrics.v2`, `.ranking.v2`) and schema-domain-separated SHA-256 evidence digests rather than a generic string.

## Provenance boundary

Local execution does not require an external signature or a Program B admission token. Authority and custody apply only when a caller makes a provenance claim about retained evidence.

`ValidateRetainedEvidence` checks an independently supplied expected revision, exact evidence bytes/digest, a schema-selected semantic validator, and equality between source and custody revision. If evidence includes an authority claim, the verifier is supplied by local policy rather than by the evidence object and checks authority/key/domain/context/digest/signature. Unsigned local evidence is valid only as local retained input and makes no authority claim.

Package-private ephemeral authority tests are explicitly synthetic. They prove rejection mechanics for byte, schema, semantic, revision, custody, authority, key, domain, context, digest, signature, and missing-authority mutations; they do not establish a real provider or real-world qualification.

## Registration and public wiring readiness

Descriptors remain package-private and are not added to CLI, MCP, generated docs, product surfaces, or public registries.

Before public wiring, an operator/maintainer must:

1. Define the accepted retained graph schema and canonical decoder at the CLI/MCP boundary.
2. Bind each request to an explicit local input digest and source/custody revision; configure any authority verifier only for provenance claims.
3. Select operation policy limits in host configuration and map typed `LIMIT` outcomes to CLI/MCP result schemas.
4. Add CLI and MCP adapters that preserve operation-specific schemas and digests without inferring relation families.
5. Run non-synthetic qualification against the exact retained inputs/provider context intended for distribution and label results explicitly as local or synthetic until that evidence exists.
6. Register descriptors only in a separately reviewed wiring change.

No external host authority is required merely to run these local analytics. A separately provisioned authority is required only if distribution claims independently authenticated provenance.

## Qualification boundary

The current positive tests support only: **synthetic fixture-qualified, package-private, unshipped**. They do not support `PASS`, `PROGRAM_B_ADMITTED`, provider-qualified, shipped, registered, or production-ready claims.
