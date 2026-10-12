# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Semantic-Residue Correction Amendment

Status: **PENDING**

## Authority and immutable boundary

This additive qualification amendment is authored against exact repository `/Users/schwa/dev/lsp-trace-adr0011-c17` at starting HEAD `fd271defe54ac474d0992a8047354322c314d7f1`.

It corrects only three finite semantic-residue defects in the accepted v3 generic corpus derivation and assertion ownership:

1. inherited v2 commitment-validity/refusal semantics in roots World 17 through World 22;
2. fabricated detailed JSON-RPC server-error payloads in no-response-frame roots World 08 through World 11; and
3. production-A classification of World-24 root cross-worker aggregate ledger occurrences.

The accepted self-contained v3 `driver` inputs and ordered transport events are the sole corrected causal operands. Prototype output, implementation output, current RED output, current v3 `expected` values, and v1/v2 expected values are never corrected operands. World names in this document locate frozen records offline only; they are not runtime inputs, semantic selectors, or dispatch keys.

This amendment grants no corpus mutation, RED mutation, test edit, source edit, generated-artifact edit, implementation, C15 assessment, activation, commit, or production authority. V1 and v2 are immutable. The accepted v3 commitment-input generation, driver-to-expected commitment equality, raw-response custody correction, exact 145-path schema, and exact 17 absent/deferred paths remain unchanged.

## Generic authority rule

V3 commitment material is opaque synthetic causal input. Its `available`, `identity`, `length`, `sha256`, and `bytes_hex` fields establish only finite byte custody and equality. They define no validity predicate, admission policy, refusal policy, error policy, terminal policy, result policy, external-execution policy, or ledger policy.

Accordingly:

- production and the RED MUST NOT interpret unavailable, empty, zero, or any other commitment metadata as invalid;
- no refusal, control, result, error, external, terminal, lifecycle, or ledger value may be selected by a commitment field or by an inherited v2 scenario label;
- request construction still attaches the exact accepted opaque commitment inputs and production observes only the resulting request attachment;
- actual outcomes come solely from executing the accepted request and ordered transport script through the generic runtime contract; and
- `name`, world number, occurrence ID, pair label, C metadata, and `expected` remain forbidden execution selectors.

## Correction 1 — roots World 17 through World 22

### Finite cause

Each of the six accepted drivers contains a normal request and one successfully emitted `response-frame` carrying a successful JSON-RPC result `[]`. The v3 opaque commitments carry no policy that can refuse those executions. The inherited labels `selected unavailable`, `handoff unavailable`, `selected identity empty`, `handoff identity empty`, `selected digest zero`, and `handoff digest zero` have no generic semantic authority.

### Correct derivation

For each of the six occurrences, execute the accepted driver without consulting its world name or current expected subtree. From the actual execution derive:

- `refusal.refused = actual generic admission/refusal result`; because no accepted driver fact requests or causes refusal here, the corrected expected value is `false`;
- `refusal.reason = actual refusal reason`; because refusal is false, the corrected expected value is `null`;
- `derived.control.actual_attachment_available = actual RoundTripResult has a non-nil generic owner attachment through the fixed private seam`;
- `derived.control.actual_reason = ""` iff actual owner/acquire/view access succeeds, otherwise the exact observed prerequisite failure reason;
- `result.available` and `result.bytes_hex` from actual raw result availability/bytes;
- every `error.*` member from the actual generic server-error observation, without synthesizing nullable members;
- `external.available`, `external.failure`, `external.notifications_hex`, and `external.responses_hex` from actual external execution;
- `terminal.available` and `terminal.value` from the actual generic terminal disposition;
- every `round_trip_ledger.dimensions.*` member from actual before/after snapshots and actual observed deltas; and
- `round_trip_ledger.terminal.before.*` and `.after.*` from actual terminal snapshots.

The accepted transport contract yields a successful empty result for each occurrence. Therefore the finite corrected scalar values are:

