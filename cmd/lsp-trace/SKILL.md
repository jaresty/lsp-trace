---
name: lsp-trace
description: Trace server-reported incoming callers through an LSP server and optionally expose dispatch-family or top-level sibling discovery relationships.
---

# lsp-trace

Use `lsp-trace` when you need an evidence-preserving upward caller trace from one or more exact source positions.

## Retrieve this skill

```bash
lsp-trace skill get
```

The command prints this complete embedded document to stdout. Static retrieval is built in; dynamic skill discovery, listing, and installation are not currently provided.

## Trace incoming callers

```bash
lsp-trace incoming \
  --workspace /path/to/workspace \
  --server language-server \
  --at path/to/file.ext:LINE:COLUMN
```

`--at` is repeatable. `--seed-file` accepts labeled seeds. Lines and columns are one-based.

### Trace a labeled seed set

Use a seed file when several known source positions should remain distinguishable in one trace. Paths are resolved against `--workspace`; labels must be non-empty and unique. For example, `seeds.json` may contain:

```json
{"seeds":[
  {"label":"report-download","at":"src/reports.ts:42:8"},
  {"label":"scheduled-export","at":"src/export.ts:19:4"}
]}
```

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

### Produce a provisional feature inventory

Use the existing trace JSON to create review candidates, not accepted features:

1. Start with labeled positions that have a stated reason for investigation, such as user-facing handlers, operator entry points, or known workflows.
2. For each same-label seed result, record its requested position, status, and direct `reached_relation_ids`. Do not borrow another seed's relations or treat transitive relations as direct seed support.
3. Follow `edges` from the seeded callee toward server-reported callers. Record stable node and relation IDs so every candidate remains traceable to the JSON.
4. Compare seed traces for overlapping callers or subgraphs. Treat overlap as a reason to review a possible shared capability, not as proof that the seeds are one feature.
5. Keep `dispatch_relationships` and `sibling_candidates` in separate evidence columns. They may nominate related code for investigation but contribute no caller support.
6. Carry terminals, frontiers, diagnostics, `traversal_complete`, and `source_graph_complete` into the inventory as gaps or boundaries. Never silently interpret omitted or truncated code as absent.
7. Emit a provisional record with a candidate label, seed labels, supporting call-relation IDs, discovery-only relation IDs, completeness limits, unknowns, and review status.

Call edges, graph overlap, module names, dispatch associations, and sibling nominations do not establish feature identity, runtime use, user value, canonical naming, or acceptance. Product or domain review must decide whether a candidate is a user feature, operator feature, platform capability, shared component, infrastructure, multiple features, or not a feature.

Important options:

- `--expand-dispatch-family` asks the server's Type Hierarchy for implementation-family members and emits `dispatch_relationships` separately from call edges.
- `--expand-topmost-siblings` asks for document symbols and emits top-level callable candidates in `sibling_candidates`; candidates do not imply calls, visibility, or equivalence.
- `--max-depth`, `--max-nodes`, `--timeout`, and `--request-timeout` bound traversal.
- `--output` publishes through owner-only custody files; v3 atomically writes a selector naming one immutable private generation containing graph and receipt, while explicit v1/v2 publish only their historical graph projection. POSIX uses selector/generation-file mode `0600` and generation-directory mode `0700`; directory entries and the selector basename may still be visible to principals able to list the destination. Windows relies on native account access controls without a POSIX mode claim. Otherwise pure graph JSON goes to stdout. Diagnostics go to stderr. A post-rename destination-directory sync failure is reported even though the selector may already be visible; retained failure evidence is reported only after both file and containing-directory sync.
- `--schema v1|v2|v3` selects compatibility; v3 is default and explicit v1/v2 retain historical projections.
- `lsp-trace schema get --schema v1|v2|v3` prints the exact embedded Draft 2020-12 contract.
- `lsp-trace validate [--schema v1|v2|v3] PATH|-` validates a file or stdin, auto-detects the version by default, rejects explicit mismatches, and applies deeper semantic validation to v3 after structural validation.
- `lsp-trace verify PATH` validates the exact-byte sidecar and embedded semantic receipt offline; success prints no graph.
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

The command asks the server which positions prepare as call-hierarchy items and walks server-reported outgoing calls toward the exact `--down-depth` layer. `frontier_node_ids` contains only nodes at that exact depth. `outgoing_terminal_node_ids` contains nodes reached earlier whose `outgoingCalls` request succeeded with an empty result. `upward_start_node_ids` is the native-ID-sorted union of those two sets, and only that union feeds the existing incoming traversal up to `--up-depth`; the command does not traverse upward from every discovered node. Null, failed, timed-out, canceled, or node-budget-truncated outgoing expansion is not a server-reported leaf. Both depths count edges; zero disables traversal in that direction. The v3 `slice` section references native node and relation IDs rather than duplicating graph records.

