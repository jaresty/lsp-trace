# Elixir Call Hierarchy

A clean-room, generic companion Mix project that captures compiler-observed Elixir calls. It currently exposes a small `Code.compile_string/2` boundary for fixtures and tooling experiments; it is not an LSP server and does not inspect user projects or claim authoritative whole-project results.

The tracer records the standard compiler events `remote_function`, `remote_macro`, `imported_function`, and `imported_macro` only when `Macro.Env.function` identifies a source function. Compiler/internal and module-body events are ignored rather than fabricated as calls.

Compiler call events expose resolved `{module, name, arity}` targets. In the currently observed API there is no unresolved-call counterpart for these events, so this milestone documents that boundary and does not invent unresolved evidence. Metadata fields are retained when supplied by the compiler; `column` may therefore be `nil`.

## Local verification

Current non-authoritative local verification was performed with Elixir 1.20.3 and OTP 29. The project targets Elixir `~> 1.16`, has no dependencies, and records Elixir, OTP, and Mix identities with each call.

```sh
mix format --check-formatted
mix test
```
