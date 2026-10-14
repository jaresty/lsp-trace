# Review request

Please review this package as a private evidence-contract producer only.

## Scope to accept or reject

- Package confinement to `docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/`.
- Frozen corpus identity and target hashes.
- Directional incoming/outgoing separation and no inference between directions.
- gopls/go/environment provenance capture.
- Raw response byte capture or immutable digest/offset references.
- Append-only attempt ledger with STARTED crash -> ABANDONED -> retry semantics.
- Exactly one final disposition per ordinal.
- Failure/refusal taxonomy.
- `--contract-check`, bounded `--smoke`, and tamper/accounting tests.

## Explicit non-scope

- Do not treat this package as production admission.
- Do not treat it as Manager custody.
- Do not treat smoke success as qualification of the 5900 workload.
- Do not run the full workload as part of review.
- Do not infer incoming evidence from outgoing or outgoing from incoming.
- Do not modify public CLI, MCP schema, public source, or profileGO.

## Reviewer questions

1. Is every required identity frozen with enough immutable data to block substitution?
2. Are incoming/outgoing ledgers and raw namespaces independently auditable?
3. Does the verifier reject duplicate final dispositions, retry without ABANDONED, and missing terminal dispositions?
4. Does the runner capture exact LSP request/response bytes and environment provenance before semantic interpretation?
5. Are all authority/admission claims explicitly refused or unknown?
