# ADR 0004: Separate symbol tracing from structural discovery

- Status: Proposed
- Date: 2026-09-12

Implementation note: this historical proposal used the provisional name `discover`. The current CLI ships the settled `census` command for accountable enumeration and ships `trace` for exact targets. This does not retroactively accept every grammar, MCP, help-layout, or migration decision proposed below; operation 34 remains unregistered and `context` remains `FUTURE/PROPOSED`.

## Context

The current CLI exposes acquisition history directly: `slice`, `incoming`, acquisition and output versions, `--production-v5`, `--graph-provenance`, seed manifests, grouping switches, and several overlapping selectors. The full MCP profile similarly exposes every historical and specialist operation. This makes implementation history look like the product model.

Two primary use cases need different traversal, accounting, and output behavior:

1. **Trace a symbol in context** while navigating a codebase.
2. **Census source symbols and direct call structure for later analysis** across selected source files. This does not itself identify communities or features.

These must not be conflated. Symbol tracing starts from one exact target and explores its bounded neighborhood. Structural discovery performs a source census, retains every preparation disposition, may require multiple native captures, and feeds separately bounded analytics. Neither establishes runtime behavior, semantic feature identity, ownership, architecture, or whole-source completeness.

## Decision

Adopt two intent-oriented acquisition commands:

```text
lsp-trace trace
lsp-trace discover
```

`discover` is a provisional working name. Before stabilization, user testing must compare narrower names such as `census` and `scan`; the final name must not imply semantic feature discovery, runtime discovery, architecture recovery, or whole-source completeness.

### First-use workflows

Examples in this section must be parser-tested before this ADR is accepted.

```sh
# Navigate from one exact symbol.
lsp-trace trace --file src/main.go --symbol Run --profile go --workspace .

# Census callable symbols and direct calls under selected source.
lsp-trace discover src --profile go --workspace .
```

Human results identify artifact kind, publication state, partiality, immutable selector when one exists, and an exact inspection command. Machine mode emits one versioned JSON result envelope on stdout; progress and diagnostics use stderr. Neither command runs derived analysis.

Keep evidence admission and derived analysis separate:

```text
lsp-trace inspect
lsp-trace verify
lsp-trace program-c
```

The ordinary CLI help presents these five lanes. Administrative, historical, and specialist operations move to advanced or legacy documentation.

Graph Provenance V5 is the only production acquisition output. Every generated target specification is retained as canonical `lsp-trace.seeds.v2`. Existing V1–V3 readers, validators, selectors, receipts, and schema identities remain supported.

## `trace`: one symbol in context

### Grammar

```text
lsp-trace trace --file PATH --symbol NAME \
  (--server COMMAND | --profile NAME [--config PATH]) \
  [--workspace PATH] [--output SELECTOR] \
  [--language-id ID] [--down-depth N] [--up-depth N] \
  [--max-nodes N] [--timeout DURATION] [--request-timeout DURATION]

lsp-trace trace --at PATH:LINE:COLUMN ...
```

`--file PATH --symbol NAME` and `--at` are mutually exclusive. Coordinates are one-based at the CLI boundary and translated to zero-based LSP positions.

`--symbol NAME` matches the documented exact managed-symbol key within `--file`; it is never substring or fuzzy matching. The implementation must specify whether that key is the simple name, qualified name, or another retained provider field before this ADR is accepted. Zero matches produces a typed not-found outcome. One match proceeds. Multiple exact matches produce a typed ambiguous-target outcome before traversal and publish no acquisition artifact. Human diagnostics show at most eight candidates with `PATH:LINE:COLUMN`; machine diagnostics retain total and omitted counts and never imply the displayed subset is exhaustive. Ambiguity is never resolved by first match, provider order, symbol kind, or lexical preference.

### Defaults

- `down-depth=2`
- `up-depth=2`
- current managed timeout and resource ceilings remain unchanged unless a separate limits ADR changes them
- sibling enrichment is not requested unless explicitly enabled

`trace` produces one bounded native Graph Provenance V5 artifact. Its absence and completeness statements are limited to the requested target, provider response, traversal bounds, and retained diagnostics.

## `discover`: structural source census

### Grammar

```text
lsp-trace discover [SOURCE...] \
  (--server COMMAND | --profile NAME [--config PATH]) \
  [--workspace PATH] [--include PATTERN...] [--exclude PATTERN...] \
  [--output SELECTOR] [--language-id ID] \
  [--down-depth N] [--up-depth N] \
  [--max-nodes N] [--timeout DURATION] [--request-timeout DURATION]
```

With no `SOURCE`, discovery uses `.`. Sources are workspace-relative regular files or directories. Traversal is root-confined, does not follow symlinks, deduplicates canonical paths, and uses lexical ordering. Include and exclude patterns use workspace-relative gitignore-style semantics; exclusions win and cannot re-admit paths forbidden by custody or resource policy.

