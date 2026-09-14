# CLI migration diagnostics

Status: Proposed ADR

This proposal adds diagnostics to the compatibility window; it does not approve command removal or replacement parity. No removal release is scheduled. Every event therefore carries `removal_release: "UNSCHEDULED"`.

The lifecycle remains at Add → Warn. The CLI `census` replacement is AVAILABLE for accountable source-symbol enumeration, and `context` is AVAILABLE for bounded transient live structural analysis; replacement parity remains incomplete because their evidence, selector, and custody contracts do not generally equal legacy `slice` and `incoming`. Primary help continues to show the historical `slice` and `incoming` acquisition commands. They remain callable and retain legacy dispatch, and removal remains `UNSCHEDULED`. Broad specialist-help cleanup is deferred until this ADR is accepted and every affected lane has full parity; this proposal does not partially reshape those help surfaces.

A valid non-help legacy invocation emits exactly one human warning on stderr after its complete syntax, flag values, arity, and required command inputs have been validated. Invalid flags, invalid values, invalid arity, missing required inputs, and help emit no deprecation warning. `slice` points to the available exact-target `trace` facade. `incoming` points to `trace` conditionally: callers must retain legacy dispatch when selectors, bounds, identity, accounting, bytes, or custody would differ.

Machine mode has one unambiguous global placement: immediately after the legacy command. The exact leading forms `--machine` and `--machine=true` enable machine mode; `--machine=false` disables it. Any other `--machine=<value>` is a closed, stable invocation error rather than Go flag-parser prose. At most one leading machine form is allowed. Repeated or conflicting leading forms are deterministic syntax errors; if any repeated form enables machine mode, the error is one strict JSONL event, otherwise it is the stable human error `duplicate --machine`.

Scanning stops at the first token that is not one of those leading forms. From that point, all tokens belong to the command FlagSet. Thus positional or non-leading `--machine` tokens, values such as `--server-arg --machine` and `--server-arg=--machine`, repeated `--server-env`, `--flag=value`, and any command-supported `--` grammar remain untouched.

Legacy v1 `incoming` warning admission parses command syntax exactly once. The pre-warning parse performs no profile/config/seed/manifest/source reads and starts no provider, session, process, network, publication, or capture work. Execution consumes that same syntax result; profile and seed-file loading and all runtime validation remain after the warning boundary.

In machine mode stderr is JSON Lines with one object per line and the closed schema `lsp-trace.cli-diagnostic.v1`:

- `schema_version`
- `code`
- `severity`
- `operation`
- `replacement`
- `replacement_status`
- `removal_release`

Every field is required and nonempty; unknown fields are rejected by strict consumers. Stable codes are `CLI_LEGACY_OPERATION` and `CLI_INVOCATION_ERROR`. The closed event structurally cannot carry arbitrary error text, filesystem paths, source content, server stderr, environment values, or secrets. Valid invocations preserve existing stdout bytes and exit codes. Machine syntax failures emit only the generic structured error event rather than human parser text.

`census` is an actionable CLI command for accountable source-symbol enumeration and batched acquisition; `trace` is the actionable exact-target facade. `context` is AVAILABLE with authority-zero, non-retained structural output. A legacy invocation maps to a replacement only when selectors, bounds, identity, accounting, output bytes, and custody are preserved; otherwise legacy dispatch remains authoritative. Command-specific structured failure details are deferred until each command has a reviewed, privacy-safe code taxonomy.
