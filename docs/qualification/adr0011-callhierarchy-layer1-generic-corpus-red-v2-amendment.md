# ADR-0011 call-hierarchy Layer-1 generic corpus RED v2 amendment

## Status and authority

**Disposition: `REVIEW_GATE_PENDING`.** This document freezes a proposed versioned successor contract at exact clean repository HEAD `e16e1bc3aa95693d84e18b2f967fa35fa04c22e0`. It creates no implementation, corpus, test, generated-output, activation, acceptance, or commit authority.

The accepted v1 corpus, RED, and commit history are immutable and must remain reachable. Do not edit, replace, regenerate in place, weaken, skip, rename, or delete:

- `sessionruntime/testdata/callhierarchy-layer1-red-v1/**`;
- `sessionruntime/b4_owned_method_pair_lease_test.go`;
- their accepted historical commits and reviews.

V1 remains accepted historical overreach evidence. It is not an implementation gate for the successor. Successor work is additive and may use only these proposed new paths:

- `sessionruntime/testdata/callhierarchy-layer1-red-v2/**`;
- `sessionruntime/b4_callhierarchy_layer1_generic_v2_test.go`.

The proposed test name is exact: it names B4 ownership, call-hierarchy Layer-1, generic semantics, and the v2 contract without implying that the immutable owned-method-pair v1 RED was edited.

## Frozen correction

V2 preserves every generic and opaque expectation whose value or derivation is already fixed by v1, but removes/defer exactly the 17 rich parsing expectations that exceed the generic Layer-1 boundary. It requires no implementation-produced oracle output.

```text
v1 expected paths                 162
preserve generic                  133
preserve opaque commitments        12
remove/defer                        17
                                 -----
reconciled                         162
successor expected paths           145 (= 133 + 12)
```

The mechanical denominator is the 146 stable record leaves common to all 25 v1 roots plus the 16 named v1 RED derivations below. Array members use `[*]` only as a schema path; order and cardinality remain literal corpus values. `frames.response.payload.*` is the v1 corpus spelling for the response-result subspan and is called “response result” in the correction.

Every v2 root and every inline record is an envelope with exactly two siblings:

- `driver` is a fixed, non-oracle causal input used to invoke the successor RED; and
- `expected` is the exact projected oracle containing only the 145 retained expectation paths.

The inventory and denominator remain exactly `162 -> 145`: the 17 named expectation paths are removed only from `expected`. No `driver` path is an expectation path, enters the 162 or 145 denominator, replaces a removed expected field, or is compared with implementation output. There is no hidden side table, naming convention, world-name switch, or other input source outside each self-contained envelope. World 24's 16 inline worker records and world 25's 22 inline execution records plus each disabled/enabled record embedded in its 11 pairs each carry their own complete `driver` alongside their own `expected`; they do not inherit a root driver or resolve one by name.

## Exhaustive v1 expected-path inventory

