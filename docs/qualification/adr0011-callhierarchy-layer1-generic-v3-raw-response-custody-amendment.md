# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Raw-Response Custody Amendment

Status: **PENDING**

## Authority and immutable boundary

This additive qualification amendment is authored against exact repository `/Users/schwa/dev/lsp-trace-adr0011-c17` at starting HEAD `f18b79dc82b876a6a2ab1d3f334b4e778c0aa203`.

It corrects only the finite v3 expectation-derivation rule for original response-frame custody and the values causally dependent on that rule. It grants no corpus mutation, RED mutation, test edit, source edit, generated-artifact edit, implementation, C15 assessment, activation, commit, or production authority. V1 and v2 are immutable. Their bytes, manifests, tests, schemas, expectations, and prior dispositions must not change.

The accepted v3 driver is the sole causal source for this correction. Prototype output, implementation output, current RED output, and existing v3 `expected` values are not derivation inputs.

The v3 expected schema remains exactly 145 paths; the exact 17 removed/deferred paths remain absent; and ownership remains exactly:

```text
A production generic owner/view fields                     116
B RED-derived assertions from direct actual executions      20
C fixture/control-only assertions and metadata                9
                                                          -----
exact retained expected paths                              145
removed/deferred paths remain absent                        17
```

No path changes A/B/C category.

## Corrected generic response-frame rule

For one self-contained occurrence, let the accepted ordered `driver.transport.events` array be `E`.

Let `F` be, in event order, the sequence of exact byte strings obtained by strict lowercase-hex decoding `event.frame_hex` for each **successfully emitted** event whose `kind` is exactly `response-frame`. A control event (`transport-error`, `cancel`, `deadline`, `shutdown`, or `unavailable`) contributes no response-frame bytes and no response-frame message. A response-frame event that is emitted successfully contributes its complete bytes even when those bytes are malformed as an LSP/JSON-RPC frame.

Define:

```text
R  = F[0] || F[1] || ... || F[n-1]       exact byte concatenation
N  = len(F)                                emitted response-frame count
L  = len(R)                                exact byte length
H  = lowercase_hex(SHA256(R))              only when N > 0
```

The original response frame is `R`, not a parsed body, not a parsed result, not a normalized frame, and not a terminal-error rendering.

The four retained response-frame expectations are derived as follows:

| Path | Correct derivation |
|---|---|
| `frames.response.available` | `N > 0` |
| `frames.response.frame_hex` | `lowercase_hex(R)` when `N > 0`; JSON `null` when `N == 0` |
| `frames.response.frame_length` | `L` when `N > 0`; `0` when `N == 0` |
| `frames.response.frame_sha256` | `H` when `N > 0`; 64 lowercase zeroes when `N == 0` |

Thus malformed emitted bytes remain available and raw. No response-frame events means frame unavailable with normalized empty metadata. An empty event array and a nonempty array containing only non-response control events have the same response-frame availability result. No terminal JSON-RPC response frame may be synthesized from an error, failure, cancellation, deadline, shutdown, unavailability, terminal value, expected field, or implementation observation.

For raw response transport and aggregate accounting:

```text
transport response bytes    = L
transport response messages = N
accounting.bytes.value       = the existing aggregate byte-accounting formula with its response-byte operand equal to L
available                    = true after an attempted external execution
known_zero                   = available && value == 0
```

For `accounting.bytes`, availability remains `true` and `known_zero` remains `false` in all five affected occurrences; only `accounting.bytes.value` changes.

The corresponding ledger dimensions use their existing causal arithmetic:

```text
before = 0
 delta = actual observed value
after  = before + delta
known_zero = available && delta == 0
```

For the generic external/raw projections affected here, `external.responses_hex` and B path `derived.external.responses_bytes` are the exact lowercase hexadecimal encoding of `R` when external observation is available; the available empty response sequence is `""`, not `null`. `derived.external.bytes = L` and `derived.external.messages = N`. The `round_trip_ledger.dimensions.external_bytes` dimension uses `L` under the same before/delta/after arithmetic. These rules retain malformed `R` without parsing it and prevent a control-specific error object from being counted as response bytes.

`result.*`, `error.*`, `external.failure`, `terminal.*`, and `round_trip_ledger.terminal.*` continue to derive from their distinct actual generic result/error/failure/terminal causes. They change only if the corrected raw-response rule changes their causal formula. For the five finite occurrences below, it does not: no result is produced; each existing generic error class/value and terminal disposition remains fixed by the accepted control or malformed-frame cause. Error serialization is not a response frame.

## Exhaustive affected-occurrence census

A strict traversal of all 85 accepted v3 occurrences was used: 25 roots, 16 World-24 workers, 22 World-25 executions, and 22 World-25 pair members. Exactly five occurrences are affected: roots World 07 through World 11. No World-24 worker, World-25 execution, or World-25 disabled/enabled pair member copies any of these five driver event scripts; therefore there are no W24/W25 copies to correct.

