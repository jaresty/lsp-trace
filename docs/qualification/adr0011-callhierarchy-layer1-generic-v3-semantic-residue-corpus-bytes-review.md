# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Semantic-Residue Corpus Bytes Review

Status: **GO**

## Review boundary

This is an independent strict review of exact repository `/Users/schwa/dev/lsp-trace-adr0011-c17` at HEAD `fa8b0af3df00bec4bce4acc06f2ebb4f73a5c412` plus exactly eleven modified v3 corpus files. This review created only this document and made no corpus or other edit.

The accepted self-contained `driver` request, commitment, derived-input, and ordered transport values are the sole causal operands. Existing or historical `expected` values, world names, RED output, implementation output, and runtime output were not used as oracle operands. This disposition accepts only the bounded corpus-byte correction; it grants no RED, implementation, runtime, production, activation, semantic, feature, or source-completeness authority.

## Exact write set and occurrence census

Before this review document was created, `git diff --name-only` was exactly:

```text
sessionruntime/testdata/callhierarchy-layer1-red-v3/manifest.json
sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-08.json
sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-09.json
sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-10.json
sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-11.json
sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-17.json
sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-18.json
sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-19.json
sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-20.json
sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-21.json
sessionruntime/testdata/callhierarchy-layer1-red-v3/worlds/world-22.json
```

Independent recursive traversal reproduced exactly 85 self-contained occurrences:

```text
25 roots + 16 World-24 workers + 22 World-25 executions + 22 World-25 pair members = 85
```

Decoded comparison to HEAD found expected-value changes in exactly ten occurrences: roots World 08 through World 11 and World 17 through World 22. Every decoded `driver` value is unchanged in all 85 occurrences. No nested worker, execution, or pair-member occurrence changes.

## Driver-only semantic-residue correction

### Worlds 08–11

The four accepted drivers contain exactly one control event and no `response-frame`: `cancel`, `deadline`, `shutdown`, and `unavailable`, respectively. Those events establish generic runtime classes but cannot establish a server-provided JSON-RPC error object.

Each root changes exactly these seven A paths:

```text
error.available
error.present
error.code
error.message
error.data_base64
error.data_form
error.serialized_hex
```

and exactly this one B path:

```text
derived.external.server_error_bytes
```

The corrected values are `error.available=false`, `error.present=false`, nullable detailed payload members `null`, `error.data_form="unavailable"`, and `derived.external.server_error_bytes=null`. The unchanged `error.class` values are independently consistent with the driver events: `cancellation`, `deadline`, `shutdown`, and `transport-unavailable`.

Causal-neighbor checks also reproduce the unchanged per-root external failure, failed terminal and terminal-ledger value, unavailable response frame with zero length and zero digest, unavailable result, empty response sequence, and available-known-zero transport response bytes/messages. Therefore the exact per-root changing set is `7 A + 1 B = 8`, and the exact schema complement is `145 - 8 = 137` unchanged occurrences.

### Worlds 17–22

Each accepted driver contains a normal request and one successfully emitted `response-frame` whose JSON-RPC result is `[]`. Opaque commitment metadata remains custody/equality input only and supplies no refusal or terminal policy.

Each root changes exactly:

```text
refusal.reason
refusal.refused
terminal.value
round_trip_ledger.terminal.after.value
```

The corrected values are `refusal.reason=null`, `refusal.refused=false`, and `successful-empty` for both terminal values. Mandatory causal neighbors remain consistent: result bytes are available `[]` (`5b5d`), the server-error observation is available with `present=false` and class `none`, external response custody remains available, and all ledger dimensions remain unchanged. No commitment field selects any outcome.

## Exact complements, commitments, and World 24

Across all 85 occurrences:

- `driver.commitment_inputs` is decoded-equal to HEAD;
- all other driver values are decoded-equal to HEAD;
- all 12 projected `expected.commitments` paths equal their corresponding driver commitment metadata, with predecessor order and cardinality preserved;
- roots World 08–11 change only their exact eight-path sets;
- roots World 17–22 change only their exact four-path sets; and
- every other expected occurrence is decoded-equal to HEAD.