| # | Exact v1 expectation path | V2 disposition | V2 source rule |
|---:|---|---|---|
| 001 | `accounting.bytes.available` | PRESERVE GENERIC | unchanged v1 literal |
| 002 | `accounting.bytes.known_zero` | PRESERVE GENERIC | unchanged v1 literal |
| 003 | `accounting.bytes.value` | PRESERVE GENERIC | unchanged v1 literal |
| 004 | `accounting.diagnostics.available` | PRESERVE GENERIC | unchanged v1 literal |
| 005 | `accounting.diagnostics.known_zero` | PRESERVE GENERIC | unchanged v1 literal |
| 006 | `accounting.diagnostics.value` | PRESERVE GENERIC | unchanged v1 literal |
| 007 | `accounting.objects.available` | PRESERVE GENERIC | unchanged v1 literal |
| 008 | `accounting.objects.known_zero` | PRESERVE GENERIC | unchanged v1 literal |
| 009 | `accounting.objects.value` | PRESERVE GENERIC | unchanged v1 literal |
| 010 | `accounting.work.available` | PRESERVE GENERIC | unchanged v1 literal |
| 011 | `accounting.work.known_zero` | PRESERVE GENERIC | unchanged v1 literal |
| 012 | `accounting.work.value` | PRESERVE GENERIC | unchanged v1 literal |
| 013 | `commitments.handoff.available` | PRESERVE OPAQUE | unchanged v1 literal; opaque metadata only |
| 014 | `commitments.handoff.identity` | PRESERVE OPAQUE | unchanged v1 literal; opaque metadata only |
| 015 | `commitments.handoff.length` | PRESERVE OPAQUE | unchanged v1 literal; opaque metadata only |
| 016 | `commitments.handoff.sha256` | PRESERVE OPAQUE | unchanged v1 literal; opaque metadata only |
| 017 | `commitments.predecessors[*].available` | PRESERVE OPAQUE | unchanged v1 literal; opaque metadata only |
| 018 | `commitments.predecessors[*].identity` | PRESERVE OPAQUE | unchanged v1 literal; opaque metadata only |
| 019 | `commitments.predecessors[*].length` | PRESERVE OPAQUE | unchanged v1 literal; opaque metadata only |
| 020 | `commitments.predecessors[*].sha256` | PRESERVE OPAQUE | unchanged v1 literal; opaque metadata only |
| 021 | `commitments.selected.available` | PRESERVE OPAQUE | unchanged v1 literal; opaque metadata only |
| 022 | `commitments.selected.identity` | PRESERVE OPAQUE | unchanged v1 literal; opaque metadata only |
| 023 | `commitments.selected.length` | PRESERVE OPAQUE | unchanged v1 literal; opaque metadata only |
| 024 | `commitments.selected.sha256` | PRESERVE OPAQUE | unchanged v1 literal; opaque metadata only |
| 025 | `comparison.available` | PRESERVE GENERIC | unchanged v1 literal |
| 026 | `concurrency.available` | PRESERVE GENERIC | unchanged v1 literal |
| 027 | `control.attachment_available` | PRESERVE GENERIC | unchanged v1 literal |
| 028 | `control.enabled` | PRESERVE GENERIC | unchanged v1 literal |
| 029 | `control.observation_available` | PRESERVE GENERIC | unchanged v1 literal |
| 030 | `control.paired_world` | PRESERVE GENERIC | unchanged v1 literal |
| 031 | `control.reason` | PRESERVE GENERIC | unchanged v1 literal |
| 032 | `error.available` | PRESERVE GENERIC | unchanged v1 literal |
| 033 | `error.class` | PRESERVE GENERIC | unchanged v1 literal |
| 034 | `error.code` | PRESERVE GENERIC | unchanged v1 literal |
| 035 | `error.data_base64` | PRESERVE GENERIC | unchanged v1 literal |
| 036 | `error.data_form` | PRESERVE GENERIC | unchanged v1 literal |
| 037 | `error.message` | PRESERVE GENERIC | unchanged v1 literal |
| 038 | `error.present` | PRESERVE GENERIC | unchanged v1 literal |
| 039 | `error.serialized_hex` | PRESERVE GENERIC | unchanged v1 literal |
| 040 | `external.available` | PRESERVE GENERIC | unchanged v1 literal |
| 041 | `external.execution_expected` | PRESERVE GENERIC | unchanged v1 literal |
| 042 | `external.failure` | PRESERVE GENERIC | unchanged v1 literal |
| 043 | `external.notifications_hex` | PRESERVE GENERIC | unchanged v1 literal |
| 044 | `external.responses_hex` | PRESERVE GENERIC | unchanged v1 literal |
| 045 | `frames.auxiliary` | PRESERVE GENERIC | unchanged v1 literal |
| 046 | `frames.request.available` | PRESERVE GENERIC | unchanged v1 literal |
| 047 | `frames.request.body.available` | REMOVE/DEFER | v1 literal; successor intentionally has no field or derivation |
| 048 | `frames.request.body.bytes_hex` | REMOVE/DEFER | v1 literal; successor intentionally has no field or derivation |
| 049 | `frames.request.body.length` | REMOVE/DEFER | v1 literal; successor intentionally has no field or derivation |
| 050 | `frames.request.body.sha256` | REMOVE/DEFER | v1 literal; successor intentionally has no field or derivation |
| 051 | `frames.request.frame_hex` | PRESERVE GENERIC | unchanged v1 literal |
| 052 | `frames.request.frame_length` | PRESERVE GENERIC | unchanged v1 literal |
| 053 | `frames.request.frame_sha256` | PRESERVE GENERIC | unchanged v1 literal |
| 054 | `frames.request.payload.available` | REMOVE/DEFER | v1 literal; successor intentionally has no field or derivation |
| 055 | `frames.request.payload.bytes_hex` | REMOVE/DEFER | v1 literal; successor intentionally has no field or derivation |
| 056 | `frames.request.payload.length` | REMOVE/DEFER | v1 literal; successor intentionally has no field or derivation |
| 057 | `frames.request.payload.sha256` | REMOVE/DEFER | v1 literal; successor intentionally has no field or derivation |
| 058 | `frames.response.available` | PRESERVE GENERIC | unchanged v1 literal |
| 059 | `frames.response.body.available` | REMOVE/DEFER | v1 literal; successor intentionally has no field or derivation |
| 060 | `frames.response.body.bytes_hex` | REMOVE/DEFER | v1 literal; successor intentionally has no field or derivation |
| 061 | `frames.response.body.length` | REMOVE/DEFER | v1 literal; successor intentionally has no field or derivation |
| 062 | `frames.response.body.sha256` | REMOVE/DEFER | v1 literal; successor intentionally has no field or derivation |
| 063 | `frames.response.frame_hex` | PRESERVE GENERIC | unchanged v1 literal |
| 064 | `frames.response.frame_length` | PRESERVE GENERIC | unchanged v1 literal |
| 065 | `frames.response.frame_sha256` | PRESERVE GENERIC | unchanged v1 literal |
| 066 | `frames.response.payload.available` | REMOVE/DEFER | v1 literal; successor intentionally has no field or derivation |
| 067 | `frames.response.payload.bytes_hex` | REMOVE/DEFER | v1 literal; successor intentionally has no field or derivation |
| 068 | `frames.response.payload.length` | REMOVE/DEFER | v1 literal; successor intentionally has no field or derivation |
| 069 | `frames.response.payload.sha256` | REMOVE/DEFER | v1 literal; successor intentionally has no field or derivation |
| 070 | `identity.binding.available` | PRESERVE GENERIC | unchanged v1 literal |
| 071 | `identity.binding.value` | PRESERVE GENERIC | unchanged v1 literal |
| 072 | `identity.generation.available` | PRESERVE GENERIC | unchanged v1 literal |
| 073 | `identity.generation.value` | PRESERVE GENERIC | unchanged v1 literal |
| 074 | `identity.method.available` | PRESERVE GENERIC | unchanged v1 literal |
| 075 | `identity.method.value` | PRESERVE GENERIC | unchanged v1 literal |
| 076 | `identity.profile.available` | PRESERVE GENERIC | unchanged v1 literal |
| 077 | `identity.profile.value` | PRESERVE GENERIC | unchanged v1 literal |
| 078 | `identity.request_identity.generation` | PRESERVE GENERIC | unchanged v1 literal |
| 079 | `identity.request_identity.request_key` | PRESERVE GENERIC | unchanged v1 literal |
| 080 | `identity.request_identity.session` | PRESERVE GENERIC | unchanged v1 literal |
| 081 | `identity.request_identity.workspace_nonce_hex` | PRESERVE GENERIC | unchanged v1 literal |
| 082 | `identity.transaction.available` | PRESERVE GENERIC | unchanged v1 literal |
| 083 | `identity.transaction.local_hex` | PRESERVE GENERIC | unchanged v1 literal |
| 084 | `lifecycle.acquired` | PRESERVE GENERIC | unchanged v1 literal |
| 085 | `lifecycle.completed` | PRESERVE GENERIC | unchanged v1 literal |
| 086 | `lifecycle.released` | PRESERVE GENERIC | unchanged v1 literal |
| 087 | `lifecycle.sequence[*]` | PRESERVE GENERIC | unchanged v1 literal |
| 088 | `name` | PRESERVE GENERIC | unchanged v1 literal |
| 089 | `refusal.reason` | PRESERVE GENERIC | unchanged v1 literal |
| 090 | `refusal.refused` | PRESERVE GENERIC | unchanged v1 literal |
| 091 | `result.available` | PRESERVE GENERIC | unchanged v1 literal |
| 092 | `result.bytes_hex` | PRESERVE GENERIC | unchanged v1 literal |
| 093 | `result.form` | REMOVE/DEFER | v1 literal; successor intentionally has no field or derivation |
| 094 | `round_trip_ledger.dimensions.callbacks.after` | PRESERVE GENERIC | unchanged v1 literal |
| 095 | `round_trip_ledger.dimensions.callbacks.available` | PRESERVE GENERIC | unchanged v1 literal |
| 096 | `round_trip_ledger.dimensions.callbacks.before` | PRESERVE GENERIC | unchanged v1 literal |
| 097 | `round_trip_ledger.dimensions.callbacks.delta` | PRESERVE GENERIC | unchanged v1 literal |
| 098 | `round_trip_ledger.dimensions.callbacks.known_zero` | PRESERVE GENERIC | unchanged v1 literal |
| 099 | `round_trip_ledger.dimensions.diagnostics.after` | PRESERVE GENERIC | unchanged v1 literal |
| 100 | `round_trip_ledger.dimensions.diagnostics.available` | PRESERVE GENERIC | unchanged v1 literal |
| 101 | `round_trip_ledger.dimensions.diagnostics.before` | PRESERVE GENERIC | unchanged v1 literal |
| 102 | `round_trip_ledger.dimensions.diagnostics.delta` | PRESERVE GENERIC | unchanged v1 literal |
| 103 | `round_trip_ledger.dimensions.diagnostics.known_zero` | PRESERVE GENERIC | unchanged v1 literal |
| 104 | `round_trip_ledger.dimensions.external_bytes.after` | PRESERVE GENERIC | unchanged v1 literal |
| 105 | `round_trip_ledger.dimensions.external_bytes.available` | PRESERVE GENERIC | unchanged v1 literal |
| 106 | `round_trip_ledger.dimensions.external_bytes.before` | PRESERVE GENERIC | unchanged v1 literal |
| 107 | `round_trip_ledger.dimensions.external_bytes.delta` | PRESERVE GENERIC | unchanged v1 literal |
| 108 | `round_trip_ledger.dimensions.external_bytes.known_zero` | PRESERVE GENERIC | unchanged v1 literal |
| 109 | `round_trip_ledger.dimensions.leases.after` | PRESERVE GENERIC | unchanged v1 literal |
| 110 | `round_trip_ledger.dimensions.leases.available` | PRESERVE GENERIC | unchanged v1 literal |
| 111 | `round_trip_ledger.dimensions.leases.before` | PRESERVE GENERIC | unchanged v1 literal |
| 112 | `round_trip_ledger.dimensions.leases.delta` | PRESERVE GENERIC | unchanged v1 literal |
| 113 | `round_trip_ledger.dimensions.leases.known_zero` | PRESERVE GENERIC | unchanged v1 literal |
| 114 | `round_trip_ledger.dimensions.reservations.after` | PRESERVE GENERIC | unchanged v1 literal |
| 115 | `round_trip_ledger.dimensions.reservations.available` | PRESERVE GENERIC | unchanged v1 literal |
| 116 | `round_trip_ledger.dimensions.reservations.before` | PRESERVE GENERIC | unchanged v1 literal |
| 117 | `round_trip_ledger.dimensions.reservations.delta` | PRESERVE GENERIC | unchanged v1 literal |
| 118 | `round_trip_ledger.dimensions.reservations.known_zero` | PRESERVE GENERIC | unchanged v1 literal |
| 119 | `round_trip_ledger.dimensions.transport_bytes.after` | PRESERVE GENERIC | unchanged v1 literal |
| 120 | `round_trip_ledger.dimensions.transport_bytes.available` | PRESERVE GENERIC | unchanged v1 literal |
| 121 | `round_trip_ledger.dimensions.transport_bytes.before` | PRESERVE GENERIC | unchanged v1 literal |
| 122 | `round_trip_ledger.dimensions.transport_bytes.delta` | PRESERVE GENERIC | unchanged v1 literal |
| 123 | `round_trip_ledger.dimensions.transport_bytes.known_zero` | PRESERVE GENERIC | unchanged v1 literal |
| 124 | `round_trip_ledger.dimensions.transport_messages.after` | PRESERVE GENERIC | unchanged v1 literal |
| 125 | `round_trip_ledger.dimensions.transport_messages.available` | PRESERVE GENERIC | unchanged v1 literal |
| 126 | `round_trip_ledger.dimensions.transport_messages.before` | PRESERVE GENERIC | unchanged v1 literal |
| 127 | `round_trip_ledger.dimensions.transport_messages.delta` | PRESERVE GENERIC | unchanged v1 literal |
| 128 | `round_trip_ledger.dimensions.transport_messages.known_zero` | PRESERVE GENERIC | unchanged v1 literal |
| 129 | `round_trip_ledger.terminal.after.available` | PRESERVE GENERIC | unchanged v1 literal |
| 130 | `round_trip_ledger.terminal.after.value` | PRESERVE GENERIC | unchanged v1 literal |
| 131 | `round_trip_ledger.terminal.before.available` | PRESERVE GENERIC | unchanged v1 literal |
| 132 | `round_trip_ledger.terminal.before.value` | PRESERVE GENERIC | unchanged v1 literal |
| 133 | `terminal.available` | PRESERVE GENERIC | unchanged v1 literal |
| 134 | `terminal.value` | PRESERVE GENERIC | unchanged v1 literal |
| 135 | `transport.bytes.available` | PRESERVE GENERIC | unchanged v1 literal |
| 136 | `transport.bytes.known_zero` | PRESERVE GENERIC | unchanged v1 literal |
| 137 | `transport.bytes.value` | PRESERVE GENERIC | unchanged v1 literal |
| 138 | `transport.messages.available` | PRESERVE GENERIC | unchanged v1 literal |
| 139 | `transport.messages.known_zero` | PRESERVE GENERIC | unchanged v1 literal |
| 140 | `transport.messages.value` | PRESERVE GENERIC | unchanged v1 literal |
| 141 | `transport.request_bytes.available` | PRESERVE GENERIC | unchanged v1 literal |
| 142 | `transport.request_bytes.known_zero` | PRESERVE GENERIC | unchanged v1 literal |
| 143 | `transport.request_bytes.value` | PRESERVE GENERIC | unchanged v1 literal |
| 144 | `transport.request_messages.available` | PRESERVE GENERIC | unchanged v1 literal |
| 145 | `transport.request_messages.known_zero` | PRESERVE GENERIC | unchanged v1 literal |
| 146 | `transport.request_messages.value` | PRESERVE GENERIC | unchanged v1 literal |
| 147 | `derived.control.actual_attachment_available` | PRESERVE GENERIC | unchanged v1 RED derivation from retained literals |
| 148 | `derived.control.actual_reason` | PRESERVE GENERIC | unchanged v1 RED derivation from retained literals |
| 149 | `derived.external.result_bytes` | PRESERVE GENERIC | unchanged v1 RED derivation from retained literals |
| 150 | `derived.external.server_error_bytes` | PRESERVE GENERIC | unchanged v1 RED derivation from retained literals |
| 151 | `derived.external.notifications_bytes` | PRESERVE GENERIC | unchanged v1 RED derivation from retained literals |
| 152 | `derived.external.responses_bytes` | PRESERVE GENERIC | unchanged v1 RED derivation from retained literals |
| 153 | `derived.external.failure` | PRESERVE GENERIC | unchanged v1 RED derivation from retained literals |
| 154 | `derived.external.messages` | PRESERVE GENERIC | unchanged v1 RED derivation from retained literals |
| 155 | `derived.external.bytes` | PRESERVE GENERIC | unchanged v1 RED derivation from retained literals |
| 156 | `derived.external.request_messages` | PRESERVE GENERIC | unchanged v1 RED derivation from retained literals |
| 157 | `derived.external.request_bytes` | PRESERVE GENERIC | unchanged v1 RED derivation from retained literals |
| 158 | `derived.lifecycle.observation_available` | PRESERVE GENERIC | unchanged v1 RED derivation from retained literals |
| 159 | `derived.lifecycle.callback_completed` | PRESERVE GENERIC | unchanged v1 RED derivation from retained literals |
| 160 | `derived.lifecycle.release_first` | PRESERVE GENERIC | unchanged v1 RED derivation from retained literals |
| 161 | `derived.lifecycle.release_second` | PRESERVE GENERIC | unchanged v1 RED derivation from retained literals |
| 162 | `derived.identity.transaction_request_equals_request_identity` | PRESERVE GENERIC | unchanged v1 RED derivation from retained literals |