### World 07 root — malformed emitted frame retained raw

Accepted event:

```json
{"at_offset_ns":0,"frame_hex":"436f6e74656e742d4c656e6774683a206e6f70650d0a0d0a7b7d","kind":"response-frame"}
```

Exact derived constants:

```text
N = 1
R hex = 436f6e74656e742d4c656e6774683a206e6f70650d0a0d0a7b7d
L = 26
H = 138a9db5f8d8eb54884d5df8d0ffba294626a607c70b7bae9191edae5226739d
```

Corrected values:

| Path | Exact value |
|---|---|
| `frames.response.available` | `true` |
| `frames.response.frame_hex` | `"436f6e74656e742d4c656e6774683a206e6f70650d0a0d0a7b7d"` |
| `frames.response.frame_length` | `26` |
| `frames.response.frame_sha256` | `"138a9db5f8d8eb54884d5df8d0ffba294626a607c70b7bae9191edae5226739d"` |
| `external.responses_hex` | `"436f6e74656e742d4c656e6774683a206e6f70650d0a0d0a7b7d"` |
| `transport.bytes.available` | `true` |
| `transport.bytes.known_zero` | `false` |
| `transport.bytes.value` | `26` |
| `transport.messages.available` | `true` |
| `transport.messages.known_zero` | `false` |
| `transport.messages.value` | `1` |
| `round_trip_ledger.dimensions.transport_bytes.before` | `0` |
| `round_trip_ledger.dimensions.transport_bytes.after` | `26` |
| `round_trip_ledger.dimensions.transport_bytes.delta` | `26` |
| `round_trip_ledger.dimensions.transport_bytes.available` | `true` |
| `round_trip_ledger.dimensions.transport_bytes.known_zero` | `false` |
| `round_trip_ledger.dimensions.transport_messages.before` | `0` |
| `round_trip_ledger.dimensions.transport_messages.after` | `1` |
| `round_trip_ledger.dimensions.transport_messages.delta` | `1` |
| `round_trip_ledger.dimensions.transport_messages.available` | `true` |
| `round_trip_ledger.dimensions.transport_messages.known_zero` | `false` |
| `round_trip_ledger.dimensions.external_bytes.before` | `0` |
| `round_trip_ledger.dimensions.external_bytes.after` | `26` |
| `round_trip_ledger.dimensions.external_bytes.delta` | `26` |
| `round_trip_ledger.dimensions.external_bytes.available` | `true` |
| `round_trip_ledger.dimensions.external_bytes.known_zero` | `false` |
| `derived.external.responses_bytes` | `"436f6e74656e742d4c656e6774683a206e6f70650d0a0d0a7b7d"` |
| `derived.external.bytes` | `26` |
| `derived.external.messages` | `1` |
| `accounting.bytes.value` | `190` |

`accounting.bytes.available=true` and `accounting.bytes.known_zero=false` remain unchanged.

The following causally adjacent values remain exact and unchanged: `result.available=false`, `result.bytes_hex=null`; all eight `error.*` values (including class `malformed`, code `-32700`, and the existing serialized generic error); `external.available=true`; `external.failure="malformed"`; `external.notifications_hex="5b5d"`; `terminal.available=true`; `terminal.value="failed-malformed"`; and both `round_trip_ledger.terminal.*` records. The generic malformed error is not serialized into `frames.response` or counted as response bytes.

### Worlds 08–11 roots — no response-frame event

Each accepted script has `N=0` and `R` equal to the available empty response byte sequence:

| Occurrence | Sole accepted event | Existing causal error/terminal retained |
|---|---|---|
| World 08 root | `cancel`, offset `0` | class/failure `cancellation`; terminal `failed-cancellation` |
| World 09 root | `deadline`, offset `30000000` | class/failure `deadline`; terminal `failed-deadline` |
| World 10 root | `shutdown`, offset `0` | class/failure `shutdown`; terminal `failed-shutdown` |
| World 11 root | `unavailable`, offset `0` | class/failure `transport-unavailable`; terminal `failed-transport-unavailable` |

For **each** of Worlds 08, 09, 10, and 11, the corrected exact values are:

