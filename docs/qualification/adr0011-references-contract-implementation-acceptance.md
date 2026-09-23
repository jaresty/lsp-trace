# ADR 0011 — REFERENCES_SYMBOL_V1 contract implementation acceptance

**Decision: `ACCEPT_REFERENCES_OCCURRENCE_CONTRACT_FOR_IMPLEMENTATION`.** Independently supplied by the maintainer in this review thread after two `REVISE` decisions. This is acceptance of the contract for implementation and testing only, **not** occurrence-specific `QUALIFIED`.

Accepted review baseline: commit `cdee217`:

- `docs/qualification/adr0011-production-admission-gate.review.md` — SHA-256 `d71ec03c9244eb70fe9d93023410e0dc5cedc9932c992b7ff1ef45f2fc42f7f9`.
- `docs/qualification/adr0011-occurrence-falsification-matrix.review.md` — SHA-256 `c370fd6ae76bb3b74a15da12a7a9c41d9730c68662830294a50dfe89177546a2`.

Authorized scope: implement the production document-symbol query-target receipt as a subordinate REFERENCES_SYMBOL_V1 artifact, occurrence admission, canonical receipts and terminal/occurrence ledgers, owner-selected publication, deterministic replay, retention and tombstone machinery, and every mandatory matrix row. Implementation must preserve the accepted claim ceiling and fail-closed policy.

Not authorized: issuing production occurrences before an independently reviewed occurrence-specific `QUALIFIED` decision over pinned implementation/schema/admission-policy digests and executed evidence; source-context completeness; definition occurrence admission; grouping input or Leiden; CLI/MCP exposure; or changing historical omitted-selector `CALLS_ONLY` behavior. The subsequent closed decision remains `QUALIFIED` / `REJECTED` / `INCOMPLETE`. Until then, production issuance remains disabled.