The 17 `REMOVE/DEFER` rows are exactly four request-body fields, four request-payload fields, four response-body fields, four response-result fields, and `result.form`. No other path is removed. Every retained row maps to its unchanged v1 literal or named v1 derivation; v2 must not obtain an expected value from production execution.

## Successor RED flow

The successor RED may assert only this flow:

1. strictly decode one self-contained envelope and separately validate its `driver` and `expected` subtrees;
2. construct the ordinary request and a generic, private, default-off request attachment from fixed corpus literals, hex-decoding `driver.request.params_hex` according to its explicit nil/empty/present state and passing those exact opaque `json.RawMessage` `Params` bytes directly to `Manager.RoundTrip` without parsing, wrapping, rewriting, or semantic reconstruction;
3. run the fake transport solely by consuming `driver.transport.events` in order and dispatching on each generic event kind and its exact bytes/class/message/timing values;
4. let that attachment cross `Manager.RoundTrip` without changing ordinary transport behavior;
5. obtain the returned generic result owner and a generic view through a generic lease;
6. compare implementation observations only with the finite 145-path `expected` oracle; and
7. let the callback return or panic under containment, with the lease releasing exactly once.

The RED must never parse a request or response frame to recover `Params` or another driver value. It must never choose behavior from world/root/record name, `expected`, `expected.result`, `expected.error`, an expected token/form label, or an expected semantic value. A changed world name with an unchanged driver must execute identically; a changed driver event with an unchanged world name must follow the changed event.

