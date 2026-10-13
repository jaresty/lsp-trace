# ADR0011 directional raw-custody C15 lease-source v2 qualification package

Status: `AUTHORED_NOT_GO`  
Exact requested gate: `ADR0011_DIRECTIONAL_RAW_CUSTODY_C15_LEASE_SOURCE_V2_GO`  
Source HEAD: `162d796f7e33fea2c2bdb98f92e41744bb728c2d`  
Authority: `0`; accepted: `false`; defaults: unchanged/off; relation authority: `CALLS_ONLY`; Definition/Reference: unchanged.

This is a documentation-only, versioned architecture qualification package. It authorizes no implementation, production/test mutation, execution, evidence mutation, profile activation, public API/schema/configuration/registry/build-tag change, deployment, commit, or admission.

## Decision frozen here

One dedicated directional-raw subledger is added conceptually inside the existing `Manager` accounting domain. It is metadata and lease coordination, not a second recorder, backing owner, terminal owner, C15 account, or source of capacity. Every owned copy is preceded by an exact reservation for component, bytes, attempt, and account. Every acquisition result carries the exact bytes and corresponding leases; bytes cannot escape that result. Aliases are zero-charge bounded views tied to an owning lease. Transfer moves bytes and lease atomically, invalidating both at source only on success. Every terminal and race converges on one exact release.

The package also freezes a correction-safe C15 current-source enumeration plan. Counts are not mechanically enumerated here and are therefore deliberately absent, provisional, and non-authoritative. `UNRESOLVED=0` is required before the requested gate can be returned.

## Immutable blocked evidence

The following remain byte-identical blocked predecessors and are not modified, adopted as sufficient, or granted authority:

- generic profile amendment package, `SHA256SUMS` digest `df44a8befc36ff69b7e903be2b78b5cf9fbb508538b8cf04ed5bbb7fd35b2df7`;
- private interface v1, `SHA256SUMS` digest `d36211da45adb55e83f0b112a62140d7bec5ef2076edee12c3facebbd692af7c`;
- private interface v2, `SHA256SUMS` digest `8f329e1012278a7e92220d0499047a7b486402fcc2db8d10f7b4b2dcb06feeda`.

Their blocker is retained: v1/v2 accept detached raw bytes plus one caller-held lease and permit custody metadata that does not itself carry all corresponding leases. They therefore do not prove lease-source closure for every owned component and cannot satisfy this gate.

## Normative order

1. `ARCHITECTURE.md`
2. `INTERFACE_V2_IMPACT.md`
3. `C15_ENUMERATION_PLAN.md`
4. `WRITESET.md`
5. `SOURCE_MANIFEST.sha256`
6. `REVIEW_REQUEST.md`
7. `SHA256SUMS`

Any required write-set expansion, public/global/default change, second owner/terminal/account, uncharged owned copy, byte reconstruction, lease-free escape, non-atomic transfer, unresolved census row, or changed D/R/CALLS authority is a hard stop.

AUTHORED_NOT_GO
