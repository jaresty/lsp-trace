# ADR-0011 call-hierarchy Layer-1 generic corpus v2 bytes review v2

## Disposition

**GO.** The corrected additive v2 corpus satisfies the frozen amendment and supersedes the prior bytes review's corpus disposition. The prior blocked review remains preserved unchanged as the record of the rejected candidate.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_V2_BYTES_GO
```

This GO authorizes only creation of the additive v2 RED under the amendment's existing boundary. It does not authorize implementation, activation, generated output, v1 mutation, corpus rewriting, or a commit.

## Reviewed state and scope

The independent rereview used exact checkout `/Users/schwa/dev/lsp-trace-adr0011-c17` at:

```text
525d565688cc019ee3190bf96bef0a7eda47ff60
```

The review was read-only with respect to both corpora, tests, implementation, and the preserved prior review. Only this superseding review file was created. Pre-existing untracked state was not modified.

## Corrected blocker

The prior review found four forbidden empty containers in every one of 85 envelopes, for 340 forbidden occurrences. Independent recursive traversal of the corrected corpus now finds zero occurrences of:

```text
expected.frames.request.body
expected.frames.request.payload
expected.frames.response.body
expected.frames.response.payload
```

`expected.result.form` is also absent. The correction removes the forbidden containers rather than retaining them as empty placeholders or replacing them with another semantic field.

## Exact expectation inventory and retained equality

The amendment's exhaustive inventory remains exact and reconciled:

```text
v1 expected paths       162
preserve generic        133
preserve opaque          12
remove/defer              17
successor expected       145
```

The 17 removed paths are exactly the four request-body leaves, four request-payload leaves, four response-body leaves, four response-payload/result leaves, and `result.form`. Driver paths remain outside this denominator.

Every root and inline v2 `expected` value was compared at its envelope boundary with the corresponding accepted v1 record after deleting exactly those 17 paths. The 16 named `derived.*` values remain the separately fixed v1 RED derivations. The rereview found:

```text
retained_literal_projection_mismatches 0
forbidden_container_occurrences       0
```

No retained scalar, null, array order, opaque commitment, complete-frame value, external byte value, accounting value, ledger value, lifecycle value, refusal value, control value, or transport value changed.

## Cardinality and self-containment

Recursive traversal reproduced the required complete-envelope cardinalities:

```text
root envelopes                         25
world-24 inline worker envelopes       16
world-25 inline execution envelopes    22
world-25 ordered pairs                 11
pair member envelopes                  22
all envelope occurrences               85
```

Every occurrence has exactly the top-level siblings `driver` and `expected`. Each world-24 worker, world-25 execution, and disabled/enabled pair member carries its own driver; no record inherits a root driver or requires an ambient table.

## Driver schema and source equality

All 85 drivers independently satisfy the finite generic schema:

- top-level keys are exactly `derived`, `request`, and `transport`;
- `driver.derived` contains exactly `external`;
- `driver.derived.external` contains exactly the four required fields, in canonical order: `notifications_bytes`, `responses_bytes`, `result_bytes`, and `server_error_bytes`;
- request fields are exactly method, session, generation, limits, deadline, `params_state`, and `params_hex`;
- transport contains exactly ordered `events`, with event kinds limited to `response-frame`, `transport-error`, `cancel`, `deadline`, `shutdown`, and `unavailable`, and only each kind's permitted keys;
- byte strings are lowercase, prefix-free, even-length hexadecimal; null, empty, and nonempty states remain distinct;
- `params_state` and `params_hex` reproduce the accepted v1 request-payload availability and bytes;
- the four external byte fields reproduce their accepted v1 sources under the required availability cross-rules; and
- every response-frame event reproduces the accepted complete v1 response-frame bytes.

The rereview found zero driver-to-v1 source mismatches. No driver key supplies `result.form`, a request/response body or payload subspan, offsets, direction, symbol/item/range/call-site/caller/callee/provider semantics, a semantic world name, or another semantic dispatch key.

## Canonical manifest, files, and aggregate

The manifest and all 25 world files strictly decode without duplicate keys and are canonical compact recursively key-sorted UTF-8 JSON with exactly one terminal LF. Paths are the exact secure lexical sequence `worlds/world-01.json` through `worlds/world-25.json`; the manifest and directory have no missing or extra world file. Every complete world envelope reproduces its manifest byte length and SHA-256.

The corrected manifest complete-file SHA-256 is:

```text
8b63fce07273a40e7630bd1d7510bc6c0618f8d3af10a1d00387e814f5878541
```

Concatenating the 25 complete world envelopes in manifest order independently reproduces:

```text
aggregate byte_length 505337
aggregate sha256      df3e703096bd5617ac2d6696800fb69e115cb9f8c58edb80513d55581a846ca8
```

The manifest hashes whole `{driver, expected}` envelopes, not `expected` alone.

## V1 immutability and reachability

The required historical commits remain reachable ancestors of the reviewed HEAD:

```text
ea049c2dad9be74e1b3e316fe042430be7f04c8e
12d9af179c49412c8203582a3fa52d7836da18b7
751b0276c46dbccc4899cfec32adf513c123b57f
4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9
```

The normative corpus commit, later accepted corpus commit, and reviewed HEAD all resolve the v1 corpus subtree to:

```text
7d278859a92b2f4438cce3c1f80f1591cd9a8c1a
```

The current v1 manifest remains SHA-256:

```text
c761601bf78195634163b98676e133bac2dff610c521be149b3445c9f620492a
```

The accepted opacity-corrected v1 RED fixture remains Git blob and file SHA-256:

```text
daea113100184745998cde6961d3164c0a3431ce
e766074f72b2ff7d43a59d9532b52b3d5c150e4d30f3b61f431c9fab6a540dc2
```

Historical internal `source_head` `4f2b5504...` remains reachable and still does not contain the v1 corpus path. It remains metadata only, not a derivation source.

## Authority boundary

This review establishes bounded corpus-byte, projection, schema, cardinality, manifest, and historical-pin conformance only. It does not establish source completeness, runtime execution, production use, feature identity, or future implementation behavior. The successor RED must still preserve generic causal dispatch, external transparency, and the finite owner/view/lease lifecycle requirements under source review.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_V2_BYTES_GO
```
