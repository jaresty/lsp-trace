# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Identity Causal-Carrier Amendment

Status: **REVIEW_REQUIRED / NO AUTHORITY**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Pinned clean HEAD: `f8839f0f5a543ace0403d29ea6cb30987cbf6d01`
- Scope: documentation-only correction for the confirmed missing A116 identity causality
- Mutation authorized by this task: this document only
- Corpus, RED, source, tests, generated artifacts, execution, classification, commit, activation, production use, and authority: **not authorized**

## 1. Confirmed defect and narrow correction

The proposed A116 observation/view maps `identity.profile`, `identity.binding`, and `identity.request_identity.workspace_nonce_hex` to production identity fields, but the current proposed causal request does not independently carry those three values. They therefore lack a lawful request-side cause. Existing profile selection, binding selection, Manager/world state, request ID, method, commitment metadata, expected values, and execution output cannot repair that omission.

The correction is one independently authored, required identity object in every self-contained occurrence:

```json
{
  "driver": {
    "request": {
      "identity": {
        "profile": "<non-empty exact profile identity>",
        "binding": "<non-empty exact binding identity>",
        "workspace_nonce_hex": "<exact 64-character lower-case non-zero hex>"
      }
    }
  }
}
```

The exact JSON paths and JSON types are:

| Path | Type | Required invariant |
|---|---|---|
| `driver.request.identity` | object | required, non-null, exactly the three members below |
| `driver.request.identity.profile` | string | required and non-empty |
| `driver.request.identity.binding` | string | required and non-empty |
| `driver.request.identity.workspace_nonce_hex` | string | required; exactly 64 lower-case hexadecimal characters; decodes to a non-zero `[32]byte` |

This object is causal driver data, not expected data and not an availability projection. All three values are mandatory for all 85 structural occurrences. There is no absent, null, empty, zero, inferred, defaulted, or unavailable form.

## 2. All-85 intended-value contract; no corpus-byte authority

A future separately authorized rematerialization must independently author the exact profile string, binding string, and workspace nonce hex for each of the exact 85 physical envelope occurrences. The authoring input must identify every occurrence explicitly and provide all three literal values; it must contain no wildcard, inherited default, world-level default, pair-level default, registry fallback, or runtime lookup.

The intended value for an occurrence is exactly the literal value independently assigned to that occurrence in that reviewed authoring input. Equality with any currently stored `expected.identity.*` value is not an authoring rule and cannot supply, repair, select, or validate the driver value before execution. This document deliberately does not claim that proposed literals are present in canonical corpus bytes: no corpus bytes, rematerializer, manifest, hash, aggregate, or byte succession are authorized here.

A later review must publish an exact all-85 assignment ledger before corpus mutation. The ledger must contain, for every occurrence ID, the three literal proposed values and their provenance independent of `expected`, method mapping, world/pair labels, request ID, commitments, or execution output. Missing provenance or any duplicate-by-default shortcut is a hard stop.

## 3. Strict decode and validation

A separately authorized RED/corpus revision must use strict JSON decoding with unknown-field rejection at every enclosing driver/request/identity object. It must reject:

- absent or null `identity`;
- absent, null, non-string, empty, or zero-valued `profile` or `binding`;
- absent, null, non-string, empty, upper-case, `0x`-prefixed, non-hex, odd-length, shorter-than-64, or longer-than-64 `workspace_nonce_hex`;
- a decoded all-zero workspace nonce;
- duplicate JSON members;
- any extra member in `driver.request.identity`; and
- any unconsumed second JSON value or trailing non-whitespace input.

`workspace_nonce_hex` must satisfy `^[0-9a-f]{64}$`, decode with strict hexadecimal decoding to exactly 32 bytes, copy into `[32]byte`, and then be compared against `[32]byte{}`; equality with zero is rejection. It must never remain an unvalidated string at the production attachment boundary.

The all-85 structural pass must prove exactly one valid identity object per physical envelope. Validation failure is corpus RED and grants no permission to infer or rewrite a value.

## 4. Exact private production representation proposed for later review

The proposed write-set already defines these exact private comparable types:

```go
type callHierarchyWorkspaceCustody struct {
	managerNonce [32]byte
}

type callHierarchyProfileIdentity string
type callHierarchyBindingIdentity string
```

No new wrapper type is needed. The exact proposed additive attachment fields are:

```go
type callHierarchyLayer1RequestAttachment struct {
	selected     callHierarchyOpaqueCommitmentMetadata
	handoff      callHierarchyOpaqueCommitmentMetadata
	ledger       callHierarchyTransportEffectLedger
	predecessors *callHierarchyOpaqueCommitmentList
	enabled      bool
	profile      callHierarchyProfileIdentity
	binding      callHierarchyBindingIdentity
	workspace    callHierarchyWorkspaceCustody
}
```

All fields remain directly comparable. The exact single value-returning identity setter is:

```go
func (attachment callHierarchyLayer1RequestAttachment) privateCallHierarchyLayer1WithIdentity(
	profile callHierarchyProfileIdentity,
	binding callHierarchyBindingIdentity,
	workspace callHierarchyWorkspaceCustody,
) callHierarchyLayer1RequestAttachment
```

It returns a copy with exactly `profile`, `binding`, and `workspace` replaced by its three arguments. It preserves `selected`, `handoff`, `ledger`, `predecessors`, `enabled`, and every future independently reviewed field verbatim. It allocates no wrapper, holder, pointer, map, slice, interface, callback, registry entry, or side table. The RED must mutation-test each owned field independently and prove every non-owned field remains equal, including predecessor-holder pointer identity and all comparable values.

The existing proposed constructor must copy the attachment fields verbatim:

```go
attempt.profile = req.callHierarchyLayer1.profile
attempt.binding = req.callHierarchyLayer1.binding
attempt.requestIdentity.workspace = req.callHierarchyLayer1.workspace
```

No normalization, lookup, availability synthesis, or replacement is allowed. The immutable owner/view subsequently receives those exact typed values through the already proposed attempt-to-observation lineage. The A116 projections remain post-execution observations:

- `identity.profile.available` is true only because a valid non-empty causal input was supplied and executed;
- `identity.profile.value` is the exact executed typed string;
- `identity.binding.available` and `.value` follow the same rule; and
- `identity.request_identity.workspace_nonce_hex` is lower-case hex of the exact executed `[32]byte`.

## 5. Absolute anti-inference boundary

The three identity values must not be derived, selected, repaired, defaulted, or checked as execution inputs from:

- method mapping, method family, incoming/outgoing direction, profile lookup, or profile registry;
- world, occurrence, fixture, root, worker, execution, pair, enabled/disabled, or scenario identity;
- `request_id`, JSON-RPC request key, session, generation, transaction identity, or commitment identity;
- commitment availability/content, frame bytes, params, transport, result, error, terminal, refusal, accounting, or lifecycle state;
- any `expected` value, including `expected.identity.*`;
- prior RED/prototype output, current production output, hidden state, or filename/path conventions.

No method mapping, profile inference, world inference, request-ID inference, commitment inference, or expected inference is permitted. Expected identity values are comparison operands only after the real execution has completed and the actual owner/view has been acquired. They never construct or validate the request before execution.

## 6. Exact future RED requirements and mutation tests

A later explicit RED authorization must require all of the following in the existing v3 RED, without changing production source first:

1. strict decode and all-85 physical enumeration of `driver.request.identity`;
2. exact validation described in §3 before request construction;
3. conversion of the three driver values only into the exact private typed arguments of `privateCallHierarchyLayer1WithIdentity`;
4. request construction that calls the identity setter exactly once per occurrence and preserves every other attachment field;
5. all 85 real executions using only independently authored driver values;
6. post-execution comparison of acquired actual A116 identity values to expected values, never pre-execution feed;
7. mutation tests for absent identity, each absent field, nulls, wrong JSON types, empty strings, upper-case hex, prefix, non-hex, lengths 63 and 65, all-zero nonce, duplicate members, extra members, and trailing JSON;
8. mutation tests changing each of profile, binding, and one nonce nibble independently, proving the corresponding actual A116 identity changes while method, request key, commitments, transport, external result/error, and all unrelated attachment fields remain unchanged;
9. setter tests proving only the three owned fields change and every other comparable field is preserved exactly;
10. constructor tests proving exact typed values are copied verbatim into the attempt and then the immutable observation/view; and
11. guards proving no forbidden inference source is read for identity construction.

Any test that manufactures expected values from driver values, driver values from expected values, or both from one shared helper is invalid.

## 7. Mechanical occurrence and binding counts

The future corpus-byte review must mechanically traverse the physical corpus exactly as stored—roots, W24 workers, W25 standalone executions, and W25 pair members—and report:

```text
all physical envelope occurrences = 85
profile literal counts             = exact frequency table by literal string
binding literal counts             = exact frequency table by literal string
incoming binding occurrences       = mechanically counted exact set
outgoing binding occurrences       = mechanically counted exact set
other/unknown binding occurrences  = mechanically counted exact set; required zero before GO
```