No call-hierarchy prepare/outgoing/incoming semantics, symbol semantics, subspan parsing, token forms, or richer predecessor interpretation is implied. Disabled and enabled executions must be externally transparent: equal request, wire, external result/error, transport, and terminal effects except for private attachment/owner availability and its explicitly accounted private lifecycle.

## Finite private owner/view/lease schema

The successor schema is private and finite. It may be implemented only in the already accepted production files `sessionruntime/sessionruntime.go` and `sessionruntime/b4_owned_method_pair_lease.go`; this amendment does not authorize either edit.

| Area | Permitted private fields | Field-by-field mapping and unavailable semantics |
|---|---|---|
| Generic identity | workspace manager nonce; session; generation; JSON-RPC request key; transaction-local identity; method; binding; profile | `identity.request_identity.workspace_nonce_hex -> workspace manager nonce`; `.session -> session`; `.generation -> request generation`; `.request_key -> request key`; `identity.transaction.local_hex -> transaction local`; `identity.method.value -> method`; `identity.binding.value -> binding`; `identity.profile.value -> profile`; `identity.generation.value -> generic generation`. Each availability-bearing source preserves its exact `available`; unavailable means zero private value plus `available=false`, never empty or absent by inference. Transaction.request may only equal the retained request identity. |
| Complete request frame | availability, raw complete frame bytes, frame length, frame SHA-256 | `frames.request.available`, `frame_hex`, `frame_length`, and `frame_sha256` map one-for-one. Unavailable means no frame bytes and `available=false`. No body/payload field or derivation exists. |
| Complete response frame | availability, raw complete frame bytes, frame length, frame SHA-256 | `frames.response.available`, `frame_hex`, `frame_length`, and `frame_sha256` map one-for-one. Unavailable means no frame bytes and `available=false`. No body/result field or derivation exists. |
| Opaque commitments | selected, handoff, and each ordered predecessor: identity, digest, length, availability | Each of the 12 `commitments.*` rows maps one-for-one. Identity is an opaque label; digest and length are commitments only. Unavailable means `available=false` and zero identity/digest/length. No commitment preimage bytes may be retained or exposed. |
| Generic result | raw result bytes and availability | `result.bytes_hex -> raw bytes`; `result.available -> availability`. Preserve nil/unavailable versus available empty bytes. Do not store or derive `result.form`. |
| Generic error | available, present, class, code, message, data bytes, data form, serialized bytes, with existing availability distinctions | Map each `error.*` row exactly; nullable values remain unavailable rather than synthesized. These are generic JSON-RPC/external observations, not call-hierarchy semantics. |
| External effects | availability, execution expected, failure, notifications bytes, responses bytes, generic result/error, request/transport measures | Map each `external.*`, `transport.*`, and retained `derived.external.*` row exactly. Unavailable is explicit; known zero remains distinct from unavailable. |
| Accounting | bytes, objects, work, diagnostics: availability, value, known-zero | Map each `accounting.*` row exactly. Never infer zero when unavailable. |
| Ledger | seven dimensions with before/after/delta/availability/known-zero; terminal before/after | Map each `round_trip_ledger.*` row exactly. The dimensions are callbacks, diagnostics, external bytes, leases, reservations, transport bytes, and transport messages. Arithmetic is checked without replacing corpus values with runtime output. |
| Terminal | available and value | Map `terminal.available` and `terminal.value`; unavailable is not an empty terminal. |
| Lifecycle | acquired, completed, released, ordered sequence; callback-completed and first/second-release derivations | Map `lifecycle.*` and retained `derived.lifecycle.*` exactly. First release succeeds and second does not. No owner means no acquisition or callback. |
| Refusal/control | enabled, attachment/observation availability, paired world, reason; refused/reason | Map each `control.*`, `refusal.*`, and retained `derived.control.*` row exactly. Refusal is private and externally transparent; unavailable observation does not imply transport did not execute. |

