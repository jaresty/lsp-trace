# ADR-0011 Call-Hierarchy Scaffold C15 Impact Assessment

Status: **CORRECTED / EXPERIMENTAL EVIDENCE RETAINED / WRITE GATE NOT ISSUED**

Audit worktree: `/Users/schwa/dev/lsp-trace-adr0011-c17`  
Audit HEAD: `2f5fd760f8d39cf0bca63258291bffd67f44a06c`  
Evidence packet: `docs/qualification/evidence/adr0011-callhierarchy-scaffold-c15-experiment-v1/`

## Corrected decision summary

The prior unreproducible `3,805→3,888` / `+83` claim is withdrawn. At the audited, unmodified HEAD, the canonical v1 verification reproduces the immutable **3,805-row** candidate and classification artifacts byte-for-byte.

A separate accepted disposable experiment applied the exact lifecycle-only `source.patch` to that HEAD and regenerated the catalog. For that pinned overlay only, the exact result is:

- candidates: **3,805→3,838 (`+33`)**;
- classifications: **3,805→3,838 (`+33`)**;
- predecessor rows retained byte-identically: **3,805**;
- additions: **33**, all classified `excluded`;
- removals: **0**;
- changed/reclassified rows: **0**;
- accepted operation mapping: **44** rows, unchanged at 15 owner / 7 alias / 22 copy;
- Manager propagation: absent under the retained check.

This is experimental impact evidence, not accepted implementation and not a write gate. The `+33` result is exact only for the pinned patch. Final lifecycle implementation spelling may produce different AST-derived identities, hashes, counts, or classifications and must receive a new exact independent finite-delta review.

## Three distinct verification states

### 1. Immutable v1 verification at HEAD

At unmodified HEAD, run:

```sh
go test ./sessionruntime \
  -run '^(TestPrivateB4C15OfflineCatalogAndCompactProductionContract|TestPrivateB4C15RegenerateOfflineCatalog|TestPrivateB4C15RegenerationDoesNotReadGeneratedOutputs|TestPrivateB4C15ProductionSiteBindings|TestPrivateB4C15MappingOrderIsDeterministic)$' \
  -count=1
```

This verifies the existing 3,805-row `c15-census-v1` bytes. It does not produce a lifecycle delta.

### 2. Experimental reproduction of the pinned patch

In a disposable clean checkout at the audited HEAD, use the packet-local patch:

```sh
git apply --check docs/qualification/evidence/adr0011-callhierarchy-scaffold-c15-experiment-v1/source.patch
git apply docs/qualification/evidence/adr0011-callhierarchy-scaffold-c15-experiment-v1/source.patch
C15_REGENERATE=1 go test ./sessionruntime \
  -run '^TestPrivateB4C15RegenerateOfflineCatalog$' \
  -count=1
```

The regeneration command mutates v1 paths and is therefore authorized only in a disposable experiment. Compare its outputs against the packet's before/after artifacts and `delta.json`, then discard/reset the checkout. It is never an accepted in-repository update procedure.

### 3. Final v2 generation after accepted lifecycle implementation

Only after lifecycle source and tests are accepted may a successor-specific generator/test contract produce `c15-census-v2`. The final accepted source spelling—not this experiment—must be used. A reviewer must independently establish exact candidate/classification bytes, sidecars, counts, additions, removals, changed/reclassified rows, unchanged or separately adjudicated mapping, determinism, and non-reading of generated outputs.

No experimental count or hash may be copied forward as an expectation unless the final run independently reproduces it.

## Exact experiment pins and packet locators