| Path | Exact value |
|---|---|
| `frames.response.available` | `false` |
| `frames.response.frame_hex` | `null` |
| `frames.response.frame_length` | `0` |
| `frames.response.frame_sha256` | `"0000000000000000000000000000000000000000000000000000000000000000"` |
| `external.responses_hex` | `""` |
| `transport.bytes.available` | `true` |
| `transport.bytes.known_zero` | `true` |
| `transport.bytes.value` | `0` |
| `transport.messages.available` | `true` |
| `transport.messages.known_zero` | `true` |
| `transport.messages.value` | `0` |
| `round_trip_ledger.dimensions.transport_bytes.before` | `0` |
| `round_trip_ledger.dimensions.transport_bytes.after` | `0` |
| `round_trip_ledger.dimensions.transport_bytes.delta` | `0` |
| `round_trip_ledger.dimensions.transport_bytes.available` | `true` |
| `round_trip_ledger.dimensions.transport_bytes.known_zero` | `true` |
| `round_trip_ledger.dimensions.transport_messages.before` | `0` |
| `round_trip_ledger.dimensions.transport_messages.after` | `0` |
| `round_trip_ledger.dimensions.transport_messages.delta` | `0` |
| `round_trip_ledger.dimensions.transport_messages.available` | `true` |
| `round_trip_ledger.dimensions.transport_messages.known_zero` | `true` |
| `round_trip_ledger.dimensions.external_bytes.before` | `0` |
| `round_trip_ledger.dimensions.external_bytes.after` | `0` |
| `round_trip_ledger.dimensions.external_bytes.delta` | `0` |
| `round_trip_ledger.dimensions.external_bytes.available` | `true` |
| `round_trip_ledger.dimensions.external_bytes.known_zero` | `true` |
| `derived.external.responses_bytes` | `""` |
| `derived.external.bytes` | `0` |
| `derived.external.messages` | `0` |

The corrected aggregate accounting values are:

| Occurrence | `accounting.bytes.value` |
|---|---:|
| World 08 root | `170` |
| World 09 root | `162` |
| World 10 root | `162` |
| World 11 root | `188` |

For each world, `accounting.bytes.available=true` and `accounting.bytes.known_zero=false` remain unchanged. `result.available=false` and `result.bytes_hex=null` remain unchanged. All eight `error.*` fields remain unchanged at their accepted world-specific values. `external.available=true`, `external.failure`, `external.notifications_hex="5b5d"`, `terminal.available=true`, `terminal.value`, and both `round_trip_ledger.terminal.*` records remain unchanged. The control event still causes its generic error/failure/terminal outcome; it does not emit a response frame.

## Exact affected A/B path sets

The correction changes expected values only at the following A paths where the old projection synthesized or accounted terminal JSON-RPC response bytes:

```text
frames.response.available
frames.response.frame_hex
frames.response.frame_length
frames.response.frame_sha256
external.responses_hex
transport.bytes.available
transport.bytes.known_zero
transport.bytes.value
transport.messages.available
transport.messages.known_zero
transport.messages.value
accounting.bytes.value
round_trip_ledger.dimensions.external_bytes.after
round_trip_ledger.dimensions.external_bytes.available
round_trip_ledger.dimensions.external_bytes.before
round_trip_ledger.dimensions.external_bytes.delta
round_trip_ledger.dimensions.external_bytes.known_zero
round_trip_ledger.dimensions.transport_bytes.after
round_trip_ledger.dimensions.transport_bytes.available
round_trip_ledger.dimensions.transport_bytes.before
round_trip_ledger.dimensions.transport_bytes.delta
round_trip_ledger.dimensions.transport_bytes.known_zero
round_trip_ledger.dimensions.transport_messages.after
round_trip_ledger.dimensions.transport_messages.available
round_trip_ledger.dimensions.transport_messages.before
round_trip_ledger.dimensions.transport_messages.delta
round_trip_ledger.dimensions.transport_messages.known_zero
```

Availability and `before` members in that list are retained as explicit causal operands even where their scalar value remains `true` or `0`; rematerialization must validate them rather than silently inherit the old record. `accounting.bytes.value` must likewise be rematerialized from the existing aggregate byte-accounting formula with corrected `L`, never copied from the prior expected record.

The only affected B paths are:

```text
derived.external.responses_bytes
derived.external.messages
derived.external.bytes
```

They must be recomputed by the RED from direct actual execution. No expected field may be used as an actual operand.

`result.*`, `error.*`, `external.failure`, `terminal.*`, `round_trip_ledger.terminal.*`, `derived.external.result_bytes`, `derived.external.server_error_bytes`, and `derived.external.failure` are explicitly reviewed causal neighbors but are unchanged for Worlds 07–11 under the formulas above.

## Unchanged paths

For the five affected occurrences, every retained expectation path not named in the exact affected A/B sets above is unchanged. In family form, the unchanged set is:

