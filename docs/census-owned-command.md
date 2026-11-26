# Ordinary census: bounded consolidated-owner command protocol

`lsp-trace census .` now creates and enumerates a fresh job through the consolidated inventory owner. It returns **operations**, not a completed feature inventory. This is a bounded command integration, not general whole-repository inventory qualification.

## Initial input and capability

```sh
lsp-trace census . --publication-root /absolute/private/output-parent \
  --interpreter interpreter --reviewer reviewer --semantic-window 10m
```

The output parent must already be canonical, private (0700), owned by the caller, and outside the workspace. With no explicit parent, the existing per-user census store is used. New operations always carry their exact canonical `--store` path, so a fresh process needs no environment override. `--publication-root` retains its historical **parent** semantics; `--store` means the **actual durable store** and is not interchangeable. An explicitly empty `--store` operand is invalid, including for status/acquire/seal, and refuses before opening a store or constructing a provider. It never selects environment/default storage. Historical omitted-store selection on legacy routes remains unchanged.

Supported new connected jobs default to distinct declared actor labels `interpreter` / `reviewer`, ten minutes per semantic task, four stages, forty minutes cumulative reserved response windows, and no overall job expiry. Actor labels are not authentication or proof of independence. The default model/worker/grammar pins are an explicitly hashed UNSPECIFIED declaration, not claims that any actual model or executable was authenticated. A positive finite `--semantic-window` may be selected only when creating a new job (implementation validation maximum 24h/task, not a product requirement). Old stored jobs do not inherit defaults or renew expired tasks.

Source roots/includes/excludes/prunes and frozen project file associations/declared eligibility are preserved. Default Go selection requires the existing root Go evidence and rejects ambiguous default roots. The existing single-segment v10 composition can acquire **exactly seventeen eligible members**, split 16+1, and retains only **two query-side R admissions**. It requires one unambiguous Function or Method in the first sorted member of each batch. All members receive document-symbol discovery; this is not seventeen descriptions or features. D/CALLS and other target selections are not performed by this bounded profile. Original references are query-to-reported-location observations, not CALLS, authenticated target-source capture, feature identity, or complete provider coverage.

Unsupported populations/profiles and incomplete enumerations return `UNSUPPORTED_SCOPE_OR_PROFILE` with the full paid counts and unknown/excluded diagnostics. They do not truncate or silently change the selected population. Explicit custom server/config overrides are metadata-only in this path. Legacy depth/node/batch/native-timeout controls cannot replace this frozen profile; use the explicit legacy `--workspace` route for those old contracts. No arbitrary multibatch continuation or physical quota/pacing guarantee is introduced.

## Follow returned operations

Outputs have `Store`, `RunID`, `Head`, `TaskID` when present, `Phase`, `Status`, `Capability`, complete `Enumeration`, and `Operations`. Nested Enumeration fields (including its historical `AcquisitionImplemented: false` / `ACQUISITION_UNAVAILABLE`) describe that metadata-only layer, not the top-level command capability. Its closed denominator is not feature completeness. Arrays are executable argv, not shell scripts. Placeholder names below explain the grammar; actual output supplies every value:

```text
lsp-trace census durable status RUN --store STORE
lsp-trace census durable acquire RUN EXACT_HEAD --store STORE
lsp-trace census durable seal RUN EXACT_HEAD --store STORE
lsp-trace census durable task RUN [EXACT_TASK] --store STORE
lsp-trace census durable read RUN EXACT_TASK input OFFSET 32768 --store STORE
lsp-trace census durable submit RUN EXACT_TASK ROLE --store STORE
lsp-trace census durable result RUN EXACT_TASK --store STORE
```