Explicitly forbidden: request/response body subspans; request payload or response-result subspans; offsets; parsed/token forms; `result.form`; raw selected/handoff/predecessor commitment preimages; symbol, item, range, call-site, caller/callee, direction, or provider semantics; rich predecessor lineage beyond ordered opaque metadata; source completeness; production authority.

### Ownership and lifecycle requirements

- The owner defensively copies every retained byte slice and ordered collection before publication. Mutation of request inputs, transport buffers, corpus loader buffers, or callback views cannot alter a later view.
- A view is admitted only while the owner is open. The callback executes outside the owner lock. Callback panics are contained as the exact generic callback error and cannot strand an active count.
- Terminal transition closes admission before waiting, then drains all active callbacks.
- Release closes admission, drains active callbacks, clears owned state, and has exactly one winner across repeated or concurrent calls.
- Enabled and disabled runs preserve identical external request/result/error/transport/terminal behavior. The only permitted delta is private generic observation availability and its exact private accounting/ledger/lifecycle consequence.

## Finite causal driver schema

`driver` is transport-only and strict: duplicate or unknown keys are rejected, every availability state is explicit, and no extension map is permitted. Its complete schema is:

- `derived.external`: exactly four required byte fields at the exact paths `driver.derived.external.notifications_bytes`, `driver.derived.external.responses_bytes`, `driver.derived.external.result_bytes`, and `driver.derived.external.server_error_bytes`. Each value uses the same canonical byte encoding as the v1 `*_hex` fields: JSON null when its source is unavailable; otherwise a JSON string containing lowercase, prefix-free, even-length hexadecimal, where `""` means available empty bytes and a non-empty string means available nonempty bytes. Uppercase hex, an odd number of hexadecimal digits, nonhexadecimal characters, `0x` or any other prefix, base64 strings, numeric byte arrays, and every other representation are rejected. After hexadecimal decoding, the bytes must equal exactly the corresponding committed v1 source bytes: `external.notifications_hex`, `external.responses_hex`, `result.bytes_hex`, and `error.serialized_hex`, respectively; no JSON reserialization or semantic reconstruction is permitted.
- `request`: exact `request_id`, `method`, `session`, `generation`, `max_messages`, `max_bytes`, and `deadline_after_ns` values needed to invoke `Manager.RoundTrip`; and `params_state`, one of `nil`, `empty`, or `present`, plus opaque `params_hex` constrained as follows. `request_id` is an exact positive JSON integer representable as `uint64`. It is non-oracle causal control metadata, not a replacement expectation, and is excluded from the unchanged 145-path expected denominator. When the retained expected control/attachment observation is available, it is derived mechanically before projection from the numeric value of retained v1 `expected.identity.request_identity.request_key` and must equal that available expected request identity plus the exact top-level JSON-RPC IDs embedded in the fixed complete request and response driver frames. For a W25 disabled execution whose retained expected control/attachment observation is unavailable, the retained expected request-identity scalar remains preserved but is inert and non-causal: derivation and validation must not consult it. That disabled driver's `request_id` is instead derived mechanically from the equal top-level IDs in its exact fixed request and response driver frames and must equal the enabled member's `request_id` and external execution ID in the same pair, preserving paired external/transport equality. Nonnumeric, zero, negative, fractional, or out-of-range causal IDs are rejected. All derivation validation uses values already fixed during corpus derivation and never parses a frame or consults `expected` at execution. For params, `nil` requires JSON null, `empty` requires the empty string, and `present` requires a non-empty even-length lowercase hexadecimal string. Thus nil, available-empty, and available-present bytes are exact and distinct without a semantic payload schema. `params_state` and `params_hex` encode the complete opaque `json.RawMessage` `Params` byte sequence passed to `Manager.RoundTrip`, mechanically copied from the accepted v1 RED fixture's request input construction at `751b0276c46dbccc4899cfec32adf513c123b57f` or, where fixed there, its exact generic request `Params` preimage. They never encode a request body/payload semantic subspan and are never reconstructed from an expected frame or implementation/runtime output. Present values include all wrapper bytes; for example, `Params` bytes `{"payload_hex":"7b7d"}` are encoded in `params_hex` as `7b227061796c6f61645f686578223a2237623764227d`. Execution only hex-decodes the field and passes the resulting bytes directly as `Params`, with no parse, wrap, rewrite, or reserialization.
- `transport.events`: an ordered array. Each event has exactly `kind` and `at_offset_ns`, plus only the payload required by its kind. `response-frame` has exact complete `frame_hex`; `transport-error` has exact `class` and `message`; `cancel`, `deadline`, `shutdown`, and `unavailable` have no payload. Event order, exact bytes, exact strings, and exact integer nanosecond timing constants are literal inputs. Zero events is represented by an explicit empty array.