World 24 is byte-identical to HEAD. Its root retains the exact 35 occurrence-sensitive cross-worker B ledger values, independently recomputed from all 16 worker ledgers:

| Dimension | Before | Delta | After | Available | Known zero |
|---|---:|---:|---:|---|---|
| callbacks | 0 | 0 | 0 | true | true |
| diagnostics | 0 | 0 | 0 | true | true |
| external_bytes | 0 | 608 | 608 | true | false |
| leases | 0 | 0 | 0 | true | true |
| reservations | 0 | 0 | 0 | true | true |
| transport_bytes | 0 | 960 | 960 | true | false |
| transport_messages | 0 | 16 | 16 | true | false |

The occurrence-sensitive ownership accounting remains:

```text
ordinary occurrence  A/B/C = 116/20/9 = 145
World-24 root         A/B/C = 81/55/9  = 145
```

The corpus-local type-derived comparator/partition test passes and independently enforces the exact 145 retained schema paths, ordinary `116/20/9` partition, and exact 17 absent/deferred paths. All 17 remain absent in all 85 occurrences.

## Canonical bytes and hashes

Strict duplicate-key decoding and recursively key-sorted compact UTF-8 serialization with exactly one terminal LF reproduced the manifest and all 25 world files byte-for-byte. The manifest lists exactly the secure lexical sequence `worlds/world-01.json` through `worlds/world-25.json`; every world length and SHA-256 matches its entry. Worlds outside 08–11 and 17–22 are byte-identical to HEAD, and their manifest entries are unchanged.

Changed worlds independently reproduce as:

| World | Byte length | SHA-256 |
|---|---:|---|
| 08 | 6306 | `a1f7cbc42928e5cec57bc3c08a927e03e324ba191aeef3e456196943b4a12018` |
| 09 | 5407 | `28a293676d221c78114abc158026836ccfa9df7c4abf5a0d83b00fbd2ba61169` |
| 10 | 5799 | `126889bc87acda19e466e58757e018ea4600933dae72ee471b5ed854374d4184` |
| 11 | 6292 | `c6b8314127363e6633ac117326cc7fe0650ebb9e0ae25b08f76ce9463532a425` |
| 17 | 6028 | `3c4fbb3d5222e56162cda8fa3be1b832584e1f92a8ec8211e0bc0966be3fb58a` |
| 18 | 5406 | `fb331259193dad56fd8ccace2d2b9d652eb44b167bee49aedd1861f42705c2f8` |
| 19 | 5653 | `62d47c2b3003a286e7db3a397f650dfa8486d1ed2d399ad2337dbfed9fb2dd91` |
| 20 | 6224 | `4a83bdc07af5d93d0937905820c4af108769b25926880e4ab10e878a83fb23f8` |
| 21 | 5392 | `17a0525a0418608d71ec4f9ffbaa1bb8cae7bb2a1168e78da034fcd1ee9add4b` |
| 22 | 5835 | `403d07c66dcc900fadfe945328d930d5e081d4add483ad7f768ac45efb2a01f2` |

Ordered concatenation of all 25 exact retained world files independently reproduces:

```text
aggregate byte_length 580502
aggregate sha256      ab627bf78437307ccaa740ff4e076bd7814f2498bae872c252d444de594271e4
manifest byte_length  4733
manifest sha256       42653cb0fd1f32adad0b3d31a1bb034db2e775132d4453dc6ee1e4ef73e11abd
```

The manifest schema and complete source object, including registry and v2 pins, are unchanged.

## Immutable predecessor boundary

No v1 or v2 path is modified. Their exact current trees remain:

```text
v1  7d278859a92b2f4438cce3c1f80f1591cd9a8c1a
v2  f728e3afa7713b8df8717a2e99c982fd95036741
```

## Disposition

No blocker was found. The exact finite write set, driver-only corrections, causal-neighbor consistency, exact affected sets and complements, commitments, unchanged World 24, `85/145/17` accounting, ordinary `116/20/9` and World-24 `81/55/9` ownership, canonical world/manifest bytes, per-world and aggregate hashes, unchanged source pins, and immutable v1/v2 boundaries pass.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_SEMANTIC_RESIDUE_CORPUS_BYTES_GO
```
