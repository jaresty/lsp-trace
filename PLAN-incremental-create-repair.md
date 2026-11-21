# Narrow Create funding and storage-label repair

## Craft before mutation
Remain within existing DurableStore and its inventory-run CAS. No journal/controller, new CLI, scope integration, limit increase, provider process or broad fixture suite. Preserve checked-reader repair and other dirty work. Before hashes are in /tmp/lsp-incremental-create-repair-iECPIf/.

1. [x] Reproduce supplied reviewer probes; write behavioral RED tests for empty-root expired Create and actual encoded config charging.
2. [x] Introduce explicit head v4 / config v2 / creation-cost model v1. First effect is the same owner's self-contained Creating/CLAIMED head (bounded embedded canonical config, exact future config reference, full creation quote). Only then publish config and CAS to READY. Cancellation before claim creates nothing; after claim keeps charged readable uncertainty and returns RunID. Injection-only faults are not OS-kill proof. Old incremental v3 is recognized/refused without migration or historical debit recomputation; legacy v2 remains untouched.
3. [x] Quote 8*canonicalConfigBytes + 32*(65536 bootstrap-head bound + 16384 final-head bound) + 32768 fixed overhead, checked against logical ceiling. Bound covers encoding/hash/copies, CAS predecessor/write/verification/readback and bounded status inspection; not physical RSS. Validate canonical config <=32768 and sufficient full quote + one admission + Seal before effects. No refunds.
4. [x] Rename config to MaxAdmittedEvidenceBytes, explicitly post-retention acceptance only; record field AdmittedEvidenceBytes/version v2. Total retained-storage requirement is UNMET/deferred, not an approved substitute. No physical quota implementation.
5. [x] Focused boundary/fault/collision/version/duplicate-Seal tests, four tagged pipe tests, existing 19 owner/legacy selector, normal/race; vet/build/format/write-set hashes and report. Parent independent review required.