The four byte fields are always present; availability is encoded by their value, not by omission. The cross-rules are exact: `driver.derived.external.result_bytes` is null if and only if v1 `result.available` is false; `driver.derived.external.server_error_bytes` is null if and only if v1 `error.available` is false, irrespective of `error.present`; and both `driver.derived.external.notifications_bytes` and `driver.derived.external.responses_bytes` are null if and only if v1 `external.available` is false. When the governing availability is true, the field must be a string: it is `""` if and only if the corresponding source byte sequence is empty, otherwise its lowercase hexadecimal encoding. A null value under true availability, a string under false availability, or disagreement between either external field's nullability and `external.available` is rejected.

The allowed generic event kinds are exactly `response-frame`, `transport-error`, `cancel`, `deadline`, `shutdown`, and `unavailable`. The driver contains no `result.form`, token/form label, body or payload offset/length/digest, request-body or request-payload semantic field, response-result semantic field, direction, symbol/item/range/call-site/caller/callee/provider meaning, target meaning, semantic world name, or dispatch key derived from such meaning. Complete response-frame bytes are transport input only; the driver does not expose a parsed body/result subspan.

## Corpus v2 derivation contract

V2 is a mechanical projection and causal-input extraction from committed v1 bytes, never a recording or update. The accepted self-contained v1 corpus bytes are pinned at the actual corpus-containing commit `ea049c2dad9be74e1b3e316fe042430be7f04c8e`. The later accepted commit `12d9af179c49412c8203582a3fa52d7836da18b7` contains the identical corpus subtree, but `ea049c2dad9be74e1b3e316fe042430be7f04c8e` is the normative corpus-byte pin. The accepted opacity-corrected v1 RED fixture is separately pinned at `751b0276c46dbccc4899cfec32adf513c123b57f`:

- corpus subtree `sessionruntime/testdata/callhierarchy-layer1-red-v1` at `ea049c2dad9be74e1b3e316fe042430be7f04c8e`: Git tree `7d278859a92b2f4438cce3c1f80f1591cd9a8c1a`;
- the same corpus subtree at `12d9af179c49412c8203582a3fa52d7836da18b7`: the identical Git tree `7d278859a92b2f4438cce3c1f80f1591cd9a8c1a`;
- `sessionruntime/testdata/callhierarchy-layer1-red-v1/manifest.json` at the normative corpus commit: Git blob `80ebe165f3b92a9678a0dc6d891c46649e24885e`, exact file SHA-256 `c761601bf78195634163b98676e133bac2dff610c521be149b3445c9f620492a`, pinning all 25 world paths, lengths, per-world SHA-256 values, aggregate byte length `431974`, and aggregate SHA-256 `7fddcd923a9299c52014334d5460cb1a5ce78604e4a732faaef11d02adc30e84`;
- `sessionruntime/testdata/callhierarchy-layer1-red-v1/worlds` at the normative corpus commit: Git tree `2d11c2a58332d02c22ef27344a2f65a6c0165ca4`; its `world-01.json`, `world-24.json`, and `world-25.json` boundary blobs are respectively `4c7bcfa1dc82401b65c58988102f75d97cefd971`, `e088a59c7e4203ab4db29102c8b2aa6129adc30c`, and `e0c12dfc2daa0390f6ef9b76960a750fb0fac6dc`, and every world is accepted only under the exact per-world length/hash entry in the manifest; and
- `sessionruntime/b4_owned_method_pair_lease_test.go` at accepted opacity-corrected RED commit `751b0276c46dbccc4899cfec32adf513c123b57f`: Git blob `daea113100184745998cde6961d3164c0a3431ce`, exact file SHA-256 `e766074f72b2ff7d43a59d9532b52b3d5c150e4d30f3b61f431c9fab6a540dc2`, as the independently fixed v1 fake-transport fixture and failure-control source.

The manifest's internal `source_head` field remains exactly `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9` as immutable historical metadata describing the source state recorded by v1. It is not a corpus-containing commit, corpus tree pin, checkout source, or derivation authority. Driver projection derives from the committed v1 corpus bytes at `ea049c2dad9be74e1b3e316fe042430be7f04c8e` plus the accepted opacity-corrected RED fixture at `751b0276c46dbccc4899cfec32adf513c123b57f`, never from the `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9` tree.

These exact commands verify the pins without consulting a working-tree copy:

```sh
git rev-parse ea049c2dad9be74e1b3e316fe042430be7f04c8e 12d9af179c49412c8203582a3fa52d7836da18b7 751b0276c46dbccc4899cfec32adf513c123b57f
git ls-tree ea049c2dad9be74e1b3e316fe042430be7f04c8e sessionruntime/testdata/callhierarchy-layer1-red-v1
git ls-tree 12d9af179c49412c8203582a3fa52d7836da18b7 sessionruntime/testdata/callhierarchy-layer1-red-v1
git ls-tree ea049c2dad9be74e1b3e316fe042430be7f04c8e:sessionruntime/testdata/callhierarchy-layer1-red-v1 manifest.json worlds
git ls-tree ea049c2dad9be74e1b3e316fe042430be7f04c8e:sessionruntime/testdata/callhierarchy-layer1-red-v1/worlds world-01.json world-24.json world-25.json
git ls-tree 751b0276c46dbccc4899cfec32adf513c123b57f sessionruntime/b4_owned_method_pair_lease_test.go
git show ea049c2dad9be74e1b3e316fe042430be7f04c8e:sessionruntime/testdata/callhierarchy-layer1-red-v1/manifest.json | shasum -a 256
git show 751b0276c46dbccc4899cfec32adf513c123b57f:sessionruntime/b4_owned_method_pair_lease_test.go | shasum -a 256
git show ea049c2dad9be74e1b3e316fe042430be7f04c8e:sessionruntime/testdata/callhierarchy-layer1-red-v1/manifest.json | python3 -c 'import json,sys; assert json.load(sys.stdin)["source_head"] == "4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9"'
test -z "$(git ls-tree 4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9 sessionruntime/testdata/callhierarchy-layer1-red-v1)"
```

