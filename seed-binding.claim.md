# Atomic pre-provider seed binding claim

## Baseline and scope

Completed from predecessor `873c4cc` on current main startup-sink baseline `6c7c639`. Work remained repository-local: no delegation, network, install, deploy, product access, live D01, LSP provider launch, full suite, or full race.

## Delivered boundary

- The closed `lsp-trace.seed-binding.v2` manifest is now transported explicitly by current-only CLI acquisition v3 private rooted selectors and by the host-owned MCP bootstrap process request object. Both converge on the same `sessionruntime.StartRequest` and semantic outcome.
- Historical v1/v2 CLI defaults, request bytes, registry/tool count, MCP operation schemas, and output bytes are unchanged. No new registry operation or public schema was added. The CLI additions are rejected unless the explicit acquisition version is v3.
- Host bootstrap authorizes one exact external semantic validator protocol (`lsp-trace.seed-validator.v1`) with exact language/authority/name/version, absolute executable and working directory, fixed arguments, explicit non-inherited environment, timeout, request cap, and response cap. It is a preflight helper, not an LSP provider.
- One canonical JSON request supplies the exact mechanically retained source bytes. One strict canonical JSON response must bind validator identity, declaration name/file/name-range/full-range, observed SHA-256, and observed revision. Unknown, extra, contradictory, malformed, oversized, timed-out, or failed output cannot become MATCH.
- Mechanical and semantic validation completes before startup-attempt allocation and before `Starter.Start`; failures retain privacy-safe terminal codes and zero starter/provider activity. No C# semantics are inferred lexically.
- Rooted selectors reject absolute, non-clean, and escaping selectors. Public CLI failures expose terminal classifications only, never source bodies or absolute private paths.

## Compatibility decision

The feature composes through additive arguments on explicit current-only v3 CLI operations and through optional host bootstrap fields. Because no frozen MCP operation request schema or historical operation was changed, a v4 operation was not warranted. The existing 25-operation historical registry boundary is preserved.

## Qualification boundary

- Focused normal: `go test ./internal/seedbinding ./sessionruntime ./cmd/lsp-trace ./cmd/lsp-trace-mcp -count=1` — 533 passed.
- Focused race: selected seed-binding/transport/bootstrap tests across the same four packages — 25 passed.
- Focused vet: the same four packages — `FOCUSED_VET_PASS`.
- Focused build: `./cmd/lsp-trace` and `./cmd/lsp-trace-mcp` — `FOCUSED_BUILD_PASS`.
- The fake host executable covers MATCH, MISMATCH, UNAVAILABLE, INVALID, oversized, extra-field, and contradictory responses; existing tests cover wrong in-bounds declaring file and starter/attempt/census zero.
- No full suite, full race, provider launch, product access, network, install, deployment, or live D01 run occurred.

## C# product adapter and D01

No trusted in-process C# parser or admitted host C# semantic validator binary exists in this repository. Therefore the product C# adapter is **unavailable**, a C# manifest without an exactly configured matching host adapter terminates as `SEED_BINDING_UNAVAILABLE`, and the provider never starts. D01 is **not ready and was not rerun**. This change supplies the host protocol and transport only; it makes no product qualification or provider-availability claim.
