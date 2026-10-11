# ADR-0011 Call Hierarchy Layer-1 RED Oracle Corpus v1 — Independent Semantic/Adversarial Review v4b

**Verdict: `ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_DESIGN_BLOCKED`**

Reviewed exact repository HEAD `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9`, the corrected untracked design `docs/qualification/adr0011-callhierarchy-layer1-red-oracle-corpus-v1.md`, and the prior blocked reviews. The worktree already contained the unchanged 454-insertion RED modification and prior review/design files. This review creates only this v4b document.

This verdict authorizes no corpus bytes, fixture update, loader, RED edit, source edit, testdata, implementation, generated/runtime output, or commit.

## Independent method and evidence boundary

I did not invoke or inspect the author validator, its source, or its recorded output as review authority. I used a fresh disposable in-memory parser that read only the design's four normative Markdown tables, decoded literal JSON, recursively replaced exact Table 3 record IDs, canonicalized with sorted keys and compact separators plus the required final LF, recomputed hashes, and inspected the expanded values. It imported no repository package, called no implementation, wrote no generated artifact, and consumed no runtime output.

Expected-value review was limited to the accepted protocol expressed by the design, its fixed fake identities/frames, frozen accounting relations, and explicit refusal policy. Actual implementation behavior was not used to select or repair any expected value.

## Mechanical closure results

The corrected registry is mechanically closed:

- atomic constants: 211; byte-length/hash defects: 0;
- complete-record IDs: 51; duplicate IDs: 0;
- world roots: 25;
- World 24 workers: 16;
- World 25 executions: 22, arranged as 11 enabled/disabled pairs;
- undefined/cyclic reference defects observed by the independent expansion: 0;
- expanded world hash mismatches: 0.

Worlds 24 and 25 require recursive Table 3 substitution before hashing. After independent substitution their hashes exactly match the design:

- `WORLD_24_MANAGER_PRODUCED_CONCURRENT_LIFECYCLE`: `bd05550dc2d427f04de6b981a63abbf9240a0490f5d5d54e6b1b8f97ef5225e6`;
- `WORLD_25_ENABLED_DISABLED_CONTROLS`: `e1e450d8eadf0e30df4b745896e27fc0bb82ca8f1641a015df5d543fcf40e608`.

The ordered digest of lines `<root-id> <expanded-world-sha256>\n` for all 25 Table 4 roots is:

`b66bcee5fdb7b32d2879f2303664210539553fd839ce473419056e3ce070f697`

World 24 arithmetic is exact over the 16 expanded worker records: callbacks 0, diagnostics 0, external bytes 608, leases 0, reservations 0, transport bytes 960, and transport messages 16. The declared aggregate has exactly those deltas.

World 25's 11 pairs resolve to 22 records in order. Every enabled record has `enabled=true`, `attachment_available=true`, available result/terminal/external/ledger state, and complete lifecycle. Every disabled record has `enabled=false`, `attachment_available=false`, unavailable result/terminal/external/ledger state, zero transport, and empty lifecycle. Pair-world identities agree. Structural enabled/disabled contrast defects: 0.

These closure results do not establish semantic correctness. Canonical bytes can faithfully preserve a wrong oracle.

## Exact blocking defects

### S1 — Worlds 3–5 erase the result distinctions named by the corpus

`WORLD_03_OUTGOING_SUCCESS_TOP_LEVEL_NULL`, `WORLD_04_INCOMING_SUCCESS_NONEMPTY`, and `WORLD_05_OUTGOING_SUCCESS_NONEMPTY` all contain the same result value:

```json
{"available":true,"bytes_hex":"5b5d","form":"nonempty"}
```

`5b5d` is `[]`. World 3 therefore does not encode top-level null, and Worlds 4–5 do not encode a nonempty result. Their response frames likewise contain `"result":[]`. The expected bytes are selected by a shared canned success record rather than derived from the named fixed fake input distinction.

### S2 — Worlds 6–11 erase every named error and terminal distinction

The roots named protocol error, malformed error response, cancellation, deadline, shutdown, and transport unavailable all contain:

- `error.available=true`, `error.present=false`, `error.class="none"`, no code/message/data;
- `result.bytes_hex="5b5d"`;
- terminal `successful-empty`;
- lifecycle `acquired, completed, released`;
- available external observations with empty arrays;
- positive request/response transport and a success ledger.

