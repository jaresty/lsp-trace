# Elixir Call Hierarchy

A clean-room, generic Elixir Call Hierarchy companion server. Build the executable with `mix escript.build`, then run it as:

```sh
elixir-call-hierarchy --stdio
```

It supports only `initialize`, `initialized`, `textDocument/didOpen`, `textDocument/prepareCallHierarchy`, `callHierarchy/incomingCalls`, `shutdown`, and `exit` over Content-Length framed JSON-RPC stdio.

## Index and trust boundary

On initialize the server uses `rootUri` (or the first workspace folder), asks Mix to load the workspace dependency paths, and invokes the standard Elixir parallel compiler with a compiler tracer. Compiler output and Mix cache/dependency paths are redirected to a disposable directory and removed; authoritative workspace source is never used for build output.

Calls come only from compiler tracer events (`remote_function`, `remote_macro`, `imported_function`, `imported_macro`, `local_function`, and `local_macro`). Source AST traversal identifies function and macro definition boundaries only; it never treats textual references as calls. Compiler and macro expansion are executable code and may have side effects, so initialize must be treated with the same trust as compiling the workspace. Tests use disposable dependency-free workspaces.

Each call record retains the caller module/name/arity and coalesced source definition range, resolved target module/name/arity, exact non-empty call-site range, compiler event kind, and Elixir/OTP/Mix toolchain identity. Dynamic or unsupported evidence is retained only when the compiler emits it; the server does not fabricate unresolved calls. The currently supported compiler event API exposes no unresolved-call counterpart.

## Clause and completeness rules

Multiple clauses are coalesced deterministically by `{module, name, arity}`. The logical definition starts at the earliest clause start and ends at the latest clause end; prepare at any position within that span returns the one logical item.

Incoming calls match the target’s exact `{module, name, arity}`. Results are grouped by caller definition and contain the compiler-observed non-empty call-site ranges. Completeness is server-relative: results are complete only for supported compiler events successfully emitted while compiling the initialized workspace. They do not claim runtime reachability, reflective/dynamic dispatch, generated code hidden from compiler metadata, external callers, or calls suppressed by failed compilation.

## Verification

```sh
mix deps.get
mix format --check-formatted
mix test
mix escript.build
ECH_WRONG_SERVER=1 mix test test/stdio_server_test.exs
```

The last command intentionally produces one assertion-specific red result for each behavior against a present-but-wrong framed stdio server.
