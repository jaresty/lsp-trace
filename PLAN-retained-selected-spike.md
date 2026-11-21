# Retained selected evidence: bounded offline spike

> Historical spike disposition, superseded for implementation on 2026-11-19: the owner explicitly accepted application-owned local-store authority, without an external issuer registry. The refusal-only characterization below remains historical, not the target solution. See `PLAN-durable-selected-describe.md` and `docs/census-durable-describe.md` for the implemented one-item cross-process Describe milestone and its unfinished controller boundary.

## Scope and current disposition

Single writer; existing dirty files remain untouched. Exact worktree `/Users/schwa/dev/lsp-trace-variable-v2-TSZxdY`, HEAD `6a7a80494418193491dad4f47437545552d643aa`. Child-local Craft derivation precedes this plan. Exact managed session lookup returned `APPLICABLE_SESSION_NOT_FOUND`, request `offline-1`: filesystem-degraded reasoning, not server-reported call relationships.

Production admission is BLOCKED on issuer trust anchoring. This is not a finding that cross-process retained evidence is impossible. The user authorized a new bounded contract, but the inspected existing mechanisms do not themselves establish an independently trusted original-owner issuer after exit. Do not implement a checksum/self-signature-to-native-authority constructor.

## Contract considered before mutation

1. **Issuer:** export may originate only from `PrivateSelectedResultV2` minted after `prepareSelectedRoleCriterionV3` completes cleanup, Finish and accounting readback. Never accept a caller-created DTO as export input.
2. **Payload:** exact selected row, unit/body, Describe request, source session/generation/revision/privacy/digest/encoding/ranges, handoff/checkpoint and completed M/P/S accounting. Hash the complete canonical bounded envelope, not just the body. No normalization or re-creation of source evidence.
3. **Export seal:** domain-separated signature by an owner-controlled ephemeral key is a candidate, NOT sufficient alone. B needs an issuer registration anchored independently of the exported payload/public key. A key in the same bundle, or a caller-selectable key/root tuple, does not prove owner issuance.
4. **Root/request/frontier binding:** bind canonical path plus device/inode, run and request identities, explicit selected frontier, schema and evidence identity. Compare to a separately trusted expected registration; do not derive the expected values from the object being admitted. Reject foreign roots/runs/frontiers and stale immediate successors.
5. **Single-use claim:** separately funded semantic admission must claim a create-only generation before returning an attempt. Reuse existing no-follow private-root checks, bounded canonical validators, exclusive claim and CAS principles; retain irreversible spent state on malformed/cancelled completion. No native grant renewal or replay.
6. **Failure disposition:** any creation/write/fsync/readback/close/CAS uncertainty yields no admitted attempt or success. Leave started claims spent. Crash after claim and before completion cannot resume the same semantic attempt. Local at-most-once success only; no exactly-once/distributed rollback guarantee.
7. **Lifetime/accounting:** A's M/P/S and acquisition finish once and remain spent. B receives a separately bounded caller interpretation budget/deadline, never a native permit. Existing normal workflow and ceilings remain unchanged.

### Necessary design decision

Select the trust anchor for B: (a) a trusted supervisor retaining an issuer/root/frontier registration obtained while A has the genuine selected result, with an explicit handoff capability; or (b) a newly designated trusted persistent issuer registry, including provisioning, storage separation and same-user tamper/rollback assumptions. A public-key field, SHA-256 receipt, successful accounting reopen, or matching current-user inode is NOT such a selection.

Existing evidence: `producer_pages_checkpoint_private_v3.go:97-103` requires original opaque owners; `producer_pages_ledger_private_v3.go:19-23,157-160` disclaims source authority/authentication; `job_head_v2.go:19-20` is ACCOUNTING only; `job_authorization_v2.go:96-98` explicitly has no credential service; `publication/boundfile_read_checked.go:12-16` leaves trusted expected bytes to caller; `ValidateAutomaticPublicationObservationPrivateV2` explicitly returns observation DTOs, never owner/postState.

## Bounded fallback execution

- [x] Add one external characterization test, no private-state constructors or production edits.
- [x] Process A: use existing genuine selected fixture once; complete existing Describe positively; snapshot exact public row/unit/request and JSON serialization of the opaque value. Exit normally so test cleanup joins provider and removes source workspace.
- [x] Parent: wait for A exit, verify the source workspace is gone, then start B with empty PATH/fresh HOME and no inherited provider configuration.
- [x] Process B: read only the bounded exported test snapshot; verify exact snapshot digest/request validity and assert that opaque JSON/DTO attempts remain unavailable and cannot prepare Describe. Label result `CHARACTERIZATION_ONLY; CROSS_PROCESS_SUCCESS_NOT_IMPLEMENTED`.
- [x] Run one focused race-enabled characterization (one new genuine preparation), existing pure DTO guard, scoped vet/format/diff checks. No acquisition rerun or counterfactual production mutation occurred. No RED-to-GREEN implementation qualification is claimed.
- [x] Record actual witnesses and limitations in phase report `retained-selected-spike-20261119-12/RESULT.md` and progress ledger.

## Acceptance distinction

Passing the characterization does NOT satisfy the requested A-exit/B-success target. New exporter/admitter, signed authority, claim lifecycle, artifact tamper/body/request/foreign-frontier refusal, duplicate/concurrent completion, stale claims and crash/uncertain commit of a new contract remain NOT_IMPLEMENTED/NOT_QUALIFIED. No claim of semantic independent assurance, full portability or normal census completion is allowed.
