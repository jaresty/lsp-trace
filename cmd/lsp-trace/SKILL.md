---
name: lsp-trace
description: Trace and slice server-reported call hierarchies while preserving bounded evidence, per-seed attribution, diagnostics, and publication custody.
---

# lsp-trace

Use `lsp-trace` when you need an evidence-preserving upward caller trace from exact source positions or a bounded outgoing-then-incoming call-graph slice.

Use only trusted language-server binaries and workspaces. The selected server runs with your permissions and may execute project build or restore logic, read workspace files, access the network, and emit sensitive data. Sandboxing is the caller's responsibility. `lsp-trace` passes arguments directly without a shell and performs no automatic network access.

## Quick start

Prefer a named server profile for normal operation:

```bash
lsp-trace incoming \
  --workspace /path/to/workspace \
  --profile typescript \
  --at path/to/file.ts:LINE:COLUMN
```

### Use named server profiles

Both `incoming` and `slice` accept explicit `--profile NAME`. Profiles are never selected automatically from `language_ids`, paths, or extensions. With no profile, configuration files are ignored and legacy flags behave unchanged. `--server` may accompany a profile and overrides its command.

Default discovery loads `$XDG_CONFIG_HOME/lsp-trace/config.toml` (or `~/.config/lsp-trace/config.toml`) and then `--workspace/.lsp-trace.toml`; `--config PATH` replaces both. CLI command-related fields override project profile fields, which override user profile fields. Malformed TOML, unknown fields, and missing profiles fail closed.

```toml
[profiles.typescript]
command = "typescript-language-server"
args = ["--stdio"]
env = ["NPM_TOKEN", "AUTH_TOKEN=${AUTH_TOKEN}"]
language_ids = ["typescript", "typescriptreact"]
```

Profile environment entries contain only variable names or exact `KEY=${VAR}` references. Values are read from the process environment at runtime; missing variables fail and plaintext values are rejected. Invocation graph output retains names/references, never environment values. The first `language_ids` entry supplies the default only after explicit profile selection; `--language-id` overrides it.

### Retrieve this skill

```bash
lsp-trace skill get
```

The command prints this complete embedded document to stdout. Static retrieval is built in; dynamic skill discovery, listing, and installation are not currently provided.

### Use the MCP offline evidence server

Configure an MCP client to launch `lsp-trace-mcp` directly over stdio for local development. Run `lsp-trace-mcp` for inline-only results, or `lsp-trace-mcp --publication-root /absolute/private/root` to permit caller-supplied relative `output_selector` publication beneath one pinned private root.

The default surface advertises ten canonical tools: the six offline tools `lsp_trace_v1_capabilities`, `lsp_trace_v1_schema_get`, `lsp_trace_v1_validate`, `lsp_trace_v1_verify`, `lsp_trace_v1_inspect`, and `lsp_trace_v1_filter`, plus `lsp_session_v1_list`, `lsp_session_v1_status`, `lsp_session_v1_stop`, and `lsp_session_v1_restart`. Their unversioned aliases remain callable but unadvertised. Call capabilities with `{}` before relying on schema identities, publication support, or limits.

The embedded Stage 1 manifest and registered schemas are authoritative. Each completed tool call returns exactly one versioned envelope in MCP `structuredContent`, selected by `envelope_schema_id`; outer MCP `content` is empty and does not mirror the envelope. Artifact output is inline through 1,048,576 bytes. Larger possible output requires `output_selector`; publication is exclusive, owner-only, no-replace, and returns a path-free receipt. The server never chooses or returns a private path. `list_page_max` is 100.

Preserve existing evidence ceilings: MCP transport, envelopes, inline bytes, publication receipts, validation, verification, inspection, and filtering add no authenticity, authority, source truth, execution proof, feature identity, coverage, confidence, or acceptance. Stage 2 lifecycle tools are enabled by default and route to the process-local runtime; Stage 3 live traversal tools remain reserved `NOT_IMPLEMENTED`.

**WARNING:** Child processes run with the developer's permissions, are not sandboxed, may access local files and network, and must be trusted. This local-development-only tool does not provide hostile-code safety, native containment, remote execution, or privileged isolation.

## Choose a command

- `incoming`: start from exact callee positions and trace callers upward.
- `slice`: discover bounded outgoing nodes first, then trace incoming callers from the exact frontier and server-reported leaves.
- `inspect`: admit and project one seed or all retained seeds without changing evidence authority.
- `filter`: mechanically compare exactly two seeds from an admitted all-seeds inspection.
- `verify`: audit a publication selector's exact-byte custody and embedded semantic receipt.
- `validate`: validate graph, inspection, or filter documents against the selected family/version contract.