Treat a slice as a deterministic bounded projection of information reported by the selected language server, not as complete feature coverage or runtime execution evidence. Dynamic dispatch, reflection, generated code, configuration, templates, framework routing, and other relationships omitted by the server may be absent.

## Interpret results honestly

### Field authority

- Caller-supplied authority: provenance flags, server command/arguments and explicit environment names, requested limits/configuration, and original seed labels/positions are recorded as claims supplied by the invoker.
- Tool-derived authority: normalized identities, digests, memberships, counters, boundaries, sensitivity/evidence metadata, and commitments are deterministic derivations from recorded inputs and observations; they are not independent authentication.
- Server-reported authority: capabilities, prepared targets, call edges, Type Hierarchy associations, document-symbol candidates, diagnostics, and opaque server data report what the configured LSP server returned.
- Custody authority: semantic and exact-byte receipts establish only the documented integrity/custody commitments.

For v3, every original `invocation.seeds` entry has exactly one `seeds` result with the same label, including a failed result when preparation, opening, capability, or traversal fails. In a slice, each successful result's `reached_node_ids` and `reached_relation_ids` are its deterministic causal closure: bounded outgoing reachability from that seed plus incoming reachability from only the upward-start nodes that seed reached. The relation memberships are direct canonical call-relation IDs in that closure, never discovery relations or relations borrowed from an unrelated seed. Shared descendants and callers may belong to multiple seeds, while failed seeds have empty membership and the global graph remains deduplicated. Do not attribute the entire union graph to every seed.

- `edges` contain only server-reported Call Hierarchy caller relationships.
- `dispatch_relationships` are association evidence, not caller evidence.
- `sibling_candidates` are discovery candidates, not usage evidence.
- `evidence_semantics` is the machine-readable claim ceiling. Call edges support only a server-reported caller-to-callee relation; they do not establish runtime execution, feature identity, whole-source completeness, or independent source confirmation.
- Discovery records use `evidence_class: "DISCOVERY_NOMINATION"` and `support_contribution: 0`. They nominate separate investigation and contribute no caller/callee support.
- `evidence_receipt` assigns domain-separated canonical `sha256:` identities to call, sibling, and dispatch relations using the caller-supplied source revision, direction, locator, evidence class, relation kind, and semantic endpoints. Separate memberships join each seed occurrence to its reached call relations and discovery evidence; discovery still contributes zero call support.
- In v3, `trace_receipt.semantic_commitment_digest` commits to canonical semantic content with the receipt omitted; the selected generation's `exact_serialized_bytes_digest` separately commits to exact serialized bytes. Replay inputs use `replay_input_content_digest` and `replay_input_manifest_digest`; process-context claims use role-specific process-context digest names. Offline verification recomputes the semantic and exact-byte commitments and validates bound structure, but these establish integrity/custody only—not authenticity, signature, producer identity, source truth, runtime behavior, or independent confirmation of hidden inputs.
- V3 identity labels caller provenance `CALLER_ASSERTED` and derives resolved seed URI/content digests plus `resolved_seed_contents_digest` scoped `RESOLVED_SEED_CONTENTS`. Failed seeds are not source identities.
- V3 embeds no environment values or working-directory path. `process_context` records tool-derived, domain-separated process-context digests for the effective inherited-plus-override environment, cwd, and explicit environment names, the environment-variable count, and explicit redaction state. The cwd identity uses the cleaned absolute path supplied by the process and does not resolve cwd symlink aliases, so two lexical aliases of the same directory can produce different identities. `automatic_redaction` is false: those omissions are specific projection rules, not general scrubbing. Hashes may confirm guesses; custodians must still control access to arguments, explicit environment names, paths, opaque data, diagnostics, server stderr, trace transcripts, and publication-failure target/error fields.
- `traversal_complete` is scoped to server-reported Call Hierarchy under the requested limits.
- `source_graph_complete` remains `UNKNOWN`.
- Unresolved evidence marks traversal incomplete; dynamic-call evidence is advisory and never fabricates edges.
- `UNKNOWN` provenance is an explicit evidence boundary, not permission to substitute the current timestamp, executable version, repository revision, or server version.

Exit code `0` means traversal completed within this server-relative scope. Exit code `2` means structured but incomplete output. Exit code `1` means invocation or unrecoverable server failure. Exit code `130` means interruption.
