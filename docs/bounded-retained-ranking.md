# Authorized bounded retained PageRank and PPR

## Scope and authorization

The user explicitly authorized this independent `bounded-retained-ranking/v1`
family on clean baseline `a2fa0fc024537b69bfc0c513e3bf851c9b63862c`, following
independently passed topology/preflight and structural metrics work. Artifact:
`lsp-trace.bounded-retained-ranking.v1`. This does not change the closed analysis
or metrics v1 contracts, historical thirteen-tool manifest, existing identities,
source authority, PRDs, or `PROGRAM_B_ADMITTED`. It is not independent approval.

Only fully admitted historical retained-calls/v1 is eligible. The result embeds
one exact base64 `input_bytes`; all retained source, execution, context, callsite,
witness and qualification ceilings remain available unchanged through that input.
Node IDs are artifact-scoped, not semantic identities. `scope` remains
`HISTORICAL_ARTIFACT_SCOPED_UNVERIFIED_INCOMPLETE`. No business, ownership,
source-completeness, authentication or runtime-behavior interpretation is made.
A coherent replacement of an entire public input can make another admissible
artifact: consistency checking does not authenticate a public artifact.

## Fixed equation and parameters

For every retained node v:

```
F(x)[v] = (1-alpha)*p[v]
        + alpha*(sum over groups u->v of x[u]/out_group_degree[u]
                 + dangling_mass(x)*p[v])
```

All retained endpoints participate, including isolates. Each historical group
is one directed caller-to-callee edge, including parallel groups, self loops,
and UNREPORTED groups. Weights are GROUP counts, never support or callsite counts.

PAGERANK uses uniform p over all nodes and forbids seeds. PPR requires an array
of `{node_id,weight}` with 1..n unique exact admitted IDs and positive integer
weights 1..1000000. Missing, duplicate, unknown, fractional and zero seeds fail.
Seeds are sorted lexically; their original integer weights are retained. The
integer sum (at most 4096000000) is exactly representable in binary64. Each weight
is divided by that sum once; unseeded nodes get +0. Proportional scaling preserves
numerical scores, while recorded weights can change the basis.

| Parameter | Default | Inclusive bounds unless indicated |
|---|---:|---|
| alpha | .85 | finite 0 < alpha <= .99 |
| tolerance | 1e-9 | finite 1e-12..1e-3 |
| max_iterations | 1000 | 1..10000 |
| max_work | 1000000 | 1..1000000 |

The Go API takes fully specified `Parameters`; `Defaults(algorithm)` supplies
explicit defaults. Zero is invalid, not an omission sentinel. CLI/MCP omission
uses defaults; explicit null, zero, NaN/infinity and unknown/case-aliased fields
are rejected. Seed integer decoding never truncates a floating-point value.

## Arithmetic, work, and completion

Initial x=p. No renormalization occurs. Nodes are scanned in lexical ID order;
groups in lexical historical group-ID order. Every arithmetic operation has an
explicit float64 rounding boundary, including products, preventing FMA
contraction. Reductions are sequential; maps are used for lookup, not numerical
reduction order. Scores are finite and nonnegative, with canonical positive zero.
JSON uses Go round-trip binary64 encoding. Scores are lexical by node ID; ranks
are a complete descending-score ordering with exact ties broken lexically.

The replay policy is `Go-binary64-explicit-rounding-sequential/v1`. Native numerical
qualification for this implementation is Go **go1.26.5, darwin/arm64**. Replay on
another Go toolchain/platform needs runtime qualification; successful crossbuilds
alone do not qualify floating-point results. No dynamic runtime metadata enters
identity.

An evaluation costs exactly **3n+m** numerical work: n dangling-mass visits,
m group contributions, n final F coordinates, and n residual/mass visits. The
last scan computes stationary L1 residual `sum(abs(F(x)-x))` and score mass
`sum(x)`. Residual evaluation is not free. Admission, topology sorting, proof,
canonicalization and publication are separately bounded, not work-budget ticks.
The iterative producer uses O(n+m) storage and O(n+m) work per evaluation, not a
dense matrix. The independent proof uses sorted incoming incidence lists.

