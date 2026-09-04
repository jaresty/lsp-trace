# Host-provisioned relation providers

`lsp-trace-mcp` can run a **host-provisioned** relation provider for explicit non-`CALLS` relation requests to `lsp_trace_v1_incoming` and `lsp_trace_v1_slice`. This is an additive graph-v4 capability: omitting `relations` retains the historical graph-v3 CALLS-only path and does not resolve or start a provider.

## Authority and compatibility

The host owns every executable path, argument, working directory, environment entry, provider identity, capability declaration, and limit. MCP callers can select only `"auto"`, `"none"`, or one configured provider identity such as `"example@1"`; they cannot supply an executable, command, environment, or provider limit.

`CALLS` remains server-reported Call Hierarchy evidence. Providers may report only configured non-`CALLS` relation kinds. Provider results do not fill, upgrade, or hide an incomplete CALLS result. A provider observation is `PROVIDER_REPORTED`; normalized graph-v4 relations are adapter-derived and retain every contributing observation ID.

This protocol is v1. Unknown configuration fields, trailing JSON, unsupported relation kinds, ambiguous `auto` selection, malformed frames, malformed envelopes, identity mismatches, invalid custody, and exceeded limits fail closed.

## Bootstrap configuration

The host supplies an absolute JSON bootstrap file to `lsp-trace-mcp --bootstrap-config`. The provider declaration is structurally defined by [`lsp-trace.bootstrap-provider.v1`](../schema/schemas/lsp-trace.bootstrap-provider.v1.schema.json). Existing configurations containing only `processes` remain valid. Add a top-level `providers` array to make relation collection available:

```json
{
  "version": 1,
  "processes": [{
    "alias": "workspace",
    "profile": {
      "trust_domain": "local-development",
      "workspace": "/absolute/path/to/workspace",
      "profile": "gopls",
      "environment_reference": "developer"
    },
    "execution": {
      "path": "/absolute/path/to/gopls",
      "directory": "/absolute/path/to/workspace"
    }
  }],
  "providers": [{
    "schema_version": "lsp-trace.bootstrap-provider.v1",
    "identity": "example@1",
    "version": "1.0.0",
    "protocol": {"name": "lsp-trace.provider-observations", "version": "1"},
    "execution": {
      "path": "/absolute/path/to/example-provider",
      "arguments": ["--stdio"],
      "directory": "/absolute/path/to/workspace",
      "environment": ["EXAMPLE_PROVIDER_MODE=production"]
    },
    "capabilities": {
      "relations": ["PASSES_CALLBACK"],
      "languages": ["typescript"],
      "frameworks": ["example-framework"]
    },
    "limits": {
      "request_bytes": 4096,
      "response_bytes": 4096,
      "protocol_messages": 1,
      "stderr_bytes": 128,
      "wall_time_ms": 1000,
      "termination_grace_ms": 50
    }
  }]
}
```

All execution paths and, when present, working directories must be absolute. Provider identities use `name@version`; declarations require non-empty protocol identity, capabilities, and positive limits within the host maxima. Environment entries replace only the provider process environment; keep secrets out of configuration and logs.

## Selection and lifecycle

Request a provider only with an explicit non-CALLS relation selection. `providers` defaults to `auto`; `auto` succeeds only when exactly one configured provider supports every requested relation. `none` disables provider admission. A selected provider is run once through a bounded subprocess lifecycle. It receives one framed request, is terminated on caller cancellation or wall-time expiry, and is reaped before a receipt is accepted. Provider stderr is bounded and may be sensitive.

The strict request payload is structurally defined by [`lsp-trace.provider-collector-request.v1`](../schema/schemas/lsp-trace.provider-collector-request.v1.schema.json) and contains the admitted provider and adapter IDs, exact managed session ID/generation, seed, selected relations, original URI and workspace revision custody, and operation limits. It uses LSP-style framing:

```text
Content-Length: <UTF-8 JSON byte count>\r\n
\r\n
<one JSON object>
```

The provider emits exactly one response with the same framing. Multiple messages, invalid lengths, malformed JSON, or payloads over the declared bound fail closed.

## Observation envelope and output

The response is one [`lsp-trace.provider-observations.v1`](../schema/schemas/lsp-trace.provider-observations.v1.schema.json) envelope with non-empty provider, protocol, and adapter identities; `authority: "PROVIDER_REPORTED"`; coverage and failure state; immutable document custody; and zero or more observations. Each observation has a selected non-CALLS kind, endpoints, exact original anchor, optional mapped virtual anchor, and explicit support/non-support claims.

Every original anchor must match a declared immutable document record. A virtual anchor is valid only with that document's deterministic mapping and never replaces the original anchor. The adapter rejects mismatched provider/protocol/adapter/request identity, document revision, URI, mapping, or selected relation.

Incoming and slice compose provider evidence into a graph-v4 response while retaining separate CALLS outcomes and receipts. A provider result can be complete, empty, partial, bounded, unavailable, unsupported, transport-failed, or malformed; absence of a relation is never evidence of absence outside its explicit coverage boundary.

## Security and upgrades

Providers run with the developer's permissions, are not sandboxed, and may read local files or access the network. Configure only trusted provider binaries and workspaces. Provider request/response payloads, source anchors, provider stderr, environment names, and published artifacts can be sensitive; protect bootstrap files, logs, and receipts accordingly.

The bootstrap declaration schema, collector request schema, and observation protocol are versioned. A provider upgrade that changes identity, protocol version, capability set, or payload semantics requires a new reviewed host declaration. Keep graph-v3 consumers on omitted `relations`; graph-v4 consumers must explicitly handle provider coverage, failure, custody, and provenance fields.
