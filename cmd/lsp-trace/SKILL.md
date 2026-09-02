---
name: lsp-trace
description: Trace and slice server-reported call hierarchies while preserving bounded evidence, per-seed attribution, diagnostics, and publication custody.
---

# lsp-trace

Use `lsp-trace` when you need an evidence-preserving upward caller trace from exact source positions or a bounded outgoing-then-incoming call-graph slice.

Use only trusted language-server binaries and workspaces. The selected server runs with your permissions and may execute project build or restore logic, read workspace files, access the network, and emit sensitive data. Sandboxing is the caller's responsibility. `lsp-trace` passes arguments directly without a shell and performs no automatic network access.

## Retrieve this skill

```bash
lsp-trace skill get
```

The command prints this complete embedded document to stdout. Static retrieval is built in; dynamic skill discovery, listing, and installation are not currently provided.

## Trace incoming callers

```bash
lsp-trace incoming \
  --workspace /path/to/workspace \
  --server typescript-language-server \
  --server-arg --stdio \
  --at path/to/file.ts:LINE:COLUMN
```

`language-server` is a placeholder. Select a server that supports LSP Call Hierarchy and pass its required startup arguments with repeatable `--server-arg`; `--stdio` is required by the example server but is not universal. Use repeatable `--server-env KEY=VALUE` for environment overrides and `--language-id` when extension-based inference is unsuitable.

`--at` is repeatable. `--seed-file` accepts labeled seeds. Lines and columns are one-based.

### Trace a labeled seed set

Use a seed file when several known source positions should remain distinguishable in one trace. Paths are resolved against `--workspace`; labels must be non-empty and unique. For example, `seeds.json` may contain:

```json
{"seeds":[
  {"label":"report-download","at":"src/reports.ts:42:8"},
  {"label":"scheduled-export","at":"src/export.ts:19:4"}
]}
```

Unknown seed-file fields are rejected. Labels must be unique and match `[A-Za-z][A-Za-z0-9._-]*`. Every `at` value requires a nonempty path and positive one-based line and column.

Run the labeled set with:

```bash
lsp-trace incoming \
  --workspace /path/to/workspace \
  --server language-server \
  --seed-file seeds.json \
  --expand-dispatch-family \
  --expand-topmost-siblings \
  --pretty > trace.json
```

Seed-file entries and repeated `--at` values may be combined. Repeated `--at` values receive generated labels. In v3 output, match each requested seed to its same-label `seeds` result; failed seeds remain represented rather than disappearing.

## Build technical inputs for a feature inventory

Use `lsp-trace` to produce bounded technical evidence beneath provisional hypotheses, not to manufacture feature identity.

Retain one transport-neutral evidence set: asserted repository paths and revisions; included and excluded source scope; workspace and server configuration; seed labels, positions, and investigation reasons; traversal bounds; selector or artifact bytes; stderr; exit status; verification result when run; inspection output; and unresolved authority limits. `--provenance-source-revision` records a caller assertion—it does not pin or authenticate source.

### Choose a traversal

Use `incoming` when the supplied positions are already the callees of interest and only caller expansion is required. Use `slice` when bounded outgoing discovery must first choose the nodes from which incoming traversal begins. `slice --from-file` is a server-reported document-symbol census, not proof that every callable or feature in the file was discovered. Slice requires exactly one start mode; incoming may combine `--seed-file` with repeated `--at` positions.

Set explicit edge-depth, node, request-timeout, and global-timeout bounds. A typical published slice is:

```bash
lsp-trace slice \
  --workspace /path/to/workspace \
  --server language-server \
  --from-file path/to/file.ext \
  --down-depth 2 \
  --up-depth 8 \
  --max-nodes 10000 \
  --request-timeout 30s \
  --timeout 5m \
  --output evidence.selector.json \
  2> evidence.stderr
status=$?
```

### Handle traversal status

Interpret status in the command-specific traversal contract:

