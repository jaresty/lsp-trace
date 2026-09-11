# Capture-to-seeds export foundation contract

## Scope

`internal/seedexport` is an offline, transport-neutral projection foundation. It accepts either already-loaded Graph Provenance V5 bytes or a caller-supplied `ValidatedCarrier`. It does not parse CLI arguments, publish MCP operations, define a seeds schema, acquire source, or perform filesystem, network, LSP, Git, clock, or live-source operations.

The neutral `Model` is intended for later encoding as `lsp-trace.seeds.v2`; this package neither defines nor duplicates that schema.

## Availability boundary

Graph Provenance V5 alone is insufficient to reconstruct exact original acquisition targets. Its embedded graph retains invocation seed labels and textual `at` values, but it does not retain a structured exact position-or-symbol discriminator together with every target's original `down_depth` and `up_depth`. `ExportV5` therefore returns typed `NOT_AVAILABLE` with `ORIGINAL_REQUEST_CARRIER_MISSING`. It never substitutes graph nodes, frontier records, names, source bodies, CALLS, or resolved targets.

An available export requires a caller-authoritatively-validated `ValidatedCarrier` containing:

- the exact admitted Graph Provenance V5 bytes;
- every original requested target, including a unique original ordinal, retained ID and label, exact locator kind and value, URI/path semantics, zero-based position or symbol, language ID when retained, and per-target down/up bounds;
- session, generation, invocation, source, and revision references;
- exact envelope and embedded graph SHA-256 references; and
- the validated manifest SHA-256 plus manifest session, generation, and invocation foreign keys.

These are binding references, not authentication or custody evidence. Available models always state `CustodyClaimed == false`.

## Canonical order

Targets are ordered by the retained zero-based original-request ordinal. The set of ordinals must be exactly the unique range `[0,n)`. Carrier slice order, graph node order, traversal order, names, and relation order do not affect output.

## Fail-closed outcomes

- `AVAILABLE`: the typed original-request carrier is complete, unique, exact, and attributable to the admitted V5 capture.
- `NOT_AVAILABLE`: exact original targets or a required original manifest binding carrier is absent. The caller may supply the missing validated carrier; the exporter does not infer it.
- `INVALID`: supplied values are malformed, duplicated, ambiguous, inconsistent, or mismatch the admitted capture.

Locators must select exactly one of a complete zero-based position or a non-empty symbol. Bounds are inclusive integers from 0 through 64. Target IDs, labels, and ordinals are unique; invocation seed labels must attribute exactly one graph seed to exactly one original target.

Diagnostics use fixed codes and generic bounded messages. They do not interpolate source bodies, private paths, locator values, or caller strings. The package emits at most eight diagnostics and current outcomes emit one. Body completeness is always `UNASSESSED`.

## Qualification mutations

`internal/seedexport/export_test.go` covers:

- absent targets;
- duplicate target IDs and ordinals;
- inconsistent invocation/manifest bindings and absent manifest binding;
- malformed and mixed locator forms;
- malformed per-target bounds;
- envelope/graph provenance mismatch;
- original-ordinal canonical ordering independent of carrier order;
- exact locator-kind, zero-based-coordinate, path/URI, ID/label, and bounds preservation;
- bounded diagnostics that do not echo private source/path/body material; and
- raw V5 with graph nodes and CALLS still returning `NOT_AVAILABLE`, proving there is no node-derived fallback.

Body completeness is deliberately not assessed by these exports.
