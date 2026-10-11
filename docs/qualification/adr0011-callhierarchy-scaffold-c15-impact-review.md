# ADR-0011 Call-Hierarchy Scaffold C15 Impact Review

Status: **INDEPENDENT REVIEW / WRITE-SET BLOCKED**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `2f5fd760f8d39cf0bca63258291bffd67f44a06c`
- Reviewed assessment: `docs/qualification/adr0011-callhierarchy-scaffold-c15-impact-assessment.md`
- Assessment SHA-256: `d7a379581782b5957a91a40f77766bc6258c3da747fae226cf1078ed9abacace`
- Review authority: write-set acceptance only; no implementation, source, test, catalog, semantic-adjudication, evidence-rewrite, production, or commit authority
- Disposition: `ADR0011_CALLHIERARCHY_SCAFFOLD_C15_WRITESET_BLOCKED`

## Decision

The proposed v2 shape is appropriately narrow in intent, and the v1 immutability boundary is correctly stated, but the write-set gate cannot be issued. The assessment's central `3,805→3,888`, `+83`, all-excluded reproduction is contradicted by the assessment's own canonical non-writing command at the reviewed HEAD. That command passes the exact current-source-to-v1 byte comparison. No independent retained artifact, command output, source pin, or generated successor bytes establish 3,888 rows or the asserted finite delta.

## Verified v1 classification and immutability

The assessment correctly classifies the retained v1 artifacts as historical evidence rather than mutable current-source outputs:

- `candidates.tsv`: SHA-256 `13d45a61cb9ad2e5dad8d944458a9b7f459c9a2cd0336cb76700fa801599ca6e`, 3,805 rows;
- `candidates.tsv.sha256`: SHA-256 `9decfd720cf1303cf4313ecb97028c595d401e79a682d8d7378b9075fa07edbc`;
- `classifications.tsv`: SHA-256 `cde6aa72034acf743c582cec56ff778470f8f258bde0ecdc4398653f3de10e04`, 3,805 rows;
- `classifications.tsv.sha256`: SHA-256 `39e398d6832636adc691919f4113ce44d196daee3ecbad3c409c404f2de54369`;
- `operation-mapping.json`: SHA-256 `34a8ffa1bbc26775c42a2329f5ca9652444e6832ba790a3cc8673f4b15c4ffd1`, 44 rows with owner/alias/copy split 15/7/22;
- `operation-mapping.json.sha256`: SHA-256 `9be2bc6e797682f364090ae65d6437d2cec9acab48c6e533fa36942472659b5b`;
- `operation-mapping-comparison.json`: SHA-256 `438ce86d7d59870fd8b4c0fec4c048e78a23b9cd45e3a71cc538e8d9ce7e16d6`, denominator 44 and no accepted mapping change;
- `owner-operations.tsv`: SHA-256 `6834dc9a157ac944e3141cb85783c36251f92d383c838c205b063a1ac37014ff`;
- `owner-operations.tsv.sha256`: SHA-256 `467501394fc8e3947442ac5a7ee6e763a41c0a5cc8033bdf09aaf2005b5f6fe9`.

The closure matrix pins the five primary C15 artifacts, identifies audited commit `1d12538874fa6cfd80e9fe5a313e70764c9740f3`, and retains the closure sequence `f820b11f…`, `0a55d8e4…`, and `1d125388…`. Those references make v1 immutable historical closure evidence; they do not authorize normalizing v1 to later source.

The scaffold commits introduce no tracked change to `sessionruntime/testdata/c15-census-v1/`, `sessionruntime/b4_c15_ownership_registry_red_private_test.go`, the accepted operation mapping, semantic contracts, Manager ownership behavior, or `docs/qualification/adr0011-c-closure-matrix.json`. The sole pre-review untracked file was the assessed document.

## Canonical generator and command verification

The assessment correctly identifies the generator mechanics:

- `discoverPrivateB4C15Candidates` enumerates build-active, non-test Go source ASTs and excludes `*_test.go`;
- `generatePrivateB4C15Catalog` emits each discovered unmapped candidate as a typed `excluded` row, then appends the accepted 44 `C15-owned` mapping rows;
- `readPinnedFile` verifies both primary bytes and sidecar content;
- `TestPrivateB4C15RegenerationDoesNotReadGeneratedOutputs` proves generation does not consume the generated candidate/classification snapshots;
- `C15_REGENERATE=1` and `C15_REFRESH_MAPPING=1` are mutation paths and are not authorized for v1.

The assessment's canonical non-writing command was run exactly from the reviewed repository:

