# Verified input delivery — local Craft phase

Sole writer in existing dirty worktree. No changes to caps, strict grammar, native acquisition, source custody, or publication engine. Parent independent review required. Historical RED remains input-delivery-probe-20261119-18d/RESULT.md.

## Design and owned scope

- Use existing publication.StoreObject/ReadObject for immutable 32768-byte chunks and a canonical ordered index bound to original digest/length and run/task/predecessor. Bind the index reference into timed acceptance. Preserve original full object for semantic consumers.
- Charge original verification, chunk publication/readback, index construction/publication/verification before effects. Page reads charge index handling and only intersecting whole chunks, plus returned copy; no raw ranges.
- Head CAS transfers one canonical complete forward delivery attempt per issued task. Arbitrary smaller/overlapping/repeated/out-of-order reads are optional charged reads and cannot spend the required canonical tail. A failed/lost response after CAS remains spent, not delivered; observation reports next funded offset without promising receipt.
- Preserve semantic completion cursor. Independently account read progress. First Result remains protected.
- Explicit staged admission: before predecessor answer effects, conservatively admit successor chunk construction and required delivery along with all remaining semantic quotes. For Describe, canonical supplied adapter length can replace its previously unknown adapter upper for unspent future stages; retain full parser allocation and unchanged grammar. Preserve previously spent stages exactly. A future answer may refuse admission; do not promise arbitrary maximal future completion. No spent allocation is refunded.
- Initial task chunk construction comes from the preparation budget. Successor construction comes from the predecessor's single outer allocation; inner charges are not charged again outside it.

Expected write set: internal/applicationinventory/{durable.go,durable_capacity.go,task_contract.go,new durable chunk implementation/tests}; cmd/lsp-trace/census_input_delivery_probe_test.go; docs/census-durable-inventory.md; this plan and external phase report/ledger. Expand only with a concrete cross-file requirement.

## Gates

1. Read contracts and exact-worktree projected Read/reserve/ReadObject definitions; retain original RED.
2. Implement index/build/page primitives with exact reconstruction and bounds/corruption tests.
3. Integrate authoritative staged admission and forward-attempt CAS, tests for optional starvation/concurrency/lost response.
4. Same legal large three-item fresh-process test must reconstruct all grouping/reviewer evidence and publish; old small/ordinary fixtures must remain green. No weakened custody assertions.
5. Focused offline test/race/vet, formatting/diff, Linux compile; disclose unexecuted crash cutpoints and non-exhaustive bounds. Report actual costs, not physical-memory containment.

Status: bounded implementation/verification complete; parent independent review pending, no production-scale acceptance claimed. Same legal large three-item trajectory is GREEN through all GroupDecision/Reviewer pages, distinct sequential correction and verified Result. Final sample58769725 bytes/1880631200work/17packets, remaining8339139. See external verified-delivery-20261119-18e/RESULT.md for full accounting, counterfactuals, exact files and limitations.

Additional directly affected tests: cmd/lsp-trace/census_returned_contract_test.go (obsolete read/skip accounting expectations) and internal/inventorysuccessor/sourcecontinuation/inventory_completion_actual_adapter_test.go (new maximal legal actual-adapter quote guard). Existing production semantic validators unchanged.

Staged admission uses two conservative CAS boundaries: claim existing funded stage before sizing; then admit successor top-up/delivery/escrow before answer interpretation or storage. No refund if either stage fails. Each issued task also protects successor construction/delivery escrow; known adapter lengths tighten only unspent future stages. Old persisted head migration is not implemented; new fields are acceptance-bound and pre-change heads fail closed. Page operation shape is unchanged.

Executed: boundary/corruption/optional-tail/read-read race; actual SIGKILL after read reservation before chunk access; actual read-submit race; strict invalid variant/input guards; old small3/ordinary2 processes; maximal encoding/group expansion; schema/vet/gofmt/diff; Linux compile. Counterfactual external overlays reject full-original reread and missing required-read protection. Not exhaustive crash, maximal-combination, physical-memory, arbitrary-scale, Linux runtime or Windows qualification.