Incoming/outgoing membership must be counted from the independently authored literal binding values themselves, not inferred from method, world, expected data, or occurrence names. The review must also list the exact occurrence IDs in each binding bucket so totals reconcile to 85 and each occurrence appears once.

These counts are mechanical accounting only. They do not authorize a binding vocabulary change, value repair, direction inference, grouping, profile selection, corpus mutation, RED mutation, source mutation, C15 classification, or production admission.

## 8. Exact corpus-byte review sequence

Only after explicit corpus/RED authorization, the byte review must proceed in this order:

1. pin and verify the authorized clean repository ancestry and exact authorized files;
2. preserve immutable pre-mutation bytes and SHA-256 for every affected v3 world, manifest, registry if named by the authorization, aggregate, and RED file;
3. verify the separately reviewed all-85 literal assignment ledger and provenance without reading expected values as authoring operands;
4. rematerialize only the authorized driver identity object in each of the 85 envelopes;
5. strict-decode and validate every resulting envelope, exact physical occurrence count, exact profile/binding frequency tables, exact incoming/outgoing binding counts, and zero other/unknown bindings;
6. compare decoded pre/post worlds after recursively removing only `driver.request.identity`; every remaining decoded value, key, array order, omission, and expected subtree must be exactly equal;
7. compare raw canonical bytes and prove changes are limited to the authorized identity additions plus mechanically necessary v3 lengths, SHA-256 values, and aggregate metadata;
8. execute the separately authorized RED and require the intended RED failure at the missing production identity carrier, with no accidental compile failure or fixture-only pass;
9. publish exact hashes, byte deltas, counts, failures, and stop conditions for independent review; and
10. issue no GO unless a reviewer explicitly accepts the byte review and the RED boundary.

## 9. C15 candidate additions and preserved exact three-file boundary

Relative to the current proposed closed C15 candidate set, this amendment proposes exactly these additions:

### Field candidates

```text
callHierarchyLayer1RequestAttachment.profile
callHierarchyLayer1RequestAttachment.binding
callHierarchyLayer1RequestAttachment.workspace
```

### Function/method candidate

```text
(callHierarchyLayer1RequestAttachment).privateCallHierarchyLayer1WithIdentity
```

### Existing body changes, not new declaration candidates

```text
newCallHierarchyLayer1GenericAttemptLineage
```

The setter introduces three plain assignments and no allocation/copy candidate beyond by-value comparable copies. Constructor assignment likewise introduces no allocation. Canonical C15 enumeration must mechanically enumerate the actual delta from the authorized source and retain every classification as `UNKNOWN`; the list above is an exact intended candidate delta, not corpus authority and not a classification. Every extra or missing field, declaration, function, method, call, composite literal, allocation, or copy candidate is a hard stop requiring a new amendment and independent review.

The exact three-production-file boundary remains unchanged:

1. `sessionruntime/sessionruntime.go`;
2. `sessionruntime/owned_method_pair.go`; and
3. `sessionruntime/b4_owned_method_pair_lease.go`.

The three fields belong with the existing comparable attachment in `b4_owned_method_pair_lease.go`; the value-returning setter belongs with the existing private attachment setters in `owned_method_pair.go`; existing request/attempt/finalization plumbing remains bounded to `sessionruntime.go` and the already proposed symbols. No fourth production file, public API, schema, command, adapter, provider, catalog, persistence, replay, telemetry, configuration, or activation file is admitted.

## 10. Prior disposable v2 remains blocked

The prior disposable v2 attempt is historical and remains blocked pre-mutation. It did not establish this independent all-85 identity carrier and produced no canonical C15 result. Its source, output, counts, proposed candidates, and failure state cannot authorize corpus, RED, source, tests, candidate classification, or a replacement disposable run. Nothing in this amendment retroactively upgrades that attempt.

## 11. Required authority sequence

The only permissible progression is:

```text
this document review
→ explicit corpus/RED authorization
→ exact corpus-byte review
→ explicit RED GO
→ exact three-file write-set update
→ independent write-set/C15 review
→ explicit authorization for a new disposable run
```

No stage implies the next. In particular, documentation review does not authorize corpus or RED mutation; byte review does not authorize source; RED GO does not authorize implementation; write-set review does not authorize a disposable run; and a disposable result does not authorize canonical C15 changes or production use.

## 12. Disposition

This document specifies the correction required to make the three missing A116 identity values causally request-owned. It is a review candidate only.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_IDENTITY_CAUSAL_CARRIER_REVIEW_REQUIRED
```

No GO token is issued. No authority is granted.
