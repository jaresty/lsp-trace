# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Raw-Response Custody Amendment Review V2

Status: **GO**

## Review boundary

This is a superseding independent strict review of `/Users/schwa/dev/lsp-trace-adr0011-c17` at exact HEAD `f18b79dc82b876a6a2ab1d3f334b4e778c0aa203` plus the untracked raw-response custody amendment. It supersedes the prior blocked review after verifying that the amendment corrects that review's sole blocking omission.

The review used accepted driver events and the retained causal formulas as derivation inputs. Existing affected `expected` values, prototype output, implementation output, and current RED output were not used as corrected oracle operands.

No corpus, RED, test, source, generated artifact, v1 artifact, or v2 artifact was changed by this review.

## Prior blocker disposition

The prior review required four corrections concerning `accounting.bytes.value`. The amendment now satisfies all four:

1. `accounting.bytes.value` is present in the exact affected A set.
2. It supplies exact corrected values for all five affected roots.
3. The unchanged-family statement is narrowed to the other 11 `accounting.*` paths.
4. Canonical rematerialization explicitly requires `accounting.bytes.value` to be recomputed from the existing aggregate formula with corrected response-byte operand `L`, not copied from an existing expected record.

The prior blocker is closed.

## Independent occurrence and raw-byte verification

Recursive traversal follows every root, World-24 worker, World-25 execution, and both members of every World-25 pair. The exact denominator is:

```text
roots                         25
World-24 workers              16
World-25 executions           22
World-25 pair members         22
                              --
total occurrences             85
```

Exact event-script comparison identifies only roots World 07 through World 11. World 24 has 17 recursively visited occurrences including its root and no copy; World 25 has 45 recursively visited occurrences including its root and no copy.

For World 07, strict lowercase-hex decoding of the sole successfully emitted `response-frame` independently gives:

```text
R hex = 436f6e74656e742d4c656e6774683a206e6f70650d0a0d0a7b7d
N     = 1
L     = 26
H     = 138a9db5f8d8eb54884d5df8d0ffba294626a607c70b7bae9191edae5226739d
```

The malformed emitted bytes remain available raw custody and are not replaced by serialized error bytes.

Worlds 08–11 contain only their respective `cancel`, `deadline`, `shutdown`, and `unavailable` control events. Each therefore has `N=0`, unavailable frame custody (`available=false`, `frame_hex=null`, length zero, zero digest), available-known-zero response byte/message accounting, and an available empty external response sequence represented by `""`. Their distinct error, failure, and terminal outcomes remain independently caused and unchanged.

## Aggregate byte-accounting verification

The retained owner/view invariant is:

```text
accounting.bytes.value = frames.request.frame_length + L
accounting.bytes.available = true
accounting.bytes.known_zero = false
```

The amendment's exact values independently recompute as:

| Occurrence | Request frame length | L | `accounting.bytes.value` |
|---|---:|---:|---:|
| World 07 root | 164 | 26 | 190 |
| World 08 root | 170 | 0 | 170 |
| World 09 root | 162 | 0 | 162 |
| World 10 root | 162 | 0 | 162 |
| World 11 root | 188 | 0 | 188 |

All five amendment values are exact. Availability remains true and known-zero remains false because every request frame is nonempty.

## Exact affected-set and complement verification

The authoritative expected-schema traversal retains exactly 145 paths. The amendment names exactly 27 affected A paths and exactly 3 affected B paths. Every named affected path exists in the retained schema, the sets are disjoint, and `accounting.bytes.value` remains category A.

Therefore the exact retained complement is:

```text
145 retained paths - 27 affected A paths - 3 affected B paths = 115 unchanged paths
```

The amendment accounts for that complete complement by declaring every retained expectation path outside the exact affected A/B sets unchanged and then enumerating the unchanged families. In particular, the accounting family has exactly 12 retained paths: the affected `accounting.bytes.value` plus exactly 11 unchanged paths. No affected path is also claimed unchanged.

The B set remains exactly `derived.external.responses_bytes`, `derived.external.messages`, and `derived.external.bytes`, each required to come from direct actual execution. Availability and ledger `before` members retained in the A set remain explicit rematerialized causal operands even where their scalar values do not change.

## Remaining qualification criteria

The amendment also correctly preserves and requires:

- exact `145/17` retained/removed accounting;
- exact ownership `A=116`, `B=20`, `C=9`, with no category migration;
- all 17 removed/deferred paths absent;
- all driver decoded values, commitment inputs and expectations, array order, and non-affected expected values unchanged;
- causal separation of `result.*`, `error.*`, `external.failure`, `terminal.*`, ledger terminal records, and the named unchanged derived fields;
- direct-actual-only RED acquisition with no expected-to-actual copying, synthesized terminal frame, error serialization as response custody, malformed-frame normalization, or control-event response counting;
- strict canonical rematerialization, complete-world byte/hash recomputation, manifest entry and aggregate recomputation, and independent reproduction;
- mutation eligibility limited to Worlds 07–11 and their causally changed manifest entries;
- immutable v1 and v2 boundaries; and
- no corpus mutation, RED mutation, implementation, C15 assessment, activation, commit, or production authority from this review.

No remaining blocker was found.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_RAW_RESPONSE_CUSTODY_GO
```
