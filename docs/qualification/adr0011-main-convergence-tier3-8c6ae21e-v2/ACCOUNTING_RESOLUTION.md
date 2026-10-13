# ADR0011 main-convergence Tier 3 v2 accounting resolution

## Result

**RESOLVED.** The number `3847` is correct for the authoritative frozen **C15 census v2 successor catalog**, but `docs/qualification/adr0011-main-convergence-tier3-8c6ae21e-v1/logs/c15-verification.txt` hashes and counts the different **C15 census v1 predecessor catalog**. The discrepancy is not a header/counting artifact and must not be normalized away.

The blocked v1 package was not edited. Its `MANIFEST.sha256` was checked successfully entry by entry after investigation.

## Source state and provenance boundary

- Candidate repository: `/tmp/adr0011-c07-c12-vet-cleanup-jnq6OQ`
- HEAD: `8c6ae21e3709b80f6abae0ca8b1df185d81d99d9`
- tree: `712a950f7f6c0b1c6a09f6d8178808dca88dbc0d`
- The blocked package `docs/qualification/adr0011-main-convergence-tier3-8c6ae21e-v1/` is wholly **untracked** at that HEAD. `git ls-tree` returns no entries for it and `git status --short` reports the directory as `??`.
- Consequently Git cannot establish that package's origin. Its current byte integrity is established by its internal `MANIFEST.sha256`, which passed completely.
- The canonical historical v1/v2 catalogs under `sessionruntime/testdata/` are tracked and clean.

## Exact artifacts

| Role | File | Bytes | Rows/newlines | Schema | SHA-256 |
|---|---|---:|---:|---|---|
| predecessor v1 candidates | `sessionruntime/testdata/c15-census-v1/candidates.tsv` | 541478 | 3805 | headerless TSV, exactly 2 fields | `13d45a61cb9ad2e5dad8d944458a9b7f459c9a2cd0336cb76700fa801599ca6e` |
| predecessor v1 classifications | `sessionruntime/testdata/c15-census-v1/classifications.tsv` | 687057 | 3805 | headerless TSV, exactly 5 fields | `cde6aa72034acf743c582cec56ff778470f8f258bde0ecdc4398653f3de10e04` |
| authoritative v2 candidates | `sessionruntime/testdata/c15-census-v2/candidates.tsv` | 547527 | 3847 | headerless TSV, exactly 2 fields | `536d11dcfa927d1ce5db24fbd8a956a5170a8f762ee53d8daa3664288172afb2` |
| authoritative v2 classifications | `sessionruntime/testdata/c15-census-v2/classifications.tsv` | 694776 | 3847 | headerless TSV, exactly 5 fields | `494d2f0045db3e34b7c03e2a959aaf21e2fee203bfd3768097df7930ddf6b730` |
| v2 provenance/contract | `sessionruntime/testdata/c15-census-v2/manifest.json` | n/a | n/a | `lsp-trace/c15-census-successor-v2` | `548e6af3625b248ab7e8a99b29ac24bbb3f674b0ffdb14365d4dfaaf49c89daf` |

All four TSVs end every row with LF. Therefore `wc -l`, `bytes.Count(data, []byte{'\n'})`, `splitlines()`, and the strict parser all return the same row cardinality for these exact files. No header is present or subtracted.

## Why the blocked package says 3805

`logs/c15-verification.txt` records:

```text
historical_bytes 541478
historical_sha256 13d45a61cb9ad2e5dad8d944458a9b7f459c9a2cd0336cb76700fa801599ca6e
historical_rows 3805
```

Those values identify `sessionruntime/testdata/c15-census-v1/candidates.tsv` exactly. Thus the verification log's mechanical result is correct for the file it selected, but its selection does not support README's phrase “The frozen historical census has 3,847 rows.”

The package does not retain the command that generated `c15-verification.txt`; `COMMANDS.txt` lists only broad command categories and the enumeration test. The exact original shell syntax is therefore unavailable. The selected artifact is nevertheless unambiguous from the unique byte length and SHA-256.

