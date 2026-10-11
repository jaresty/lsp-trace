# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Raw-Response Custody Amendment Review

Status: **BLOCKED**

## Review boundary

This is an independent strict review of `/Users/schwa/dev/lsp-trace-adr0011-c17` at exact HEAD `f18b79dc82b876a6a2ab1d3f334b4e778c0aa203` plus the untracked raw-response custody amendment. The review used accepted driver events as the only response-frame derivation input. Existing `expected` values, prototype output, implementation output, and current RED output were not used as corrected oracle operands.

No corpus, RED, test, source, generated artifact, v1 artifact, or v2 artifact was changed by this review.

## Independent recursive occurrence census

Recursive traversal followed the corpus structure used by `callHierarchyLayer1GenericV3Walk`: each root, every World-24 worker, every World-25 execution, and both members of every World-25 pair.

The independently recomputed finite denominator is exact:

```text
roots                         25
World-24 workers              16
World-25 executions           22
World-25 pair members         22
                              --
total occurrences             85
```

Exactly five occurrence event scripts match the proposed correction:

| Occurrence | Accepted driver events | N | L |
|---|---|---:|---:|
| World 07 root | one `response-frame` containing exact malformed bytes | 1 | 26 |
| World 08 root | one `cancel` control event | 0 | 0 |
| World 09 root | one `deadline` control event at offset `30000000` | 0 | 0 |
| World 10 root | one `shutdown` control event | 0 | 0 |
| World 11 root | one `unavailable` control event | 0 | 0 |

Exact event-script comparison over all 85 occurrences found those five roots and no copies. World 24 has 17 recursively visited occurrences including its root and no matching event script. World 25 has 45 recursively visited occurrences including its root and no matching event script. Thus the proposed five-occurrence census and absence of World-24/World-25 copies are correct.

## Raw malformed and no-frame semantics

For World 07, strict hexadecimal decoding of the sole successfully emitted response-frame event gives:

```text
R hex = 436f6e74656e742d4c656e6774683a206e6f70650d0a0d0a7b7d
N     = 1
L     = 26
H     = 138a9db5f8d8eb54884d5df8d0ffba294626a607c70b7bae9191edae5226739d
```

The bytes are malformed as an LSP frame but were emitted, so raw custody remains available. They must not be replaced by the existing serialized generic malformed error.

For Worlds 08–11, the accepted scripts contain no `response-frame` event. Therefore frame custody is unavailable (`available=false`, `frame_hex=null`, length zero, zero digest), while the attempted external execution makes response byte/message accounting available-known-zero. The available empty external response sequence is `""`, not `null`. The distinct control events continue to determine their existing error, failure, and terminal outcomes and do not synthesize response frames.

These semantics and the amendment's direct-actual-only anti-circularity rules are correct.

## Blocking affected-path omission

The amendment's exact affected A/B path set is not complete. It declares all `accounting.*` paths unchanged and omits:

```text
accounting.bytes.value
```

That omission contradicts the retained owner/view invariant in `sessionruntime/b4_owned_method_pair_lease_test.go`:

```go
view.accounting.bytes == callHierarchyMeasure{
    available: true,
    value: view.request.frameLength + view.response.frameLength,
}
```

The current five root fixtures confirm that the old `accounting.bytes.value` is request-frame length plus the old synthesized response-frame length. Once the amendment replaces response-frame length with driver-event-derived `L`, the exact causal formula requires:

```text
accounting.bytes.available  = true
accounting.bytes.known_zero = false
accounting.bytes.value      = frames.request.frame_length + L
```

Consequently the corrected values are:

| Occurrence | Request frame length | L | Required `accounting.bytes.value` |
|---|---:|---:|---:|
| World 07 root | 164 | 26 | 190 |
| World 08 root | 170 | 0 | 170 |
| World 09 root | 162 | 0 | 162 |
| World 10 root | 162 | 0 | 162 |
| World 11 root | 188 | 0 | 188 |

Only `accounting.bytes.value` changes in that family; its availability remains true and its known-zero value remains false because every request frame is nonempty. Nevertheless, the amendment explicitly says every retained path outside its listed A/B sets is unchanged. That finite-set claim is false, and canonical rematerialization under the proposed whitelist would preserve five incorrect owner/view expectations.

## Other reviewed boundaries

Subject to the blocking omission above:

- the response-frame, transport response byte/message, external response byte, and named ledger formulas are internally consistent with `(R,N,L,H)`;
- `result.*`, `error.*`, `external.failure`, `terminal.*`, and ledger terminal records remain causally separate and unchanged for these five occurrences;
- commitments are unaffected by response-frame derivation, and all 12 commitment paths retain their explicit driver-input custody;
- the retained schema denominator remains 145 paths;
- all 17 removed/deferred paths remain absent;
- the ownership partition remains `A=116`, `B=20`, `C=9`; adding `accounting.bytes.value` to the affected set changes no category because it is already A;
- no valid basis was found to alter v1 or v2; both remain immutable;
- the canonical rematerialization procedure is otherwise appropriately driver-only and rejects expected/prototype/runtime circularity.

## Required correction before re-review

Amend the finite affected A set to include `accounting.bytes.value`, provide the five exact values above, remove `all accounting.*` from the unchanged-family statement or narrow it to the eleven actually unchanged accounting paths, and require canonical rematerialization to replace that path in Worlds 07–11. Then re-run the exact affected/unchanged complement proof over all 145 retained paths.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_RAW_RESPONSE_CUSTODY_AMENDMENT_REVIEW_BLOCKED
```