```text
refusal.refused                              = false
refusal.reason                               = null
terminal.available                           = true
terminal.value                               = "successful-empty"
round_trip_ledger.terminal.after.available   = true
round_trip_ledger.terminal.after.value       = "successful-empty"
```

`round_trip_ledger.terminal.before.available=false` and `.value=null` remain exact. The successful response/result/error/external values and dimension ledger values currently present are not inherited as operands: they must be rematerialized from the accepted driver and formulas and independently shown to reproduce their values.

### Exact affected path sets

The paths whose scalar values change in each of the six roots are exactly:

```text
refusal.reason
refusal.refused
terminal.value
round_trip_ledger.terminal.after.value
```

The following causally adjacent paths are mandatory rematerialization/check operands even when their current scalar values reproduce unchanged:

```text
derived.control.actual_attachment_available
derived.control.actual_reason
result.available
result.bytes_hex
error.available
error.present
error.class
error.code
error.message
error.data_base64
error.data_form
error.serialized_hex
external.available
external.failure
external.notifications_hex
external.responses_hex
terminal.available
round_trip_ledger.terminal.before.available
round_trip_ledger.terminal.before.value
round_trip_ledger.terminal.after.available
round_trip_ledger.dimensions.callbacks.{before,after,delta,available,known_zero}
round_trip_ledger.dimensions.diagnostics.{before,after,delta,available,known_zero}
round_trip_ledger.dimensions.external_bytes.{before,after,delta,available,known_zero}
round_trip_ledger.dimensions.leases.{before,after,delta,available,known_zero}
round_trip_ledger.dimensions.reservations.{before,after,delta,available,known_zero}
round_trip_ledger.dimensions.transport_bytes.{before,after,delta,available,known_zero}
round_trip_ledger.dimensions.transport_messages.{before,after,delta,available,known_zero}
```

The five `control.*` C paths remain structural fixture metadata only. They are never runtime policy and are not copied into production. The two `derived.control.*` B paths are computed from actual execution only. Any inherited `derived.lifecycle.*` assertion must likewise be computed from actual acquisition/view/callback/release events under the existing B formulas; opaque commitments cannot select it.

## Correction 2 — no-response-frame roots World 08 through World 11

### Available facts and forbidden fabrication

The four accepted drivers provide the existing request inputs and exactly one generic control event respectively:

| Offline locator | Event fact |
|---|---|
| World 08 root | `cancel` at offset `0` |
| World 09 root | `deadline` at offset `30000000`, matching the request deadline |
| World 10 root | `shutdown` at offset `0` |
| World 11 root | `unavailable` at offset `0` |

Those event kinds establish only generic cancellation, deadline, shutdown, or transport-unavailable execution facts. No event supplies a JSON-RPC response frame or a server-provided JSON-RPC error object. Therefore code, message, data, data form, and serialized server-error bytes are unavailable and MUST NOT be fabricated from inherited v2 payloads.

For each occurrence the exact error values are:

```text
error.available       = false
error.present         = false
error.class           = <generic runtime class caused by the event>
error.code            = null
error.message         = null
error.data_base64     = null
error.data_form       = "unavailable"
error.serialized_hex  = null
```

where `error.class` is respectively `cancellation`, `deadline`, `shutdown`, and `transport-unavailable`. `error.class` identifies the generic runtime failure class; it does not claim receipt of a JSON-RPC server-error payload.

The exact terminal/failure facts derivable from the driver/runtime contract are:

| Event kind | `external.failure` | `terminal.value` | `round_trip_ledger.terminal.after.value` |
|---|---|---|---|
| `cancel` | `"cancellation"` | `"failed-cancellation"` | `"failed-cancellation"` |
| `deadline` | `"deadline"` | `"failed-deadline"` | `"failed-deadline"` |
| `shutdown` | `"shutdown"` | `"failed-shutdown"` | `"failed-shutdown"` |
| `unavailable` | `"transport-unavailable"` | `"failed-transport-unavailable"` | `"failed-transport-unavailable"` |

