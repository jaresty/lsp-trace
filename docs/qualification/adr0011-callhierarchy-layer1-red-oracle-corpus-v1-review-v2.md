# ADR-0011 Call Hierarchy Layer-1 RED Oracle Corpus v1 — Independent Superseding Strict Rereview

**Verdict: `ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_DESIGN_BLOCKED`**

Reviewed exact repository HEAD `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9` and the corrected untracked design `docs/qualification/adr0011-callhierarchy-layer1-red-oracle-corpus-v1.md`. This rereview supersedes `docs/qualification/adr0011-callhierarchy-layer1-red-oracle-corpus-v1-review.md` as the current qualification disposition while preserving its BLOCKED boundary.

This review authorizes no corpus bytes, loader, RED edit, source edit, testdata, behavior change, production path, or commit.

## Mechanical verification performed

The immutable source pins match the reviewed worktree exactly:

- `sessionruntime/roundtrip_test.go`: 15,859 bytes; SHA-256 `036c7476829588a8966f339ef548fa69c791d45da8539650f5bb019211a9e1d3`.
- Fixture excerpt `[2044,4304)`: 2,260 bytes; SHA-256 `8df633f6ca573050e11b4a2c1cf166177653bd715c802f6298c51d1d25b0e54b`.
- HEAD `sessionruntime/b4_owned_method_pair_lease_test.go`: 21,523 bytes; SHA-256 `74561056ea73dc0b00a8378d6f8ec47a897213afd4271662c76a1b1106f116c4`.
- Reviewed working-tree RED: 50,425 bytes; SHA-256 `b73f0178636bf4bd64efe6926549be8cd562b174fb9139fe005a4478abd909a9`; Git blob `097a34abac5cc44da55662cd1d72e5d5a01937dc`.
- RED prefix `[0,19437)`: SHA-256 `6ac0b1773a9cea77d5abdd06ad02707c5a46fe3ff69c7ddc748ae9412f832b00`.
- Exact binary diff: 30,279 bytes; SHA-256 `37d0e7d623481bcb9a4d8ce32b6cab6025a92b2de0b2314f72496ff5821dc8d2`; exactly 454 insertions and 0 deletions.

Independent construction from only the design literals and `FRAME` formula also reproduced all frozen trial values:

- request 101: body 300 / `7ab4da43ee70d88e17633fc52cec2fb2a0162beecf59e7d0162f80b1707f8bac`; frame 323 / `e1b88e53914a7b5bfc5fdcdb1dc0bb09f7e44ebaffa6ce703c5d4c2b60931a57`;
- response 101: body 38 / `2eee4270cf08d89e4e2a26f34143792a4d49c55d77056efe6d4e85d598ce7d58`; frame 60 / `550c7f5b3ba909c36268c7b2a4366e9f9e34afe387900528c428853ef1d3904c`;
- cancellation 108 and 109: body 64, frame 86, with every listed digest matching;
- request 117: body 300 / `311650b99dc87f2ef8b0b6ed24662cb1c3494f75b40202a67af00c552bbef438`; frame 323 / `9f8cb52f164c46f527b66d3b5889c82555d754147f2f66dac4a9c206f368e101`;
- response 117: body 38 / `50d81c619f7b03f2fc1f44dee2d2a86d36a4a00662e4b4d347471c97d60a1777`; frame 60 / `2ecea7be977ecf543e6959245edaa8d4f242d4f0c96b6f3f0b9eb0e416af0bbf`;
- opaque inputs `selected`, `handoff`, and `predecessor-opaque-v1`: lengths and SHA-256 values all match;
- world-1 arithmetic: observation bytes 383, external body bytes 38, response transport bytes 60.

Thus the frozen primitive literals and the displayed arithmetic are internally reproducible. The blocker is complete-object specification, not those primitive calculations.

## Exact blocking defects

### B3/B6/B8/B11 — complete canonical world bytes remain under-specified

The design lists the 13 top-level world keys, but it never defines complete exact JSON schemas and canonical member sets for the nested `accounting`, `frames`, `control`, `external`, `provenance`, `refusal`, `terminal`, and `transport` objects. It likewise does not define the exact containing shape and keys for the round-trip ledger or for request, response, and auxiliary frame collections. `FRAMESET` says that frames contain seven named members, but the `body` member's exact JSON representation is not defined, nor is the exact arrangement/cardinality of frame records within `frames`.

Consequently §12 does not permit either representative world to be serialized as complete canonical JSON. It supplies values but not the complete object tree into which those values must be placed. Two independent authors can reproduce the listed primitive bytes yet still produce different valid-looking world JSON.

The value-level ledger in §13 cannot cure this omission because it names value families, not the missing complete schemas. The canonical manifest remains downstream of world bytes that are not uniquely derivable.

**Required correction:** provide an exhaustive strict schema or one fully expanded canonical JSON object for every distinct nested form, including exact keys, availability encodings, array placement/order, and ledger/terminal representation. Then provide fully expanded canonical world 1 and world 17 trial bytes, including their final LF, byte length, and SHA-256. Independent authors must reproduce those complete bytes without filling any structural gap.

### B4/B5/B6 — the 25-world worksheet still uses undefined and prohibited shorthand

