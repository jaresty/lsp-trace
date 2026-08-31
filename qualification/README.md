# Language-server qualification

These TypeScript, C#, and Elixir fixtures qualify the public `lsp-trace incoming` command without adding language-specific behavior to the Go binary. Each run has exactly one state: **PASS, BLOCKED, or FAIL**.

- **PASS**: the server ran and the expected caller graph and call-site ranges matched.
- **BLOCKED**: a required executable or SDK is unavailable, or the server lacks useful `prepareCallHierarchy`/`incomingCalls` support. This is a retained release blocker/skip, never a pass.
- **FAIL**: the server ran but setup, capability, command execution, or graph assertions failed.

A PASS requires retained server version, initialize capability response, command, stdout graph, stderr, and assertion report under `qualification/evidence/<language>/`. Evidence is intentionally ignored by Git because it may contain machine paths or opaque server data; attach the directory to the release record. Never promote BLOCKED to PASS based only on fixture presence.

Both fixtures contain branching, recursion, and `StaticButNotExecuted`, a caller behind a runtime branch the fixture entry point does not take. Its possible appearance in Call Hierarchy demonstrates a static language-server report, not observed execution.

Run:

```sh
./scripts/qualify.sh typescript
./scripts/qualify.sh csharp
./scripts/qualify.sh elixir
```

For ElixirLS distributions that do not install `elixir-ls` or `language_server.sh` on `PATH`, set `ELIXIR_LS_COMMAND` to the executable path. The qualification path records the version probe, capability result, exact expected callers, and non-empty call-site ranges when supported. It never substitutes references, AST inspection, compiler tracing, or language-specific traversal.

Qualification is opt-in because language servers may restore packages, run project logic, read workspace files, access the network, and emit sensitive data.