For all four, actual attempted execution establishes `external.available=true`, `terminal.available=true`, and `round_trip_ledger.terminal.after.available=true`; the pre-execution terminal remains unavailable/null. No response frame and no result are produced, so the accepted raw-response correction remains authoritative:

```text
frames.response.available       = false
frames.response.frame_hex       = null
frames.response.frame_length    = 0
frames.response.frame_sha256    = 64 zeroes
result.available                = false
result.bytes_hex                = null
external.responses_hex          = ""
derived.external.responses_bytes = ""
transport.bytes                 = available-known-zero value 0
transport.messages              = available-known-zero value 0
```

`derived.external.server_error_bytes` is `null` because `error.available=false`; it is not the hex encoding of a fabricated object and not the serialization of the event class.

### Exact affected paths

The detailed-error correction changes expected values at exactly these seven A paths in each of the four roots:

```text
error.available
error.present
error.code
error.data_base64
error.data_form
error.message
error.serialized_hex
```

`error.class` is rederived from the event/runtime class and its scalar value is unchanged; it is not inherited from a payload.

The correction also changes this B path in each root:

```text
derived.external.server_error_bytes
```

Thus the exact changing occurrence set in each root is `7 A + 1 B = 8`, and the exact complement is the other 137 occurrences. The response-frame, result, external-failure, terminal, and ledger paths listed above are causal neighbors that must be rederived and proven unchanged except where the accepted raw-response custody amendment already corrects them. No detailed error member may be used to derive response custody, response accounting, terminal, or failure.

## Correction 3 — World-24 root occurrence-sensitive aggregate ownership

### Exact cross-worker occurrences

At the World-24 root only, the following 35 expected occurrences are cross-worker aggregates/comparisons, not production owner/view fields:

```text
round_trip_ledger.dimensions.callbacks.before
round_trip_ledger.dimensions.callbacks.after
round_trip_ledger.dimensions.callbacks.delta
round_trip_ledger.dimensions.callbacks.available
round_trip_ledger.dimensions.callbacks.known_zero
round_trip_ledger.dimensions.diagnostics.before
round_trip_ledger.dimensions.diagnostics.after
round_trip_ledger.dimensions.diagnostics.delta
round_trip_ledger.dimensions.diagnostics.available
round_trip_ledger.dimensions.diagnostics.known_zero
round_trip_ledger.dimensions.external_bytes.before
round_trip_ledger.dimensions.external_bytes.after
round_trip_ledger.dimensions.external_bytes.delta
round_trip_ledger.dimensions.external_bytes.available
round_trip_ledger.dimensions.external_bytes.known_zero
round_trip_ledger.dimensions.leases.before
round_trip_ledger.dimensions.leases.after
round_trip_ledger.dimensions.leases.delta
round_trip_ledger.dimensions.leases.available
round_trip_ledger.dimensions.leases.known_zero
round_trip_ledger.dimensions.reservations.before
round_trip_ledger.dimensions.reservations.after
round_trip_ledger.dimensions.reservations.delta
round_trip_ledger.dimensions.reservations.available
round_trip_ledger.dimensions.reservations.known_zero
round_trip_ledger.dimensions.transport_bytes.before
round_trip_ledger.dimensions.transport_bytes.after
round_trip_ledger.dimensions.transport_bytes.delta
round_trip_ledger.dimensions.transport_bytes.available
round_trip_ledger.dimensions.transport_bytes.known_zero
round_trip_ledger.dimensions.transport_messages.before
round_trip_ledger.dimensions.transport_messages.after
round_trip_ledger.dimensions.transport_messages.delta
round_trip_ledger.dimensions.transport_messages.available
round_trip_ledger.dimensions.transport_messages.known_zero
```

