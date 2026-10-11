# ADR-0011 Call-Hierarchy Layer-1 Generic V2 Oracle/Assertion Ownership Amendment

Status: **REVIEW_GATE_PENDING**

## Authority and immutable boundary

This amendment is authored against exact repository `/Users/schwa/dev/lsp-trace-adr0011-c17` at starting HEAD `8ff8bdf6`. It changes ownership and RED assertion rules only. It grants no corpus, test, source, generated-artifact, implementation, C15, activation, commit, or production-writer authority.

The accepted corpus/amendment/RED remain immutable. The v2 expected schema remains exactly 145 paths. The 17 paths already removed/deferred remain absent. No retained expectation is weakened, deleted, skipped, renamed, or made optional.

This amendment partitions every retained path exactly once:

```text
A production generic owner/view fields                     117
B RED-derived assertions from actual executions             20
C fixture/control-only organization metadata                  8
                                                          -----
exact retained expected paths                              145
removed/deferred paths remain absent                        17
```

`A`, `B`, and `C` are disjoint and their union is the exact 145-path schema. Category placement is semantic, not name-based dispatch authority.

## Ownership rule

- **A — production generic owner/view field:** an actual finite generic observation retained by the private owner and exposed by its view. It must be derivable solely from authorized `Manager` request, response, terminal, accounting, and opaque-commitment inputs. The RED compares each A path field-by-field to the independently fixed corpus expectation.
- **B — RED-derived assertion:** an assertion computed by the RED from actual A fields or actual external/lifecycle executions. It is not stored in the production owner or view. The corpus value is only the expected assertion result; the RED must compute the actual side independently and must never compare an expected value to itself.
- **C — fixture/control-only metadata:** data used solely to name, route, pair, label, or organize test execution and relationships. It is never stored in or compared to the production owner/view. It may constrain schema, cardinality, ordering, and declared fixture relationships only.

## A — exact production owner/view field inventory (117)

### Exact paths

- `accounting.bytes.available`
- `accounting.bytes.known_zero`
- `accounting.bytes.value`
- `accounting.diagnostics.available`
- `accounting.diagnostics.known_zero`
- `accounting.diagnostics.value`
- `accounting.objects.available`
- `accounting.objects.known_zero`
- `accounting.objects.value`
- `accounting.work.available`
- `accounting.work.known_zero`
- `accounting.work.value`
- `commitments.handoff.available`
- `commitments.handoff.identity`
- `commitments.handoff.length`
- `commitments.handoff.sha256`
- `commitments.predecessors[*].available`
- `commitments.predecessors[*].identity`
- `commitments.predecessors[*].length`
- `commitments.predecessors[*].sha256`
- `commitments.selected.available`
- `commitments.selected.identity`
- `commitments.selected.length`
- `commitments.selected.sha256`
- `error.available`
- `error.class`
- `error.code`
- `error.data_base64`
- `error.data_form`
- `error.message`
- `error.present`
- `error.serialized_hex`
- `external.available`
- `external.execution_expected`
- `external.failure`
- `external.notifications_hex`
- `external.responses_hex`
- `frames.auxiliary`
- `frames.request.available`
- `frames.request.frame_hex`
- `frames.request.frame_length`
- `frames.request.frame_sha256`
- `frames.response.available`
- `frames.response.frame_hex`
- `frames.response.frame_length`
- `frames.response.frame_sha256`
- `identity.binding.available`
- `identity.binding.value`
- `identity.generation.available`
- `identity.generation.value`
- `identity.method.available`
- `identity.method.value`
- `identity.profile.available`
- `identity.profile.value`
- `identity.request_identity.generation`
- `identity.request_identity.request_key`
- `identity.request_identity.session`
- `identity.request_identity.workspace_nonce_hex`
- `identity.transaction.available`
- `identity.transaction.local_hex`
- `refusal.reason`
- `refusal.refused`
- `result.available`
- `result.bytes_hex`
- `round_trip_ledger.dimensions.callbacks.after`
- `round_trip_ledger.dimensions.callbacks.available`
- `round_trip_ledger.dimensions.callbacks.before`
- `round_trip_ledger.dimensions.callbacks.delta`
- `round_trip_ledger.dimensions.callbacks.known_zero`
- `round_trip_ledger.dimensions.diagnostics.after`
- `round_trip_ledger.dimensions.diagnostics.available`
- `round_trip_ledger.dimensions.diagnostics.before`
- `round_trip_ledger.dimensions.diagnostics.delta`
- `round_trip_ledger.dimensions.diagnostics.known_zero`
- `round_trip_ledger.dimensions.external_bytes.after`
- `round_trip_ledger.dimensions.external_bytes.available`
- `round_trip_ledger.dimensions.external_bytes.before`
- `round_trip_ledger.dimensions.external_bytes.delta`
- `round_trip_ledger.dimensions.external_bytes.known_zero`
- `round_trip_ledger.dimensions.leases.after`
- `round_trip_ledger.dimensions.leases.available`
- `round_trip_ledger.dimensions.leases.before`
- `round_trip_ledger.dimensions.leases.delta`
- `round_trip_ledger.dimensions.leases.known_zero`
- `round_trip_ledger.dimensions.reservations.after`
- `round_trip_ledger.dimensions.reservations.available`
- `round_trip_ledger.dimensions.reservations.before`
- `round_trip_ledger.dimensions.reservations.delta`
- `round_trip_ledger.dimensions.reservations.known_zero`
- `round_trip_ledger.dimensions.transport_bytes.after`
- `round_trip_ledger.dimensions.transport_bytes.available`
- `round_trip_ledger.dimensions.transport_bytes.before`
- `round_trip_ledger.dimensions.transport_bytes.delta`
- `round_trip_ledger.dimensions.transport_bytes.known_zero`
- `round_trip_ledger.dimensions.transport_messages.after`
- `round_trip_ledger.dimensions.transport_messages.available`
- `round_trip_ledger.dimensions.transport_messages.before`
- `round_trip_ledger.dimensions.transport_messages.delta`
- `round_trip_ledger.dimensions.transport_messages.known_zero`
- `round_trip_ledger.terminal.after.available`
- `round_trip_ledger.terminal.after.value`
- `round_trip_ledger.terminal.before.available`
- `round_trip_ledger.terminal.before.value`
- `terminal.available`
- `terminal.value`
- `transport.bytes.available`
- `transport.bytes.known_zero`
- `transport.bytes.value`
- `transport.messages.available`
- `transport.messages.known_zero`
- `transport.messages.value`
- `transport.request_bytes.available`
- `transport.request_bytes.known_zero`
- `transport.request_bytes.value`
- `transport.request_messages.available`
- `transport.request_messages.known_zero`
- `transport.request_messages.value`

