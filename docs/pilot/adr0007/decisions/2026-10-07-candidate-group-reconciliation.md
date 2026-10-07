# Candidate-group reconciliation decision

- **Date:** 2026-10-07
- **Status:** proposed successor API; migration blocked
- **Authority:** 0
- **Accepted:** false

## Decision

The candidate-group implementation at revision `448a1f4f` remains wholesale **BLOCKED** pending an explicit migration. This decision does not partially admit that implementation or rewrite its rejected predecessor status.

Current main already contains the low-level bound files needed by the bounded Group work. The minimal current-main delta is therefore documentation and a future additive parent synchronization only: preserve the existing low-level bindings, add nested parent synchronization without replacing existing behavior, and propose a distinct verified successor API rather than mutate the legacy API in place.

The legacy selectors, receipts, `Get`, and `Put` remain unchanged. No code or migration is authorized by this record. The resource profile and retained-V6 admission remain separately gated and must not be inferred from the bounded Group custody result.

## Boundary

The successor proposal remains `authority=0`, `accepted=false`, `completeness=UNKNOWN`, and `featureIdentity=UNRESOLVED`. It does not establish semantic quality, navigation value, feature identity, public enablement, or production authority.