Evaluate the initial candidate (iteration 0). A completed evaluation with
residual <= tolerance is COMPLETE and exposes that candidate's scores/ranks.
Otherwise, if the update count has reached max_iterations, return
INCOMPLETE/NOT_CONVERGED. Otherwise assign x=F(x), increment the update count and
evaluate again. A completed converged evaluation at exactly max_work succeeds;
a completed nonconverged evaluation at max_iterations is NOT_CONVERGED even if
its work also equals max_work. Exhaustion during an evaluation is INCOMPLETE/LIMIT.
The default budget is **not guaranteed to converge the maximum admitted graph**.

INCOMPLETE always has empty scores and ranks. `residual`, `score_mass` and
`residual_iteration` describe the last fully evaluated candidate or are null if
none completed. Partial residuals are never exposed. `iterations` counts updates,
`work` counts consumed numerical visits. These are computation statuses over the
retained graph, not source-completeness claims. Successful CLI/MCP delivery of an
INCOMPLETE artifact must not be interpreted as successful convergence.

Cancellation checked at numerical visits and before encoding yields
INCOMPLETE/CANCELLED; it precedes coincident budget exhaustion. Cancellation is
canonicalized to the observed work boundary for exact replay. A zero-work update
between evaluations is collapsed back to its last evaluated iteration when
cancellation occurs at that boundary. A cancellation observed after computation
is replayed at that boundary, suppressing scores/ranks. This bounded replay is
additional validation work, not evidence that a runtime cancellation event
actually occurred. Admission, sorting and encoding remain size-bounded rather
than wall-clock interruptible. Empty PAGERANK is COMPLETE with empty scores/ranks,
zero iterations/work/residual/mass and residual_iteration 0; pre-cancellation
instead gives CANCELLED with null residual metadata. Empty PPR is invalid.

## Identity and independent semantic validation

Policy: `retained-CALLS-unit-group-ranking/v1`. The `policies` array fixes
initialization, dangling redistribution, residual norm, scan ordering, numeric
replay and work accounting. Basis hash domain is
`lsp-trace.bounded-retained-ranking.v1:basis` over the canonical object
`{policy,policies,input_bytes,parameters}`. Exact input whitespace matters.
Result domain is `lsp-trace.bounded-retained-ranking.v1:result` over the entire
result with digest blank. Both hash UTF-8 domain, NUL, then canonical JSON, with
`sha256:` lowercase hex. Canonical JSON sorts object keys, retains array order,
uses Go string escaping and json.Number preservation. Artifact encoding ends
with one LF; digest canonicalization has none. No fresh timestamps/request IDs.

Composed validation performs bounded known-carrier preflight before recursive
historical/schema validation, exact recursive field checking, retained admission,
exact endpoint/personalization coverage, independent incoming-equation proof,
and deterministic producer replay. The independent proof builds destination
incidence lists and degrees from retained tables, not the producer's index or
kernel, and checks mass, residual, finite scores and ranking. Complete residual
must satisfy the requested tolerance without widening it. Replay additionally
fixes parameters, policy IDs, digests, exact scores, iterations and work/status.
Shape-only semantic validation deliberately fails closed.

Mass is checked against a fixed floating-error bound, not a tunable epsilon.
Let u=2^-53 and gamma(k)=k*u/(1-k*u), delta=gamma(8n+4m+32).
For n>0 the bound on `abs(sum(x)-1)` is

```
(gamma(n+2)+delta)/(1-alpha-delta) + gamma(n)
```