## Why 3847 is authoritative

`sessionruntime/testdata/c15-census-v2/manifest.json` states:

```json
"base_v1_count": 3805,
"final_v2_count": 3847,
"added_excluded": 42,
"removed": 0,
"reclassified": 0
```

It also pins the v1 predecessor hashes and records the canonical generation command:

```sh
C15_V2_REGENERATE=1 go test ./sessionruntime -run '^TestPrivateB4C15V2SuccessorCatalog$' -count=1
```

`sessionruntime/b4_c15_census_v2_successor_private_test.go` independently enforces:

- generated v2 candidate and classification newline counts equal 3847;
- v1 candidate and classification strict parsed counts equal 3805;
- candidates have exactly 2 TSV fields;
- classifications have exactly 5 TSV fields;
- IDs are nonempty and unique;
- every v1 candidate and classification row remains byte-identical in v2;
- exactly 42 IDs are added;
- every added classification is `excluded`, has an empty owner field, and has nonempty category and reason;
- zero rows are removed or reclassified;
- the 44 `C15-owned` mapping rows remain unchanged;
- generated v2 bytes and sidecars equal the frozen files.

The v2 candidate hash and count were independently recomputed above. Therefore 3847 is proven, not inferred.

## What the 42 rows represent

Strict candidate-ID set comparison gives:

- `v2 - v1`: exactly 42 IDs;
- `v1 - v2`: 0 IDs;
- shared candidate rows changed: 0;
- shared classification rows changed/reclassified: 0.

All 42 IDs are allocation candidates introduced for the call-hierarchy observation lease implementation (`alloc.b4.owned.method.pair.lease.*` and `alloc.sessionruntime.privatecallhierarchyobservationlease.*`). Their classifications are all `excluded`:

- **external-library** sites: mutex lock/unlock, condition wait/broadcast, `errors.New`, and composite literals outside `privateB4ByteAccountV2` custody;
- **ephemeral-runtime** sites: append/callback/copy/view/recover/call-expression operations with no `privateB4ByteAccountV2` owner operation at those sites.

They are not headers, blank lines, parser losses, filtered-out current rows, or the 44 accepted `C15-owned` operation-mapping rows. They are the explicitly retained v2 successor delta over v1.

## Separate current enumeration

The Tier 3 run outputs are a third dataset:

- `c15/run{1,2}/candidates.tsv`: 4155 headerless records, SHA-256 `aa54b17734ff8136bbe1df1b559cd5103a3b5a86c04547fc6b78e093928b4341`;
- `classifications.tsv`: 4155 headerless records, SHA-256 `bfbecdd3df47a415adfcfd36b3dd6db1cd45fcd64f1ead89596f8ac92f77989f`;
- `all-source-locations.tsv`: one header plus 4155 data records (4156 physical lines), SHA-256 `53bab2230a4ccbc12992b5f9163a882deb3f31bac792c252749e0cab47020043`.

The enumerator log reports `SUCCESS rows=4155 duplicates=0 mapping=44 locations=4155`. Its strict verification parses candidates/classifications as headerless TSV but parses locations with `csv.DictReader`, consuming the one location header. This header behavior is unrelated to the 3805/3847 discrepancy.

The committed package `docs/qualification/adr0011-c15-main-convergence-successor-v1/` identifies 3847 as its baseline and 4155 as current, with delta 310 added, 2 removed, 3 changed, and 3842 unchanged.

## Corrected successor-v2 statements

A successor package should say all of the following explicitly:

