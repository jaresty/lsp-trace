# ADR-0011 call-hierarchy Layer-1 generic corpus RED v2 amendment review v2

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_RED_AMENDMENT_BLOCKED`

Reviewed independently at exact repository HEAD `e16e1bc3aa95693d84e18b2f967fa35fa04c22e0`. The preserved prior review and corrected amendment were the only pre-existing worktree changes. This review creates no corpus, RED, implementation, behavior, activation, generated-output, commit, or successor-authoring authority.

## Verified correction

The corrected amendment resolves the prior causal-input contradiction in principle:

- Every root record and every complete W24/W25 inline record is required to be a self-contained envelope with exactly two siblings, `driver` and `expected`.
- `driver` is independently strict-validated, is never traversed by expected-path validation, is never compared with implementation output, and contributes no path to either expectation denominator.
- `driver.request.params_state` and `params_hex` replace the removed request-payload fields only as causal input. They are mechanically extracted from the fixed v1 request-payload bytes before deletion and are passed directly to `RoundTripRequest.Params`; the successor must not parse the complete request frame.
- `driver.transport.events` replaces the old `result.form`-selected fake-transport behavior only as causal input. Event kind, exact bytes, class, message, ordering, and nanosecond timing are derived from fixed v1 corpus/fixture inputs, never from runtime or implementation output.
- Dispatch is solely by the six generic event kinds `response-frame`, `transport-error`, `cancel`, `deadline`, `shutdown`, and `unavailable`. The contract forbids semantic token/form labels, request/response body or payload/result subspans, offsets, symbol/call-hierarchy semantics, world-name dispatch, expected-value dispatch, hidden side tables, ambient resolvers, and implementation-produced inputs.
- Renaming a world with an unchanged driver must not change execution; changing a driver event with an unchanged world name must change execution accordingly.

This is sufficient to make a strict successor RED execution contract possible once its source provenance is made internally consistent.

## Expectation accounting and projection

The preserved independent mechanical review remains applicable and the corrected amendment does not alter the inventory:

- 162 consecutively numbered and unique v1 expectation paths;
- 133 `PRESERVE GENERIC` paths;
- 12 `PRESERVE OPAQUE` paths;
- 17 `REMOVE/DEFER` paths;
- `133 + 12 + 17 = 162`;
- successor expected denominator `133 + 12 = 145`.

The 17 removals remain exactly four request-body fields, four request-payload fields, four response-body fields, four response-result fields, and `result.form`. Driver paths are explicitly outside this accounting and do not replace removed expectation fields. Every retained expected literal or named derivation remains fixed from v1 input bytes; no expected value may originate in production execution, recording, updating, or observed implementation output.

The projection remains additive and strict: delete only the 17 named expected paths, preserve every retained decoded value and occurrence, canonicalize whole `{driver, expected}` envelopes, separately validate the finite driver schema and exact 145-path expected schema, and hash each whole envelope.

## W24/W25 and self-containment

The corrected contract retains the exact cardinalities:

- 25 root envelopes;
- 16 complete inline worker envelopes in W24;
- 22 complete inline execution envelopes in W25;
- 11 ordered W25 pairs whose disabled and enabled records are themselves complete envelopes.

No inline record may inherit a root driver or resolve a driver by record/world name. There is no permitted hidden table or external driver resolver.

## Finite owner/view/lease boundary

The finite private schema remains confined to the two already accepted production files:

- `sessionruntime/sessionruntime.go`;
- `sessionruntime/b4_owned_method_pair_lease.go`.

The inspected owner/view/lease seam supports the required finite lifecycle: defensive copying before publication, admission only while open, callback execution outside the owner lock, panic containment, terminal admission closure, active-callback drain, owned-state clearing, and exactly one release winner. The amendment authorizes no edit to either file and no implementation behavior.

The private data boundary remains generic: identity, complete frames, opaque commitment metadata, raw result/error/external effects, accounting, ledger, terminal, lifecycle, and refusal/control state. It excludes subspans, token forms, `result.form`, commitment preimages, rich predecessor lineage, symbol/item/range/call-site/caller/callee/direction/provider semantics, source completeness, and production authority.

## Blocking provenance contradiction

The amendment pins all derivation sources at accepted source HEAD:

`4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9`

That commit does **not** contain:

`sessionruntime/testdata/callhierarchy-layer1-red-v1/manifest.json`

An exact Git lookup fails with:

```text
fatal: path 'sessionruntime/testdata/callhierarchy-layer1-red-v1/manifest.json' exists on disk, but not in '4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9'
```

The manifest first appears in reachable commit `fee5cbe0eb76c3eb15acf8bb1b06b095f69b2c96`. The current accepted files do independently reproduce the amendment's byte pins:

- manifest SHA-256 `c761601bf78195634163b98676e133bac2dff610c521be149b3445c9f620492a`;
- v1 RED test SHA-256 `e766074f72b2ff7d43a59d9532b52b3d5c150e4d30f3b61f431c9fab6a540dc2`.

But correct individual file hashes do not repair an impossible combined provenance statement. Mandatory derivation step 1 requires verifying the pinned manifest and v1 RED fixture at the stated source HEAD before deriving anything. That strict verification cannot succeed as written. Corpus or RED authoring against a different commit would violate the exact provenance contract.

## Required correction

Replace the impossible single source-HEAD pin with exact, mutually consistent provenance. Either:

1. pin one accepted commit that contains every named derivation source at the stated exact hashes; or
2. explicitly pin each source to its own exact reachable commit and exact file hash, then update derivation step 1 to verify that split provenance without ambiguity.

The correction must not change the verified `162 -> 145` accounting, generic driver schema, W24/W25 self-containment, fixed-input-only derivation, forbidden-semantic boundary, finite two-file owner/view/lease scope, or no-output-custody rule.

Until that exact provenance contradiction is corrected and independently reviewed, successor corpus and RED authoring are not authorized. Implementation and behavior remain unauthorized regardless.

`ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_RED_AMENDMENT_BLOCKED`
