# Private guarded directional callback candidate

Base: c7e3d8b41cc18314a07ec2a10eead1c1e7f2ccdc. Fake fixtures only; no acceptance decision.

## Source seam

`nativeClient()` remains default-off. The package-private
`nativeDirectionalClientPrivate()` installs `WireClient.SetDirectionalPrivate`
with `nativeDirectionalRoundTripLocked`. The enclosing `scopeNativeClient`
invocation lock spans Manager transport, byte accounting, WireClient's single
`BindDirectionalResponse`, shape checks and typed decoding.

The callback forwards the same opaque retained guard to the actual
`Manager.RoundTripDirectionalPrivate`, with native-declared wire/message limits
intersected with adapter lifetime, request timeout and remaining aggregate budget.
It creates neither reservations nor leases. Manager supplies both allocation and
terminal authority. Completion metadata supplies result-length accounting;
validation scratch from the lease-checked reader is temporary, cleared, and never
returned as ordinary-result custody. Duplicate object keys are strictly refused;
array order, duplicate elements, null versus empty, and item.data remain distinct.

No runtime eligible-method, framing, owner or guard logic changed. Constructor
six-member supply/freeze and four-nomination/two-context policy remain unchanged.
External returned observations do not authorize an external request URI.

## Bounded qualification

- `TestRetainedScopeDirectionalPrivateConjunction`: real Manager fake transport,
  retained guard plus genuine unattached reservation, all three allowed methods,
  neutral binding, completion metadata, Manager account/owner and actual terminal
  partition/preparation release. This is a runtime probe, not a producer.
- `TestRetainedScopeDirectionalPrivateAuthorityRefusals`: missing/copied/foreign/
  retired guard, session/generation/member drift, external URI, cancellation and
  poison all refuse before traffic or custody attachment.
- Native binding fixture: default client RED at
  `ASSERT_NATIVE_DIRECTIONAL_RESOLVED`; explicit opt-in GREEN, with original
  reserved backing, counters, terminal owner release and invalidated views.
- Lexical fixtures: all three methods, null/empty, distinguishable array ordering
  with repeated elements, opaque item.data, strict duplicate-key refusal.
- Wire fixtures: invalid bounds/reservation/guard/URI refuse before transport;
  consumed malformed/budget failures latch terminal, prevent retry, and retain
  actual Manager-owned allocation/release behavior.

Fixture-created reservations in callback tests do not establish owning-producer
integration. They do not manufacture custody callbacks or receipts.

## Negative frozen-supply fixture — not the Census producer

The existing `incomingops.Executor.ExecuteDirectionalPrivate` creates its
own reservation, then `Execute` calls `PrepareDocument` before traversal
(`incomingops/executor.go:118-123`). Manager deliberately returns
`DOCUMENT_SUPPLY_UNAVAILABLE` after the retained guard has frozen the document set
(`sessionruntime/sessionruntime.go:492-494`). The observed result was
`failure=DOCUMENT_SUPPLY_UNAVAILABLE attached=false`, before semantic traffic.
`TestScopeDirectionalPrivateExistingProducerFrozenSupplyBlocker` retains this
boundary using a test runtime wrapper that forwards the same retained guard;
it does not replace preparation with synthetic success or substitute reservations.

This is a negative frozen-supply fixture, not the owning Census producer, and
is not a blocker to bounded transport-seam qualification. The actual Census chain
is `acquisitionorchestration.ExecutePlannedBatch` →
`acquisitionengine.ExecuteAuthorized` → `sessionclient.New(runtime)` →
`acquisition.Acquire` → `CaptureV5WithSeedSpec`. Its private scoped-client
selection and producer-owned reservation lifetime remain missing; no preparation
bypass is authorized. NativeV5 completion, a scope-bound constituent receipt and
profile/ProgramC pipeline integration remain unresolved. No full ProgramC bridge,
new checkpoint/storage machinery, public CLI/MCP switch, or live/retained-root
product operation was added or run. Tests use disposable retained-bundle fixtures.

Raw custody does not establish source closure, analyzed-source, full-definition,
or caller-occurrence authority. No acceptance decision is made here.

## Process limitation

A child-local Bar build and token loads were performed, but the required durable
boundary authorization was not emitted before adapter edits. The initial direct
runtime probe preceded the callback; the later incomingops check exposed a
frozen-supply refusal on a different producer path, not Census integration. Passing fixture checks do not repair
these sequencing gaps or establish producer completion.
