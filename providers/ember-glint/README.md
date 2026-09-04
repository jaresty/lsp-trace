# Ember/Glint external provider

This package owns the `ember-glint` Content-Length-framed stdio executable and the stable provider identity `ember-glint@1`. It has no runtime dependencies and performs no downloads or executable discovery.

## Install or run offline

From this directory:

```sh
npm link --offline
ember-glint
```

It can also be invoked without installation:

```sh
node ./bin/ember-glint.mjs
```

A host must register the resulting absolute executable path explicitly.

## Analyzer interface

`createProvider({ analyzers })` accepts qualified analyzer adapters with this narrow interface:

```js
{
  id: 'qualified-analyzer@1',
  languages: ['glimmer-js'],
  frameworks: ['ember'],
  relationKinds: ['BINDS_ARGUMENT'],
  async analyze(genericRequest) {
    return { outcome: 'COMPLETE', observations: [], coverage: { status: 'BOUNDED' } };
  }
}
```

The protocol package validates custody-bearing generic requests, chooses an unambiguous compatible adapter, invokes it, deterministically orders and bounds its observations, and maps its result into the generic observation envelope. Analyzer adapters retain all framework and language semantic responsibility. With no injected qualified analyzer, metadata advertises no capabilities and requests return `UNAVAILABLE`.

## Wire contract

Input is exactly one canonical `Content-Length: N\r\n\r\n` frame containing a `lsp-trace.provider-request.v1` JSON object. Output is one deterministic frame containing a `lsp-trace.provider-observations.v1` object. `logical_digest` is SHA-256 over canonical logical response bytes before transport framing and before the digest field is attached.