Use `incoming` when the supplied positions are already the callees of interest and only caller expansion is required. Use `slice` when bounded outgoing discovery must first choose the nodes from which incoming traversal begins. `slice --from-file` is a server-reported document-symbol census, not proof that every callable or feature in the file was discovered. Slice requires exactly one start mode; incoming may combine `--seed-file` with repeated `--at` positions.

A direct server command remains available when a profile is unsuitable:

```bash
lsp-trace incoming \
  --workspace /path/to/workspace \
  --server typescript-language-server \
  --server-arg --stdio \
  --at path/to/file.ts:LINE:COLUMN
```

`language-server` is a placeholder. Select a server that supports LSP Call Hierarchy and pass its required startup arguments with repeatable `--server-arg`; `--stdio` is required by the example server but is not universal. Use repeatable `--server-env KEY=VALUE` for environment overrides and `--language-id` when extension-based inference is unsuitable.

## Operational workflows

### Trace incoming callers

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

### Build technical inputs for a feature inventory

Use `lsp-trace` to produce bounded technical evidence beneath provisional hypotheses, not to manufacture feature identity.

Retain one transport-neutral evidence set: asserted repository paths and revisions; included and excluded source scope; workspace and server configuration; seed labels, positions, and investigation reasons; traversal bounds; selector or artifact bytes; stderr; exit status; verification result when run; inspection output; and unresolved authority limits. `--provenance-source-revision` records a caller assertion—it does not pin or authenticate source.

### Choose a traversal

The command router above defines when to use each traversal. In either case, set explicit edge-depth, node, request-timeout, and global-timeout bounds. The slice workflow below shows a typical published invocation.

### Handle traversal status

Interpret status in the command-specific traversal contract:

- `0`: traversal completed relative to the configured server and bounds; continue with retained evidence.
- `2`: structured incomplete evidence may have been emitted or published. Preserve it with stderr, failed seeds, diagnostics, and boundaries. Slice cancellation follows this structured-incomplete path.
- `1`: invocation, unrecoverable server, inspection/verification, or publication failure. An incoming or slice publication failure after graph marshalling may leave complete graph JSON on stdout; preserve it as failure evidence rather than treating it as a successful packet.
- `130`: interrupted `incoming`; JSON may still have been emitted or a selector published. Retain it explicitly as interrupted evidence, not completed traversal.

A selector's existence does not override the recorded traversal status.

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

### Trace a bounded call-graph slice

```bash
lsp-trace slice \
  --workspace /path/to/workspace \
  --profile typescript \
  --from-file path/to/file.ts \
  --down-depth 2 \
  --up-depth 8 \
  --max-nodes 10000 \
  --request-timeout 30s \
  --timeout 5m \
  --output evidence.selector.json \
  2> evidence.stderr
status=$?
```

Choose exactly one starting mode: `--from-file FILE` recursively enumerates document symbols; repeatable `--at PATH:LINE:COLUMN` uses explicit positions with automatic labels; `--seed-file FILE` uses the existing labeled seed format. Explicit seed occurrences retain their labels and requested positions even when prepared nodes deduplicate.

Slice accepts `--server-arg`, `--server-env`, `--language-id`, `--down-depth`, `--up-depth`, `--max-nodes`, `--timeout`, `--request-timeout`, `--output`, and `--pretty`. Defaults are down-depth 2, up-depth 100, max-nodes 10000, global timeout 5m, and request timeout 30s. Zero disables traversal in either direction; max-nodes and global timeout use zero for unlimited. Request timeout must be positive. Slice is v3-only and does not accept incoming-only schema, max-depth, provenance, expansion, logging, concurrency, or transcript flags.

The command asks the server which positions prepare as call-hierarchy items and walks server-reported outgoing calls toward the exact `--down-depth` layer. `frontier_node_ids` contains only nodes at that exact depth. `outgoing_terminal_node_ids` contains nodes reached earlier whose `outgoingCalls` request succeeded with an empty result. `upward_start_node_ids` is the native-ID-sorted union of those two sets, and only that union feeds the existing incoming traversal up to `--up-depth`; the command does not traverse upward from every discovered node. Null, failed, timed-out, canceled, or node-budget-truncated outgoing expansion is not a server-reported leaf. Both depths count edges; zero disables traversal in that direction. The v3 `slice` section references native node and relation IDs rather than duplicating graph records.

Treat a slice as a deterministic bounded projection of information reported by the selected language server, not as complete feature coverage or runtime execution evidence. Dynamic dispatch, reflection, generated code, configuration, templates, framework routing, and other relationships omitted by the server may be absent.

