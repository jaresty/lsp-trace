# ADR-0011 call-hierarchy Layer 1 generic v3 production-admission evidence v2

**Status: `REVIEW_REQUIRED`**  
**Bounded adversarial disposition: `ADVERSARIAL_CORRECTED_PATCH_GO`**

This is a durable, fail-closed evidence package rooted at parent HEAD `57b088e56f4ac5b0e5162a03a2f7d695f34085b1`. It preserves the corrected four-file patch and the complete supplied disposable evidence tree byte-for-byte under `evidence/`.

The bounded GO disposition means only that the retained corrected-patch guards, focused/count/race/targeted runs, C15 enumeration/adjudication, exact-two-pin delta proof, and cleanup evidence support continued human review of this exact patch. It is not admission or authorization.

This package grants **NO production writer authority, NO reconstruction authority, NO merge authority, NO admission authority, NO directional-behavior authority, NO public authority, NO release authority, and NO deploy authority**. It also grants no implementation, acceptance, approval, lifecycle, inventory, runtime-frequency, allocator, execution, custody, completeness, or source-authentication authority.

## Exact patch boundary

`PATCH.diff` has SHA-256 `ca762452805215750a122ee250e92326387c343e92c36b844fcaf12ad2cefcba` and changes exactly:

- `sessionruntime/b4_callhierarchy_layer1_generic_v3_test.go`
- `sessionruntime/b4_owned_method_pair_lease.go`
- `sessionruntime/owned_method_pair.go`
- `sessionruntime/sessionruntime.go`

The retained replay evidence proves the three production files equal the prior accepted patch byte-for-byte. The only source correction is the test map-key alignment from `W04`, `W05`, and `W23` to `world-04`, `world-05`, and `world-23`.

## Corpus boundary

The canonical corpus is restored. The admitted patch contains no corpus path and no corpus patch. `evidence/corpus-placement.patch` is retained only as historical disposable evidence; it is not part of `PATCH.diff`, is not admitted, and grants no reconstruction authority.

## C15 and source ownership

The complete supplied C15 tree is retained at `evidence/c15/`, including the derived enumerator, canonical/rerun logs, complete corrected adjudication in TSV/JSON/Markdown, source trace, cleanup/status receipts, prior-replay production equality, and exact-two-pin test-source delta proof.

The retained C15 disposition is `C15_DELTA_GO` with `29 NECESSARY / 4 REUSE / 319 FALSE_POSITIVE / 0 UNRESOLVED`. Source-ownership review disposition remains **`REVIEW_REQUIRED`**: the retained records are review evidence and do not confer production ownership, writer, reconstruction, merge, or admission authority.

See `ADMISSION.md` for the fail-closed gate interpretation and `MANIFEST.json` for bytes and SHA-256 of every package file except the self-referential checksum file.

## Integrity

From this directory:

```sh
sha256sum -c SHA256SUMS
```

`SHA256SUMS` covers every other regular file, including `MANIFEST.json`, and excludes only itself.