### Finite field mapping and availability

| A family | Authorized actual source | Exact production mapping and availability rule |
|---|---|---|
| `identity.*` | Manager/runtime request identity and transaction allocation | Copy workspace nonce, session, generation, request key, transaction-local identity, method, binding, and profile into their corresponding fields. Every explicit `available` bit governs only its sibling value. Unavailable means `available=false` plus the zero private value; it never means available-empty. |
| `frames.request.*`, `frames.response.*`, `frames.auxiliary` | Complete encoded request and consumed complete response frames | Retain only complete frame bytes, byte length, SHA-256, and availability. `frames.auxiliary` is the actual ordered generic auxiliary-frame collection. No removed body/payload/result subspan is recreated. Unavailable frames have no retained bytes and `available=false`. |
| `result.*` | Actual `RoundTripResult.Result` bytes and availability | Copy raw result bytes defensively and preserve nil/unavailable versus available-empty. `result.form` remains absent. |
| `error.*` | Actual generic server-error observation | Copy availability, presence, class, nullable code/message/data, data form, and serialized bytes exactly. Nullable/unavailable members are not synthesized. |
| `external.*` | Actual external request/response execution observation available to Manager | Retain execution availability/expectation, failure, notification bytes, and response bytes. These are generic effects, not call-hierarchy semantics. |
| `transport.*` | Actual request/response transport accounting | Copy byte/message and request-byte/request-message observations with exact availability and known-zero states. Unavailable is not zero. |
| `accounting.*` | Authorized Manager private accounting snapshot | Copy bytes, diagnostics, objects, and work triples exactly. `known_zero=true` is valid only when available and actually zero. |
| `round_trip_ledger.*` | Authorized Manager before/after transaction ledger snapshots | Copy each actual before, after, delta, availability, known-zero, and terminal before/after field. Production stores observations, not corpus arithmetic. The RED separately checks arithmetic relationships. |
| `terminal.*` | Actual Manager terminal disposition | Copy availability and value. Unavailable is not an empty terminal string. |
| `commitments.*` | Authorized opaque selected/handoff/predecessor commitment metadata supplied to the Manager request | Copy ordered identity/digest/length/availability metadata defensively. No preimage bytes or richer lineage semantics are retained. Each unavailable commitment has zero identity/digest/length and `available=false`. |
| `refusal.*` | Actual private generic admission/refusal result | Copy refused and reason. It is private, generic, and externally transparent; it does not encode fixture control labels. |