Derivation is ordered and mandatory:

1. Verify the pinned corpus commit/tree/blob identities, manifest file hash and historical internal `source_head`, all 25 listed world files, per-file lengths/hashes, exact path policy, aggregate length/hash, the identical corpus subtree at `12d9af179c49412c8203582a3fa52d7836da18b7`, absence of the corpus path at historical `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9`, and the pinned opacity-corrected v1 RED commit/blob/file hash before deriving anything.
2. Parse each v1 world strictly with duplicate- and unknown-key rejection.
3. Recursively visit every complete v1 record occurrence, including the 16 complete workers in world 24 and the 22 complete executions plus every inline disabled/enabled record in the 11 world-25 pairs.
4. Before deleting any expectation path, derive each occurrence's exact positive `driver.request.request_id` under the availability-aware cross-rule. When the retained expected control/attachment observation is available, derive it mechanically from the numeric value of v1 `expected.identity.request_identity.request_key`. For a W25 disabled execution whose retained expected control/attachment observation is unavailable, do not consult that retained expected scalar; derive the ID from the equal top-level JSON-RPC IDs in its exact fixed request and response frames and require equality with the enabled member's request ID and external execution ID in the same pair. Derive `driver.request.params_state` and exact `params_hex` mechanically by copying the complete opaque `json.RawMessage` `Params` bytes supplied by the accepted v1 RED fixture's request input construction at `751b0276c46dbccc4899cfec32adf513c123b57f`, or the exact generic request `Params` preimage fixed there. Preserve the v1 nil versus available-empty versus available-nonempty distinction explicitly, including every wrapper byte in present values; do not select a request body/payload semantic subspan and do not reconstruct params from an expected or complete request frame, production execution, or implementation output. Each root and each W24/W25 inline envelope has its own explicit `request_id`; paired equality does not permit inheritance from a root, pair, worker, execution, world name, or side table.
5. Before projecting `expected`, derive the ordered `driver.transport.events`, the four exact `driver.derived.external.*_bytes` fields under their pinned encoding and availability cross-rules, and exact request ID/method/session/generation/limits/timing constants from those committed v1 corpus bytes, the independently fixed opacity-corrected v1 fake-transport inputs at `751b0276c46dbccc4899cfec32adf513c123b57f`, exact v1 complete request and response frames, and fixed v1 cancellation/deadline/shutdown/unavailable controls. Using the accepted generic wire encoder, validate during corpus derivation that the fixed exact `Params` bytes together with the fixed method and request ID produce the unchanged complete expected request frame. This validation does not make the frame an input to Params derivation. Strict derivation validation always requires `driver.request.request_id` to equal the exact top-level ID embedded in both fixed driver frames. When the retained expected control/attachment observation is available it must also equal the retained expected generic request identity. For every W25 disabled/enabled pair, both members' fixed external request frames, fixed external response frames, and explicit driver request IDs must be equal; the enabled member's external execution ID must equal that common ID; and the disabled member must retain `control.enabled=false`, `control.attachment_available=false`, and `control.observation_available=false`. Its retained expected request-identity scalar is preserved only as inert control metadata and is not a derivation or validation input. Do not derive any event, byte, class, message, order, timing value, or request ID from production execution or observed implementation output. This derivation is mechanical and fully specified by the pinned v1 worlds plus pinned v1 RED fixture; no uncommitted mapping or hidden table is allowed.
6. Form `expected` by deleting only the 17 inventory paths marked `REMOVE/DEFER` from each complete v1 record. Preserve every retained key, scalar, null, array order, object occurrence, and value byte-for-byte at the decoded-value level. No expected value may come from a runtime, implementation, recorder, updater, or test execution.
7. Strict-validate `driver` against the finite causal driver schema and separately validate `expected` against exactly the unchanged 145-path expected schema. Driver validation must enforce a positive `uint64` `driver.request.request_id` and equality with the exact top-level ID embedded in both fixed driver frames. When the retained expected control/attachment observation is available, it must also enforce equality with retained `expected.identity.request_identity.request_key`. For a W25 disabled execution with unavailable control/attachment observation, it must instead enforce the disabled availability scalars as false, must not consult the retained expected request-identity scalar, and must enforce equality of the pair's fixed external request frames, fixed external response frames, explicit driver request IDs, and enabled external execution ID. It must also enforce all four required external-byte paths, their JSON null/string availability cross-rules, lowercase prefix-free even-length hexadecimal syntax, and decoded-byte equality with the pinned v1 sources. It rejects zero, negative, fractional, out-of-range, mismatched, or omitted causal request IDs; uppercase, odd-length or nonhexadecimal strings; base64; arrays; omitted fields; and extra representations. Driver paths are not traversed by expected-path validation or expectation accounting.
8. Serialize each whole `{driver, expected}` envelope as canonical JSON UTF-8: every object at every depth uses ascending lexical key order by the JSON member-name strings, arrays retain their specified order, separators are compact, no insignificant whitespace is present, and exactly one terminal LF follows each complete file. Consequently the three `driver` keys are ordered `derived`, `request`, `transport`; the sole `driver.derived` key is `external`; and its four keys are ordered `notifications_bytes`, `responses_bytes`, `result_bytes`, `server_error_bytes`.
9. Author a new v2 manifest with schema `adr0011.callhierarchy.layer1.red-oracle-corpus/v2`, exact lexical order `worlds/world-01.json` through `worlds/world-25.json`, secure relative slash-separated paths, per-file byte lengths and SHA-256 values over each complete canonical envelope file, and aggregate SHA-256 over the ordered exact complete envelope bytes. The manifest therefore hashes both causal driver and oracle; it does not hash `expected` alone. Rematerialization for this correction is limited to corrected driver fields and the lengths/hashes mechanically changed by those driver bytes; all `expected` content and its unchanged 145-path denominator remain byte-for-byte fixed at the decoded-value level.
10. Reject absolute paths; empty, `.` or `..` segments; normalization changes; symlink escape; duplicate paths/names; name/path mismatch; missing listed files; and extra files under `worlds/`.

