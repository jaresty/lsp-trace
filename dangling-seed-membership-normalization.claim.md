# Published seed memberships are reference-closed

Every published seed membership whose evidence kind identifies a node (`PREPARED_TARGET` or `REACHED_NODE`) references a node published in the same graph artifact.

The generic slice pipeline preserves the raw prepare/outgoing/incoming observations and failed-preparation diagnostics. A call-hierarchy item rejected by structural validation is not published as a node and is not represented as a successful prepared-target membership. Valid prepared items from the same response remain published and retain their memberships.

No node is invented, no text is hashed into a substitute identity, malformed relations remain rejected, and partial evidence remains incomplete with its diagnostics. This change does not add or alter AngularJS provider semantics.

## Retained RED

The assertion-specific Survey/DASL-shaped fixture returned one valid and one malformed prepared item, traversed the valid outgoing leaf and incoming boundary, then failed publication at the seed foreign key:

```text
ASSERT_SLICE_REJECTED_PREPARED_SEED_MEMBERSHIP_REFERENCES_PUBLISHED_NODE: failure=INTERNAL: json: error calling MarshalJSON for type graph.Result: dangling seed node id "ddc2b3381d7cee510a5c86f73ff4530421358e31eecf20351abd72b34a25ebb8" calls=[textDocument/prepareCallHierarchy: callHierarchy/outgoingCalls:surveyControl callHierarchy/outgoingCalls:loadQuestions callHierarchy/incomingCalls:loadQuestions]
```

Command:

```text
go test ./sliceops -run '^TestSliceRejectedPreparedSeedDoesNotPublishDanglingMembership$' -count=1 -v
```

## GREEN

- Exact regression, including repeated byte-for-byte replay: PASS.
- Malformed outgoing node, ambiguous presentations, and existing deterministic slicer replay: 6/6 PASS.
- Focused graph/slicer/traverse/sliceops/direct CLI: 392/392 PASS.
- Full Go: 3343/3343 PASS.
- Race: 3343/3343 PASS.
- CI: format, test, vet, build, Python, shell, release, cleanliness, and GoReleaser dry-run PASS.
- Release: documentation, graph-v1/v2/v3 schema bytes, custody, CLI/MCP parity, omission, archive, and core archive boundaries PASS.
- GoReleaser: configuration check and snapshot release PASS.
- Ember provider: 95/95 PASS (89-test baseline plus 6 preserved analysis-only type-overlay tests from the approved parent commit).
- Caller-project qualification: 48/48 attempts PASS.
- Exact immutable DASL/Survey/Market six-example replay: UNAVAILABLE (no real corpus path/config was present; tracked Market View data is synthetic and has a different denominator).

## Derivation

1. `sliceops.Executor.Execute` retains the raw prepare response and passes it to `slicer.DiscoverPrepared`.
2. `slicer.DiscoverPrepared` validates each prepared item. It records a `slice-prepare` diagnostic and excludes malformed items from `Discovery.Nodes` and `Discovery.StartNodeIDs`; valid items continue through outgoing and incoming traversal.
3. Before this fix, `sliceops.decorate` independently recomputed `PreparedTargetIDs` from the raw prepare response. That reintroduced the rejected item ID after discovery had intentionally excluded its node.
4. Graph v3 publication projected every `PreparedTargetID` into a `PREPARED_TARGET` seed membership. `ValidateReferences` correctly rejected the resulting dangling seed node ID, producing `OUTPUT_VALIDATION_FAILED`/internal publication failure rather than silently publishing invalid evidence.
5. The minimal normalization rule is to derive successful prepared-target membership from the already validated, sorted `Discovery.StartNodeIDs`. This retains authoritative valid prepared nodes and removes membership only for items intentionally rejected by structural validation.
6. Reached-node membership continues to derive from the merged published node set; failed-preparation accounting, terminal/boundary diagnostics, incompleteness, ordering, canonical bytes, custody, and graph-v1/v2/v3 behavior remain unchanged.
