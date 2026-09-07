# Preparatory retained PATH kernel

`internal/retainedpath` is a dependency-neutral extraction of the historical
bounded-retained-analysis v1 path implementation. **This is not FR20 acquisition
support or FR20 completion.** No new CLI, MCP, schema, or admission contract is
introduced. Component, metric and ranking algorithms and numeric policies are
unchanged.

## Coordinator-facing API

```go
out, in := retainedpath.Indexes(edges)
work := &retainedpath.Budget{Context: ctx, Left: maxPathWork}
result, err := retainedpath.Search(nodes, edges, startID, endID, work)
err = retainedpath.Prove(edges, startID, endID, result.Status, result.Path)
```

- `Edge` carries caller/callee IDs, group ID and ordered opaque witness strings
  in `OccurrenceIDs`, plus uninterpreted historical provenance fields.
  `boundedanalysis.Edge` and `boundedanalysis.Path` remain local defined types
  with the same underlying fields/tags, converted explicitly at the boundary.
  This preserves existing unkeyed literals without external-type vet warnings.
- Callers admit node/edge consistency and resource ceilings. Missing search
  endpoints are errors, including equal-but-missing endpoints; they consume no
  work. IDs must be exact, not names or inferred identities.
- `Indexes` allocates adjacency slices ordered by `(callee, group ID)` outgoing
  and `(caller, group ID)` incoming, without sorting the caller's edge slice.
- Search minimizes unit-group hops. It neither interprets weights nor reverses
  stored edges. A selected group retains all its witnesses in their original
  order, including zero-site groups (empty lists remain distinct from nil).
- `Budget` is caller-owned, sequentially reusable, and not concurrency-safe.
  Supply a non-nil context and a nonnegative remaining work count. Each dequeue
  and examined edge costs one tick, including parallel or already-seen edges.
  A zero-hop path costs one tick. Cancellation takes priority over exhaustion.
  Index construction and proof are outside the historical path-work counter;
  callers must bound admitted graph cardinalities separately.
- Results are `FOUND`, `NOT_FOUND_IN_RETAINED_GRAPH`, or `INCOMPLETE`.
  Incomplete reasons are `LIMIT` or `CANCELLED`; incomplete witnesses are empty.
  `Budget.Left` records remaining work and `Budget.Reason` the terminal reason.
  Do not reuse a terminal budget to resume a truncated query.
- `Prove` uses reverse distances, independently of producer predecessor BFS,
  checking shortestness, lexical ties, continuity and exact witnesses. It is
  only the historical path proof, not a complete artifact validator: callers
  still check admission, status/payload shape, cancellation/resource accounting
  and deterministic replay. It has no source/authentication authority.

An acquisition coordinator may use native group IDs and callsite pointers as
opaque witness strings, then map them to exported occurrence IDs later. No
artifact digest, retained-call schema or provider identity enters this kernel,
so it does not require a circular digest preimage.

## Regression evidence

`internal/boundedanalysis/testdata/path-kernel-*.json` was captured from the
pre-extraction implementation at baseline `464fff8`: six complete canonical
analytical artifacts and a diamond/parallel-group topology result. Tests compare
bytes without an update mode. The compiling rejecting stub failed a parity
assertion before production extraction. The existing 512-topology oracle and
independent proof tests remain unchanged; direct kernel tests additionally pin
work boundaries, shared budgets, missing IDs, direction and witness corruption.
