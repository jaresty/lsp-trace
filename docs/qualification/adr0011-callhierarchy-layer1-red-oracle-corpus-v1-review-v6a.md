# ADR-0011 Call Hierarchy Layer-1 RED Oracle Corpus v1 — Final Independent Mechanical Review v6a

**Verdict: `ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_DESIGN_BLOCKED`**

Reviewed exact repository HEAD `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9`, the corrected untracked design `docs/qualification/adr0011-callhierarchy-layer1-red-oracle-corpus-v1.md`, and no runtime or implementation-produced oracle output. Pre-existing worktree changes remain untouched. This review creates only this v6a document and authorizes no runtime edit, testdata, corpus bytes, fixture update, loader, source or RED edit, generated output, commit, or production claim.

## Fresh mechanical validator

A fresh disposable read-only validator parsed the normative markdown directly, canonicalized JSON as UTF-8 with sorted keys, compact separators, and one LF, recomputed stored row lengths and SHA-256 pins, expanded the explicit constructors according to Table 2, counted roots/workers/executions, checked reference closure, and directly compared World 12 with World 13.

The document contains exactly four normative tables. It contains 25 roots (`world-01` through `world-25`), 16 World 24 workers, and 22 World 25 executions. All 860 Table 1 and 63 Table 3 stored canonical values match their stated byte lengths and SHA-256 pins. No undefined references were found.

The requested zero-defect result does not hold under the normative constructor contract. The validator reports:

```text
normative tables=4
world roots=25
W24 workers=16
W25 executions=22
stored row length/hash defects=0
undefined references=0
World12/World13 contrast defects=0
C_RECORD invocation/output defects=63
Table 4 expanded length/hash defects=88
```

## Blocking constructor/pin contradiction

Table 2 defines `C_RECORD` over an explicit `kind` plus the named record fields and states:

> Neutral total assembly only: output fields are exactly the supplied fields.

Every one of the 63 Table 4 `C_RECORD` invocations supplies a `kind` field. For example, `CORE_01` supplies `"kind":"world"`. None of the 63 corresponding Table 3 complete records contains `kind`.

For the world records, this omission is exactly 15 canonical bytes: `"kind":"world",`. Resolving the Table 4 invocation field-for-field therefore produces 4,550 bytes for `CORE_01`, while the pinned Table 3/Table 4 value is 4,535 bytes. The resulting SHA-256 also differs. The same contract violation occurs across all 63 `C_RECORD` invocations. Because all 25 `C_WORLD` roots copy those records, every Table 4 expanded pin is inconsistent with the normative field-for-field constructor rule: 63 record outputs plus 25 world roots, or 88 defects total.

This cannot be dismissed as a harmless discriminator. Table 2 explicitly includes `kind` in the constructor domain, says output fields are exactly supplied fields, prohibits unknown fields, and describes neutral assembly. Either the constructor contract must explicitly define `kind` as a validated non-output discriminator, or `kind` must appear in every corresponding complete record with all dependent byte lengths and hashes repinned.

## World 12 null versus World 13 empty

The corrected semantic contrast is mechanically coherent:

- World 12 request payload is unavailable: `available=false` with null bytes, length, and SHA-256.
- World 12 response body contains top-level `"result":null`; its explicit result bytes are `6e756c6c`, form `null`; terminal and ledger terminal-after are `successful-null`.
- World 13 request payload is available with zero length, empty bytes, and SHA-256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.
- World 13 response body contains top-level `"result":[]`; its explicit result bytes are `5b5d`, form `empty`; terminal and ledger terminal-after are `successful-empty`.

Thus the prior World 12 result contradiction is corrected, and null remains distinct from available-empty.

## Disposition

Cardinality, stored row pins, reference closure, and the World 12/13 contrast pass. Final mechanical closure does not pass because the normative `C_RECORD` mapping and all dependent Table 4 expansion pins contradict each other.

`ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_DESIGN_GO` is not issued. Correct the `kind` constructor/output contract and repin whichever representation is made normative, then obtain another fresh independent mechanical review.
