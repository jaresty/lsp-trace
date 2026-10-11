# ADR-0011 call-hierarchy Layer-1 generic corpus RED v2 request-ID amendment review v5

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_RED_AMENDMENT_GO`

Reviewed at repository HEAD `1a9a71cb2e2b35bb99800cafeb21c12076a550c6` with the existing modified amendment, untracked RED, and committed corpus otherwise unchanged. This disposition accepts only the request-ID amendment contract. It does not accept, activate, or grant authority to the unchanged v2 corpus or RED, which do not yet contain or execute the amended `request_id` contract.

## Independent findings

- **Derivation and validation:** `driver.request.request_id` is a non-oracle causal input derived before projection from the numeric retained v1 `expected.identity.request_identity.request_key`. The amendment requires a positive JSON integer representable as `uint64`, rejects omitted, zero, negative, fractional, out-of-range, and mismatched values, and requires equality with the IDs already fixed in both complete request and response frames and with the retained expected request identity. Frame values are derivation-time validation inputs; execution-time frame parsing is forbidden.
- **Self-contained causality:** every root and every W24/W25 inline record must carry its own request ID. Inheritance, side tables, record/world-name dispatch, expected-subtree dispatch, and implementation-produced values are forbidden. Execution takes the ID and opaque Params directly from `driver`.
- **Allocator algorithm:** for each envelope the amendment requires a fresh `lspwire.NewPending(0)`, then exactly `request_id - 1` matched `Begin(generation)`/`Accept` operations. In the current `lspwire.Pending`, `Begin` increments the allocator and creates one active entry; matching `Accept` deletes that entry. Capacity zero retains no tombstones. The sequence therefore positions the next allocation at the requested ID without leaving active pending entries or tombstones.
- **Locking and ownership:** allocator assignment is permitted only while holding `Manager.mu`, after verifying the exact runtime session/generation is idle and not protocol-owned. `Manager.mu` must be released before `Manager.RoundTrip`. This is consistent with the existing runtime fields and RoundTrip admission checks and does not authorize concurrent replacement of an in-use allocator.
- **No pending leaks:** every synthetic advancement is matched before installation; no unmatched pending operation is permitted. The amendment also expressly forbids unmatched pending state.
- **No production change:** setup is test-only. The amendment authorizes no edit to production files and the reviewed worktree diff for the amendment changes only the qualification document.
- **Accounting unchanged:** `request_id` is excluded from expected-path traversal and from the `162 -> 145` accounting. The retained denominator remains exactly 145 (`133` generic plus `12` opaque), with the same 17 removed/deferred rich paths.
- **No frame rewrite or semantic creep:** placeholder rewriting, frame rewriting, execution-time request/response parsing, body/payload/result-subspan recovery, token/form reconstruction, and call-hierarchy symbol/item/range/caller/callee/provider semantics remain forbidden. The request/response frames are fixed validation evidence, not execution-time semantic inputs.
- **Driver-only causal boundary:** transport behavior remains selected only by finite driver event kind, bytes, class, message, timing, request ID, and opaque Params. Expected values remain comparison-only and cannot select behavior.

## Boundary

The unchanged committed corpus currently omits `driver.request.request_id`, and the unchanged untracked RED currently has neither the request-ID field nor allocator installation logic. Those facts do not block this amendment-only disposition, but they do block corpus/RED authority until a later additive derivation and independent review proves every amended requirement, reproduces canonical hashes, and preserves the stated no-production-change boundary.