Section 11 claims that every row assigns every field and forbids `same/current/control` shorthand, but the worksheet uses non-expanding prose that is not a named immutable template, including:

- world 6: `response-error analogue of LED_SUCCESS`;
- many rows: bare `§9` in place of complete accounting and transport selection;
- worlds 4, 5, and 15: `exact notification/response arrays` or `exact arrays` without an exact array-serialization template;
- world 16: an `outgoing comparison identity` without a defined field path or containing schema;
- world 25: `§6 forms corresponding to worlds 1–11`, `independently repeated`, and disabled semantic fields described collectively rather than as exact records.

The design therefore does not fix complete error/accounting/external values for every world record despite fixing several primitive rows. In particular, `response-error analogue of LED_SUCCESS` is not a formula and does not state the exact terminal transition and every ledger dimension. The 22 controls are ordered and their IDs are derivable, but their complete literal records are not. The 16 worker identities and elementary success vectors are derivable, but the containing worker and aggregate object schemas are not.

**Required correction:** replace every prose analogue, section pointer, ellipsis, and “corresponding” reference in the worksheet with a named total template whose exact JSON expansion is defined. Fully enumerate all 16 worker records, the aggregate object, and all 22 enabled/disabled records or define total indexed constructors that fix every nested field and byte string without semantic interpretation.

### B5/B8 — disabled-control semantic unavailability is not a complete validity row

World 25 says disabled semantic result, commitments, terminal, accounting, and ledger are unavailable while external/request transport is repeated. The validity matrix defines isolated unavailable forms for some leaves, but no total disabled-execution object or exact unavailable form for all composite accounting, ledger, commitments, and terminal fields. Authors must choose whether each composite remains present with unavailable children, becomes a tagged unavailable object, or uses another shape.

**Required correction:** define one exact complete disabled-execution schema and literal expansion, including every nested member, null, availability flag, measure, ledger endpoint, commitment array, and frame/external field.

### B1/B6 — world 16 comparison data has no authorable identity location

The worksheet says published commitments use `SELECTED_OK` and `HANDOFF_OK`, then introduces comparison identities `opaque-selected-outgoing` and `opaque-handoff-outgoing` with lengths and derived hashes. No top-level key or nested schema identifies where those comparison values are stored. Omitting them loses the stated separation evidence; placing them anywhere invents a schema member forbidden by strict unknown-key rejection.

**Required correction:** define the exact existing field path and containing schema for comparison metadata, or remove it from the world and define separation entirely through separately enumerated records whose schemas are already frozen. Do not broaden Layer-1 commitments.

## B1–B11 rereview disposition

| Blocker | Disposition |
|---|---|
| B1 | **Partial.** Request, transaction, worker, control, and opaque primitive identities are deterministic, but world-16 comparison identity has no exact authorable location. |
| B2 | **Closed.** All current HEAD, fixture, RED content/excerpt, diff byte, digest, blob, and 454/0 pins mechanically match. |
| B3 | **Partial.** JSON-RPC body/frame primitives are exact and verified; the complete nested frame/world JSON representation is not. |
| B4 | **Partial.** Error rows are materially improved, but world selection still includes an undefined response-error analogue and incomplete external-array expansions. |
| B5 | **Blocked.** Elementary request-scoped vectors are deterministic; complete world-6, disabled-control, aggregate-container, and nested ledger representations are not frozen. |
| B6 | **Blocked.** Exactly 25 names are present, but several rows do not assign complete values and retain prohibited shorthand. |
| B7 | **Closed.** Request and transaction identity schemas and recursive equality are exact. |
| B8 | **Blocked.** Leaf validity rows are useful, but total nested/composite and disabled-execution validity is incomplete. |
| B9 | **Closed.** Published selected and handoff commitments are limited to availability plus opaque identity/digest/length. |
| B10 | **Closed.** Predecessor is limited to the same opaque schema and transaction identity remains separate. |
| B11 | **Blocked.** The dependency order is noncircular in principle, but missing total schemas/templates prevent the ledger from deriving canonical world and manifest bytes. |

## Scope, authority, and direction

The design correctly forbids Manager, RoundTrip, lease, attachment, generated/update/bless, current-runtime, and current-output authoring inputs. Nothing reviewed establishes current runtime behavior or production attachment output.

The only acceptable future corpus amendment remains exactly these test-only paths:

```text
sessionruntime/testdata/callhierarchy-layer1-red-v1/manifest.json
sessionruntime/testdata/callhierarchy-layer1-red-v1/worlds/<world-name>.json
```

No semantic direction, cross-capture CALLS, production reachability, source completeness, runtime authenticity, or production authority is granted. Selected, handoff, and predecessor commitments remain opaque identity plus digest plus length plus availability only.

## Final disposition

`ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_DESIGN_GO` is **not issued**.

The corrected design successfully freezes and verifies many primitive inputs, but two independent authors still cannot produce byte-identical complete representative worlds, all 25 world files, or the canonical manifest without inventing nested JSON structure and resolving undefined worksheet shorthand. The exact defects above keep the prior BLOCKED review in force.

A future GO, if earned after correction and another independent strict rereview, may authorize independently authored corpus bytes only. It must not authorize implementation, runtime-derived expectations, production behavior, or any file outside the two exact test-only path forms above.