- `0`: traversal completed relative to the configured server and bounds; continue with retained evidence.
- `2`: structured incomplete evidence may have been emitted or published. Preserve it with stderr, failed seeds, diagnostics, and boundaries. Slice cancellation follows this structured-incomplete path.
- `1`: invocation, unrecoverable server, inspection/verification, or publication failure. An incoming or slice publication failure after graph marshalling may leave complete graph JSON on stdout; preserve it as failure evidence rather than treating it as a successful packet.
- `130`: interrupted `incoming`; JSON may still have been emitted or a selector published. Retain it explicitly as interrupted evidence, not completed traversal.

A selector's existence does not override the recorded traversal status.

### Admit and inspect the retained evidence

`inspect` performs its own admission gates. For a selector it verifies generation metadata and exact-byte custody before structural and semantic artifact validation. For a direct artifact it performs structural and semantic validation; its computed exact-byte digest is identity only and makes no custody claim. Standalone `verify` is a selector-only explicit audit/preflight and is not required before selector inspection:

```bash
lsp-trace verify evidence.selector.json
lsp-trace inspect evidence.selector.json --all-seeds --json > evidence-inspection.json
```

A successful projection identifies `inspection_schema_version: "lsp-trace.inspect.v1"`, `projection_kind: "ALL_SEEDS"`, and `authority: "NON_AUTHORITATIVE_DERIVED_VIEW"`.

### Reconcile all stored seeds

The aggregate stores native records copied once from the admitted artifact and uses normalized per-seed ID/index projection collections. Shared records referenced by multiple seeds are not independent observations. Before downstream use, require:

```text
requested_seed_count = successful_seed_count + failed_seed_count
successful_seed_count = successful_seed_with_membership_count + successful_seed_without_membership_count
```

Also require every global record count to equal its corresponding `records` array length and every reference count to equal the sum of corresponding per-seed collection lengths. Reference counts are occurrences and may exceed copied global record counts. Failed seeds remain in the requested denominator with failure details and empty memberships/references. `correlated_diagnostic_indexes` are zero-based references into global `records.diagnostics`; they are tool-derived node correlations, not diagnostic custody or causation.

Use each seed's own stored result, exact `seed_memberships`, reached IDs, and native references for attribution. Never substitute the deduplicated union graph, borrow another seed's closure, treat transitive paths as direct support, or treat discovery nominations as call evidence.

Each reversible technical hypothesis must record a provisional bounded claim, exact supporting record IDs, known missing or failed evidence, competing interpretations, contrary evidence or an observation that would reject it, and the action for withdrawing or splitting it. Shared callers, overlap, labels, and mechanical counts alone do not merge hypotheses.

Stop before proposition admission, merge/split adjudication, canonical feature identity or naming, user purpose, production use, value, priority, lifecycle, coverage, or acceptance. Those remain external even when several technical signals agree.

Important options:

- Repeatable `--server-arg` passes one argument directly to the server; repeatable `--server-env KEY=VALUE` supplies environment overrides; `--language-id` overrides extension-based inference.
- `--expand-dispatch-family` asks the server's Type Hierarchy for implementation-family members and emits `dispatch_relationships` separately from call edges.
- `--expand-topmost-siblings` asks for document symbols and emits top-level callable candidates in `sibling_candidates`; candidates do not imply calls, visibility, or equivalence.
- `--max-depth`, `--max-nodes`, `--timeout`, and `--request-timeout` bound traversal.
- `--output` publishes through owner-only custody files; v3 atomically writes a selector naming one immutable private generation containing graph and receipt, while explicit v1/v2 publish only their historical graph projection. POSIX uses selector/generation-file mode `0600` and generation-directory mode `0700`; directory entries and the selector basename may still be visible to principals able to list the destination. Windows relies on native account access controls without a POSIX mode claim. Otherwise pure graph JSON goes to stdout. Diagnostics go to stderr. A post-rename destination-directory sync failure is reported even though the selector may already be visible; retained failure evidence is reported only after both file and containing-directory sync. If publication fails after traversal, exit code `1` may carry the complete marshaled graph on stdout; preserve stdout separately from stderr before discarding a failed run.
- `--trace-lsp PATH` writes an opt-in JSON Lines transcript of raw sent and received JSON-RPC messages with deterministic sequence numbers and directions. It may contain source text, paths, identifiers, opaque server data, or secrets; protect and redact it before sharing. Environment values are omitted, and POSIX creates the file with mode `0600`.
- `--schema v1|v2|v3` selects compatibility; v3 is default and explicit v1/v2 retain historical projections.
- `lsp-trace schema get --schema v1|v2|v3` prints the exact embedded Draft 2020-12 contract.
- `lsp-trace validate [--schema v1|v2|v3] PATH|-` validates a file or stdin, auto-detects the version by default, rejects explicit mismatches, and applies deeper semantic validation to v3 after structural validation.
- `lsp-trace verify SELECTOR` validates the selected generation's exact-byte custody and embedded semantic receipt offline; success prints `verified integrity and custody` rather than graph JSON. Direct artifacts are not custody-verifiable by this command.
- `--provenance-invocation-id`, `--provenance-caller`, `--provenance-source`, `--provenance-source-revision`, `--provenance-server-version`, `--provenance-timestamp`, and `--provenance-tool-version` add caller-supplied receipt metadata. Omitted values remain `UNKNOWN`; the tool never infers them from Git, the clock, the environment, or the server.

