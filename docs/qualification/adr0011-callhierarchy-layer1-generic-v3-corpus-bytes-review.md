# ADR-0011 call-hierarchy Layer-1 generic v3 corpus bytes review

## Disposition

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_CORPUS_BYTES_GO
```

Reviewed independently in exact checkout `/Users/schwa/dev/lsp-trace-adr0011-c17` at repository HEAD `f73233a8ca4ff42a1ab399b61b8ed95788c16ead`. Before this review document was created, the only untracked files were the v3 manifest and its 25 world files. This disposition accepts only those frozen v3 corpus bytes and their provenance/accounting. It grants no RED, implementation, runtime, production, activation, generated-output, commit, semantic, or feature authority.

## Canonical bytes, secure paths, and manifest accounting

Strict duplicate-key JSON decoding and an independent recursively key-sorted compact UTF-8 rebuild with exactly one terminal LF reproduced the manifest, registry, and every world byte-for-byte. The manifest has exact schema `adr0011.callhierarchy.layer1.red-oracle-corpus/v3` and lists only the lexical sequence `worlds/world-01.json` through `worlds/world-25.json`.

Every listed path is a relative slash-separated path with no empty, `.`, or `..` segment and no normalization change. The 25 targets are regular nonsymlink files, resolve beneath the corpus root, have unique paths and names, and exactly equal the complete contents of `worlds/`; there are no missing or extra world files.

Independent raw-byte reproduction yielded:

```text
manifest byte_length  4733
manifest sha256       960f0b72357492833cbf4d0f3039ed29504d65b7b67bcd915d8e731b57777a4c
registry byte_length  51098
registry sha256       8c660fa8450ddaf79ba6237fb6cb82695d28f4ea0c929420f5c80022def50933
aggregate byte_length 585600
aggregate sha256      32935f66e9f4310d1484bf420207ed32d65d972fafb35ef86d0439eebad96e24
```

Each complete world file independently matches its manifest byte length and SHA-256. Ordered concatenation of the 25 exact complete world bytes reproduces the aggregate length and digest above. The manifest registry pin exactly matches the tracked canonical registry bytes.

## Self-contained occurrence inventory

Independent structural traversal, without trusting manifest or registry totals, reproduced:

```text
root envelopes                    25
world-24 inline worker envelopes  16
world-25 inline executions        22
world-25 ordered pairs            11
pair-member envelopes             22
all envelope occurrences          85
```

All 85 occurrences are complete `{driver, expected}` envelopes. World 24 carries all 16 workers inline; world 25 carries all 22 executions and both complete members of each of its 11 ordered pairs inline. No envelope inherits a driver or commitment input from a root, sibling, name, pair label, registry lookup, or ambient resolver.

## Exact registry join and independent deterministic recomputation

The canonical registry strictly contains 85 unique records with exact class counts `25/16/22/22`. Stable IDs cover `root/01..25`, `w24-worker/01..16`, `w25-execution/01..22`, and explicit disabled/enabled members of `w25-pair/01..11`. Class-local ordinals, pair ordinals, member labels, order, uniqueness, and record shapes all join exactly to the corresponding structural corpus occurrence.

For every record, I independently recomputed the global-ordinal availability schedule and predecessor cardinality. I then independently regenerated each available selected, handoff, and predecessor input from the fixed seed and the normative `SHA256-DOMAIN-LP-V1` algorithm: big-endian length-prefixed seed/occurrence/role domains, independent length/material/identity domains, one-based role index, and zero-based material blocks. This recomputation did not read any corpus `expected` subtree.

All generated opaque materials reproduce the driver `bytes_hex`, byte length, identity, and SHA-256 exactly. Every unavailable commitment has `available=false`, `bytes_hex=null`, empty identity, zero length, and 64 zero digest characters. Predecessor order and cardinality are exact. For all 85 occurrences, registry metadata equals driver metadata, and expected selected/handoff/predecessor metadata equals the corresponding driver metadata after excluding the driver-only raw bytes.

The commitment material remains opaque synthetic causal input. Digest agreement grants no call-hierarchy semantics, authenticity, source completeness, producer identity, or production authority.

## V2 isolation and expected-path accounting

The manifest pins accepted v2 source commit `abef88564d02c240f354f3392f809407744b72d0` and path `sessionruntime/testdata/callhierarchy-layer1-red-v2`. That commit is reachable from reviewed HEAD. Its corpus tree equals the current HEAD v2 tree exactly:

```text
v2 tree f728e3afa7713b8df8717a2e99c982fd95036741
```

For every one of the 85 structural occurrences, recursive decoded comparison against that pinned v2 occurrence established:

- after removing only `driver.commitment_inputs` at every nested envelope, every other driver field, value, null, array member, order, and object is exactly unchanged;
- after removing only `expected.commitments` at every nested envelope, all other expected decoded values are exactly unchanged from v2;
- the 12 commitment expectation schema paths contain exactly the driver-derived availability, identity, length, digest, predecessor cardinality, and predecessor order, with no `bytes_hex` admitted to expected;
- the other 133 retained expected paths equal v2 at decoded-value level; and
- the expected denominator remains exactly `145 = 133 + 12`, with driver paths excluded.

The exact 17 removed/deferred v1 paths remain absent in all 85 expected records: four request-body fields, four request-payload fields, four response-body fields, four response-result fields, and `result.form`. No removed semantic field is restored under another expected key or through commitment inputs.

## Immutable predecessor trees

The accepted v1 pins and v2 source pin are reachable from reviewed HEAD. The current v1 and v2 corpus trees remain byte-identical to their immutable predecessors:

```text
v1 tree at HEAD, ea049c2, and 12d9af1
  7d278859a92b2f4438cce3c1f80f1591cd9a8c1a

v2 tree at HEAD and abef885
  f728e3afa7713b8df8717a2e99c982fd95036741
```

No v1 or v2 corpus byte was changed by the untracked v3 corpus.

## No reference, hidden-dispatch, or implementation-output dependency

The final v3 corpus contains no occurrence ID, registry reference, `$ref`, side-table key, inheritance rule, ambient resolver, record/update mode, golden rewrite, implementation-output marker, or runtime-output marker. The manifest source object contains exactly the canonical registry pin and the exact v2 commit/path pin. Each driver contains its own complete commitment bytes plus its unchanged v2 request, transport, and derived-external inputs.

Corpus bytes and hashes establish authoring provenance and self-contained oracle inputs only. They do not prove that a future RED executes all records, constructs production inputs only from driver data, prevents expected-fed actuals, avoids name/world dispatch, or observes implementation behavior. Those remain separate RED and implementation review obligations.

## Final disposition

The canonical manifest/world bytes, per-world and aggregate hashes, secure-path policy, `25/16/22/11/85` self-contained structure, exact 85-record registry join, independent deterministic commitment regeneration, driver-to-expected commitment equality, unchanged other 133 expected paths, exact `145/17` expected accounting, unchanged non-commitment driver fields, immutable reachable v1/v2 trees, and absence of runtime references or hidden provenance dependencies all pass.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_CORPUS_BYTES_GO
```
