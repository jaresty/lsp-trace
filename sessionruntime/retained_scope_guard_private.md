# Private retained-scope equality boundary

`Manager.BindRetainedScopePrivate(ctx, sessionID, generation, members)` returns
`(*RetainedScopeGuardPrivate, session.Failure)`. This non-wire, default-off pilot
requires exactly four `NOMINATION` and two `CONTEXT_ONLY` members, canonical local
file URIs, explicit languages, at most 2 MiB per expectation and 8 MiB total.
Expectations are comparisons only: each member must already have an independent,
successfully captured Manager document supply. The existing CaptureSupply 1 MiB
acquisition ceiling is unchanged; the comparison ceiling does not enlarge it.
Cached capture copies of prior supplies are supported, not presumed nil.

Binding is all-or-nothing under Manager.mu. It freezes copied expectations and
supply metadata/bytes, generation, runtime-session identity, and lease identity.
Binding cannot replace an active scope. Pending document preparations prevent
binding; after binding, PrepareDocument and RefreshDocumentSupply refuse before
checkout reads, including attempted cache refreshes. No source is read by bind.

Attach the exact returned pointer as `RoundTripRequest.RetainedScopePrivate`.
Copying that pointer (including copying a request containing it) preserves the
capability. Copying the pointed-to Go struct, zero values, guessed state, another
Manager's pointer, and cross-session/generation use do not. MembersPrivate returns
only detached copies. Lease retirement invalidates the capability; without a lease,
the generation/lifecycle boundary owns its lifetime. No rebind or boolean bypass
recovers a retired or invalid scope.

The common pre-pending transaction seam checks every member, not just the queried
URI, for current language, version, supply identity, digest, and exact bytes. The
four guarded methods are documentSymbol, prepareCallHierarchy, incomingCalls, and
outgoingCalls. They require the invocation guard whenever a scope is active, even
through ordinary or directional transaction routes. Attached guards reject other
methods. The private parser rejects duplicate JSON keys at every depth and accepts
only canonical method DTO key spellings, including exact `textDocument.uri` or
`item.uri` as appropriate. Case aliases and alias collisions refuse; captured
supply notification DTOs have the same exact-key constraint. Supported item fields
(name, kind, tags, detail, range, selectionRange, and data) are preserved in the
original Params; `item.data` is opaque and its case-distinct nonsemantic keys are
not interpreted as DTO keys. Guarded method eligibility and the inclusive 2 MiB
Params ceiling are checked before cloning, even for bogus tokens. Nil bind context
refuses with the existing LifecycleConflict failure. External query URIs
refuse; returned external endpoints remain observations, never added members.
Roles label inventory only; neither is a semantic-admission grant.

This boundary establishes equality with independently supplied document bytes,
**not** server consumption or server-analyzed source proof. It adds no custody,
caller-occurrence, owned-pair, frame-correspondence, public schema, permission, or
production authority. Historical owned-method eligibility is deliberately unchanged.
CLI/acquisition adapters do not yet bind or propagate this guard; integrating those
adapters is a separate step. Terminal lifecycle, cancellation, and restart policies
are unchanged, with no automatic recovery introduced here. At the existing explicit
successor-generation reset, the old binding is retired alongside the old documents;
the successor starts default-off and cannot use the predecessor's guard.
