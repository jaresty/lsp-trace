# Outgoing custody-complete correction addendum v1

## Disposition

`OUTGOING_CUSTODY_RERUN_ADDENDUM_AUTHORED_NOT_GO` — authored, not GO. No full campaign was run. No historical evidence was overwritten.

## Scope

This addendum is direction `outgoing` only and binds, if later reviewed and executed, only to `CALLS_GO_GOPLS_OUTGOING_EXACT_V1` / `CALLS_LSP_OUTGOING_EXACT_V1`. It creates a fresh atomic namespace `lsp-trace.adr0011.calls.go.gopls.outgoing.custody_rerun.v1`.

The committed incoming custody addendum is cited only as provider-neutral implementation precedent for custody mechanics: private exact custody, public allowlist summaries, no-follow bounded reads, fresh namespace reservation, package-owned final seals, and tamper rejection. It contributes no outgoing evidence authority, no profile GO, and no transferable occurrence ledger.

## Frozen workload denominator and corpus

- Target denominator: `5900`.
- Batch size/count: `50` / `118`.
- Direction: `outgoing` only.
- Retry policy: retry-free final ledger; append after final and retry before terminal are rejected by the package verifier/sealer.
- Raw response ceiling: `33554432` bytes (32 MiB).
- Completeness/frontier: `UNKNOWN`; target-set-relative only.

## Mandatory custody before launch

A reviewed full run must capture before `gopls serve` launch:

1. exact complete environment object and digest;
2. exact object passed explicitly to `subprocess.Popen(..., env=captured_env, ...)`;
3. gopls canonical path, byte length, SHA-256, version command result, argv, and cwd;
4. Go executable path/digest/length/version and closed GOROOT/GOTOOLDIR/go-tool inventory;
5. Python executable/runtime and wrapper/original-runner/contract source digests;
6. exact initialize body/framed bytes, rootUri, workspaceFolders, and callHierarchy client capability;
7. public allowlist summary with no private path, raw env, key-name, argv, cwd, rootUri, or workspaceFolders leakage.

## Non-authorities

This package grants no implementation GO, production GO, public API/runtime change, custody-owner status, composition authority, incoming-provider authority, ceiling change, or historical evidence rewrite. No automatic full run may occur before independent review.
