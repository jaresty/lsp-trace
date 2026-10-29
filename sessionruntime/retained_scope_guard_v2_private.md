# Private Manager retained-scope V2 equality boundary

`BindRetainedScopePrivateV2` mints an opaque exact-instance wrapper for 1–8
NOMINATION and 0–8 CONTEXT_ONLY members, at most 16 total. It shares the existing
`runtimeSession.retainedScopePrivate` storage, preparation exclusion counter,
freeze checks, rebinding refusal, and generation retirement. After binding, both
`PrepareDocument` and `RefreshDocumentSupply` refuse before reading checkout
source, including cached preparation and refresh attempts. Its immutable
private profile and wrapper owner are initialized under Manager.mu before slot
publication. It adds no second lifecycle slot.

Before copying expectations or parsing URIs, V2 limits each Expected to 2 MiB,
all Expected bytes to 8 MiB, each URI to 8192 bytes, aggregate URIs to 128 KiB,
and each language ID to 256 bytes. Members must have canonical distinct local
file URIs, valid UTF-8 text and language IDs, and matching existing Manager
captured supply. URI canonicality is lexical, not a filesystem alias/symlink
resolution claim. Binding performs no source or provider IO and never supplies
Expected. Strings use ordinary immutable Go string semantics; byte slices are
cloned. Callers must not race mutations of input slices with binding.

The V2 wrapper cannot be passed to V1 APIs. The V1 check and request attachment
also reject a V2 common base, including the 4/2 case. Copying the wrapper struct
invalidates authority even when its base pointer is unchanged. Manager, session,
generation, exact slot, lease nonce and lease-token checks remain common. The
boundary is irreversible for the generation; losing a lease does not reopen it.
Explicit normal generation retirement clears the common slot, invalidating old
tokens without automatic restart or recovery.

Requests use a separate JSON-excluded V2 attachment. Mixed, zero, copied,
wrong-profile and wrong-owner attachments fail closed before copying Params.
This immutable preflight does not replace the locked live-state matcher. These
newly early invalid tokens produce no admission diagnostic; they are not owners
of a substantive bound diagnostic buffer. Valid-token bounded diagnostics are
unchanged. No translation into the V1 attachment occurs. Missing attachments on bound eligible requests are denied.
The same four method eligibility and strict DTO checks apply, including opaque
item.data. Every request and explicit Check validates every bound document's
full source, supply parameters, version and language. External URIs fail before
pending/write. Diagnostics are the existing bounded 64-event/8000-byte common
buffer, copied under the Manager lock after exact wrapper validation.
V2 JSON serialization refuses both values and non-nil pointers, including zero
values. Nil pointers retain encoding/json's standard `null` behavior. V1's
historical marshaling behavior is unchanged.

This is source equality, **not evidence of server analysis**, semantic feature
identity, MethodResult custody, or production authority. No scope freezer,
withholding V2, source-policy bridge, acquisition/schema change, or qualification
pipeline is implemented here. Those are remaining integration seams.