### Inspect retained seeds

`inspect` performs its own admission gates. For a selector it verifies generation metadata and exact-byte custody before structural and semantic artifact validation. For a direct artifact it performs structural and semantic validation; its computed exact-byte digest is identity only and makes no custody claim. Standalone `verify` is a selector-only explicit audit/preflight and is not required before selector inspection:

```bash
lsp-trace verify evidence.selector.json
lsp-trace inspect evidence.selector.json --all-seeds --json > evidence-inspection.json
```

A successful projection identifies `inspection_schema_version: "lsp-trace.inspect.v1"`, `projection_kind: "ALL_SEEDS"`, and `authority: "NON_AUTHORITATIVE_DERIVED_VIEW"`.

Choose exactly one mode for an existing v3 `artifact.json` or publication selector:

```bash
lsp-trace inspect SELECTOR_OR_ARTIFACT --seed LABEL --json
lsp-trace inspect SELECTOR_OR_ARTIFACT --all-seeds --json
```

Both modes emit `lsp-trace.inspect.v1` JSON and are read-only. Single-seed output uses `projection_kind: "SEED_INSPECTION"`; aggregate output uses `projection_kind: "ALL_SEEDS"`. Both use authority `NON_AUTHORITATIVE_DERIVED_VIEW`, not graph or receipt authority. Unknown labels, invalid inputs, and missing or duplicate same-label aggregate results fail without JSON output.

Direct artifacts pass Draft 2020-12 structural and v3 semantic validation but gain no custody claim. Selectors additionally require a complete generation and valid exact-byte custody receipt. Aggregate native records are copied once; per-seed collections reference them. `TOOL_DERIVED_NODE_CORRELATION` identifies reached-node diagnostic correlation only, never exact per-seed custody or causation. Inspection adds no feature or consumer semantics and does not replace graph validation or selector custody verification.

### Compare two retained seed-evidence sets

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

### Use a comparison

Validate both files before interpreting the result:

```bash
lsp-trace validate --family inspect --version v1 evidence-inspection.json
lsp-trace validate --family filter --version v1 evidence-comparison.json
```

Read the selected seed states and global completeness boundary first:

```bash
jq '{seeds, global_boundary}' evidence-comparison.json
```

`FAILED` means preparation failed and all filterable references are empty. `SUCCESSFUL_EMPTY` means preparation succeeded but the admitted inspection attributes no filterable references to that seed. `SUCCESSFUL_WITH_EVIDENCE` means at least one typed reference is present. Before treating an empty or exclusive partition as informative, check both seed states plus `global_boundary.truncated`, `global_boundary.traversal_complete`, and `global_boundary.source_graph_complete`.

Inspect exact overlap and directional differences by namespace:

```bash
jq '.partitions.nodes' evidence-comparison.json
jq '.partitions.call_relations' evidence-comparison.json
jq '.partitions.dispatch_relationships' evidence-comparison.json
jq '.partitions.sibling_candidates' evidence-comparison.json
jq '.partitions.diagnostic_correlations' evidence-comparison.json
```

Each namespace contains `shared`, `left_only`, and `right_only`. These arrays contain references, not copied records. Join node IDs, relation IDs, and diagnostic indexes back to `evidence-inspection.json` when record details are needed. Keep namespaces separate even when raw values match. Diagnostic indexes are zero-based positions in `records.diagnostics` and express correlation only.

Use `.accounting` to reconcile mechanical counts, not to rank evidence. For every namespace, `shared + left_only` equals the left reference count, `shared + right_only` equals the right reference count, and `shared + left_only + right_only` equals the pairwise universe count.

Operand order controls only the directional names. Re-run with the labels reversed when checking a consumer that treats left and right differently: `shared` and the completeness boundary must remain unchanged, while `left_only` and `right_only`, selected-seed order, and their counts swap. `input_identity.inspection_exact_bytes_digest` identifies the exact inspection bytes used; optional `execution_bundle_id` is copied when present and omitted when absent, never invented.

## Interpretation boundaries

Feature and product adjudication is explicitly external to `lsp-trace`. The tool supplies bounded technical evidence and mechanical projections; external authorities must decide proposition admission, merge/split outcomes, canonical feature identity and naming, user purpose, production use, value, priority, lifecycle, coverage, and acceptance.

### Interpret results honestly

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

## Reference

Use `lsp-trace skill get` to retrieve this complete embedded operator skill, `lsp-trace schema get --schema v1|v2|v3` for exact embedded graph schemas, and the options and contracts above as the command reference. Static retrieval is built in; dynamic skill discovery, listing, and installation are not currently provided.
