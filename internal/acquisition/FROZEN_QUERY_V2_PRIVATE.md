# Private V2 frozen query metadata: first reviewed increment

This seam implements variable-cardinality **restrict-only query membership** under
existing acquisition traversal. It is not a connected variable-source pipeline,
standalone completion, authorization, or producer qualification.

## API and limits

- `NewFrozenQueryDeclarationPrivateV2([]string)` constructs pure metadata with
  1–16 total URIs, at most 8192 UTF-8 bytes per member and 128 KiB aggregate.
  These are named private V2 limits, not public request defaults or a legacy-cap
  widening. Six members have no special V2 meaning; an eventual eight nominal /
  eight context scope is not implemented by this declaration.
- Input must already be sorted, unique, valid UTF-8, and canonical according to
  the existing `canonicalURI`. There is no automatic sorting or normalization.
  Membership is exact, including path case. Case/path aliases rejected by that
  canonicalizer remain rejected; canonical distinct path spellings remain distinct.
- Count and all byte lengths are checked before UTF-8 scanning, URI parsing,
  defensive copies, or maps. Subtraction against remaining aggregate capacity
  avoids arithmetic wrap. With 16 members of at most 8192 bytes, the aggregate
  limit is redundant: equality is reachable, but aggregate +1 necessarily also
  violates another cap. No aggregate-only rejection is claimed.
- `FrozenQueryDeclarationPrivateV2` contains unexported immutable state. Its zero
  value and unsealed fabricated state are invalid. Construction owns defensive
  slice and string copies; declaration and Request copies may share immutable
  state without acquiring execution authority.
- `MembersPrivateV2()` returns a detached slice, or nil for invalid metadata.
  It exists for reconstruction by a **future validated retained V2 codec**;
  it is not a V1 decoder extension, a mutable member handle, or a trust predicate.
- `Request.WithFrozenQueryDeclarationPrivateV2(d)` returns a request copy after
  checking declaration validity, absence of a V1 declaration, and inclusion of
  root and every required target URI. This is not full request validation.
  Other Request fields retain their ordinary Go copy semantics. Subsequent target
  edits are rechecked by `ValidateRequest`, including at `ValidateResult` entry.

Neither constructor nor binder performs file/provider IO, source-path IO,
Manager binding, source-custody checks, source preparation, or an authorization
callback. The constructor seal denotes metadata construction only; it is not an
acquisition grant or a security boundary against code inside this package.

## Compatibility and traversal

The public `[6]string` V1 field and its `json:"-"` tag remain unchanged.
V1-only invalid declarations preserve the existing error strings. Any simultaneous
V1 and V2 declaration refuses before client traffic, even with identical members.
A nonnil invalid V2 declaration is still declared and refuses validation rather
than falling open. Undeclared ordinary requests retain their existing behavior.

V2 dispatches only membership/validation. Existing coordinator and join scheduling,
incoming starts at callees (not reseeded roots), retained CALLS, stop reasons,
boundary origin/direction text, duplicate handling, null versus empty responses,
and finite resource accounting remain owned by their original implementations.
External endpoints remain real retained observations, not fabricated responses.

The new Request field is unexported and JSON-excluded: request JSON bytes are
unchanged and a fresh decoded Request has neither private declaration. Standard
Go JSON decoding into a pre-populated Request may retain private fields, including
V1 and V2 metadata. No clearing, authorization, or custom public unmarshalling
behavior is claimed. Decoded retained results that lack restriction metadata do
not thereby reconstruct valid frozen frontiers.

## Remaining connected V2 work — not implemented

- `variableScopeV2` source binding and equality with actual owned source members.
- Variable source cardinality, withholding policy/metadata, source qualification,
  preparation, and connected sealed-producer validation.
- A validated retained V2 codec carrying/reconstructing this restrict-only data.
  Such reconstruction must never authorize an operation or rewrite V1
  SourceWithholding metadata.
- Higher-level producer/workflow admission and independent integration review.

Ordinary Native, Census, Describe, and workflow admission are unchanged. Tests use
owned fake clients only. They establish local mechanics, not SourceComplete,
feature identity, production behavior, generalized source cardinality, or live
provider qualification. No schema, pins, or public caps change in this increment.
