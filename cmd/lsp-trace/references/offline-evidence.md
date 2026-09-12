# Offline evidence operations

Offline operations do not start a language server, reacquire source, authenticate producers, or admit feature semantics.

## Inspect retained evidence

Choose exactly one mode:

```sh
lsp-trace inspect SELECTOR_OR_ARTIFACT --seed LABEL --json
lsp-trace inspect SELECTOR_OR_ARTIFACT --all-seeds --json
```

Both emit `lsp-trace.inspect.v1` with authority `NON_AUTHORITATIVE_DERIVED_VIEW`. Selector ingress verifies complete-generation exact-byte custody before structural and semantic validation. Direct artifacts gain no custody claim from their computed digest. Aggregate inspection copies native records once and references them from per-seed collections. Unknown labels, invalid input, and missing or duplicate same-label aggregate results fail without JSON.

`inspect-hydrated` retrieves exact retained node/relation context. Keep bodies opt-in and bounded. Hydration does not reacquire current source or upgrade authority. Use it when exact retained context is required after a structural trace identifies focus IDs.

## Compare seeds mechanically

```sh
lsp-trace inspect evidence.selector.json --all-seeds --json > evidence-inspection.json
lsp-trace filter evidence-inspection.json \
  --compare-seeds LEFT_LABEL --compare-seeds RIGHT_LABEL \
  --json > evidence-comparison.json
```

Validate inspection and filter documents against their exact families and versions. Filter output partitions typed node, CALLS relation, dispatch, sibling, and diagnostic-correlation references into `shared`, `left_only`, and `right_only`; namespaces never collapse even when raw values match. Resolve references against the admitted inspection.

Read seed states and global boundaries before interpreting an empty or exclusive partition. `FAILED`, `SUCCESSFUL_EMPTY`, and `SUCCESSFUL_WITH_EVIDENCE` are distinct. Shared references do not establish shared feature/workflow identity; exclusive or empty references do not establish distinct identity or absence. Operand reversal must preserve shared/boundary data while swapping directional partitions.

## Validate and verify

Validation does not canonicalize or rewrite input. Historical V1/V2 graph validation is structural; V3 performs structural validation before deeper semantic validation.

`lsp-trace verify SELECTOR` checks selected-generation exact-byte custody and semantic receipt. Direct artifacts are not selector-custody-verifiable by that command. Verification establishes the documented integrity/custody commitments only.

`lsp-trace verify passage` binds one expected passage to exact artifact, inspection, seed, node, URI, range, encoding, and digest inputs. It is offline and read-only. Graph V3 is attribution-only when retained bytes are unavailable; body completeness remains `NOT_EVALUATED`. Only verified publication-selector ingress can establish selector custody; content-addressed or private-root modes retain narrower claims.

## Retained CALLS and analytics

`export-retained-calls` projects admitted historical CALLS evidence without source checkout or server. Preserve exact input bytes, source bindings/receipts, group memberships, bundle context, and UNREPORTED no-range groups. Support is per historical relation group, not per callsite; acquisition method, repeated-report count, source authentication, and full normative acceptance remain unavailable.

Bounded retained analysis, metrics, ranking, communities, hubs, and instability are structural projections over admitted inputs. They may reveal dependencies, components, centrality, or sensitivity to structural perturbation. They do not establish runtime behavior, feature identity, purpose, value, confidence, coverage, or acceptance.

## Capture sets

A capture set or Program C composition preserves each exact constituent byte string, digest, length, and identity, and may union native constituent nodes and CALLS records. It never infers cross-capture CALLS and is not native single-capture custody. Incompatible coordinates or identities fail closed. A composite cannot be passed off as native Graph Provenance V5 or admitted to a native-only projection such as Leiden without a separately approved contract.

## Publication and rendering

Publication is exclusive, owner-only, no-replace beneath a caller-pinned root and returns path-free receipts. The server never chooses a private path. For potentially broad output, choose an output destination before acquisition. Compact traversal requires both `detail: "compact"` and `output_selector`; the complete immutable graph is published while the response remains bounded.

An output destination is caller-provided; an artifact selector is product-generated; neither is a schema identity. Rendering is deterministic and non-authoritative. Exact-byte and semantic receipts, deterministic bytes, and opaque generation coordinates do not authenticate source, execution, or producer identity.