These 35 occurrences are category **B**, computed by the RED from the 16 independently executed workers' actual A ledger fields. They MUST NOT be stored in, projected by, requested from, or compared against a production root aggregate view.

All other World-24 root A occurrences compare only the root execution's own actual production fields. In particular, root `accounting.*`, `transport.*`, frames, identity, commitments, refusal, result, error, external, terminal, and `round_trip_ledger.terminal.*` are root-local and remain A. `concurrency.available` and the `concurrency.workers` container remain C fixture structure; each worker envelope retains its ordinary occurrence-local `116/20/9` ownership.

### Exact formulas over 16 worker actuals

Let workers be `w = 1..16`, dimensions be

```text
D = {callbacks, diagnostics, external_bytes, leases,
     reservations, transport_bytes, transport_messages}
```

and let `W[w,d,m]` denote worker `w`'s actual production-A ledger member for dimension `d` and member `m`. For every `d`:

```text
root.B.available[d]  = AND(w=1..16, W[w,d,available])
root.B.before[d]     = 0
root.B.delta[d]      = SUM(w=1..16, W[w,d,delta])
                       only when root.B.available[d] is true
root.B.after[d]      = root.B.before[d] + root.B.delta[d]
                       only when root.B.available[d] is true
root.B.known_zero[d] = root.B.available[d] && root.B.delta[d] == 0
```

If any required worker actual is unavailable, the aggregate is unavailable and the RED applies the existing unavailable normalization; it MUST NOT substitute an expected worker value or zero. The current accepted 16 worker actuals yield:

| Dimension | `before` | `delta` | `after` | `available` | `known_zero` |
|---|---:|---:|---:|---|---|
| callbacks | 0 | 0 | 0 | true | true |
| diagnostics | 0 | 0 | 0 | true | true |
| external_bytes | 0 | 608 | 608 | true | false |
| leases | 0 | 0 | 0 | true | true |
| reservations | 0 | 0 | 0 | true | true |
| transport_bytes | 0 | 960 | 960 | true | false |
| transport_messages | 0 | 16 | 16 | true | false |

These values are RED-derived comparison results, not production root state.

### Occurrence-sensitive accounting

The global schema remains exactly 145 paths. Ownership is asserted per occurrence rather than by path name alone:

```text
ordinary root/worker/execution/pair-member occurrence:
  A=116, B=20, C=9, total=145

World-24 root occurrence:
  A=81, B=55, C=9, total=145
```

The World-24 root count is exactly `116 - 35 = 81` A and `20 + 35 = 55` B. No schema path is added, removed, duplicated, skipped, or asserted twice. Each retained path is asserted exactly once in each structural occurrence under that occurrence's category. Category migration is limited to the 35 listed World-24 root occurrences; the same path names remain A in every ordinary occurrence, including all 16 workers.

## Canonical rematerialization rule

A later separately authorized canonical v3 rematerialization MUST:

1. verify the accepted v3 manifest, all 25 canonical world bytes, the accepted causal-input registry pin, and immutable reachable v1/v2 pins;
2. strict-decode all 85 self-contained occurrences with duplicate/unknown-key rejection;
3. preserve every `driver` decoded value byte-for-byte in canonical representation, including all commitment inputs and ordered transport events;
4. preserve exact driver-to-expected equality for all 12 commitment expectation paths, without interpreting commitment metadata;
5. rematerialize roots World 17–22 from actual generic driver execution under Correction 1, never from their names, inherited v2 labels, current expected values, or prototype output;
6. rematerialize roots World 08–11 under both the accepted raw-response correction and Correction 2, with unavailable/null detailed server-error payloads;
7. retain the World-24 expected scalar values but record and enforce the occurrence-specific 35-path B ownership and no-production-storage rule; no expected aggregate may be used as a worker actual operand;
8. prove the exact complement for every affected occurrence: for each World 08–11 root, the exact changing set is `7 A + 1 B = 8` occurrences and the other 137 occurrences are unchanged after causal rederivation or structural preservation as applicable; every retained expected occurrence outside the other exact correction sets is likewise unchanged;
9. preserve the exact 145-path schema, exact 17 absences, all array order, and all unaffected world bytes;
10. serialize changed worlds as canonical UTF-8 JSON with recursively ascending lexical object-member order, retained array order, shortest decimal integers, minimal required escaping, no insignificant whitespace, and exactly one terminal LF;
11. recompute every changed complete-world byte length and SHA-256, rebuild the manifest per-world entries and lexical aggregate length/digest, and independently reproduce all changed bytes and hashes before acceptance; and
12. never mutate v1 or v2.