The operation allowance dominates normalization/division, dangling and incoming
nonnegative reductions, two products plus the final addition per coordinate,
and mass measurement. A nonnegative reduction's absolute error is bounded by
its gamma factor times input mass; the step's mass error is therefore at most
delta*(1+previous mass error). Iterating the affine contraction bounds accumulated
mass error by the displayed geometric denominator. Initial p mass error is at
most gamma(n+2); final measurement adds gamma(n). At admitted n,m and alpha<=.99
the denominator is positive. This deliberately conservative bound is independent
of convergence tolerance and iteration count. Empty mass is exactly zero.

## Public routes and bounds

```
lsp-trace bounded-retained-ranking --algorithm PAGERANK retained.json
lsp-trace bounded-retained-ranking --algorithm PPR --seed EXACT_HEX_ID=3 retained.json
lsp-trace bounded-retained-ranking --algorithm PAGERANK --max-work 1000000 --output selected.json retained.json
lsp-trace schema get --family bounded-retained-ranking --version v1
lsp-trace validate --family bounded-retained-ranking --version v1 ranks.json
lsp-trace verify --family bounded-retained-ranking --version v1 selected.json
```

Flags precede PATH or `-` (stdin). Repeat `--seed` for multiple PPR seeds. Seed
syntax is exactly lowercase hexadecimal ID, one `=`, decimal digits; no URI
splitting, signs, whitespace, fractional or exponent weight notation. CLI
`--output` publishes an immutable no-replace generation selector. Default
validate/verify remains graph-only. The Go API is `boundedranking.Analyze` and
`boundedranking.ValidateFor`.

MCP adds `lsp_trace_v1_bounded_retained_ranking`, alias
`lsp_trace_bounded_retained_ranking`, through one shared offline operation handler.
The original thirteen-tool manifest and prior sixteen operations remain intact;
runtime discovery now has **seventeen** tools and five new envelope variants.

```json
{"input":"<exact retained JSON text>","algorithm":"PAGERANK"}
{"input":"<exact retained JSON text>","algorithm":"PPR","seeds":[{"node_id":"<exact ID>","weight":3}],"output_selector":"ranks.json","detail":"compact"}
```

Raw object input is also accepted and its exact value bytes are preserved before
generic map decoding. Text is JSON, never a filesystem path. Prefer text to retain
original whitespace; a client serializing an object can already have altered it.
MCP schema_get/validate use explicit `schema:{family:"bounded-retained-ranking",
version:"v1"}`; validate returns exact admitted bytes. MCP verify retains its
existing graph-only custody contract; use family-explicit CLI verify for ranking.

Input <=8 MiB; result <=16 MiB; nodes <=4096; groups <=8192; decoded known JSON
carrier depth <=64 before historical recursion. Source content and opaque
receipts are not interpreted as JSON. Existing retained occurrence limits apply.
MCP wire limit is still 4 MiB. Inline results >1 MiB and compact results require a
safe output_selector and configured publication root. Overwrites fail closed.
Publication and compact receipts preserve full artifacts, not partial scores.

## Tests and derivation

Compiling public RED tests preceded production changes and failed on missing
schema/tool/algorithm assertions. Independent exact-rational Gaussian elimination
checks both algorithms over all 512 directed three-node topologies; fixed
closed-form vectors, parallel groups, loops, isolates, seed scaling, permutation,
work/iteration/cancellation boundaries and coherently resealed mutations are
also checked. Public tests exercise exact CLI/MCP bytes, raw large integers,
input validation, schema/validation, immutable selectors, tamper rejection,
compact/error/oversize publication, opaque content and encoded carrier preflight.
The existing installed-gopls six-file/five-group/ten-call pipeline invokes both
ranking algorithms after source deletion. Dense matrices exist only in tests.

The derivation is deductive from the user-assigned equation and explicit policies,
not a new discovery or approval exercise. Execution commands/results and remaining
qualification scope are recorded in
`/tmp/lsp-trace-bounded-ranking-a2fa0fc/CLAIM.md`. Crossbuild, public consistency,
and source-deleted fixture success do not upgrade any authority ceiling.
