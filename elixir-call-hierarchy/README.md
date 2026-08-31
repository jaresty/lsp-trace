# Elixir Call Hierarchy

A clean-room, generic Elixir Call Hierarchy companion server. Build the executable with `mix escript.build`, then run it as:

```sh
elixir-call-hierarchy --stdio [--cache-dir PATH] [--reindex] [--profile]
```

`--cache-dir` overrides the compatible per-user cache location. `--reindex` clears and rebuilds only the current fingerprinted entry under its lock. `--profile` emits machine-readable `ECH_PROFILE` JSON lines on stderr for fingerprinting, cache lookup and lock wait, dependency compilation/loading, definition parsing, project compilation, tracer draining, index serialization, and total initialization. Unknown options are rejected on stderr.

It supports only `initialize`, `initialized`, `textDocument/didOpen`, `textDocument/prepareCallHierarchy`, `callHierarchy/incomingCalls`, `shutdown`, and `exit` over Content-Length framed JSON-RPC stdio.

## Index and trust boundary

On initialize the server uses `rootUri` (or the first workspace folder), reads already-restored dependency sources from the workspace's `deps/` directory, and invokes Mix plus the standard Elixir parallel compiler with a compiler tracer. It never runs `deps.get`. `MIX_DEPS_PATH` remains the workspace `deps/`, while `MIX_BUILD_PATH` is a stable fingerprinted directory under an external user cache, so project and dependency build artifacts never enter the workspace.

The cache identity covers cache/indexer schema, Elixir/OTP/Mix, OS and architecture, `MIX_ENV`, workspace identity, and deterministic content digests for `mix.exs`, `mix.lock`, `config/`, `lib/`, and restored `deps/` sources. VCS/build junk and directory symlinks are excluded. Indexes use validated versioned JSON; corrupt entries rebuild atomically. An entry-specific bounded cross-process lock performs a second hit check, so a valid hit loads the complete persisted index without compiling dependencies or project files again. The authoritative source repository must still be trusted: a cold miss executes compiler and macro code.

Calls come only from compiler tracer events (`remote_function`, `remote_macro`, `imported_function`, `imported_macro`, `local_function`, and `local_macro`). Source AST traversal identifies function and macro definition boundaries only; it never treats textual references as calls. Compiler and macro expansion—including dependency compilation—are executable code and may have side effects, so initialize must be treated with the same trust as compiling the workspace. Tests use disposable workspaces, including a hermetic restored-dependency fixture.

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