1. Creation/enumeration returns `BOUNDED_ACQUISITION_READY` only within the profile's metadata eligibility. This is not a capability or provider-installation/behavior proof; `acquire` revalidates before effects. A start diagnostic also supplies the run/store/status command if later work fails.
2. Execute the returned `acquire`. That one process holds the original native owner across funded discovery, original query capture, cleanup, selected preparation and admission. No native handle or original is exported. Native/source deadlines remain at most fifteen seconds; no daemon or second overall controller exists.
3. First success returns a clean `READY` pause, one admission and a new exact-head `acquire`. The process may exit, and the expired first native window does not expire the job. Controlled command tests pause past that window and verify unchanged head/byte/packet state. That returned operation in a fresh process selects **only the remaining member**, with cumulative spending and original first admission preserved.
4. Second success returns two admissions and `READY_TO_SEAL`. Arrange actual interpreters/reviewer **before** executing `seal`; it starts the first semantic lease. Do not reseal or replay an old head.
5. The returned task has current role, actor, immutable `InputSHA256`, finite issuance/deadline/policy, and `Contract` with actual supported operations, response schema, encoding and examples. Read every page: base64-decode `Data`, advance only OFFSET by decoded byte count, stop at `Total`, verify SHA-256 of the exact concatenated bytes. For another page change only the documented offset operand of the returned read argv. Repeated/overlapping reads are separately charged and may refuse; the reservation cursor is not proof of delivery.
6. Submit **exact authored response bytes on stdin, then close stdin**, using the returned role/task/store operation. Describe requires the canonical ordered adapter JSON plus LF defined in the task contract; grouping/review requires its actual input revision and actor. These are semantic response serialization, not evidence conversion. Schema examples are not ready-made judgments.
7. Use the returned successor task; never replay a predecessor. If submit output is lost or ambiguous, use the saved task Contract `Operations.task` or `Operations.current_task`: both observe the current run task, including a successor. Do not repeat Submit. `Operations.exact_task` checks the supplied current TaskID and refuses once that ID becomes a predecessor; it is not a recovery alias. These observation operations do not renew leases or reset accounting. A separate reviewer task is always retained. Published tasks advertise Result and corrected-catalog Read, but no Submit. Result verifies actual publication; it remains PROVISIONAL, Authority 0, Accepted false, completeness UNKNOWN.

Waiting consumes response opportunity but not incremental logical byte/work reservations. Read/Submit remain separately bounded active processing. The cumulative semantic counter reserves whole issued windows, not measured active model time. Task/status do not renew leases. A running command owns its in-memory capability; a persisted claimed head alone cannot prove whether it is still active. `PARTIAL_UNCERTAIN` therefore exposes status-only observation, never a replay action. Native failure/unsupported evidence has no fabricated success. Stale, late, wrong-role/actor, malformed, corrupt/missing and spent work refuse under existing owner rules; use previously returned status/current-task operations to observe, not to reset.

## Scope of verification and remaining acceptance

Controlled command tests use constructor-owned io.Pipe runtime injection **inside the test command process**. The external caller imports no internal package and follows only returned public argv/JSON/stdin, with follow-up commands running from a different working directory. Default-store observation is also tested with a different HOME. The executable test helper embeds those public argv behind a fixed test-only process prefix; this is not a claim that the installed CLI was run against a real provider. Fixture setup is test substrate, not required caller coordination. Successful test responses are mechanical fixtures, not independently authored judgments. They establish neither real gopls behavior nor arbitrary-repository readiness.

The production constructor reuses the existing current gopls profile/managed starter; no provider is started until the funded acquire operation executes. It was source-inspected, not executed against a real provider in this delivery. Retained revision labels are inherited bounded-owner declarations, not a fresh Git-revision attestation. Runtime construction/fingerprint is fresh per batch; frozen file eligibility is not a claim that the installed provider binary/configuration is immutable.

Independent implementation review and parent backup must precede live public-caller activation acceptance. Preserve actual task inputs and attributable independently authored judgments in that later run. Completed prior runs are immutable references, not templates for identity rebinding. Slice 3 physical quotas, pacing and deep quote optimization remain deferred.

Older explicit acquisition/caller-HOST/one-item routes and stored task versions remain compatibility paths. Explicit `LSP_TRACE_CENSUS_CALLER` declarations also select the historical producer rather than being silently ignored; new semantic-policy flags are refused on those legacy lanes. Fresh ordinary callers need no declaration environment; `docs/census-durable-inventory.md` describes that historical surface, not the new ordinary-entry timing policy.
