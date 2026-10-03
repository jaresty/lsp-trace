# Bounded host-assisted caller inference example

This is one reusable example of a host LLM answering a caller-provided-inference question from bounded lsp-trace evidence. It is not isolated item-independent Describe execution, qualification, feature acceptance, completeness, reproducibility, or public enablement. It adds no retention or index service.

- `authority=0`
- `accepted=false`
- `completeness=UNKNOWN`
- `source_graph_complete=UNKNOWN`

## Fresh-checkout recipe

This procedure depends only on the checked-out repository, its advertised managed lsp-trace tools, and source disclosure authorized for the current host session. It does not depend on ignored `.pi/evidence` files or retained local-model packets.

1. State the exact question and target, and confirm that the selected source may be disclosed to the current host provider.
2. List managed sessions. Select only the exact-workspace session in `READY`; record its session ID, generation, server, and position encoding. Do not launch a replacement server to repair an unavailable managed session.
3. Use source or symbol search only to locate the target. Obtain `CALLS` from a bounded server-reported incoming request, preserving every returned caller alternative, request limit, partial result, and diagnostic.
4. Select one caller using an explicit bounded rationale. Do not describe that choice as global deterministic consumer selection or SCC analysis.
5. Request source-bearing `lsp_trace_v2_structural_context` with `PROJECTED`, `INCLUDE`, and `FULL_DEFINITION` for the target and selected caller. Preserve custody, identities, ranges, accounting, and omissions. Retained evidence may instead use its advertised retained-inspection operation, but must not be silently repaired with live source.
6. In the host session, explain only the target's visible inputs and result or failure, and how the selected caller visibly uses that outcome. Quote exact source and distinguish a visible wrapper from delegated implementation that was not selected.
7. Give an independent reviewer all selected evidence—not only the quoted lines—and assess source support, usefulness, citation accuracy, and limitation accuracy separately. Preserve the initial review and any later correction as different events.
8. Stop rather than infer when the managed session, server-reported relationship, complete selected bodies, or disclosure authorization is unavailable.

A fresh-checkout verification should confirm that every documentation link and referenced source path exists, that the recipe contains no ignored-evidence dependency, and that the managed exact-workspace gate succeeds in that checkout. Tool availability establishes a contract shape; each execution must still preserve its own result and limitations.

## Question

> What does `(*lifecycleops.Service).List` provide to `(*Service).ListForURI`, and how does that immediate caller use it?

## Evidence and selection

The host used exact-workspace READY managed session `sk1:949e027243da188035cc5c6532c4cb10df4210f2bdde80acc58b7b7b9af5f283` for `file:///Users/schwa/dev/lsp-trace`, preserving generation `1`, `gopls`, and `utf-16`. The source identity was digest `sha256:a1060b0c3b56b335762d041509967b356cc5bee597c28ec80a39180740f2f683`, document version `2`, with `LIVE` custody and authority `0`.

Text search may locate a symbol but never establishes `CALLS`. The incoming edge was server-reported as `SERVER_REPORTED_CALL_HIERARCHY`: relation `sha256:b715a8caea4f67fdef70ac5e166b6026916a61018d1f2226e60f45c926f1184d`, caller to callee, at `[128:15-128:19]`.

The depth-1 incoming result was `PARTIAL` at `max_nodes=32`: 6 nodes and 5 relationships. It returned every preserved caller alternative: `Executor.actionableFailure`, `Executor.Execute`, `Service.ListForURI`, `TestListIsImmutableDeterministicAndBounded`, and `TestConcurrentCallersOnlyConsumeRuntimeSnapshots`. `ListForURI` was selected because it was the smallest returned production caller with a complete body and direct occurrence in a compact single-document projection. This is not deterministic global consumer selection or SCC analysis.

Selected bodies used `PROJECTED` + `INCLUDE` + `FULL_DEFINITION`. The broader projection was `COMPLETE`, selected 10 of 13 units, omitted 3, and emitted 9,895 bytes. The focused caller projection was `COMPLETE`, selected 1 of 1, and omitted none. A prior candidate, `acquisitionops.Executor.Execute`, failed `NODE_BOUND` with `max_nodes=20`, observed 21; that attempt is preserved as workflow history, while this recipe uses the compact target.

Source ranges:

- target `(*Service).List`: declaration `[119:18-119:22]`, full body `[119:0-125:1]`
- caller `(*Service).ListForURI`: declaration `[127:18-127:28]`, full body `[127:0-160:1]`

## Host answer

`List` takes no explicit argument and creates a snapshot from the service runtime. It copies records and observations into new slices, sorts records by `SessionID` and observations by `Sequence`, and returns those plus the runtime census:

```go
records := append([]sessionruntime.Record(nil), s.runtime.Records()...)
sort.Slice(records, func(i, j int) bool { return records[i].SessionID < records[j].SessionID })
observations := append([]sessionruntime.Observation(nil), s.runtime.Observations()...)
sort.Slice(observations, func(i, j int) bool { return observations[i].Sequence < observations[j].Sequence })
return ListSnapshot{Sessions: records, Observations: observations, Census: s.runtime.Census()}
```

`ListForURI` consumes that returned snapshot—not a separate failure from `List`—by iterating `snapshot.Sessions`. Invalid or non-file input returns `FailureInvalidURI`. It keeps only `Ready` sessions whose normalized workspace equals or contains the requested file URI path, prefers the longest matching workspace, and returns `FailureApplicableNotFound` for zero best matches or `FailureApplicableAmbiguous` for multiple best matches. The filtering citation preserves the complete condition:

```go
if record.State != session.Ready || (uriPath != workspace && !strings.HasPrefix(uriPath, strings.TrimSuffix(workspace, "/")+"/")) { continue }
```

On exactly one match, it augments the snapshot with `SessionResolution` and returns it with `FailureNone`:

```go
snapshot.Resolution = &SessionResolution{URI: rawURI, SessionID: matches[0].SessionID, Generation: matches[0].Generation}
return snapshot, FailureNone
```

The visible wrapper therefore supplies a copied runtime snapshot whose records are sorted by `SessionID` and observations by `Sequence`, and resolves one applicable READY session for a file URI. The delegated implementations of `runtime.Records()`, `runtime.Observations()`, and `runtime.Census()` were not present in the selected source, so their behavior is not inferred here.

## Review and limitations

An independent reviewer saw all selected evidence. The initial assessment was: source support `PASS`, usefulness `PASS`, citation accuracy `NEEDS_CORRECTION`, limitation accuracy `PASS`, overall `USEFUL`. The answer above subsequently received mechanical corrections: it cites the observations-by-`Sequence` sort, quotes the full `Ready`/workspace-prefix condition rather than an ellipsis, and avoids claiming deterministic tie ordering. Those corrections were verified against the cited source; they were not a second independent semantic acceptance.

The initial review checked source support, usefulness, citation accuracy, and limitation accuracy against all selected evidence. Neither that review nor the subsequent mechanical correction raises authority or acceptance. Incoming traversal remained `PARTIAL`; the evidence establishes neither global caller completeness nor SCC analysis. It did not execute the runtime, identify a feature, establish source-graph completeness, or inspect the delegated runtime implementations. Ordinary host context is not reproducible isolated execution.