```sh
go test ./sessionruntime \
  -run '^(TestPrivateB4C15OfflineCatalogAndCompactProductionContract|TestPrivateB4C15RegenerateOfflineCatalog|TestPrivateB4C15RegenerationDoesNotReadGeneratedOutputs|TestPrivateB4C15ProductionSiteBindings|TestPrivateB4C15MappingOrderIsDeterministic)$' \
  -count=1
```

Result: five tests passed.

A separate exact run of `TestPrivateB4C15RegenerateOfflineCatalog` also passed. That test calls `generatePrivateB4C15Catalog` over current build-active source and byte-compares the result to the pinned v1 `candidates.tsv` and `classifications.tsv`. Therefore the canonical command does **not** reproduce 3,888 rows or a `+83` delta at this HEAD; it reproduces the existing 3,805-row v1 bytes.

## Exact blocking defects

### B1 — the claimed 3,805→3,888 reproduction is false under the stated canonical command

The assessment states that a clean non-writing reproduction yields 3,888 current-source candidates and `+83` additions, all excluded. Its stated command instead passes `TestPrivateB4C15RegenerateOfflineCatalog`, whose success requires current generated candidate and classification bytes to equal the pinned 3,805-row v1 bytes exactly.

The repository search found no independent 3,888-row artifact or `+83` reproduction record outside the assessment. The assessment also records no scratch harness, alternate generator revision, source overlay, output digest, row-ID list, or retained command transcript capable of establishing the claimed delta.

Required correction: preserve the present assessment as predecessor evidence, then produce a corrected, independently reproducible assessment bound to the final source spelling. It must name the exact source revision or overlay, exact generator revision, exact non-writing command, exact generated byte hashes/counts, and a retained finite-delta artifact or deterministic test that proves additions, removals, and reclassifications. Do not infer `+83` from source-shape intuition.

### B2 — “all excluded” and zero-removal/zero-reclassification are unsupported without exact successor rows

The generator rule explains how newly discovered unmapped candidates would be classified, but it does not prove that the alleged 83 rows exist, that their IDs are additions rather than ordinal renames, or that no historical row was removed or reclassified. AST-derived IDs contain enclosing and per-kind ordinal components, so inserted expressions can renumber later IDs even when semantic ownership is unchanged.

Required correction: compare exact predecessor and successor row identities and digests, retain the complete finite delta, and prove every successor-only row's typed classification. If final source spelling changes the count or IDs, report the actual result rather than preserving 3,888 as an expectation.

### B3 — the causal statement exceeds the available reproduction

Defensive-copy and constructor expressions can mechanically create allocation-like AST rows. That fact can explain candidate growth once exact rows are reproduced. It does not establish this assessment's specific count, row identity, or all-excluded classification. It also confers no semantic ownership, authority, runtime-volume, execution, or Manager behavior.

Required correction: narrow the causal claim to the exact reproduced successor-only rows. Preserve the authority statement that AST presence alone cannot alter C15 ownership.

## Successor write-set review

Subject to resolving B1–B3 after final source spelling, the proposed successor boundary is the smallest acceptable shape:

1. preserve every v1 byte, sidecar, closure reference, and semantic mapping read-only;
2. create only versioned `c15-census-v2` candidate/classification outputs and their sidecars from final source;
3. add one narrowly scoped successor generator/test file owning deterministic generation, source/input pins, finite v1→v2 comparison, sidecar/hash verification, no-generated-output-read proof, and the unchanged-44 guard;
4. make no v1 operation-mapping change unless a separate semantic adjudication explicitly authorizes it;
5. perform no broad regeneration, source change, Manager change, catalog change, closure rewrite, unrelated test change, or cleanup.

The final-successor caveat is required: row counts, bytes, byte lengths, hashes, and final spelling remain `UNKNOWN` until the final lifecycle source is fixed and a corrected successor reproduction is run. The current `3,888` value must not be treated as frozen or normative.

## Authority boundary

This review accepts neither feature identity nor semantic ownership from AST structure. Defensive-copy allocations may require mechanically enumerated AST rows, but they cannot change accepted C15 owner/alias/copy behavior, Manager authority, runtime allocation volume, production execution, or any evidence/qualification status. Any mapping change requires separate semantic adjudication and its own exact authority.

No implementation authority is granted. No v2 artifact, generator, test metadata, source, catalog, closure, mapping, Manager, or commit change is authorized by this review.

## Gate

`ADR0011_CALLHIERARCHY_SCAFFOLD_C15_WRITESET_GO` is **not issued**.

Exact disposition:

```text
ADR0011_CALLHIERARCHY_SCAFFOLD_C15_WRITESET_BLOCKED
```