Validation does not canonicalize or rewrite input. V1 and v2 validation is structural; v3 runs structural validation before deeper semantic validation.

Validation and verification do not authenticate producer identity or prove that a declared process executed. Invocation provenance is caller-supplied; normalized identities, digests, and receipts are tool-derived.

Raw environment values and the working-directory path are intentionally not retained. The raw server-stderr stream is not retained as a standalone artifact, but captured stderr text may be retained as a sensitive `server-stderr` diagnostic when traversal is incomplete or fails; early slice lifecycle failures relay captured stderr before the transport error.

## Trace a bounded call-graph slice

```bash
lsp-trace slice \
  --workspace /path/to/workspace \
  --server language-server \
  --from-file path/to/file.ext \
  --down-depth 2 \
  --up-depth 8
```

Choose exactly one starting mode: `--from-file FILE` recursively enumerates document symbols; repeatable `--at PATH:LINE:COLUMN` uses explicit positions with automatic labels; `--seed-file FILE` uses the existing labeled seed format. Explicit seed occurrences retain their labels and requested positions even when prepared nodes deduplicate.

Slice accepts `--server-arg`, `--server-env`, `--language-id`, `--down-depth`, `--up-depth`, `--max-nodes`, `--timeout`, `--request-timeout`, `--output`, and `--pretty`. Defaults are down-depth 2, up-depth 100, max-nodes 10000, global timeout 5m, and request timeout 30s. Zero disables traversal in either direction; max-nodes and global timeout use zero for unlimited. Request timeout must be positive. Slice is v3-only and does not accept incoming-only schema, max-depth, provenance, expansion, logging, concurrency, or transcript flags.

The command asks the server which positions prepare as call-hierarchy items and walks server-reported outgoing calls toward the exact `--down-depth` layer. `frontier_node_ids` contains only nodes at that exact depth. `outgoing_terminal_node_ids` contains nodes reached earlier whose `outgoingCalls` request succeeded with an empty result. `upward_start_node_ids` is the native-ID-sorted union of those two sets, and only that union feeds the existing incoming traversal up to `--up-depth`; the command does not traverse upward from every discovered node. Null, failed, timed-out, canceled, or node-budget-truncated outgoing expansion is not a server-reported leaf. Both depths count edges; zero disables traversal in that direction. The v3 `slice` section references native node and relation IDs rather than duplicating graph records.

Treat a slice as a deterministic bounded projection of information reported by the selected language server, not as complete feature coverage or runtime execution evidence. Dynamic dispatch, reflection, generated code, configuration, templates, framework routing, and other relationships omitted by the server may be absent.

## Inspect retained seeds

Choose exactly one mode for an existing v3 `artifact.json` or publication selector:

```bash
lsp-trace inspect SELECTOR_OR_ARTIFACT --seed LABEL --json
lsp-trace inspect SELECTOR_OR_ARTIFACT --all-seeds --json
```

Both modes emit `lsp-trace.inspect.v1` JSON and are read-only. Single-seed output uses `projection_kind: "SEED_INSPECTION"`; aggregate output uses `projection_kind: "ALL_SEEDS"`. Both use authority `NON_AUTHORITATIVE_DERIVED_VIEW`, not graph or receipt authority. Unknown labels, invalid inputs, and missing or duplicate same-label aggregate results fail without JSON output.

