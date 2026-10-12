# ADR-0011 Call Hierarchy Layer-1 RED Oracle Corpus v1 — Independent Mechanical Rereview v5a

Verdict: **DESIGN_GO**

Scope: corrected corpus design bytes only at repository HEAD `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9`. This review authorizes no runtime behavior, fixture update, loader, testdata, RED edit, source edit, implementation, generated output, commit, or production claim.

## Independent method

I did not inspect, invoke, or trust the author validator or its output. I wrote a fresh disposable parser outside the repository. It parsed the four normative Markdown tables directly with duplicate-key rejection, applied only the explicit constructor mappings, validated exact field sets and reference closure, canonicalized sorted-key compact UTF-8 JSON with one trailing LF, recomputed every length and SHA-256 pin, and independently checked the requested semantic contrasts. It imported no repository package and consumed no runtime output.

## Frozen source pins

All pins match:

- HEAD: `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9`.
- `sessionruntime/roundtrip_test.go`: 15,859 bytes; SHA-256 `036c7476829588a8966f339ef548fa69c791d45da8539650f5bb019211a9e1d3`.
- Fixture excerpt `[2044,4304)`: 2,260 bytes; SHA-256 `8df633f6ca573050e11b4a2c1cf166177653bd715c802f6298c51d1d25b0e54b`.
- HEAD RED file: 21,523 bytes; SHA-256 `74561056ea73dc0b00a8378d6f8ec47a897213afd4271662c76a1b1106f116c4`.
- Reviewed RED working-tree file: 50,425 bytes; SHA-256 `b73f0178636bf4bd64efe6926549be8cd562b174fb9139fe005a4478abd909a9`; Git blob `097a34abac5cc44da55662cd1d72e5d5a01937dc`.
- Layer-1 excerpt `[0,19437)`: 19,437 bytes; SHA-256 `6ac0b1773a9cea77d5abdd06ad02707c5a46fe3ff69c7ddc748ae9412f832b00`.
- Exact binary diff: 30,279 bytes; SHA-256 `37d0e7d623481bcb9a4d8ce32b6cab6025a92b2de0b2314f72496ff5821dc8d2`; 454 insertions, 0 deletions.

## Mechanical closure

| Check | Observed |
|---|---:|
| normative tables | 4 |
| total constructors | 4 |
| world roots | 25 |
| World 24 workers | 16 |
| World 25 executions | 22 |
| undefined references | 0 |
| cycles | 0 |
| partial constructors | 0 |
| missing fields | 0 |
| unexpected fields | 0 |
| canonical length/hash defects | 0 |
| embedded byte/frame pin defects | 0 |

The constructors are total over their declared shapes. `C_RECORD` is neutral assembly of supplied fields and does not select result, error, terminal, lifecycle, ledger, refusal, or control semantics from enabled state, name, kind, or scenario label. `C_WORLD` adds only the supplied provenance and root identity around the supplied complete record; it does not select behavior.

World 24 resolves exactly 16 unique complete worker IDs. Independent worker-ledger sums are callbacks 0, diagnostics 0, external bytes 608, leases 0, reservations 0, transport bytes 960, and transport messages 16. World 25 resolves exactly 22 unique complete execution IDs in 11 enabled/disabled pairs; each pair preserves equal external and transport records while internal observation availability differs as declared.

## Canonical world hashes