Only `worlds/world-08.json` through `worlds/world-11.json` and `worlds/world-17.json` through `worlds/world-22.json`, plus their causally changed manifest entries and aggregate fields, are eligible for scalar rematerialization under this amendment. World 24 requires ownership/assertion correction but no scalar expected-value rewrite unless independent actual recomputation disproves a listed formula result.

## RED update rules

A later separately authorized RED MUST:

1. perform all 85 real executions from each self-contained accepted driver;
2. construct production requests only from driver inputs, preserving commitment bytes while assigning them no semantic policy;
3. compare each ordinary occurrence's 116 A fields to its own actual production owner/view fields;
4. compare only the World-24 root's 81 root-local A occurrences to its own actual production view and require that no production API or storage supplies the 35 aggregate occurrences;
5. compute the World-24 root's 35 aggregate B occurrences solely from the 16 workers' independently executed actual A ledgers using the exact formulas above;
6. compute the ordinary 20 B paths, including control, lifecycle, external, and identity assertions, solely from direct actual execution;
7. validate all nine C paths structurally only;
8. enforce per-occurrence accounting of `116/20/9` except exact World-24 root accounting `81/55/9`, with union 145 and every path asserted exactly once;
9. preserve the accepted raw-response rules: control events emit no response frame, available empty external response sequences are `""`, and errors are never response frames;
10. preserve all causal negatives, defensive-copy, lifecycle, panic-containment, drain, release-winner, external-transparency, canonicality, and no-world-dispatch checks; and
11. forbid expected-fed actuals, expected self-comparison, fabricated observations, prototype/current-output oracle operands, skip lists, wildcard exemptions, fallback, record/update mode, and golden rewrite.

A missing A field is implementation RED. A failed direct or aggregate B formula is assertion RED. Invalid C structure is fixture RED. A production World-24 aggregate field is an ownership RED even if its scalar value equals the independently computed B value.

## Preserved commitments and exclusions

This amendment preserves without qualification:

- exact v3 driver-to-expected equality for all commitment metadata paths;
- the accepted raw-response custody correction for roots World 07–11;
- the exact 17 absent/deferred paths, with no alias restoration;
- immutable v1 and v2 bytes, schemas, manifests, tests, and dispositions;
- exact 85-occurrence traversal and all existing root/worker/execution/pair cardinalities;
- generic authority only, with no call-hierarchy semantic interpretation of opaque material; and
- no corpus, RED, source, test, generated-artifact, implementation, activation, or production authority from this pending document.

## Pending review gate

An independent review must verify the six-occurrence commitment-policy correction, the four-occurrence unavailable detailed-error correction with exact per-root changing set `7 A + 1 B = 8` and unchanged 137-occurrence complement, the exact changed and mandatory-rematerialization paths, the 35 World-24 root occurrence-specific B classifications, formulas over 16 worker actuals, strict no-production-storage rule, `81/55/9` versus `116/20/9` occurrence accounting, global 145-path schema, exact 17 absences, commitment equality, accepted raw-response correction, canonical rematerialization, direct-actual-only RED rules, and immutable v1/v2 boundaries.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_SEMANTIC_RESIDUE_CORRECTION_AMENDMENT_REVIEW_GO — NOT ISSUED
STATUS — PENDING
```