Direct artifacts pass Draft 2020-12 structural and v3 semantic validation but gain no custody claim. Selectors additionally require a complete generation and valid exact-byte custody receipt. Aggregate native records are copied once; per-seed collections reference them. `TOOL_DERIVED_NODE_CORRELATION` identifies reached-node diagnostic correlation only, never exact per-seed custody or causation. Inspection adds no feature or consumer semantics and does not replace graph validation or selector custody verification.

## Compare two retained seed-evidence sets

First retain an aggregate inspection, then compare exactly two distinct stored labels:

```bash
lsp-trace inspect evidence.selector.json --all-seeds --json > evidence-inspection.json
lsp-trace filter evidence-inspection.json \
  --compare-seeds LEFT_LABEL \
  --compare-seeds RIGHT_LABEL \
  --json > evidence-comparison.json
```

`filter` accepts only a path to a valid `lsp-trace.inspect.v1` `ALL_SEEDS` document. It emits a deterministic, read-only `lsp-trace.filter.v1` `SEED_EVIDENCE_COMPARISON`; it starts no server and does not modify or replace the inspection. The result contains typed explicit references partitioned into `shared`, `left_only`, and `right_only`, selected-seed states, global boundaries, and mechanical counts. It contains no copied native records, so resolve references against the admitted inspection document.

Treat reference namespaces independently. Equal raw values in node, call-relation, dispatch, sibling, and diagnostic-correlation namespaces are not equal evidence keys. Do not infer node references from relation endpoints, partition membership records, silently deduplicate malformed input, or reinterpret diagnostic correlation as custody or causation. Failed and successful-empty seeds remain distinct and are valid operands.

Shared references do not establish shared feature or workflow identity. Exclusive or empty references do not establish distinct identity, absence, or a merge/split result. The filter projection has zero support contribution and cannot upgrade native evidence authority, recover selector custody, authenticate source or execution, prove runtime behavior, rank evidence, measure coverage, or determine confidence or acceptance. Use it only as a mechanical pairwise review input; retain semantic and product adjudication outside `lsp-trace`.

### Verify filter contract changes

Run the focused matrix whenever changing filter admission, projection, schemas, accounting, or authority boundaries:

```bash
go test ./cmd/lsp-trace -run '^TestFilter' -count=1
go test ./internal/schema -count=1
./scripts/release-check.sh
```

The matrix covers CLI validation before input reads; inspection-family and seed admission; failed, successful-empty, and evidence-bearing states; all five typed reference namespaces; domain-separated equal raw values; pairwise disjointness and exhaustion; outside-universe omission; canonical ordering and operand reversal; every namespace accounting equation; schema and authority constants; deterministic output; input immutability; and operation without a language-server executable. Mutation subtests deliberately alter each stable filter constant or accounting rule and require validation to fail closed.

Treat the matrix as regression evidence about the implementation contract, not evidence about any traced program. Passing tests do not authenticate an inspection producer, establish selector custody, prove language-server completeness, strengthen native evidence, or justify feature/workflow conclusions. `execution_bundle_id` remains optional: copy a present admitted digest exactly and omit it when absent; never invent a replacement identity.

## Interpret results honestly

### Field authority

- Caller-supplied authority: provenance flags, server command/arguments and explicit environment names, requested limits/configuration, and original seed labels/positions are recorded as claims supplied by the invoker.
- Tool-derived authority: normalized identities, digests, memberships, counters, boundaries, sensitivity/evidence metadata, and commitments are deterministic derivations from recorded inputs and observations; they are not independent authentication.
- Server-reported authority: capabilities, prepared targets, call edges, Type Hierarchy associations, document-symbol candidates, diagnostics, and opaque server data report what the configured LSP server returned.
- Custody authority: semantic and exact-byte receipts establish only the documented integrity/custody commitments.

