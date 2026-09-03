# Stage 2 scenario specification v1

Specification ID: `stage2-scenario-v1`
Ledger ID: `stage2-ledger-v1`
Catalog ID: `stage2-catalog-v1`

The scenario and ledger documents have hermetic JSON Schema Draft 2020-12 contracts under their stable candidate IDs. These IDs are local, unadvertised candidate identities only: they are not runtime MCP artifact identities and are not registered in the accepted Stage 1 manifest.

This is a deterministic reference-evidence format. It does not enable or advertise lifecycle tools, establish containment, exercise an MCP runtime transcript, or satisfy the Stage 2 acceptance gate.

## Canonical document

A scenario is exactly `{"version":"stage2-scenario-v1","steps":[...]}`. Both objects are closed; trailing JSON is forbidden. UTF-8 object keys are inspected in lexical order, so the earliest forbidden-field diagnostic is deterministic. Step index is zero-based. Required-field checks precede bounds and semantic prerequisite checks. Identities are nonempty and at most 256 UTF-8 bytes. `generation` and `ordinal` are positive integers. Manager request IDs are globally unique in one scenario. LSP request IDs are unique per `(session,generation)` and may be reused in another session or generation; child ordinals are contiguous positive declaration-order integers per `(session,generation,manager_request)`.

## Operation grammar

The field sets below are exact. A field not listed is forbidden. `expect` is a closed `ManagerExpectation`; omitted members mean zero values and exact equality is required against the complete normalized return projection.

| op | required | optional | prerequisites/barrier |
|---|---|---|---|
| `startup`, `initialize`, `crash`, `poison` | `session,generation,outcome` | none | positive generation; declared order is the barrier |
| `request` | `session,generation,request` | none | unique manager request |
| `child` | `session,generation,request,lsp_request,child,ordinal` | none | exact existing request tuple; next ordinal |
| `respond`, `late_response` | `session,generation,lsp_request,child` | none | exact owned child tuple; response requires unresolved, late response requires tombstone |
| `timeout`, `cancel` | `session,generation,request` | none | exact active request tuple; terminalizes every unresolved child before subsequent step |
| `cancel_write_failed` | `session,generation,request,lsp_request,child` | none | exact owned child tuple after cancellation was sent; records `WRITE_FAILED` without changing the terminal winner |
| `lifecycle_register` | `session,generation,state,expect` | `reaped` | state in public Manager vocabulary |
| `lifecycle` | `session,generation,operation,caller,expect` | `bind,child_risk,has_work` | operation is `STATUS|STOP|RESTART`; `bind` immutably binds returned nonempty intent ID |
| `lifecycle_complete` | `session,expect` and exactly one of `intent_id|intent_ref` | `death,reaped,ready` | target resolves to an immutable explicit ID; no latest-session lookup |
| `lifecycle_detach` | `session,caller,outcome,expect` and exactly one of `intent_id|intent_ref` | none | outcome is `REQUEST_CANCELLED|REQUEST_TIMEOUT`; explicit target only |
| `admit` | `session,expect` | none | invokes Manager admission once |
| `evict_complete` | `success,expect` and exactly one of `intent_id|intent_ref` | none | reference identifies a declared reservation; invokes Manager completion once |

Closed vocabularies are the operation names above, `NOT_REQUESTED|SENT|WRITE_FAILED` cancellation states, Manager public states, `STATUS|STOP|RESTART`, and detach outcomes above. Boolean false remains distinguishable from absence only where requiredness gives it meaning. Diagnostics occur in this order: document decode/trailing value; version; steps presence; operation recognition; forbidden fields in lexical order; required fields in operation-table order; positivity/bounds/vocabulary; identity uniqueness; exact prerequisite relation.

## Race and ownership model

The interpreter executes one declared step at a time. Declaration order is the only barrier and race order; no sleeps, clock, watcher, network, or map iteration determines output. Lifecycle/session decisions are delegated to one `session.Manager`. Every return is normalized into a closed reference result, tagged `simulated_reference:true`, emitted, and compared exactly with `expect`; mismatch stops at that step.

Ownership is correlated by `(session_id,generation,manager_request_id,lsp_request_id,parent_manager_request_id,ordinal)`. Response and cancel states are orthogonal closed values. Dispatch, response, cancel, terminal, and tombstone references are positive ledger event sequences when present. Before generation restart, each active request must terminalize; its unresolved children receive that terminal event sequence, become tombstones, and are never replayed or rebound to a successor. Manager request IDs remain scenario-global; LSP IDs may be reused only outside the prior `(session,generation)` scope. Tombstones are retained by reference-event age first and then by deterministic oldest-first count bounds; consumed tombstones remain counted as retained history until eviction.

## Canonical JSONL ledger

Every LF-terminated line is one compact JSON object with `ledger_version:"stage2-ledger-v1"`, `record_type`, and exactly one matching payload. Structural schema validation precedes semantic, prerequisite, ordering, and exact-byte validation for scenarios, catalog rows, replacement declarations, and every golden ledger. Record types are: `event`, `mcp_reference_result`, `child_ownership`, `lifecycle_intent_history`, `session_generation_history`, and `final_resource_census`. The MCP record is explicitly a simulated reference result, not a wire/runtime transcript. All public fields are stable `snake_case`; Manager structs are projected field-by-field and never marshaled directly. Record order is interpreter events, Manager reference results at their steps, Manager lifecycle history in Manager order, complete retained session-generation history by stable `(session_id,generation)` key, ownership by deterministic request/ordinal order, then one census. Each generation record retains its final within-session `event_seq`; session-local sequences are never compared across sessions. The census discloses retained/omitted/truncated Manager-ledger metadata; equality with the Manager bound is conservatively marked truncated because the Manager projection cannot expose the exact omitted count.

`IntegratedLedgers.Ownership`, `.Intent`, `.TerminalHistory`, and `.ResourceCensus` are legacy compatibility aliases of one combined ledger. They are byte-identical views for existing callers and do not claim independent ledgers or independent evidence.

The retained generation projection is the single modeled authority for both `session_generation_history` and session census classification. It retains every observed `(session_id,generation)` and identifies the greatest generation as the current generation for each session. Only that current generation contributes to `live_sessions` or `terminal_sessions`: `STOPPED`, `CRASHED`, and `POISONED` are terminal; all other public states, including `READY`, are live. Historical generations contribute only to `generations`, never to terminal-session count.

The census bounds and counts only modeled resources: current live and terminal sessions, every retained generation, waiters, reservations, intents, callers, manager requests, LSP requests, retained/evicted/consumed tombstones, and modeled fake children/pipes/goroutines. Modeled registration, admission, lifecycle completion, and eviction updates occur only after an exact successful Manager observation and expectation match. `os_processes_exercised:false`; no process, pipe, child, goroutine, containment, framing, or cleanup claim is made by pure fixtures.

## Catalog acceptance

The catalog's explicit `acceptance_state` remains `open` whenever any required coverage item is `partial` or `runtime_gate`. A row may claim capacity coverage only when its scenario actually executes `admit` or `evict_complete`; the current catalog makes no capacity claim.

## Runtime gate

`RUNTIME_GATE_PRODUCTION_EQUIVALENT_FAKE_SERVER_PATH` remains open. This owned package cannot add a fake process through the production-equivalent manager/framing/containment/runtime path without modifying excluded layers. The catalog therefore labels such rows `runtime_gate`; reference ledgers do not simulate that proof. Stage 2 acceptance remains open.