Thus none of Worlds 6–11 represents its named condition. In particular, transport-unavailable still has available request and response frames and positive transport measures. Error, terminal, ledger, transport, external, and lifecycle semantics are mutually coherent only as one ordinary success case, not as the six cases the oracle claims to distinguish.

### S3 — Worlds 12–15 erase payload and mutation distinctions

The roots named payload-nil, payload-empty, payload-nonempty, and mutation-defensive-copies-before-and-after all carry the same empty-array result, error-none state, successful-empty terminal, complete lifecycle, and ordinary success ledger. No field distinguishes nil from empty from nonempty payload, and no before/after value demonstrates defensive-copy isolation. These expected values are not derived from distinct fixed fake inputs; the root names are labels over identical semantic content.

### S4 — `C_RECORD` smuggles canned semantics through its constructor

The total constructor explicitly maps every enabled worker or execution to:

- `A_COMMITMENTS_OK`;
- `A_ERROR_NONE`;
- `A_EXTERNAL_EMPTY`;
- `A_LIFECYCLE_COMPLETE`;
- `A_RESULT_EMPTY`;
- `A_TERMINAL_SUCCESS`;
- one fixed success ledger shape.

Its typed domain contains no error, result, terminal, refusal, lifecycle, or ledger-semantic argument capable of deriving the distinct cases. The constructor therefore embeds the answer instead of remaining a neutral total expansion mechanism. This is the prohibited constructor-smuggling failure and explains the collapses in S1–S3.

### S5 — Refusal worlds contain an internally contradictory error form

Worlds 17–23 correctly distinguish refusal reasons, unavailable results, and terminal `refused`; selected, handoff, and predecessor commitments remain opaque-only. However, each refusal world's error object simultaneously states:

- `available=true`;
- `present=false`;
- `class="refusal"`;
- `message="layer-1 publication refused"`;
- a nonempty serialized error value.

A value that says no error is present while carrying a refusal class, message, and serialization is not a coherent tagged error form. The constructor table's stated exact-atom restriction does not cure the contradiction because the selected atom itself is semantically invalid.

## Opacity and authority findings

Selected, handoff, and predecessor commitments expose only availability, opaque identity, length, and SHA-256. World 16's comparison remains confined to opaque commitments plus separate transaction custody; it does not decode commitment payloads or add CALLS direction. No opacity defect was found.

The design states that its author self-check is non-authoritative and that no GO is issued. This review likewise grants no fixture-update or bless authority. It does not establish runtime behavior, production behavior, source completeness, cross-capture CALLS, producer authenticity, or production authority.

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_DESIGN_GO` is **not issued**.

The registry now passes byte closure, all 25 expanded hashes, exact root/worker/execution cardinalities, World 24 aggregate arithmetic, and World 25 structural control contrasts. It remains semantically unfit as an oracle because named result/error/terminal/transport/payload/mutation cases collapse to canned success values, the enabled constructor smuggles those values, and refusal error objects contradict their own `present` tag.

A corrected design must derive each distinct world's result, error, terminal, ledger, transport, external, lifecycle, and refusal values from accepted protocol plus fixed fake inputs; make constructors neutral or explicitly parameterized by those accepted inputs; and make refusal error presence internally coherent. Another independent review is required. Any future GO may authorize corpus bytes only at the two already named test-only path forms and may not authorize fixture updates, implementation, runtime-derived expectations, production behavior, or commit.

## Model interpretation

The decisive distinction is between canonical closure and oracle validity: exact hashes prove stable bytes, while the expanded semantic values determine whether those bytes describe the required worlds.

Key assumptions:

- World names and the corpus's explicit case taxonomy are claims about the expected semantic distinctions, not decorative labels.
- A constructor is non-smuggling only when its output distinctions are derived from admitted typed inputs rather than selected unconditionally by `enabled`.
- `error.present=false` means the record cannot simultaneously carry a substantive refusal class, message, and serialized error as though an error were present.

Gaps and verification items:

1. Replace the collapsed root records and rerun independent expansion/hash verification for all 25 worlds.
2. Demonstrate one accepted-input derivation for each distinct success, error, terminal, refusal, payload, mutation, and transport state.
3. Recheck World 24 and World 25 after semantic correction because their current arithmetic and control structure are exact but inherit canned enabled-record semantics.

Improved framing: qualification requires both a closed byte-producing registry and an independently derivable semantic contrast for every named oracle world.