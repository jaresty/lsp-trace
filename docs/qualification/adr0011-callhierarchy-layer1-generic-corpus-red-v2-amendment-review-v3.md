# ADR-0011 call-hierarchy Layer-1 generic corpus RED v2 amendment review v3

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_RED_AMENDMENT_GO`

Final independent rereview was performed at exact repository HEAD `e16e1bc3aa95693d84e18b2f967fa35fa04c22e0`. The three prior untracked amendment/review files were preserved unchanged. This review accepts only the corrected amendment contract. It creates no implementation, behavior, activation, production-authority, generated-output, corpus-hash, RED-result, or commit authority beyond the amendment's explicitly staged successor-authoring gate.

## Exact historical source verification

The corrected split provenance is internally consistent and independently reproduced from committed Git objects:

- normative corpus-containing commit: `ea049c2dad9be74e1b3e316fe042430be7f04c8e`;
- later accepted identical-corpus commit: `12d9af179c49412c8203582a3fa52d7836da18b7`;
- accepted opacity-corrected RED fixture commit: `751b0276c46dbccc4899cfec32adf513c123b57f`;
- immutable historical manifest metadata only: `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9`.

Both corpus-containing commits resolve `sessionruntime/testdata/callhierarchy-layer1-red-v1` to Git tree `7d278859a92b2f4438cce3c1f80f1591cd9a8c1a`. At the normative corpus commit:

- `manifest.json` is blob `80ebe165f3b92a9678a0dc6d891c46649e24885e` and independently hashes to SHA-256 `c761601bf78195634163b98676e133bac2dff610c521be149b3445c9f620492a`;
- `worlds` is tree `2d11c2a58332d02c22ef27344a2f65a6c0165ca4`;
- boundary world blobs are `4c7bcfa1dc82401b65c58988102f75d97cefd971`, `e088a59c7e4203ab4db29102c8b2aa6129adc30c`, and `e0c12dfc2daa0390f6ef9b76960a750fb0fac6dc` for worlds 01, 24, and 25 respectively.

All 25 manifest entries were independently read from the normative commit and checked against their exact path, byte length, and SHA-256. Their ordered complete bytes reproduce aggregate length `431974` and aggregate SHA-256 `7fddcd923a9299c52014334d5460cb1a5ce78604e4a732faaef11d02adc30e84`.

At the RED fixture commit, `sessionruntime/b4_owned_method_pair_lease_test.go` is blob `daea113100184745998cde6961d3164c0a3431ce` and independently hashes to SHA-256 `e766074f72b2ff7d43a59d9532b52b3d5c150e4d30f3b61f431c9fab6a540dc2`.

The manifest's internal `source_head` is exactly `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9`, while that commit has no `sessionruntime/testdata/callhierarchy-layer1-red-v1` path. The amendment now correctly treats this value only as immutable historical metadata, never as a corpus-containing commit, checkout source, tree pin, or derivation authority. This resolves the v2 review's provenance blocker without rewriting history.

## Mechanical expectation accounting

Independent extraction of the amendment table verifies:

- 162 rows, consecutively numbered `001` through `162`;
- 162 unique expectation paths;
- 133 `PRESERVE GENERIC` paths;
- 12 `PRESERVE OPAQUE` paths;
- 17 `REMOVE/DEFER` paths;
- exact reconciliation `133 + 12 + 17 = 162`;
- exact successor expected-path universe `133 + 12 = 145`.

The first 146 paths match the common v1 root-record schema after treating array notation as schema notation and excluding specialized nested W24/W25 record containers. The remaining 16 paths are the named v1 RED derivations.

The 17 removals are exactly four request-body paths, four request-payload paths, four response-body paths, four response-result paths, and `result.form`. No other expected path is removed. Driver paths remain outside both the 162 and 145 denominators, are separately strict-validated, and are never compared with implementation output.

The projection rule is mechanically bounded: delete only those 17 paths, preserve every retained decoded value and occurrence, then canonicalize and hash each complete `{driver, expected}` envelope. Empty arrays and occurrence-specific unavailable structures do not synthesize missing values; the 145 count is the strict expected-schema path universe, not permission to invent absent occurrence data.

## Generic causal driver correction

The corrected amendment resolves the original causal-input blocker:

- `driver.request.params_state` plus opaque `params_hex` preserves nil, available-empty, and available-nonempty request params as causal input derived from committed v1 request-payload bytes before deletion.
- `driver.transport.events` supplies only ordered generic transport events and exact bytes/class/message/timing derived from the pinned v1 corpus and opacity-corrected RED fixture.
- The only event kinds are `response-frame`, `transport-error`, `cancel`, `deadline`, `shutdown`, and `unavailable`.
- Successor execution takes Params directly from the driver and dispatches solely on event kind and literal generic event fields. It must not parse frames, inspect world or record names, consult expected values, use `result.form`, recover rich payload/result subspans, or consult an ambient/hidden table.
- No driver or expected value may originate from production execution, observed implementation output, recording, update mode, or golden rewrite.

Accordingly, causal inputs no longer masquerade as expected outputs, and removal of the 17 rich expectation paths does not make deterministic successor execution impossible.

## Self-contained corpus and RED criteria

The committed v1 corpus independently reproduces the required structural cardinalities: 25 roots, 16 W24 workers, 22 W25 executions, and 11 W25 ordered disabled/enabled pairs. The amendment requires each successor occurrence—not merely each root—to become its own self-contained `{driver, expected}` envelope.

Acceptance of this amendment therefore requires successor authoring to preserve all of the following:

1. each root, worker, execution, disabled record, and enabled record carries its own complete applicable driver and projected expected occurrence;
2. no inline occurrence inherits a root driver or resolves one by world/record name;
3. strict driver validation and strict 145-path-universe expected validation remain separate;
4. whole-envelope canonical bytes, per-file hashes, aggregate hash, lexical world order, and secure path rules are independently reproduced;
5. the eventual v2 corpus and RED remain additive, while v1 corpus, RED, commits, and prior blocked reviews remain reachable and unchanged;
6. the later exact v2 lengths and hashes remain unknown until authored and independently reviewed.

## Owner/view/lease boundary

The amendment keeps the successor private schema finite and generic and confines any later implementation proposal to the already accepted files `sessionruntime/sessionruntime.go` and `sessionruntime/b4_owned_method_pair_lease.go`.

The existing seam contains the required ownership/lifecycle mechanisms: private owner and lease types, closed-admission checks, active-callback accounting, callback execution with panic containment, drain-before-release, released-state publication, state clearing, and exactly-one release initiation/winner. The corrected contract additionally requires defensive copying of all retained bytes and ordered collections, view admission only while open, callback execution outside the owner lock, terminal closure before drain, and enabled/disabled external transparency.

The permitted private data remains generic: identity, complete frames, opaque commitment metadata, raw result/error/external effects, accounting, ledger, terminal, lifecycle, and refusal/control. Body/payload/result subspans, offsets, parsed/token forms, `result.form`, commitment preimages, rich predecessor lineage, call-hierarchy semantics, source completeness, and production authority remain forbidden.

## Final gate

The two prior blockers are corrected:

- causal request/transport inputs now live in a strict corpus-resident generic driver outside oracle accounting; and
- actual corpus-containing and RED-fixture commits are separately pinned, while `source_head` is correctly bounded as historical metadata only.

The amendment is self-contained enough to govern later additive v2 corpus and RED authoring, subject to its own mandatory independent verification of the newly produced canonical bytes and hashes. It does not authorize implementation or behavior.

`ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_RED_AMENDMENT_GO`
