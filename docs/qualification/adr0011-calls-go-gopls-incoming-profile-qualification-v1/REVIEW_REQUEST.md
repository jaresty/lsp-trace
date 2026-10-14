# Review request: incoming Go/gopls provider-profile correction v1

Requested reviewer action: independently verify this exact package and either issue `ADR0011_QUALIFIED_CALLS_INCOMING_PROFILE_GO` or return exactly one bounded blocker. Producer disposition remains `AUTHORED_NOT_GO`; the producer does not self-issue GO.

## Scope

Review only `CALLS_GO_GOPLS_INCOMING_EXACT_V1` bound to hash-pinned accepted generic semantics for `CALLS_LSP_INCOMING_EXACT_V1`, using the exact 5900-target incoming workload evidence and the accepted incoming-custody correction v2 run pinned in `MANIFEST.json`.

## Required checks

1. Verify worktree HEAD `b4a86914702fd595f66bd1e0085c7063fe517144` and that changed files are confined to this provider-profile package.
2. Verify accepted generic dependency token `CALLS_LSP_INCOMING_EXACT_V1_SEMANTICS_GO`, commit `c3d77247f5a2e06c0d6ac5b1b55a3df7dbea588a`, package `docs/qualification/adr0011-calls-lsp-incoming-exact-v1-generic-semantics-v1`, `CHECKSUMS.sha256` hash `844e2c072474f6b3756ec221ea0b6df2888d775e9b866e1da0bbef030e35b351`, and `MANIFEST.json` hash `8657876ab14ad6c4939218331d8a58e3c222da61b21ae55200644e5c43674f7d`.
3. Verify accepted incoming custody correction token `INCOMING_CUSTODY_CORRECTION_EVIDENCE_GO`, run package commit `b4a86914702fd595f66bd1e0085c7063fe517144`, final checksums SHA-256 `ba003061790bb4033bcabf85480e92ec97aba4522ab9877d99ccabbf24db60a8`, final seal SHA-256 `a15ddcb57a0f908e13e7742dd0697dff717fd08e66e2aee28a478020f1c3ba34`, and 53380 artifacts.
4. Verify provider text does not re-author generic semantics and preserves every accepted generic semantic statement in both MANIFEST and RESULT: top-level response null vs []; missing fromRanges malformed; null fromRanges malformed; present-empty [] retained relationship/member with zero occurrence/range rows; nonempty fromRanges supplied ranges/order/duplicates/ordinals without synthesis; endpoint from.range/from.selectionRange identity/display spans never occurrence evidence and occurrences solely from fromRanges.
5. Verify v2 mandatory provider/runtime custody roles are present: gopls executable byte length `41038194`, complete prelaunch environment digest/key count, Go executable digest/byte length, and closed toolchain inventory.
6. Verify exact workload counts from historical and correction evidence: denominator 5900, attempted 5900, unattempted 0, one final disposition per ordinal 5900, non-empty 5608, empty 225, LSP-error failures 67, refusals 0.
7. Verify all 67 failure identities in `FAILURE_IDENTITIES.tsv` against retained failure evidence and ledger finals.
8. Verify numeric maxima and target witnesses from the accepted historical `NUMERIC_BOUNDS.json`; keep them measured-not-universal and do not derive or alter runtime memory ceilings.
9. Verify frontier status remains `UNKNOWN`; do not invent frontier/completeness beyond target-set accounting.
10. Verify historical-vs-correction accounting: final counts equal exactly; raw artifact inventories and custody records differ only by the bounded v2 custody rerun packaging and sealing.
11. Verify the reviewer NO_GO lineage: the prior final reviewer NO_GO was solely for verifier/tamper coverage, and the corrected verifier now fails semantic/authority mutations through assertion-specific checks rather than checksum mismatch alone.
12. Verify the occurrence-source policy escape correction: one canonical exact policy is present in MANIFEST, RESULT, and role artifacts, and tampering RESULT.semantic_artifacts.occurrence_source_policy to endpoint-span-derived occurrences or deleting it fails `RESULT_OCCURRENCE_SOURCE_POLICY_EXACT`.
13. Run verifier and tamper tests:

```sh
python3 docs/qualification/adr0011-calls-go-gopls-incoming-profile-qualification-v1/verify_incoming_provider_profile.py
python3 docs/qualification/adr0011-calls-go-gopls-incoming-profile-qualification-v1/verify_incoming_provider_profile.py --tamper-tests
```

## Explicit non-scope

No production admission, runtime code authority, schema publication, public activation, raw-custody redesign, memory-ceiling change, source-graph completeness, repository completeness, universal provider proof, composition authority, outgoing authority, custody-policy change, custody ownership transfer, or source-runtime change.

## Checksums

Current authored package checksums are recorded in `CHECKSUMS.sha256`.

AUTHORED_NOT_GO
