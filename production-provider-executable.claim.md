# Production Provider Executable claim

Frame: Production Provider Executable
Baseline: 0702d12
Owner: production command and package tests only

Claimed paths:

- `cmd/ember-glint-provider/`
- `internal/emberglintprovider/`
- `production-provider-executable.claim.md`

The frame will add an independently compilable production `--stdio` command with stable identity `ember-glint@1`. It will strictly consume one bounded `Content-Length` framed `lsp-trace.provider-collector-request.v1` request and produce one deterministic framed `lsp-trace.provider-observations` version `1` envelope. Request/provider identity, declared bounds, custody handoff, and relation capability admission will fail closed.

Semantic analysis and document custody remain narrow injected interfaces. This frame will not implement an Ember parser, analyzer semantics, fake fallback, qualification tooling, B05 lifecycle, or release admission. The baseline production wiring has no qualified relation adapter and therefore advertises no relation kinds; unavailable or unsupported is reported honestly until a qualified adapter is injected.

Owned tests cover command/package framing, identity, bounds, deterministic output, interface orchestration, capability honesty, and distinct unsupported/unavailable/partial/empty/failed outcomes.
