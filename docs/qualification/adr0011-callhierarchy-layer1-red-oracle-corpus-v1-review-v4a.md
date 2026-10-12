# Independent Review A — ADR-0011 Call Hierarchy Layer-1 RED Oracle Corpus v1

Verdict: **`ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_DESIGN_GO`**

Scope: corpus bytes only. This verdict authorizes no loader, runtime behavior, testdata generation, implementation, or commit.

## Reviewed baseline and pins

- Exact repository HEAD: `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9`.
- Fixture: 15,859 bytes, SHA-256 `036c7476829588a8966f339ef548fa69c791d45da8539650f5bb019211a9e1d3`.
- Fixture excerpt `[2044,4304)`: 2,260 bytes, SHA-256 `8df633f6ca573050e11b4a2c1cf166177653bd715c802f6298c51d1d25b0e54b`.
- Reviewed RED working-tree file: 50,425 bytes, SHA-256 `b73f0178636bf4bd64efe6926549be8cd562b174fb9139fe005a4478abd909a9`, Git blob `097a34abac5cc44da55662cd1d72e5d5a01937dc`.
- Exact binary diff: 454 insertions, 0 deletions; SHA-256 `37d0e7d623481bcb9a4d8ce32b6cab6025a92b2de0b2314f72496ff5821dc8d2`.

## Independent mechanical validation

I implemented and ran a disposable validator independently of `/tmp/validate_adr0011_registry.py`. It parsed the four normative Markdown tables directly, rejected duplicate JSON keys, expanded exact Table 3 record references recursively with cycle detection, checked constructor declarations and record shapes, validated byte-bearing fields, canonicalized every expanded root as sorted-key compact UTF-8 JSON plus one LF final byte, and hashed every root twice from independently repeated expansion.

Observed registry cardinalities:

- Atomic constants: 211 unique IDs.
- Total constructors: 4 (`C_IDENTITY`, `C_LEDGER`, `C_RECORD`, `C_WORLD`).
- Complete records: 51.
- World roots: 25.

Required closure/accounting result:

| Check | Observed |
|---|---:|
| roots | 25 |
| workers | 16 |
| executions | 22 |
| undefined | 0 |
| cycles | 0 |
| partial | 0 |
| missing | 0 |
| unexpected | 0 |

Additional results:

- Atomic IDs are unique.
- Every atomic literal parses without duplicate keys; every declared byte length and SHA-256 pin matches its compact canonical value bytes.
- Byte-bearing body and frame fields match their embedded lengths and SHA-256 pins.
- All four constructors have explicit typed domains, complete output mappings, and invalid-tuple exclusions; all 25 root invocations name `C_WORLD` and provide the complete invocation envelope.
- All 16 worker and 22 execution records are complete for their declared shapes; all pair records share one complete shape; all 25 roots have the complete world field set.
- Selected, handoff, and predecessor values remain opaque availability/identity/length/SHA-256 commitments. Root provenance records `runtime_recorded:false`; no generator, manager-output, RoundTrip-output, bless, or runtime-output derivation directive appears in the normative values.
- Repeated independent expansion/canonicalization produced 25 of 25 identical root hashes.

## Expanded world hashes