Self-contained cardinalities remain exactly 25 root envelopes, 16 complete inline worker envelopes in world 24, 22 complete inline execution envelopes in world 25, and 11 ordered pairs whose disabled and enabled inline records are each complete envelopes with their own drivers. New canonical per-file and aggregate lengths/hashes are **to be authored later from this derivation and independently reviewed**; this amendment does not predict them.

The successor RED consumes only committed v2 envelope bytes and their manifest. Its only permitted request-ID setup is test-only: for each envelope create a fresh `lspwire.NewPending(0)`; advance it to `driver.request.request_id - 1` solely through matched `Begin(generation)`/`Accept` operations; then, while holding `Manager.mu`, verify the target runtime session and generation are idle and not protocol-owned before assigning that allocator to `runtimeSession.pending`. Release `Manager.mu` before calling `Manager.RoundTrip`. The setup must leave no unmatched pending operation and may not rewrite placeholders, parse request or response frames, consult any expected subtree to choose execution behavior, or dispatch on a world/root/record name. It authorizes no production change.

There is no record mode, update mode, golden rewrite, runtime capture, implementation fallback, ambient resolver, semantic-name dispatcher, hidden driver table, world dispatch, placeholder rewriting, frame parsing at execution, expected-subtree consultation for execution, unmatched pending state, or hash-acceptance switch.

## Review gate

Status remains `REVIEW_GATE_PENDING`. A later independent review must verify:

- v1 corpus/test/commits remain byte-for-byte unchanged and reachable;
- `ea049c2dad9be74e1b3e316fe042430be7f04c8e` resolves to corpus tree `7d278859a92b2f4438cce3c1f80f1591cd9a8c1a`, `12d9af179c49412c8203582a3fa52d7836da18b7` resolves to that identical corpus tree, and historical `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9` does not contain the corpus path;
- the manifest retains internal `source_head` `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9` only as historical metadata, while all corpus-byte projection uses the accepted corpus-containing commit and all fake-transport/control derivation uses opacity-corrected RED commit `751b0276c46dbccc4899cfec32adf513c123b57f`, blob `daea113100184745998cde6961d3164c0a3431ce`;
- the 162-row inventory has no duplicate, omission, or extra path and reconciles `133 + 12 + 17 = 162`;
- the successor expected denominator is exactly 145 and excludes every driver path;
- every root and W24/W25 inline record is a self-contained `{driver, expected}` envelope with its own exact positive `uint64` `driver.request.request_id`, with no ID or driver inheritance and no hidden table;
- each request ID was mechanically derived before projection and is excluded from the unchanged 145 denominator: where retained expected control/attachment observation is available it equals the retained available request identity and both fixed driver-frame top-level IDs; for each W25 disabled execution with unavailable observation, the retained expected identity scalar is inert and unconsulted, the ID derives from its equal fixed driver-frame top-level IDs, and the disabled/enabled pair has equal fixed external request/response frames and request IDs equal to the enabled external execution ID, with disabled control/attachment/observation availability strictly false;
- driver params state/bytes are the exact opaque `json.RawMessage` `Params` bytes mechanically copied from the accepted v1 RED fixture's request construction at `751b0276c46dbccc4899cfec32adf513c123b57f` (or its exact generic request `Params` preimage fixed there), preserve nil/empty/present and all wrapper bytes, are not a body/payload semantic subspan, and are not reconstructed from expected frames or implementation/runtime output; corpus derivation confirms with the accepted generic wire encoder that fixed Params plus method/ID reproduce the unchanged complete expected request frame, while execution only hex-decodes and directly passes the bytes without parse/wrap/rewrite; transport scripts were derived only from the pinned v1 fake-transport fixture, exact complete response frames, and fixed failure controls, and all four exact `driver.derived.external.*_bytes` paths use the pinned null/empty/lowercase-hex encoding and availability cross-rules and decode byte-for-byte to their committed v1 sources;
- canonical serialization applies ascending lexical key order recursively, including exact `driver` and `driver.derived.external` key order, while retaining array order;
- projection deletes only the 17 named expected paths and preserves all retained decoded expected values;
- the manifest hashes each whole envelope, `expected` separately strict-validates at 145 paths, and `driver` separately strict-validates against only the finite transport schema;
- all 25/16/22/11 cardinalities and path/canonicalization rules hold;
- successor dispatch depends solely on driver event kind/bytes/class/message/timing, takes opaque Params and the explicit request ID directly from driver, and never parses frames or consults world names or expected values, including the inert retained identity scalar of a W25 disabled execution;
- request-ID setup uses a fresh `lspwire.NewPending(0)`, matched `Begin(generation)`/`Accept` advancement through `request_id - 1`, and assignment to an idle, non-protocol-owned runtime session only while holding `Manager.mu`, which is released before `RoundTrip`;
- the setup leaves no unmatched pending state and performs no production change, placeholder rewriting, frame parsing, expected-subtree consultation, or world dispatch;
- the finite schemas and RED flow do not regain forbidden rich semantics, replacement expected fields, or hidden tables;
- no expected or driver value originates in implementation/runtime output;
- enabled/disabled transparency and owner/lease lifecycle requirements are causal;
- corpus rematerialization changes only corrected driver fields and the consequent canonical lengths/hashes, never `expected` values or the unchanged 145-path denominator, and exact new canonical hashes are independently reproduced before corpus or RED authority is granted.

No corpus, RED, implementation, source, generated artifact, activation, or commit may proceed before an explicit accepted review disposition.

Suggested exact user gate token:

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_RED_V2_AMENDMENT_REVIEW_GO
```