No A field may be populated from `expected`, `name`, `comparison.*`, `concurrency.*`, `control.*`, a world number, a scenario label, pair position, or a hidden side table. In particular, the RED must not construct production commitment inputs from `expected.commitments`; authorized request attachment inputs must be independently supplied as causal driver/request inputs before execution.

## B — exact independently derived RED assertions (20)

B paths stay in the strict corpus schema as expected assertion values, but do not exist in the production owner/view.

| Exact B path | Independently computed actual and exact inputs |
|---|---|
| `lifecycle.acquired` | `actual = observation lease acquisition returned true`; input is the actual acquisition call result. |
| `lifecycle.completed` | `actual = callback was admitted and returned or was panic-contained before lifecycle assertion completion`; inputs are actual callback invocation/return containment events. No callback means false. |
| `lifecycle.released` | `actual = first actual lease release succeeded`; input is the first `Release()` return. |
| `lifecycle.sequence[*]` | Append `"acquired"` only after successful actual acquisition, `"completed"` only after actual callback completion/containment, and `"released"` only after the first successful actual release. Compare the independently recorded ordered sequence, including empty versus nonempty and element order. |
| `derived.control.actual_attachment_available` | `actual = RoundTripResult` contains a non-nil generic owner attachment discoverable through the fixed private attachment seam; input is the actual returned result, not `control.attachment_available`. |
| `derived.control.actual_reason` | Empty on successful actual owner/acquire/view access; otherwise the RED’s actual failed prerequisite reason (`field`, `owner:nil`, `acquire:false`, `view:false`, or signature failure) from inspection. It is never copied from `control.reason`. |
| `derived.external.result_bytes` | Defensive byte copy of actual `RoundTripResult.Result`; null iff the actual result is unavailable. |
| `derived.external.server_error_bytes` | Exact actual serialized server-error bytes under the frozen generic serializer; null iff actual error availability is false. Inputs are the actual returned server error and its availability, not expected error bytes. |
| `derived.external.notifications_bytes` | Exact concatenation/canonical retained bytes of actual returned notification messages in actual order under the frozen generic byte rule; null iff actual external observation is unavailable. |
| `derived.external.responses_bytes` | Exact concatenation/canonical retained bytes of actual returned response messages in actual order under the frozen generic byte rule; null iff actual external observation is unavailable. |
| `derived.external.failure` | Actual `RoundTripResult.Failure`, preserving nil versus present string. |
| `derived.external.messages` | Actual `RoundTripResult.Messages`. |
| `derived.external.bytes` | Actual `RoundTripResult.Bytes`. |
| `derived.external.request_messages` | Actual `RoundTripResult.RequestMessages`. |
| `derived.external.request_bytes` | Actual `RoundTripResult.RequestBytes`. |
| `derived.lifecycle.observation_available` | `actual = acquisition succeeded && View() returned true`; inputs are actual acquisition and view results. |
| `derived.lifecycle.callback_completed` | `actual = one admitted callback returned normally or its panic was contained and returned as the frozen callback error`; inputs are actual callback calls/returns. |
| `derived.lifecycle.release_first` | Exact boolean returned by the first actual `Release()` call. |
| `derived.lifecycle.release_second` | Exact boolean returned by the second actual `Release()` call; expected false proves one-winner release. |
| `derived.identity.transaction_request_equals_request_identity` | `actual = (A identity transaction request identity == A identity request_identity)` using the actual production A identity fields only, including availability; no expected identity participates. |

Additional B relationship assertions are required by the corpus but are not additional paths: for each World-25 pair, compute `transport_equal` by deep equality of the two independently executed members’ actual complete request frame, response frame, messages, request messages, bytes, and request bytes; compute `external_equal` by deep equality of their independently captured actual generic external observations. Compare those computed booleans to the fixture relationship declarations only after both executions. World-24 aggregate ledger arithmetic is computed from the 16 independently executed workers’ actual A ledgers: for each dimension, `delta = Σ worker.actual.delta`, `before = 0`, `after = before + delta`, availability is true only when all required actual inputs are available, and `known_zero = available && delta == 0`.

No B assertion may use an expected field as its actual operand. Expected-to-expected, expected-to-fixture, or expected self-comparison is forbidden.

## C — exact fixture/control-only inventory (8)

| Exact C path | Sole permitted use |
|---|---|
| `comparison.available` | Declares whether this fixture root carries comparison organization. It gates fixture schema/cardinality traversal only, never production behavior or a view field. |
| `concurrency.available` | Declares whether this fixture root carries concurrent-worker organization. It gates fixture schema/cardinality traversal only, never production behavior or a view field. |
| `control.attachment_available` | Expected fixture classification used to relate an execution to the B actual-attachment assertion; never copied into production or used to enable attachment creation. |
| `control.enabled` | Labels the enabled/disabled member of a declared test pair. It selects which independently authored causal driver/request attachment is submitted by the test harness; production receives only that request and never this label. |
| `control.observation_available` | Expected fixture classification used to constrain the B observation-availability relationship; never a production field. |
| `control.paired_world` | Pairing key used only to validate the declared one-enabled/one-disabled fixture relationship and ordering. |
| `control.reason` | Fixture explanation for the expected control classification (for example `control-disabled`); never a production refusal or runtime reason. |
| `name` | Human/test subtest label and manifest relationship key only. It must not select request contents, transport events, owner construction, expected values, or production execution. |