1. “The frozen C15 **v1 predecessor candidate catalog** has 3,805 headerless two-field rows, 541,478 bytes, SHA-256 `13d45a…ca6e`.”
2. “The frozen C15 **v2 successor candidate catalog**, which is the historical baseline for the 4,155-row main-convergence comparison, has 3,847 headerless two-field rows, 547,527 bytes, SHA-256 `536d11…fb2`.”
3. “The v2 successor is v1 plus exactly 42 candidate IDs: zero removed, zero shared candidate changes, zero shared classification changes/reclassifications; every added row is classified `excluded`.”
4. “The current main-convergence enumeration has 4,155 candidate/classification records; the location file has one header plus 4,155 data records.”
5. “The previous v1 Tier 3 package remains immutable and contains an accounting mismatch: README names 3,847 while its verification log selects and proves the 3,805-row v1 predecessor.”
6. “No claim should use the bare phrase ‘historical census’; every count/hash must name catalog version and exact path.”
7. “The Tier 3 package is untracked at the stated HEAD/tree; its byte integrity is self-manifest-backed, not Git-tree-backed.”

## Mechanical recurrence checks

Successor v2 should fail closed unless one script performs all checks below and emits a retained receipt:

1. Verify the blocked v1 package before and after work:
   ```sh
   (cd docs/qualification/adr0011-main-convergence-tier3-8c6ae21e-v1 && shasum -a 256 -c MANIFEST.sha256)
   ```
2. Pin exact role/path/count/bytes/hash tuples for v1, v2, and current catalogs; forbid an unlabeled `historical_rows` field.
3. Strictly parse v1/v2 candidates as headerless 2-field TSV and classifications as headerless 5-field TSV; reject malformed rows, empty IDs, duplicates, missing terminal LF, or any detected header.
4. Assert exact v1 hashes/counts: candidate `13d45a…ca6e`/3805 and classification `cde6aa…0e04`/3805.
5. Assert exact v2 hashes/counts: candidate `536d11…fb2`/3847 and classification `494d2f…3a2f`/3847; verify their `.sha256` sidecars.
6. Parse and hash `c15-census-v2/manifest.json`; require schema, canonical command, predecessor hashes, `3805`, `3847`, `42`, `removed=0`, `reclassified=0`, and `unchanged_mapping=44`.
7. Compare by candidate ID, not line ordinal: require 42 added, 0 removed, 0 changed candidate rows, 0 changed shared classification rows, and equal candidate/classification ID sets within each version.
8. Require each of the 42 v2-only classifications to have exactly five fields: `[id, "excluded", "", nonempty_category, nonempty_reason]`.
9. Separately validate current outputs: candidates/classifications 4155 headerless rows; locations exactly one recognized header plus 4155 data rows; equal ID sets; duplicate count zero; 44 `c15.*` mapping rows.
10. Generate human-readable README count/hash statements from the verified role-labeled receipt, or mechanically compare README assertions to it. Do not hand-copy counts.
11. Record the exact verification command, selected source path, parser identity, header policy, byte length, row count, and SHA-256 together in one machine-readable row.
12. Check `git ls-tree HEAD` and `git status --short` for the successor package and state tracked/untracked provenance accurately.

## Reproduction commands used

```sh
cd /tmp/adr0011-c07-c12-vet-cleanup-jnq6OQ
git rev-parse HEAD HEAD^{tree}
git status --short -- docs/qualification/adr0011-main-convergence-tier3-8c6ae21e-v1 sessionruntime/testdata/c15-census-v1 sessionruntime/testdata/c15-census-v2
shasum -a 256 sessionruntime/testdata/c15-census-v1/{candidates,classifications}.tsv sessionruntime/testdata/c15-census-v2/{candidates,classifications}.tsv sessionruntime/testdata/c15-census-v2/manifest.json
wc -l sessionruntime/testdata/c15-census-v1/{candidates,classifications}.tsv sessionruntime/testdata/c15-census-v2/{candidates,classifications}.tsv
(cd docs/qualification/adr0011-main-convergence-tier3-8c6ae21e-v1 && shasum -a 256 -c MANIFEST.sha256)
```

A strict Python parser was additionally used to validate field cardinality, unique/nonempty IDs, terminal-newline equivalence, exact set delta, shared-row identity, and all 42 v2-only classification tuples.
