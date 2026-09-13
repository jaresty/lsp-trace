# CLI migration diagnostics

Status: current compatibility-window contract

Primary help hides the historical `slice` and `incoming` acquisition commands. They remain callable so existing scripts and artifact production retain their stdout bytes and exit codes. `lsp-trace legacy` in primary help is a documentation pointer, not a new command implementation.

A non-help legacy invocation emits exactly one human warning on stderr after the command token selects a legacy operation. `slice` points to the available exact-target `trace` facade. `incoming` points to `trace` conditionally: callers must retain legacy dispatch when selectors, bounds, identity, accounting, bytes, or custody would differ.

Passing `--machine` after a legacy command changes diagnostics only. Stderr becomes JSON Lines with one object per line and the closed schema `lsp-trace.cli-diagnostic.v1`:

- `schema_version`
- `code`
- `severity`
- `operation`
- `replacement`
- `replacement_status`

Stable codes currently are `CLI_LEGACY_OPERATION` and `CLI_INVOCATION_ERROR`. The closed event intentionally carries no arbitrary error text, filesystem path, source content, server stderr, environment value, or secret. Parse, validation, and domain failures retain their existing exit code and stdout bytes and add the generic structured error event after the single deprecation event.

`census` and `context` remain `FUTURE/PROPOSED`; this diagnostic layer does not implement them. Command-specific structured failure details are deferred until each command has a reviewed, privacy-safe code taxonomy. Until then machine mode reports the stable generic invocation-error code rather than guessing or exposing raw diagnostics.