Discovery requests document symbols and attempts `prepareCallHierarchy` for every document symbol. Only exact preparation successes become targets.

It retains separate closed file and symbol ledgers. Every enumerated file receives exactly one terminal disposition: selected, excluded-by-pattern, forbidden-by-custody, unreadable, unsupported, document-symbol-failed, processed, or typed-incomplete. Every returned document symbol receives exactly one terminal disposition: non-callable, preparation-unsupported, preparation-failed, prepared, or typed-incomplete. Ledger counts reconcile to explicit denominators; interrupted or unknown items never disappear. A census does not establish endpoint, feature, source, or graph completeness.

`--include` and `--exclude` are repeatable one-pattern-per-occurrence options. Flag order does not alter semantics; all includes form one inclusion set and exclusions always win. Empty patterns fail before discovery. Patterns should be quoted to prevent shell expansion, for example `--include '**/*.go' --exclude '**/*_test.go'`.

### Defaults

- `down-depth=1`
- `up-depth=0`
- current managed timeout and resource ceilings remain unchanged unless separately revised

Every selected callable is already a root, so one outgoing level captures its direct server-reported calls. Upward expansion is opt-in because it widens beyond the selected census and can redundantly rediscover selected callers.

### Large discoveries

Graph Provenance V5 admits at most 63 required targets. Discovery must never truncate or silently sample a larger census.

When more than 63 targets are prepared, `discover`:

1. orders targets by ascending bytes of their canonical Seeds V2 target encoding, using original census ordinal as the final tie-breaker;
2. applies only a versioned, manifest-retained exact-duplicate policy;
3. partitions consecutive targets into maximal non-empty batches of at most 63 under a schema-versioned batching algorithm;
4. acquires and publishes each batch as a separate native V5 artifact whose custody independently identifies the acquisition request, canonical Seeds V2 bytes, provider/session evidence, bounds, diagnostics, publication receipt, and immutable artifact identity;
5. emits a capture-set manifest with its own versioned schema, canonical encoding, logical digest, and immutable selector, committing to ordered constituent identities and digests, census policy, closed ledgers, and batch assignment.

Discovery ledgers and capture-set manifests are private by default. Publication applies an explicit retained disclosure policy to paths, symbol labels, diagnostics, provider/config details, and selectors. Redacted views have distinct identities and cannot claim byte equivalence to private evidence. Digests do not authorize content access.

The capture-set manifest references but neither inherits nor replaces constituent custody. The capture set is not a native single capture. Composition remains a separate operation and cannot forge native custody or infer cross-capture `CALLS`. Composite-to-Leiden remains unauthorized until separately admitted.

## Advanced inputs

Manually authored Seeds V2 files remain supported for exact replay and compatibility, but are not part of beginner-facing help. They preserve original targets, bounds, labels, ordinals, and canonical bytes. Targets are never reconstructed from graph nodes.

Explicit depth and resource overrides are advanced options. They are retained in seed and acquisition evidence and may not silently replace manifest-owned values.

## Derived structural analysis

These remain separate from acquisition:

- `program-c leiden` computes a structural partition from one admitted native V5 graph.
- `program-c instability` computes bounded A-08 cross-seed instability evidence.
- `aggregate-communities` builds a graph-bound technical community register from canonically replayed partitions.
- `program-c compose` preserves compatible V5 constituents without claiming native single-capture custody.
- retained-passage verification stays under `verify passage`.
- hydrated inspection remains offline and uses inline, immutable-selector, content-addressed, or explicitly enabled root-confined private ingress.

No analysis runs implicitly during `trace` or `discover`.

### After discovery

`discover` ends with acquisition and closed census accounting. If the result is one admitted native V5 graph, human output may present the exact `program-c leiden` invocation as an explicit next step and machine output may carry typed admissibility metadata. If the result is a capture-set manifest, neither output may recommend or invoke Leiden over the set: composite-to-Leiden remains unauthorized. Suggested commands describe available future actions, not analysis already performed.

## MCP surface

Add two beginner-facing MCP tools with the same intent split:

```text
lsp_trace_v1_trace
lsp_trace_v1_discover
```

The default advertised MCP profile is small and workflow-oriented. It advertises trace, discover, hydrated inspection, verification, and the settled Program C operations appropriate for ordinary agents.

Historical `slice` and `incoming` variants, versioned acquisition tools, and specialist analytics stop appearing in the default profile. They remain dispatchable through canonical execute during the compatibility window with unchanged names, schemas, envelopes, limits, and behavior. Operation 31 remains composition and operation 32 remains A-08 instability. MCP operation numbers are not renumbered.

Profiles become:

- **default**: small supported workflow surface;
- **advanced**: specialist and administrative operations;
- **legacy**: non-advertised compatibility dispatch.

