# Review request

Required review token on approval:

`ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_DESIGN_GO`

Otherwise return exactly `REVISE` or `REJECT` with reasons. A GO authorizes only proposal of a separately authorized causal-RED phase; it authorizes no implementation, tests, execution, public change, corpus/evidence mutation, profile activation, commit, or deployment.

## Reviewer checklist

1. Verify HEAD `7f09e5357b117d20449fc16097631e32d4eead7f`, parent `4b910fc0c517cb7801fb51d17d295897230d11fa`, corpus manifest SHA-256 `316afe2501aebe4195894d1334be566ae49eb32c88276ebee7b2e4d86062f44d`, and 5900 targets.
2. Verify predecessor untracked package byte identity and exclusion from this package manifest.
3. Confirm one generic terminal owner and generic direction-neutral Layer 1.
4. Confirm the Manager-owned directional ledger is genuinely separate from C15: no child lease, nested charge, or debit against `privateB4ByteAccountV2`/`privateB4MaxOwnedBytes`.
5. Confirm exhaustive backing-to-ledger mapping and non-duplication, while C15 delta and directional delta remain separate.
6. Confirm v1 freezes one active slot, per-attempt `33554432`, and aggregate-live `33554432` as qualification policy rather than workload inference.
7. Confirm atomic reservation-before-effect under Manager synchronization, checked arithmetic, exact-limit acceptance, plus-one/overflow/second-slot refusal, clear-before exact release, and fresh-identity reuse.
8. Confirm `internal/lspwire/read_frame.go`, exact scanner/read/result-span seams, Manager request path, selected ordinal/raw handoff, and separate incoming/outgoing assemblers appear in hashes and minimal write-set.
9. Confirm the causal claim is limited to demonstrated Manager/private-B4 and `lspwire` guards; provider ceiling is not falsely asserted as a universal call edge.
10. Confirm any 32 MiB reachability is private, default-off, reservation-first, transaction-scoped, and leaves public/global defaults unchanged.
11. Confirm malformed, timeout, cancellation, and partial reads retain only bounded actually read scanner bytes as incomplete opaque prefix, never reconstructed/complete/semantic, with exact owner and release.
12. Confirm no reconstruction, duplicate terminal/owner, production effect, or shared incoming/outgoing qualification authority.
13. Confirm no history change: disabled and `CALLS_ONLY` controls preserve history entries/bytes/digests/directions/order/identities, receipt and borrower counts, lifecycle state, predecessor commitments, and generic attempt lineage byte-for-byte or value-for-value.
14. Confirm prefix mapping IDs are directional correlation metadata only and cannot mutate, replace, append to, reorder, or reinterpret predecessor/history semantics.
15. Confirm every history/lineage dependency touched by the proposed generic-owner seam is pinned and remains outside the authorized write-set.
16. Confirm managed evidence limitations remain authority 0/completeness unknown and do not establish CALLS.

## Hard stop

Return `REVISE` if the private seam is impossible, a public/global change is required, a duplicate owner/terminal results, accounting scopes merge, raw bytes are reconstructed, a direct-injection test is called causal, or any history/predecessor/lineage bytes, values, identities, ordering, ownership counts, or semantics change.

Current disposition: `AUTHORED_NOT_GO`.
