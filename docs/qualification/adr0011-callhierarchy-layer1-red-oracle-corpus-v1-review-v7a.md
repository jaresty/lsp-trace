# ADR-0011 Call Hierarchy Layer-1 RED Oracle Corpus v1 — final mechanical qualification review v7a

Baseline reviewed: `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9`.

## Independent method

I used a fresh disposable read-only validator outside the repository. It parsed only the corrected design Markdown, rejected duplicate JSON keys, executed the four Table 2 mappings literally, partitioned every constructor input into emitted or validation-only sets, resolved exact Table 1 and Table 3 references, serialized canonical JSON as sorted-key compact UTF-8 plus one LF, and independently recomputed every declared length and SHA-256. It did not use runtime output, implementation behavior, generated corpus files, testdata, or the author self-check as authority.

The constructor mapping is now mechanically complete. `C_IDENTITY` and `C_LEDGER` emit every supplied input. `C_RECORD.kind` is validation-only and every other accepted input is emitted verbatim. `C_WORLD.name` is validation-only; provenance identifiers remain literal, the named Table 3 record alone is expanded, and `root_identity` is emitted verbatim. No constructor input is unaccounted for, and no emitted input overlaps a validation-only input.

## Exact reproduction

The design contains exactly four normative tables:

- Table 1: 860 rows; 860/860 canonical byte lengths and SHA-256 values reproduced.
- Table 2: 4 total constructors; all emitted/validation-only mappings and rejection key sets executed.
- Table 3: 63 complete records; 63/63 exact canonical bytes, lengths, and SHA-256 values reproduced.
- Table 4: 88 invocations: all 63 `C_RECORD` outputs and all 25 `C_WORLD` roots reproduced byte-for-byte with their exact published lengths and SHA-256 values.

The root-name-sorted aggregate over UTF-8 lines `root<TAB>expanded-sha256<LF>` is:

`88b897ed1e59e7d53c167d6cbd30c943f59b396e7e7ea6520a28b50598a56ac0`

## Closure and requested counts

```text
normative tables=4
roots=25
World 24 workers=16
World 25 executions=22
unaccounted constructor inputs=0
emitted/validation-only overlap=0
undefined references=0
cycles=0
partial constructors=0
missing fields=0
unexpected fields=0
canonical defects=0
byte mismatches=0
length mismatches=0
SHA-256 mismatches=0
```

World 12 is explicitly and coherently the null case: request payload unavailable with null bytes/length/hash, available response body containing top-level JSON-RPC `result:null`, `Result.form="null"` with exact `null` bytes, and terminal `successful-null`. World 13 is the distinct available zero-length request-payload and empty-result case.

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_DESIGN_GO`
