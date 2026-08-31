# Language-server qualification

These TypeScript, C#, and Elixir fixtures qualify the public `lsp-trace incoming` command without adding language-specific behavior to the Go binary. Each run has exactly one state: **PASS, BLOCKED, or FAIL**.

- **PASS**: the server ran and the expected caller graph and call-site ranges matched.
- **BLOCKED**: a required executable or SDK is unavailable, or the server lacks useful `prepareCallHierarchy`/`incomingCalls` support. This is a retained release blocker/skip, never a pass.
- **FAIL**: the server ran but setup, capability, command execution, or graph assertions failed.

A PASS requires server version, initialize capability response, command, stdout graph, stderr, and assertion report under `qualification/evidence/<language>/`. Raw evidence is ignored by Git because it may contain machine paths, binaries, or opaque server data. After all required runs pass, run `./scripts/retain-qualification.py`; it refuses non-PASS inputs, excludes binaries, normalizes the repository path to `${REPO}`, and writes reviewable evidence under `qualification/retained/<language>/`. Never promote BLOCKED to PASS based only on fixture presence.

All three fixtures contain branching, recursion, and a static-only caller behind a runtime branch the fixture entry point does not take. Its possible appearance in Call Hierarchy demonstrates a static language-server report, not observed execution.

Run:

```sh
./scripts/qualify.sh typescript
./scripts/qualify.sh csharp
./scripts/qualify.sh elixir
```

For ElixirLS distributions that do not install `elixir-ls` or `language_server.sh` on `PATH`, set `ELIXIR_LS_COMMAND` to the executable path. The Elixir fixture targets a multi-clause `leaf/1` and requires same-module direct, recursive, and static-only callers plus cross-file calls from another module through both an alias and a fully qualified module name. The assertion report must separately state `protocol support, same-module resolution, cross-module resolution, and multi-clause resolution`, followed by exact non-empty call-site range coverage. A graph that advertises Call Hierarchy but omits either cross-module caller edge is **BLOCKED**, never PASS.

Current retained graphs use `lsp-trace.graph.v2`; callers that need the compatibility projection may still request `lsp-trace.graph.v1`. TypeScript and C# retain PASS evidence. ElixirLS retains a BLOCKED graph and clean assertion report naming the missing aliased and fully qualified cross-module callers; it does not establish cross-module or multi-clause support.

Qualification is opt-in because language servers may restore packages, run project logic, read workspace files, access the network, and emit sensitive data. It never substitutes references, AST inspection, compiler tracing, or language-specific traversal.