| Root | SHA-256 |
|---|---|
| `WORLD_01_INCOMING_SUCCESS_EMPTY_ARRAY` | `bc94eecaa7d5086dcc7e223f5d035da83353abeaea537763cdc0442bc899ab3f` |
| `WORLD_02_OUTGOING_SUCCESS_EMPTY_ARRAY` | `306ded4b47254b24e193273bd635c260fca0feb88596dabacea1f5dfb096033b` |
| `WORLD_03_OUTGOING_SUCCESS_TOP_LEVEL_NULL` | `b4dbe3be76ffb7ee6f1bf035ac1a93fb830d086bf89cc8702885a90dc921e748` |
| `WORLD_04_INCOMING_SUCCESS_NONEMPTY` | `93d74f692aba4b18dae2c657f4471644d0aa7f8ecd9d0908cf16ba699755577d` |
| `WORLD_05_OUTGOING_SUCCESS_NONEMPTY` | `8a64582ed3839b3aa769a91e5d3c677240fc63269ecdeec94ffa8ad0449881b8` |
| `WORLD_06_PROTOCOL_ERROR` | `7ed07d4b91315d3c6b650146d8a72362a38bd05883f190e86657ff787393729a` |
| `WORLD_07_MALFORMED_ERROR_RESPONSE` | `76b47a0c71af56805b0bc5a2fb595a2c1633f32cfe19213db419c3e5341489c6` |
| `WORLD_08_CANCELLATION` | `84d43d4d6b8eac0fbdfe0e3fe6593ce1310d5361db87547548ee0c3c13c64731` |
| `WORLD_09_DEADLINE` | `b9ceb2ad94c0c1be8e77f100881f357f7776944e149b164fb2fa3cca27b8f744` |
| `WORLD_10_SHUTDOWN` | `8136dcb14a34a56bc4d4ea88a7913ccb6788cd5ffe723f89e8e6cf8d34dbfb06` |
| `WORLD_11_TRANSPORT_UNAVAILABLE` | `a714fafbf4078cab003ff64b009bc31997b5268bece28f70361f2bd3a425068b` |
| `WORLD_12_PAYLOAD_NIL` | `179f54b4330cd51e7c8d80a0b9c9e04caa8d930ba4290765e6f261f3f77f9be2` |
| `WORLD_13_PAYLOAD_EMPTY` | `68878ded0f39cc64dc8334f03f65e5e996a895cda0cd36b1024349c20acc139b` |
| `WORLD_14_PAYLOAD_NONEMPTY` | `251fa70ed87605243744d55fa38b76c0d07f405a27b512c7e201d11fafbc975a` |
| `WORLD_15_MUTATION_DEFENSIVE_COPIES_BEFORE_AND_AFTER` | `691e15c434c73c8f43ca944fe4cdca356f5b95ea479e78bafb609e9eababdfd6` |
| `WORLD_16_PREPARE_OUTGOING_SEPARATION` | `f4df8deeabfabe0540f259c1cc6d6952cbba6ed822ae429a91172ac340303e3a` |
| `WORLD_17_REFUSAL_SELECTED_UNAVAILABLE` | `cb73ad185348fcaec2cc681cc6136b58b189653b00f58e14f69c25be038908ac` |
| `WORLD_18_REFUSAL_HANDOFF_UNAVAILABLE` | `79e1183544a934b689fe414151708891be5b0bf5946db2a0897b1eebe7490f6e` |
| `WORLD_19_REFUSAL_SELECTED_ZERO_IDENTITY` | `822ed09e788f766165ed9c608996db1c9633634f52bd691123d1eac83d46a7fd` |
| `WORLD_20_REFUSAL_HANDOFF_ZERO_IDENTITY` | `c638bbde0bbc7036cc164817da6bd276d8451bbd8e71101888e6b0d80f0f2cc6` |
| `WORLD_21_REFUSAL_SELECTED_ZERO_DIGEST` | `06cea3afc662c35890b8f158277034b7cb629118f8db058b60e38aee46b29d5f` |
| `WORLD_22_REFUSAL_HANDOFF_ZERO_DIGEST` | `f60158010ae1d64913a5d58b4c7f2a1619a463ab5b1e5b8fa0b0abd2f730abc6` |
| `WORLD_23_REFUSAL_METHOD_UNAVAILABLE` | `a008ba250d2634fbe157bc163610b4afdb3ced42a1d59d47f914a681b49f3593` |
| `WORLD_24_MANAGER_PRODUCED_CONCURRENT_LIFECYCLE` | `bd05550dc2d427f04de6b981a63abbf9240a0490f5d5d54e6b1b8f97ef5225e6` |
| `WORLD_25_ENABLED_DISABLED_CONTROLS` | `e1e450d8eadf0e30df4b745896e27fc0bb82ca8f1641a015df5d543fcf40e608` |

Hash-list digest: SHA-256 `e02e8a30f555480bea3c311873a76e05c7eaf932159dc856c0ef28ec8787de9b`, over root-ID-sorted UTF-8 lines `ROOT_ID<TAB>SHA256<LF>`.

No exact defect remains within the corpus-byte design boundary.