| Item | Exact value | Packet locator |
|---|---|---|
| Base HEAD | `2f5fd760f8d39cf0bca63258291bffd67f44a06c` | `head-before-reset.txt`, `context.txt` |
| Lifecycle-only patch SHA-256 | `2934e297165b05cd986c96c20888f74a5f22e7cb3ba555f4c099883ece48c269` | `source.patch`, `source.patch.sha256` |
| Before candidates SHA-256 | `13d45a61cb9ad2e5dad8d944458a9b7f459c9a2cd0336cb76700fa801599ca6e` | `candidates.before.tsv`, `generated-before.sha256` |
| After candidates SHA-256 | `d602e69e37eb64adf80b58bd29096ae405ec0eb5dbea36ab7d601b0ecf6e0818` | `candidates.after.tsv`, `generated-after.sha256` |
| Before classifications SHA-256 | `cde6aa72034acf743c582cec56ff778470f8f258bde0ecdc4398653f3de10e04` | `classifications.before.tsv`, `generated-before.sha256` |
| After classifications SHA-256 | `ea61d01ef5f1010a0b6bf1c8956f2c889bdb20ee19122986e147c68231c99365` | `classifications.after.tsv`, `generated-after.sha256` |
| Finite delta | 3,805→3,838; +33; 0 removed; 0 changed/reclassified | `delta.json`, `delta-summary.txt`, `*.added.tsv`, `*.removed.tsv`, `*.changed.tsv` |
| Classification result | 33 added, all excluded | `classification-verification.txt`, `classifications.added.tsv` |
| Mapping result | 44 total; 15/7/22 | `mapping-count.txt`, `mapping.sha256` |
| Nonpropagation result | Manager propagation absent; retained sessionruntime Go diff empty | `manager-nonpropagation.txt` |
| Verification/status | temporary test removed; package compiles; exact mutation/reset inventory retained | `lifecycle-verification.txt`, `package-compile.txt`, `status.*.txt`, `changed-paths.before-reset.txt`, `numstat.before-reset.txt` |
| Complete repository-relative packet manifest | SHA-256 for every retained file | `SHA256SUMS` |

`attempt-overbroad-149/**`, `analyze_delta.py`, and ambiguous/self-referential packet manifests are intentionally excluded from the repository packet.

## Authority and causal boundary

The pinned lifecycle patch adds AST-visible construction and synchronization expressions. The retained exact delta proves that those particular bytes add 33 source-derived candidates and 33 matching typed exclusions while retaining all predecessor rows byte-identically. It does not establish runtime allocation volume, production execution, feature identity, semantic ownership, Manager ownership, or final implementation shape.

AST presence is not semantic admission. Any future proposal to classify a successor-only row as C15-owned requires separate semantic adjudication and explicit operation, boundary, capacity, failure, nil/default-off, binding, adjacency, and falsification evidence.

## Immutable v1 and versioned-successor policy

1. **MUST NOT** overwrite, regenerate in place, rename, delete, or normalize any retained `sessionruntime/testdata/c15-census-v1` artifact or sidecar.
2. **MUST NOT** rewrite closure pins or historical evidence to make v1 appear current.
3. **MUST retain** the 3,805-row v1 census and hashes as immutable historical evidence.
4. **MUST retain** this experiment as versioned non-authoritative evidence; corrections require a new successor packet rather than mutation of `experiment-v1`.
5. **MUST NOT** treat the pinned `+33` as the final v2 delta without independent reproduction from accepted final source.
6. **MUST NOT** alter the accepted 44-row mapping without separate semantic adjudication.
7. **MUST NOT** overwrite provisional, review, reproduction, or audit evidence to collapse predecessor and successor states.

## Proposed write-set after a future gate

If a later gate accepts the lifecycle implementation and separately authorizes successor generation, the proposed write-set may include only:

1. accepted lifecycle source files and their lifecycle tests;
2. exact `sessionruntime/testdata/c15-census-v2/candidates.tsv`;
3. exact `sessionruntime/testdata/c15-census-v2/candidates.tsv.sha256`;
4. exact `sessionruntime/testdata/c15-census-v2/classifications.tsv`;
5. exact `sessionruntime/testdata/c15-census-v2/classifications.tsv.sha256`;
6. successor-specific generator/test metadata needed to bind v2 inputs, deterministic generation, finite-delta checks, sidecar verification, non-reading of generated outputs, and the unchanged-44 mapping guard.

The exact source/test paths and exact v2 bytes must be named by that future accepted implementation and independent delta review. No v1 artifact may be overwritten, redirected, or refreshed. No mapping, closure-matrix, unrelated source/test, catalog, or generated production binding change is implied.

## Gate

No GO token is issued. No implementation, v2 generation, v1 mutation, or commit is authorized by this assessment.