The `v1` suffix versions each tool's request, response, error, and envelope contract independently of Graph Provenance V5. Implementations advertise exact tool-schema revisions and capabilities. Additive optional fields must preserve existing interpretation. Changed defaults, required fields, error semantics, limits, or authority require a new tool name or future MCP protocol version.

Hard removal of legacy MCP names requires a future MCP protocol version and separate migration evidence.

## Deprecations and removals

The following production CLI vocabulary is deprecated:

- `slice --production-v5`
- `incoming --production-v5`
- `--production-v5`
- `--acquisition-version`
- `--output-version`
- `--graph-provenance`
- grouping and community-analysis flags attached to acquisition
- ordinary V1–V3 producer paths

Migration mapping:

- exact `slice` invocations map to `trace`;
- file/directory production acquisition maps to `discover`;
- production `incoming` receives a `trace` replacement only when explicit selectors, bounds, identity, accounting, bytes, and custody remain equivalent; otherwise it remains legacy-only with no claimed equivalent;
- grouping maps to explicit Program C analysis after acquisition.

Rollout is atomic:

1. **Add:** introduce `trace` and `discover`; prove V5 and Seeds V2 equivalence.
2. **Warn:** legacy human invocations emit one exact replacement and removal release on stderr.
3. **Hide:** remove legacy producers from top-level help and completions while retaining parser compatibility.
4. **Remove:** delete legacy producer routing from the main binary after one migration window.
5. **Retain readers:** preserve historical validation, verification, schema retrieval, hydration, and immutable artifact admission indefinitely.

`--machine` preserves JSON stdout purity and emits typed deprecation records on stderr. Legacy aliases preserve stdout bytes and exit status during the warning window. Warnings never alter artifacts, selectors, receipts, or logical digests.

## Failure and conflict rules

All selector and option conflicts fail before server start or source discovery. There is no silent precedence.

- `trace` requires exactly one exact-target mode.
- `discover` accepts source census inputs, not exact target selectors.
- generated discovery and supplied seed files cannot be mixed.
- `--config` requires `--profile`; explicit server/profile precedence remains unchanged.
- legacy version and grouping flags are invalid on the new commands.
- empty discovery is a typed no-preparable-symbols outcome, not fabricated success.
- excluded, unsupported, unreadable, failed, bounded, and empty outcomes remain distinct.
- partial/incomplete structured evidence retains the existing typed exit behavior.

## Documentation

Top-level help shows only the five primary lanes and points to `lsp-trace advanced` and migration documentation. `program-c` remains a compatibility namespace pending a separate naming decision; beginner-facing help labels it “derived structural analysis” and explains each ordinary subcommand. Simplification may hide tuning controls and historical spellings, but never artifact kind, custody status, partiality, census denominators, batching, or analysis admissibility. `trace --help` groups exact targets, traversal, server, and output options. `discover --help` groups sources, filters, census accounting, server, batching, and output options.

README quick starts demonstrate both intents. A migration guide classifies every deprecated spelling as either having exactly one semantics-preserving replacement or having no equivalent and requiring continued legacy dispatch. It never maps an invocation when identity, bounds, accounting, output bytes, or custody would change silently. Shell completions and manuals derive from one command metadata source so advertised, advanced, deprecated, and removed states cannot drift.

## Required verification

Before changing defaults or removing legacy paths, tests must establish:

- exact grammar and every conflict pair;
- trace/discover V5 and retained Seeds V2 equivalence with their legacy sources;
- trace depth defaults and caller/callee bounds;
- discovery path ordering, deduplication, gitignore semantics, symlink/escape rejection, preparation accounting, and empty outcomes;
- deterministic over-63 batching with no target loss and no custody promotion;
- stdout/stderr purity, structured machine warnings, exit codes, immutable publication, and partial outcomes;
- no implicit grouping, composition, instability, passage verification, or hydration;
- unchanged historical readers and selectors;
- unchanged legacy MCP dispatch contracts and exact default/advanced/legacy advertisement sets;
- documentation examples accepted by the parser.

Representative real-application replay overrides synthetic confidence before legacy removal.

## Consequences

The product vocabulary becomes intent-oriented rather than version-oriented. Navigation remains fast and locally bounded; discovery remains accountable and scalable without pretending a multi-capture set is one native graph.

The cost is two primary acquisition verbs instead of one. This is intentional: one name would hide materially different traversal, denominator, batching, and output contracts. Implementation complexity also increases for deterministic discovery batching and capture-set custody, which must land before `discover .` can be the reliable default.

## Non-goals

This ADR does not authorize inferred calls, semantic feature naming, composite-to-Leiden admission, source-completeness claims, new MCP authority, changed Program C algorithms, relaxed hydration ingress, installation, or managed-service restart.
