# Program B v2 repair

## Outcome

Status: **synthetic fixture-qualified, package-private, unshipped**.

False real-world `PROGRAM_B_ADMITTED` and external-authority-as-execution-permission claims are withdrawn in `INDEPENDENT-REVIEW.md`. Program B analytics are local computations; provenance authority is evaluated only when evidence asserts an authority-backed provenance claim.

## Implementation

- Replaced the generic `ADMITTED_NORMATIVE_RESULT` executor with deterministic retained-graph analysis, metrics, and ranking.
- Analysis reports structural roots/leaves with retained edge-ID witnesses.
- Metrics reports counts, per-node degrees, density numerator/denominator, and explicit undefined-denominator omissions.
- Ranking reports deterministic scores and `score-desc,node-asc` tie-breaking.
- Added operation-specific schemas and domain-separated evidence digests.
- Moved limits into operation policy and consumes measured units before each bounded work item; no execute-unbounded-then-compare path remains.
- Added retained-evidence validation for exact bytes/digest, schema-selected semantics, revision/source custody, and optional policy-supplied authority verification.
- Kept descriptors dormant and unregistered.

## Guards

Focused package tests cover deterministic outputs, retained witnesses, metric denominators/omissions, ranking ties, distinct digests, malformed retained graphs, work-boundary limits, unsigned local provenance, synthetic authority mechanics, and mutations of bytes, schema, semantics, revision, custody, policy revision, authority, key, domain, context, digest, signature, and authority configuration.

The synthetic authority is package-private test code and proves mechanism only. It is not a real provider, host authority, or qualification.

## Public wiring readiness

Before wiring CLI/MCP surfaces, maintainers must define the accepted retained graph decoder/schema, configure local operation limits, preserve typed operation result schemas, bind source/custody revision and digest, label qualification as local/synthetic until non-synthetic evidence exists, and separately review descriptor registration. An external authority is needed only for an authenticated provenance claim, not local execution.

## Validation scope

Only focused normal/race tests, focused vet, and build are run. No full suite, full race, network, install, deploy, product, live environment, or public registry activation is performed.
