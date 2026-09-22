# Fresh dual-root census — blocked before installation

Date: 2026-09-21
Repository: `/Users/schwa/dev/lsp-trace`
Commit observed before execution: `f5ac8ac43a0d61a53da75a214b00025b983795a5`

## Authorized operation

One fresh never-reused dual-root census over:

- `internal/censuscontinuation/pipeline.go`
- `internal/censuscontinuation/capture.go`
- down depth 1, up depth 0
- max nodes 10000
- batch targets 16
- timeout 60s, request timeout 30s
- stop after `DESCRIBE_REQUESTS`

## Blocking prerequisite

The required focused pre-install smoke command was:

```text
go test ./cmd/lsp-trace ./cmd/lsp-trace-mcp ./internal/censuscontinuation ./internal/captureset ./internal/continuationhost ./internal/describerequest
```

Observation 1: timed out after 120 seconds; no test verdict.

Observation 2: the unchanged command timed out after 600 seconds; no test verdict.

Because installation was authorized only after smoke, no binaries were installed and no census prerequisites or census operation were executed.

## Custody and non-actions

- Authorized fresh census attempts consumed: 0
- Census request/receipt/fingerprint: not created
- Fresh qualification roots: not created
- Base/continuation/ledger configuration: not changed
- Public census diagnostic: not produced
- Private failure ledger entry: not produced
- PAUSED selector/checkpoint: not produced
- Replay: not executed
- Offline V6/TARGET inspection: not executed
- Model/worker invocation: none
- Source changes by this execution: none
- Existing unrelated modified and untracked work: preserved
- Bootstrap/tracing/private-ledger defaults: untouched; therefore still at their pre-execution state

## Rollback

No rollback action is required because this execution changed no bootstrap, tracing, ledger, publication, continuation, or session configuration and created no qualification roots. Preserve this report as the sole execution artifact.