For v3, every original `invocation.seeds` entry has exactly one `seeds` result with the same label, including a failed result when preparation, opening, capability, or traversal fails. Failures expose `failure.phase` and `failure.message`. In a slice, each successful result's `reached_node_ids` and `reached_relation_ids` are its deterministic causal closure: bounded outgoing reachability from that seed plus incoming reachability from only the upward-start nodes that seed reached. The relation memberships are direct canonical call-relation IDs in that closure, never discovery relations or relations borrowed from an unrelated seed. Shared descendants and callers may belong to multiple seeds, while failed seeds have empty membership and the global graph remains deduplicated. Do not attribute the entire union graph to every seed.

For structured incomplete slices, stderr lists each failed seed's label, failure phase, and failure message before summarized diagnostics. `SERVER_CALL_SITE_OUTSIDE_CALLER_RANGE` is advisory when caller, callee, call-site range, and deterministic relation identity remain attributable: retain the relation and original ranges, continue traversal, and do not treat that warning alone as incomplete. Malformed ranges, unattributable callers, identity collisions, and dangling references remain fail-closed.

- `edges` contain only server-reported Call Hierarchy caller relationships.
- `dispatch_relationships` are association evidence, not caller evidence.
- `sibling_candidates` are discovery candidates, not usage evidence.
- `evidence_semantics` is the machine-readable claim ceiling. Call edges support only a server-reported caller-to-callee relation; they do not establish runtime execution, feature identity, whole-source completeness, or independent source confirmation.
- Discovery records use `evidence_class: "DISCOVERY_NOMINATION"` and `support_contribution: 0`. They nominate separate investigation and contribute no caller/callee support.
- `evidence_receipt` assigns domain-separated canonical `sha256:` identities to call, sibling, and dispatch relations using the caller-supplied source revision, direction, locator, evidence class, relation kind, and semantic endpoints. Separate memberships join each seed occurrence to its reached call relations and discovery evidence; discovery still contributes zero call support.
- In v3, `trace_receipt.semantic_commitment_digest` commits to canonical semantic content with the receipt omitted; the selected generation's `exact_serialized_bytes_digest` separately commits to exact serialized bytes. With identical invocation inputs and server observations, canonical artifact bytes are deterministic; generation directory basenames are opaque publication coordinates, not content identities, and may differ between publications. Changing an invocation input such as `--output` may change invocation-derived bundle identity and artifact bytes. Replay inputs use `replay_input_content_digest` and `replay_input_manifest_digest`; process-context claims use role-specific process-context digest names. Offline verification recomputes the semantic and exact-byte commitments and validates bound structure, but these establish integrity/custody only—not authenticity, signature, producer identity, source truth, runtime behavior, or independent confirmation of hidden inputs.
- V3 identity labels caller provenance `CALLER_ASSERTED` and derives resolved seed URI/content digests plus `resolved_seed_contents_digest` scoped `RESOLVED_SEED_CONTENTS`. Failed seeds are not source identities.
- V3 embeds no environment values or working-directory path. `process_context` records tool-derived, domain-separated process-context digests for the effective inherited-plus-override environment, cwd, and explicit environment names, the environment-variable count, and explicit redaction state. The cwd identity uses the cleaned absolute path supplied by the process and does not resolve cwd symlink aliases, so two lexical aliases of the same directory can produce different identities. `automatic_redaction` is false: those omissions are specific projection rules, not general scrubbing. Hashes may confirm guesses; custodians must still control access to arguments, explicit environment names, paths, opaque data, diagnostics, server stderr, trace transcripts, and publication-failure target/error fields.
- `traversal_complete` is scoped to server-reported Call Hierarchy under the requested limits.
- `source_graph_complete` remains `UNKNOWN`.
- Unresolved evidence marks traversal incomplete; dynamic-call evidence is advisory and never fabricates edges.
- `UNKNOWN` provenance is an explicit evidence boundary, not permission to substitute the current timestamp, executable version, repository revision, or server version.

Exit code `0` means traversal completed within this server-relative scope. Exit code `2` means structured but incomplete output and may still have published a valid selector or emitted graph JSON; inspect failed-seed stderr and the structured evidence instead of discarding it. Exit code `1` means invocation or unrecoverable server failure, including publication failure whose complete graph may be on stdout. Exit code `130` is specific to interrupted `incoming`; slice cancellation follows the structured-incomplete status `2` path.
