# @lsp-trace/ember-glint-provider

This is an independently installable and versioned external provider package. Core `lsp-trace` does not import it and core release archives do not bundle it. Hosts install it separately and register its absolute executable path.

## Install

From this repository checkout:

```sh
npm install ./providers/ember-glint
npm link --prefix ./providers/ember-glint
```

The executable is `ember-glint`, with stable provider identity `ember-glint@1`. It performs no downloads or executable discovery.

## Analyzer boundary

Framework semantics remain inside this package. The pinned analyzer stack uses Ember Template Compiler, Glint, TypeScript, and Tree-sitter only within retained qualification ceilings. Missing Glint configuration is `BLOCKED`/unavailable, never an empty relation result.

`createProvider({ analyzers })` consumes narrow qualified adapters. The protocol layer validates custody-bearing requests, selects one unambiguous compatible analyzer, bounds and deterministically orders observations, and emits the generic observation envelope. Unsupported relation, language, or framework selections fail explicitly.

## Wire contract

Input is exactly one `Content-Length: N\r\n\r\n` frame containing the generic collector request. Output is exactly one deterministic framed `lsp-trace.provider-observations.v1` envelope. `logical_digest` is SHA-256 over canonical logical response bytes before transport framing and before adding the digest field.

## Registration

Register the installed absolute executable path in the host-owned `providers` section of the `lsp-trace-mcp` bootstrap. The core never downloads, discovers, or selects this package implicitly.