| Root | SHA-256 |
|---|---|
| `world-01` | `6e709f397b1df965caadc2cf6571a50de0e8c2f063ec8b36cae7b925a8a36b76` |
| `world-02` | `a1d85f21d7accc1624667c85eafeeffa5ec19d8bb18e0ea75d1c6e20848e9cdd` |
| `world-03` | `0f6dc06dffd360f0fa26be3948c93e8a1ca9d7e65785931b169e3502b1afdc7e` |
| `world-04` | `4d9158efa26d8c94ac7a158c827b1ffe25c42f9a18a6f3d039c0d3d760f4cbdc` |
| `world-05` | `8c4dfa725b98b97c05d5b6b2b5b96366a1f999ce4ae91491d84a7a0c2a2298a2` |
| `world-06` | `d34e08df0fe3495a8c7033fa7ed2063f33cf794357eeb4b6f380b80a2095d569` |
| `world-07` | `28d4a9fe8b4f52072de4e95dd49524a183cb1c55e9902f5d5456d7cf51dd8f6d` |
| `world-08` | `c775bb6bc875664c1d0974a8d27ad1728d0e6f39e3984ee7992c3baeeb44621c` |
| `world-09` | `370706f64b22e0ad207cb6ba4949807641866de6d65f377e7f9c0ee4085e30ad` |
| `world-10` | `d029136b9f1e75b5d6f647bedf0b1313d34050bd3ca8d4fa6e84aee13f28ef84` |
| `world-11` | `85e173f31d395ff79af1ebd4aaec824d9522e4be58eeed663a703a5242a8cb09` |
| `world-12` | `73c45f30f81b9720e8749215ffcb1a6c0b96db024e79e7577e752c5eb5975d6d` |
| `world-13` | `2436607380d6f983665bb3100dbc73654d0af90e71f16246adf043566c38ec41` |
| `world-14` | `1277a72b0cca2ae5f74d23b957b75e67c5235209389b76361cd70f6e463875fe` |
| `world-15` | `c541dc6791190a535e77a571323235388e973178f0e24edf26314524fa590aeb` |
| `world-16` | `1d09e2ff8704650b849dda9911a2c624d0761d8ea355e96eb5a26a900268a1f2` |
| `world-17` | `181b25eab4a28deffa3ac75d164b2e75b72e7841232a756751d74c7b005ffe54` |
| `world-18` | `31e6e2651124b7c1ffac792ef7ba9fc9b32fb786b7d8587ff6fb954e6430e642` |
| `world-19` | `a009e9ddfab01f63511186ac179441fc561e6c9ce95bc2e197ee52811fdc7bef` |
| `world-20` | `01ae59af0adb1e6f49d12fed92c80914f239a5d0e78f6d0584a5ddabd2e5a868` |
| `world-21` | `fce273bf3e8bfa0bc7521e71f214a3dd3d5bf9360b1b099d3d03b8b9727a1040` |
| `world-22` | `aeacf026559a2f8654b4806c9f3ff8a1c43a017c7feae054e6447164249ae970` |
| `world-23` | `a92e6f9a7c18da1195c122d4871001b0b7f3911bdff81ec39395499d62b1e912` |
| `world-24` | `b524b1d08295f06f089248f5cf400c97aabf28a4187cdca7d5b1aec08b7c125b` |
| `world-25` | `d954ef0e54a68fe95b3ca2a05fa5111507162b3246617a5077f69892f67e1d86` |

Aggregate digest: SHA-256 `d70d3107a7eac21ef57b3488ebec91c09798ca6005582ef0e03830abcbfbfba2` over root-ID-sorted UTF-8 lines `ROOT_ID<TAB>SHA256<LF>`.

## Required semantic assertions

- World 3 carries exact JSON `null` bytes (`6e756c6c`), `form:"null"`, and a response body containing top-level `result:null`.
- Worlds 4 and 5 carry nonempty, non-null, non-`[]` result bytes and are distinct from each other.
- Worlds 6–11 have six pairwise-distinct tuples over error, terminal, result, transport, external, lifecycle, and round-trip ledger. Each has an available present error, unavailable result, and a non-success terminal coherent with its named protocol, malformed, cancellation, deadline, shutdown, or transport-unavailable state.
- Worlds 12–14 respectively encode unavailable nil payload, available zero-length payload with the empty SHA-256, and available nonempty payload with matching byte length and SHA-256. All three payload forms are distinct.
- World 15 sets `no_alias:true`; source pre/post value+bytes+digest differ, retained pre/post value+bytes+digest are identical, and every digest matches its exact bytes.
- Worlds 17–23 have seven distinct nonempty refusal reasons, `refused:true`, terminal `refused`, and coherent transparent error form: available, not present, class `none`, null code/message/data, and canonical serialized `null`. Their exact external `[]` result remains available, consistent with the stated externally transparent publication-refusal policy.
- Selected, handoff, and predecessor commitments remain opaque availability/identity/length/SHA-256 records; no CALLS direction or payload interpretation is introduced.

No design defect remains within this bounded corpus-byte review.
