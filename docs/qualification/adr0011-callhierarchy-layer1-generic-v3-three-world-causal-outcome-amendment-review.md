# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Three-World Causal Outcome — Independent Review

Status: **DOCUMENTATION GO ONLY**

- Reviewed HEAD: `382e06017c03bca5b145ebd6135c3eaafcbc1414`
- Scope: world-04/request 104, world-05/request 105, world-23/request 123
- No corpus, RED, source, tests, prototype, or C15 execution was modified.

The amendment defines independently authored typed causal inputs:

- worlds 04 and 05 carry an opaque accepted terminal classification `successful-empty`, consumed verbatim after transport without inspecting result content;
- world 23 carries an explicit pre-effect refusal class/reason `method-unavailable` / `method unavailable`, consumed before pending, writer, transport, or accounting effects.

The values are not derived from request IDs, world or pair names, methods, expected values, decoded result content, transport bytes, errors, or implementation output. The other 82 occurrences retain the exact zero/absent behavior.

The proposed private attachment fields, finite comparable types, and value-returning setter remain within the existing three-file boundary and preserve attachment comparability. Invocation order and attempt custody are coherent with the existing request/result carrier design. The pre-effect refusal bypasses transport and admitted finalization while using the reviewed refusal publication lifecycle.

Expected path ownership and aggregate accounting remain unchanged. Strict decoding, anti-oracle mutation guards, and exact C15 additions are finite. The rejected patch `4c379a0709addbdfe84ca716dbefd317ec8037668658cf80b920c97096df02bd` remains negative evidence only.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_THREE_WORLD_CAUSAL_OUTCOME_GO
```
