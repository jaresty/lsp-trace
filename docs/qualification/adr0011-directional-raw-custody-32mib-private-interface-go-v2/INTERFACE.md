# Frozen private interface contract

**Token:** `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRIVATE_INTERFACE_GO_V2`  
**Disposition:** `AUTHORED_NOT_GO`

## Boundary

This package freezes compile-ready private declarations only. It implements nothing, runs no production test, changes no production/test file, creates no public API/schema/profile/registry/configuration, and grants no qualification, activation, admission, completeness, or GO.

The generic contracts are independently identified:

- `CALLS_LSP_INCOMING_EXACT_V1` / `lsp-trace.adr0011.calls.lsp.incoming.exact.v1`;
- `CALLS_LSP_OUTGOING_EXACT_V1` / `lsp-trace.adr0011.calls.lsp.outgoing.exact.v1`.

Each constructor computes `SHA256(prefix || uint32be(len(domain)) || domain || uint32be(len(id)) || id)`, where prefix is the exact byte string `lsp-trace.adr0011.calls.profile-identity.digest.v1\x00`. Lengths are unsigned four-byte big-endian values. This canonical binary preimage is injective across the two exact domain/ID pairs and the constructors always populate `Digest`.

Incoming and outgoing never inherit identity, evidence, terminal state, target accounting, acceptance, or GO. Provider bindings, including `CALLS_GO_GOPLS_*`, remain outside this generic interface. The sole permitted provider field is an opaque binding identity/receipt; it has no semantic authority.

Generic Layer 1 remains opaque and direction-neutral. Definition and Reference data are not CALLS. `CALLS_ONLY`, disabled paths, defaults, history, and lineage remain unchanged.

## Exact declarations

`sessionruntime_private.go.txt` and `lspwire_private.go.txt` are normative, complete `gofmt` declarations. All introduced names are package-private. They freeze:

- constants and constructors for both generic profile identities;
- independent direction, disposition, stage, terminal, refusal, lifetime, state, and component closed enums;
- complete profile, target, target-set accounting, frontier, occurrence, custody, reservation, ledger, view, transfer, and bounded-frame structs;
- constructors and full method/function sets used by the planned exact production/test write-set;
- exact selected prepared-item ordinal retention through Manager-owned synchronized handoff;
- opaque provider-binding receipt only.

Zero values are inert or invalid: zero ledger/state is unreserved; zero terminal is none; zero refusal is none only before refusal; all other zero enum values are invalid. Nil Manager, owner, reservation, link, charge, custody, source, or identity fails closed without mutation.

## Custody and lifetime

The existing `callHierarchyLayer1GenericOwner` is the sole generic owner. Its existing C15-backed lease is moved into one owner slot; no second owner or charge exists. A successful transfer accepts `*privateB4ByteLeaseV2`, installs bytes and the exact lease, then atomically assigns `*charge = privateB4ByteLeaseV2{}` under both locks. Failure leaves source and destination unchanged.

Lifetime is explicit: borrowed input, copied metadata/bytes, aliased non-owning reference, synchronized view, and released invalidation. Custody never owns bytes or a C15 lease. A view/handoff that wins both locks may copy or validate a bounded reference; release invalidates every reference. No returned alias can reacquire released backing.

## Reservation and 32 MiB pre-effect rule

`Manager` owns the one-slot reservation ledger. Before any acquisition/read/allocation/provider effect, it checks the exact requested amount against both per-attempt and aggregate bounds of `32 << 20`. Exactly 32 MiB may reserve; 32 MiB + 1 refuses at `Reservation/Limit` with zero production/provider/read/allocation effects and unchanged owner, lease, ledger counters, history, and lineage. Overflow refuses before comparison. The reservation ledger is metadata only and never duplicates C15 charging.

## Synchronization and exact-once terminal release

Every directional reservation, transfer, transition, view, handoff, terminal, and release operation uses the universal order:

```text
Manager.mu -> callHierarchyLayer1GenericOwner.mu
```

No path may acquire `Manager.mu` while holding owner lock. `callHierarchyLayer1GenericLease.Release`, owner `requestTerminal`, STOP, RESTART, Shutdown, cancellation, normal teardown, and duplicate races converge through `Manager.releaseCallHierarchyTerminalV1` before owner locking. Exactly one release wins, clears backing, releases each moved C15 lease once, removes `ledger.active`, decrements live accounting, assigns a monotonically increasing nonzero release identity, clears links, and makes the slot reusable. Losers are no-op and cannot change terminal or release identity.

## Direction and terminal rules

Preparation is a separate ordered transaction. The selected zero-based prepared-item ordinal is immutable and unchanged across handoff. Incoming assembles only incoming params; outgoing assembles only outgoing params. No flip or cross-direction repair is allowed.

Terminal and disposition remain distinct. Complete, complete-empty-null, complete-empty-array, failed transport/protocol, unsupported, cancelled, timed out, refused, partial, malformed, and missing payload are not interchangeable. `Released` is not a disposition. Release is represented only by lifetime/state invalidation and the exact nonzero release identity. Refusal records exact reached stage plus exact reason. Terminal selection is immutable; release cannot overwrite it.

## Target-set-relative accounting

Each direction carries its own target-set identity, denominator-known bit, denominator, attempted/unattempted totals, terminal class counts, exact attempted target ordinal/outcome records, exact frontier, and ordered unattempted target ordinals. Required equations are:

```text
denominator = attempted + unattempted
attempted = complete_success + complete_empty + failed + unsupported + cancelled + timed_out + refused + partial
```

The attempted-outcome ordinals are unique; the unattempted ordinals are strictly ordered from the frontier; the two sets are disjoint and their union is the exact target set. Thus every target ordinal appears exactly once. Unknown denominator, missing role, partial wire/parse/target execution, overflow, stale/substituted custody, incomplete accounting, or bounded stop yields completeness `UNKNOWN`. A verified zero denominator means only target-set-empty; zero occurrences means only zero admitted occurrences for that exact target set/profile/evidence. No repository, workspace, runtime, language, provider-family, or production completeness is claimed.

## Hard stops

Stop with `BLOCKED_PUBLIC_SURFACE_OR_RAW_CUSTODY_CHANGE` for any public/dependency/duplicate-owner/duplicate-charge/architecture/default/activation/write-set change. No fallback or write-set expansion exists.

AUTHORED_NOT_GO
