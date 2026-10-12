# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Three-World Corpus Bytes and Provenance — Independent Review

## Disposition

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_THREE_WORLD_CORPUS_BYTES_PROVENANCE_GO
```

Reviewed independently in exact checkout `/Users/schwa/dev/lsp-trace-adr0011-c17` at pinned repository HEAD `3cf71757cf5f7eb36c87b0d115bcfc7cccde5ca6`, including the uncommitted V3 manifest and worlds 04, 05, and 23. Before this document was created, those were the only modified paths. This review created only this document and made no corpus, RED, test, source, implementation, runner, or production change.

This GO is limited to the exact reviewed corpus bytes and their manifest provenance. It grants no RED, source, test, implementation, runtime, activation, semantic, feature, or production authority.

## Independent method

A fresh strict duplicate-key JSON decoder and independent recursive comparison:

1. verified HEAD and the exact four-path pre-review worktree scope;
2. decoded the manifest and all 25 V3 worlds and rebuilt each as recursively key-sorted compact UTF-8 JSON with exactly one terminal LF;
3. compared every decoded target world recursively with its exact HEAD version;
4. enumerated all 85 physical root and embedded envelope occurrences and compared each corresponding occurrence with HEAD;
5. verified all pre-existing driver leaves and all expected-owned values remained unchanged;
6. independently recomputed each target world length and SHA-256, every manifest world entry, and the ordered aggregate length and SHA-256;
7. checked manifest path membership, lexical order, source pin, and exact diff scope; and
8. verified V1 and V2 remained unmodified at their accepted tree identities.

No rematerializer, prior verdict, or expected-owned value was used to derive the causal additions.

## Exact source and semantic diff scope

Before this review file was created, Git reported exactly these four modified paths:

```text
sessionruntime/testdata/callhierarchy-layer1-red-v3/manifest.json
sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-04.json
sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-05.json
sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-23.json
```

The decoded world diff against HEAD is exactly three additions, all at the root envelope's causal-input path:

```text
world-04  driver.request.causal_outcome =
          {"class":"accepted-terminal","terminal":"successful-empty"}

world-05  driver.request.causal_outcome =
          {"class":"accepted-terminal","terminal":"successful-empty"}

world-23  driver.request.causal_outcome =
          {"class":"pre-effect-refusal","refusal_class":"method-unavailable","refusal_reason":"method unavailable"}
```

The literal object shapes are exact: worlds 04 and 05 contain only `class` and `terminal`; world 23 contains only `class`, `refusal_class`, and `refusal_reason`. There are no extra, omitted, null, alternate-spelling, or alternate-value members.

Removing only `driver.request.causal_outcome` from each changed root makes the complete decoded world exactly equal to HEAD. Existing driver leaves compare unchanged at `35/35` for world 04, `40/40` for world 05, and `40/40` for world 23. Every expected-owned value is exactly unchanged.

## Physical occurrence preservation

Independent recursive traversal reproduced all 85 physical envelope occurrences:

```text
root envelopes                    25
world-24 inline worker envelopes  16
world-25 standalone executions    22
world-25 pair-member envelopes     22
all envelope occurrences           85
```

Exactly three occurrences differ from HEAD: the root envelopes of worlds 04, 05, and 23. The other 82 corresponding root or embedded occurrences are semantically identical to HEAD, and their deterministic compact subtree encodings are unchanged. No embedded worker, standalone execution, or pair member changed.

The other 22 V3 world files are exact byte-for-byte HEAD copies and decode to exactly the same values as HEAD.

## Canonical bytes, hashes, and manifest provenance

The manifest and all 25 worlds are strict-decode valid and canonical under the stated compact sorted-key UTF-8 plus terminal-LF rule. Manifest paths are exactly the regular files `worlds/world-01.json` through `worlds/world-25.json`, in lexical order. Every world length and SHA-256 matches its manifest entry.

```text
world-04 byte_length  8527
world-04 sha256       3d69449faa66007f3e310ae20e69a848a79ba5158236677b0a097217c63ddc87

world-05 byte_length  8856
world-05 sha256       e2e253772d588ac81409b063cd22cfb07db49b27bc55b43740f6a739216f52b4

world-23 byte_length  6496
world-23 sha256       6a7c5b428049a2fc9d9e5a6ab4c6f749bf8e52775e57bc882dfcca6b135942f8

aggregate byte_length 596611
aggregate sha256      c1dc668b0362abba95478759b288cf26e5e8ccc99cce7810ae8dd5e55e35ee0e

manifest byte_length  4733
manifest sha256       bc826f925ac1c9a67bd9ba5241a9525fee733edda75c59c0896de350660556b0
```

The ordered aggregate was recomputed directly by concatenating the 25 world files in manifest order. Relative to HEAD, the aggregate changes from `596334` / `b77c087868a00b45ef6f1510f3ed4f8914c1612c079dd20a353233ff983008bc` to the values above. The `277`-byte increase is exactly `77 + 77 + 123` from worlds 04, 05, and 23.

The manifest semantic diff is exactly:

```text
aggregate.byte_length
aggregate.sha256
worlds[3].byte_length
worlds[3].sha256
worlds[4].byte_length
worlds[4].sha256
worlds[22].byte_length
worlds[22].sha256
```

No schema, source, name, path, order, or unrelated manifest value changed. The manifest registry pin was independently verified:

```text
path         docs/qualification/adr0011-callhierarchy-layer1-generic-v3-causal-input-registry.json
byte_length  51098
sha256       8c660fa8450ddaf79ba6237fb6cb82695d28f4ea0c929420f5c80022def50933
```

## Immutable predecessors

V1 and V2 have no worktree modifications and retain their accepted HEAD tree identities:

```text
v1  7d278859a92b2f4438cce3c1f80f1591cd9a8c1a
v2  f728e3afa7713b8df8717a2e99c982fd95036741
```

No V1 or V2 corpus byte changed.

## Final status

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_THREE_WORLD_CORPUS_BYTES_PROVENANCE_GO
CHANGED CORPUS PATHS: MANIFEST + WORLD-04 + WORLD-05 + WORLD-23 ONLY
SEMANTIC WORLD DIFF: EXACTLY THREE ROOT CAUSAL-INPUT ADDITIONS
EXPECTED VALUES: UNCHANGED
UNRELATED DRIVER FIELDS: UNCHANGED
NON-TARGET PHYSICAL OCCURRENCES: 82 / 82 UNCHANGED
OTHER V3 WORLDS: 22 / 22 EXACT HEAD BYTES
MANIFEST DIFF: THREE WORLD LENGTH/HASH PAIRS + AGGREGATE LENGTH/HASH ONLY
V1/V2: UNCHANGED
```

The GO token applies only to these exact uncommitted V3 corpus bytes and their manifest accounting at pinned HEAD `3cf71757cf5f7eb36c87b0d115bcfc7cccde5ca6`.