- the other 11 `accounting.*` paths, meaning every `accounting.*` path except affected A path `accounting.bytes.value`;
- all 12 `commitments.*` paths;
- `comparison.available` and `concurrency.available`;
- all seven `control.*` paths;
- all `frames.request.*` and `frames.auxiliary`;
- all `identity.*`;
- all `lifecycle.*`;
- `name`;
- all `refusal.*`;
- all `result.*`;
- all `error.*`;
- `external.available`, `external.execution_expected`, `external.failure`, and `external.notifications_hex`;
- all callback, diagnostic, lease, and reservation ledger dimensions;
- both ledger terminal records;
- all `terminal.*`;
- all `transport.request_bytes.*` and `transport.request_messages.*`;
- `derived.control.*`;
- `derived.external.result_bytes`, `derived.external.server_error_bytes`, `derived.external.notifications_bytes`, `derived.external.failure`, `derived.external.request_messages`, and `derived.external.request_bytes`;
- all `derived.lifecycle.*`; and
- `derived.identity.transaction_request_equals_request_identity`.

All 17 removed/deferred paths remain absent: four request-body fields, four request-payload fields, four response-body fields, four response-result fields, and `result.form`. This amendment does not restore them under aliases or infer them from raw bytes.

## Canonical corpus rematerialization rule

A later separately authorized v3 corpus rematerialization must:

1. verify the accepted v3 manifest, all 25 canonical world bytes, the accepted v3 causal-input registry pin, and the immutable reachable v1/v2 pins before deriving anything;
2. strict-decode every self-contained v3 envelope with duplicate/unknown-key rejection;
3. traverse all 85 occurrences in the accepted structural order and derive `F`, `R`, `N`, `L`, and `H` solely from each occurrence's accepted `driver.transport.events`;
4. identify exactly the five affected roots above and independently prove that W24/W25 contain no copies under exact driver event scripts or the four control-event kinds;
5. rematerialize every exact affected A/B expected value specified here, including `accounting.bytes.value`, from the accepted causal inputs and formulas; do not read prototype, RED, test, implementation, or runtime output and do not use an existing affected expected value as a derivation operand;
6. prove the exact complement: for each affected occurrence, the 145 retained paths minus the exact affected A/B path sets is unchanged; preserve every complement path and every driver decoded value, all commitment inputs and expectations, all array order, the exact 145-path denominator, the exact `116/20/9` ownership partition, and all 17 absences;
7. serialize each complete world as canonical UTF-8 JSON: recursively ascending lexical object-member order, retained array order, shortest decimal integers, minimal required string escaping, no insignificant whitespace, and exactly one terminal LF;
8. recompute each changed complete world byte length and SHA-256, retain unchanged world bytes exactly, then rebuild the manifest's exact per-world length/digest entries, aggregate byte length, and aggregate SHA-256 over the lexical ordered concatenation `worlds/world-01.json` through `worlds/world-25.json`;
9. independently reproduce all changed canonical bytes and hashes before any corpus acceptance; and
10. never mutate v1 or v2.

Only `worlds/world-07.json` through `worlds/world-11.json` and their causally changed manifest length/hash/aggregate entries are eligible for later rematerialization. Exact future canonical file lengths and hashes are outputs of that authorized rematerialization and independent review; this pending amendment does not predict them.

## RED requirement: direct actual only

A later separately authorized RED must obtain the actual response-frame byte sequence only from the fake transport's successfully emitted response-frame events and the production owner/view observation produced by the real `Manager.RoundTrip` execution. It must compare every A field to the corrected expected value field-by-field and compute every B field from direct actual execution.

The RED must not:

- copy an expected frame, error, terminal, or derived value into an actual;
- synthesize a terminal JSON-RPC frame;
- serialize an actual error and call those bytes the original response frame;
- parse or normalize malformed response-frame bytes before custody;
- treat a cancel/deadline/shutdown/unavailable event as a response-frame message;
- use prototype output, current implementation output, current RED output, v1/v2 expected values, world names, occurrence names, or fixture labels as the corrected oracle;
- compare expected to expected or fabricate an unavailable/empty observation; or
- weaken, skip, wildcard, rename, delete, or make optional any of the 145 retained paths.

World 07 must be RED unless the direct actual frame observation is exactly the 26 malformed bytes above. Worlds 08–11 must be RED unless direct actual response-frame custody is unavailable while response byte/message accounting is available-known-zero and their independently caused error/failure/terminal observations remain exact. A missing A field is implementation RED; a failed B formula is assertion RED; invalid C structure is fixture RED.

## Pending review gate

An independent review must verify the five-occurrence census, absence of W24/W25 copies, exact raw bytes/length/digest for World 07, exact unavailable/empty semantics for Worlds 08–11, exhaustive affected and unchanged path sets, mandatory rematerialization of every affected value including `accounting.bytes.value`, an exact complement proof that every retained non-affected path is unchanged, preservation of commitments and causal inputs, exact `145/17` and `116/20/9` accounting, direct-actual-only RED rules, canonical rematerialization rules, and immutable v1/v2 boundaries.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_RAW_RESPONSE_CUSTODY_AMENDMENT_REVIEW_GO — NOT ISSUED
STATUS — PENDING
```
