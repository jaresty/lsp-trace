# ADR-0011 call-hierarchy Layer-1 generic corpus v2 bytes review

## Disposition

**BLOCKED.** The candidate corpus is not eligible to authorize the v2 RED. Although its retained scalar and array values reproduce the accepted v1 oracle after the 17 specified leaf deletions, every one of the 85 `{driver, expected}` envelopes retains four forbidden empty subspan container fields. The strict successor schema says those fields do not exist.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_V2_BYTES_BLOCKED
```

This disposition authorizes no corpus correction, v2 RED, implementation, activation, generated output, or commit.

## Reviewed state and scope

The review used exact checkout `/Users/schwa/dev/lsp-trace-adr0011-c17` at:

```text
525d565688cc019ee3190bf96bef0a7eda47ff60
```

Before this review file was created, the checkout had exactly 26 untracked files: `manifest.json` and 25 world files under `sessionruntime/testdata/callhierarchy-layer1-red-v2/`. There were no other tracked or untracked changes. The candidate corpus was reviewed read-only. No corpus, test, implementation, generated artifact, or other source file was edited.

## Manifest, canonical bytes, paths, and aggregate

The v2 manifest strictly decoded without duplicate keys and is canonical compact JSON with recursively sorted object keys and one terminal LF. Its complete-file SHA-256 is:

```text
d5d9468373257112a767f88215c623d4314669d6f95f2ebdb6251d61a1f610b4
```

It declares the exact schema and source pin:

```text
schema      adr0011.callhierarchy.layer1.red-oracle-corpus/v2
source_head ea049c2dad9be74e1b3e316fe042430be7f04c8e
```

The manifest contains exactly 25 unique secure relative paths in lexical order, `worlds/world-01.json` through `worlds/world-25.json`. Those are exactly the files present under `worlds/`; there are no missing or extra world files. Each world strictly decoded, was canonical compact JSON with one terminal LF, and reproduced its manifest byte length and SHA-256.

Concatenating the 25 complete world files in manifest order independently reproduced:

```text
aggregate byte_length 509247
aggregate sha256      019df85533b60baf9c63249c6767efc1522e14f07e04c6ec435a78897c3893ff
```

The manifest hashes complete envelopes, not `expected` alone.

## Cardinality and self-containment

Mechanical envelope traversal reproduced all required cardinalities:

```text
root envelopes                         25
world-24 inline worker envelopes       16
world-25 inline execution envelopes    22
world-25 ordered pairs                 11
pair member envelopes                  22
all envelope occurrences               85
```

Every occurrence is self-contained with exactly the top-level siblings `driver` and `expected`; W24 workers, W25 executions, and both members of every W25 pair carry their own driver.

## V1 projection and retained values

For every root and inline record, the review recursively unwrapped v2 envelopes and compared `expected` with the corresponding accepted v1 `record` after deleting exactly these 17 leaf paths:

- four `frames.request.body.*` leaves;
- four `frames.request.payload.*` leaves;
- four `frames.response.body.*` leaves;
- four `frames.response.payload.*` leaves; and
- `result.form`.

The 16 named `derived.*` expectations were assessed separately as RED derivations rather than v1 corpus literals. After that separation, the retained decoded-value comparison produced:

```text
retained_literal_projection_mismatches 0
```

Therefore all 133 generic and 12 opaque retained expectation values are unchanged, including scalar/null values, array order, opaque commitment metadata, frame bytes/digests, external bytes, accounting, ledger, lifecycle, refusal, control, and transport values. No replacement semantic value was found.

## Blocking strict-schema defect

Assumption and basis: the amendment states that the finite successor schema has no request/response body or payload/result subspan field, and explicitly forbids those subspans. An empty object is still a present JSON field. The candidate retains these fields in every envelope:

```text
expected.frames.request.body      {}
expected.frames.request.payload   {}
expected.frames.response.body     {}
expected.frames.response.payload  {}
```

Observed count:

```text
4 forbidden fields × 85 envelopes = 340 forbidden field occurrences
```

These empty containers are not among the 145 retained expectation paths and are not permitted placeholders. Consequently each concrete `expected` subtree is not strict-schema-equal to the required v1-minus-17 projection with no removed semantic fields. This fails the exact 145-path strict expected-schema requirement even though the retained leaf values themselves are unchanged.

## Driver validation

Across all 85 envelopes, each driver has only the finite generic top-level keys `derived`, `request`, and `transport`. The checked nested schemas contain:

- exactly four required `driver.derived.external.*_bytes` fields;
- the exact request fields for method, session, generation, limits, deadline, `params_state`, and `params_hex`;
- exactly `transport.events`, with event kinds limited to `response-frame`, `transport-error`, `cancel`, `deadline`, `shutdown`, and `unavailable` and kind-specific key sets only.

All driver byte fields obey lowercase, prefix-free, even-length hexadecimal syntax when present. Null, empty string, and nonempty string states preserve unavailable, available-empty, and available-nonempty distinctions. `params_state` and `params_hex` reproduce the committed v1 request-payload availability and bytes. The four external byte fields reproduce the corresponding v1 notifications, responses, result, and serialized-error bytes under the required availability cross-rules. Response-frame event bytes reproduce the committed complete v1 response-frame bytes; world 7 correctly uses a transport-error event rather than a response-frame event.

No driver key provides `result.form`, request/response body or payload subspans, offsets, direction, symbol/item/range/call-site/caller/callee/provider semantics, or a semantic world-name dispatch key. World/root/record names occur only in expected oracle data, not as driver dispatch fields. No implementation-produced output was used in this review or is needed to reproduce the retained oracle values: driver/oracle consistency is mechanically anchored to the pinned v1 corpus and accepted v1 RED fixture. This is corpus evidence, not proof that a future RED implementation will avoid semantic dispatch; that must remain a RED source-review requirement.

## Source pins, v1 bytes, and reachability

All required commits resolve and remain ancestors of the reviewed HEAD:

```text
ea049c2dad9be74e1b3e316fe042430be7f04c8e reachable
12d9af179c49412c8203582a3fa52d7836da18b7 reachable
751b0276c46dbccc4899cfec32adf513c123b57f reachable
4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9 reachable
```

The normative and later accepted corpus commits, and the current HEAD, all resolve the v1 corpus subtree to the identical tree:

```text
7d278859a92b2f4438cce3c1f80f1591cd9a8c1a
```

The accepted opacity-corrected RED fixture and current HEAD both resolve `sessionruntime/b4_owned_method_pair_lease_test.go` to blob:

```text
daea113100184745998cde6961d3164c0a3431ce
```

Current v1 file hashes reproduce the accepted pins:

```text
v1 manifest SHA-256  c761601bf78195634163b98676e133bac2dff610c521be149b3445c9f620492a
v1 RED fixture       e766074f72b2ff7d43a59d9532b52b3d5c150e4d30f3b61f431c9fab6a540dc2
```

The historical internal `source_head` commit `4f2b5504...` remains reachable but does not contain the v1 corpus path, as required. It remains metadata only, not a derivation source.

## Required correction and authority boundary

A corrected candidate must remove the four empty forbidden container fields from every `expected` occurrence, regenerate canonical world bytes and manifest hashes, and undergo a fresh independent bytes/schema review. This review does not authorize that correction.

A future GO at this gate would authorize only creation of the additive v2 RED. It would not authorize implementation, corpus rewriting, v1 changes, activation, generated output, or a commit. The present disposition is BLOCKED, so even v2 RED creation is not authorized.

ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_V2_BYTES_BLOCKED