`comparison.executions`, `comparison.pairs`, pair `disabled`/`enabled`, pair relationship declarations/reasons, `concurrency.workers`, manifest names/paths, and traversal positions are non-path fixture containers surrounding the 145-path schema. They are likewise C metadata. Their only assertions are strict shape, exact cardinality, ordering, uniqueness, closure, and declared relationship consistency.

### No-world-dispatch proof obligation

The RED must prove the harness and production path have no world dispatch:

1. execution input is built only from the strict envelope’s causal `driver` and independently authorized request attachment inputs;
2. transport behavior dispatches only on ordered `driver.transport.events[*].kind` and that event’s exact bytes/class/message/timing;
3. `name`, `comparison.*`, `concurrency.*`, `control.*`, world index, pair index, and expected values are not passed to `Manager.RoundTrip` and are not consulted by production code;
4. renaming a world or changing only C labels leaves the constructed request, transport script, and actual production execution identical;
5. changing a causal driver value while retaining all C labels changes execution according to that driver; and
6. repository review finds no switch/map/lookup keyed by world name, record name, kind, scenario, enabled label, pair label, or expected semantic value in the production execution path.

`control.enabled` does not dispatch production. The harness materializes an enabled request with an authorized generic attachment or a disabled request without one before calling the same `Manager.RoundTrip`; the Manager responds only to the presence and validity of the request attachment.

## Intended successor RED rule

The corrected RED must perform the following without weakening or deleting any retained expectation:

1. **Strict-load all 145:** strict-decode each complete envelope and every inline worker/execution/pair member; reject duplicate/unknown keys; validate the exact disjoint A/B/C registry with counts `117/20/8`, union 145, no duplicate or unowned path, and all 17 removed/deferred paths absent.
2. **Execute causally:** construct requests solely from strict driver fields and independently authorized generic request attachment inputs. Never source production inputs from `expected` or C metadata.
3. **Assert A field-by-field:** acquire the actual production owner/view and compare exactly all 117 A paths, preserving null, unavailable, available-empty, order, byte identity, integer value, and known-zero distinctions.
4. **Assert B from independently computed actuals:** run actual acquisition/view/callback/release and actual external executions; compute each of the 20 B values by the formulas above; compute World-24 actual aggregate arithmetic and World-25 actual pair equality from independently executed members; compare only the resulting actual values to their fixed expected assertions.
5. **Validate C only structurally:** check C schema, exact root/worker/execution/pair cardinalities (`25/16/22/11`), pairing, one enabled and one disabled member per declared pair, pair-key consistency, ordering, uniqueness, reasons/classifications, and fixture relationship closure. Never project a C path from the production view and never compare C to production/view.
6. **Account exactly:** per complete record require `A compared = 117`, `B computed = 20`, `C structurally validated = 8`, `errors = 0`, total `145`; globally retain the existing 85 actual execution count and all existing corpus/manifest/hash/cardinality checks.
7. **Preserve causal negatives:** retain mutation/defensive-copy, external-transparency, lifecycle ordering, callback panic containment, callback drain, terminal closure, first/second release, and no-world-dispatch checks. No skip list, wildcard exemption, fallback-to-expected, record/update mode, or golden rewrite is permitted.

A missing A field is an implementation RED. A failed B formula is an assertion RED. Invalid C shape or relationship is a fixture RED. None may be converted into a skip or removed from the denominator.

## C15 and implementation assessment boundary

This finite ownership amendment enables a later implementation/C15 assessment to count only the 117 A production fields and their finite owner/view storage/plumbing. The 20 B assertions and 8 C metadata paths require no production field and must not inflate the production view or C15 candidate set. This is an assessment boundary only; it does not authorize implementation, prototype mutation, C15 binding/classification, source changes, or production custody.

## Review gate

A later independent review must verify the exact 145-row partition, formulas, availability rules, no-world-dispatch proof, strict RED accounting, preservation of the corpus and 17 absences, and absence of any expected-sourced production input. Until then:

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V2_ORACLE_ASSERTION_OWNERSHIP_GO — NOT ISSUED
STATUS — REVIEW_GATE_PENDING
```
