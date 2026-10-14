# ADR0011 CALLS_GO_GOPLS_OUTGOING_EXACT_V1 provider qualification package v1

Status: `CALLS_GO_GOPLS_OUTGOING_PROFILE_QUALIFICATION_AUTHORED_NOT_GO`. Authority: `0`; accepted: `false`; provider GO: `false`; production admission: `REFUSED_NOT_REQUESTED_NOT_GRANTED`; completeness/frontier: `UNKNOWN`.

This is a complete outgoing provider qualification package for independent review, not a NO_GO stub. The producer grants no GO and changes no incoming authority, composition, public surface, runtime behavior, custody owner, or ceiling.

## Frozen provider binding semantics

`caller queried A -> callee B; to endpoint identity/display only; fromRanges sole occurrence source; null/empty/malformed from generic package`

Occurrence source policy: `fromRanges only; to.range and to.selectionRange are endpoint identity/display only; D/R, source text, documentSymbol, incoming reverse evidence, provider heuristic, and raw endpoint display spans are not CALLS occurrence sources`

The provider binding is direction-isolated: queried prepared caller `A` is the relationship source; returned `to=B` is callee endpoint identity/display; `to.range` and `to.selectionRange` are not call-site occurrence rows; only valid `fromRanges[]` entries emit occurrence evidence. Null, empty, missing, and malformed handling is inherited from the pinned generic outgoing package, not from Go/gopls behavior.

## Exact run results

Both the historical outgoing workload and the fresh outgoing custody-correction run have the exact final counts: attempted `5900`, non-empty `4378`, empty `1455`, LSP error `67`, refusals `0`. Target order, failure identities, outgoing request objects, outgoing response objects, and measured maxima are equal. Ledger timestamps and raw custody inventories/checksums differ, as expected for separate runs.

Fresh final checksums SHA-256: `b01bcfd5018432eed725af79a41ce049ac3ba2941d7d2f3a40077367047bb93b`. Fresh final seal SHA-256: `a511c0f2ff435a30ba963f7b70671880959cf234bf37c7dc7a195b7886ba795c`. Artifact count: `53640`.

## Review

Run:

```bash
PYTHONDONTWRITEBYTECODE=1 python3 docs/qualification/adr0011-calls-go-gopls-outgoing-profile-qualification-v1/verify_outgoing_provider_profile.py
PYTHONDONTWRITEBYTECODE=1 python3 docs/qualification/adr0011-calls-go-gopls-outgoing-profile-qualification-v1/verify_outgoing_provider_profile.py --tamper-tests
```
