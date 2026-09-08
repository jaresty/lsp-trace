# FR23 public V3 qualification

## History and scope

- Exact base history retained: `c35bf9b` → `ea79a9e` → `0d023c6`; no merge or history rewrite.
- No network, install, deploy, product repository, D01 live action, full suite, or full race was run.

## Deterministic outcome matrix

`TestFR23DeterministicPublicOutcomeMatrix` uses a deterministic fake diagnostics authority at the public V3 wrapping boundary.

- Admitted V2 rows: success, protocol error, deadline, process exit, transport closure, capability unsupported, document supply, unavailable diagnostics, and evicted diagnostics.
- Every admitted row produces an offline-admissible V3 artifact and queries diagnostics exactly once.
- Pre-V2 rows for protocol error, deadline, process exit, transport closure, capability, and unavailable document supply preserve the inherited error, return no artifact, and perform zero diagnostic queries.
- Privacy checks inspect only the V3 diagnostics projection (not embedded frozen V2 bytes) and reject raw stderr/error payload fields, RPC params, source, environment, command, arguments, path, URI, secret text, and invented matched/unmatched/late/read-loop/write-start/write-complete vocabulary.
- Result: 16/16 passed.

## Canonical surface byte fixture

`TestFR23CanonicalPublicSurfaceByteParity` drives the shared canonical acquisition operation twice with identical deterministic fake runtimes: once from typed CLI-style input and once from raw MCP-style JSON input.

- Exact generation: `9007199254740993`.
- V3 artifact bytes are identical, each execution queries diagnostics once, and the artifact passes offline V3 admission.
- V2 typed/raw default bytes remain identical.
- Result: 1/1 passed.

Boundary: this is canonical transport-neutral public-operation parity, not an OS-process CLI-versus-MCP fixture. The process CLI always creates generation 1 and exposes no bounded initial-generation seam, so an actual process fixture at `9007199254740993` was not fabricated or claimed. Existing process publication/inline tests remain separate.

## Persisted mutation RED/GREEN

The persisted mutation guard now has independently named cases for wrong session, wrong generation, unknown status, wrong retained bytes, wrong omitted count, duplicate sequence, reordered sequence, hidden value, structural omission, and structural unknown field.

Disposable exact pre-repair run:

```text
commit: 6902281f45418f78131c0567ed791c701f50baf6
command: go test ./internal/graphprovenance -run '^TestV3OfflineAdmissionRejectsSemanticMutations$' -count=1 -v
result: 2 passed, 9 failed
```

Eight semantic subtests failed assertion-specifically: session, generation, status, bytes, omitted, duplicate, reorder, hidden value. The parent test is the ninth failure. Structural omission and structural unknown already passed at pre-repair and are honestly classified as invariant guards, not invented RED evidence.

Repaired run:

```text
command: go test ./internal/graphprovenance -run '^(TestV3OfflineAdmissionRejectsSemanticMutations|TestV3EnvelopeUsesInheritedV2Limit)$' -count=1 -v
result: 12 passed
```

## Envelope boundary

The guard proves:

- `MaxEnvelopeBytesV2 + MaxDiagnosticBytes == 201392128`.
- a representative semantic V2 artifact wraps and admits as V3.
- a `201392129`-byte envelope is rejected by the limit seam.
- the legacy 48 MiB-derived boundary is not incorrectly treated as the V3 limit.

It does **not** construct or claim a semantically valid artifact of exactly 201392128 bytes. Constructing a valid 192 MiB V2 payload was avoided under bounded resources; no timeout was increased.

## Focused verification

- Normal: `go test ./internal/graphprovenance ./internal/schema ./internal/mcpcontract ./internal/mcp ./acquisitionops ./cmd/lsp-trace ./cmd/lsp-trace-mcp -count=1` — 860 passed in 7 packages.
- Race: `go test -race ./internal/graphprovenance ./internal/mcp ./acquisitionops -run 'TestV3|TestFR23|TestAcquisitionV3' -count=1` — 35 passed in 3 packages.
- Vet: focused seven-package `go vet` — exit 0, no output.
- Build: `go build ./cmd/lsp-trace ./cmd/lsp-trace-mcp` — exit 0, no output.

## Qualification verdict

Source-level V3 outcome, mutation, privacy, arithmetic, and transport-neutral parity guards are green. Mandatory OS-process CLI/MCP parity at exact generation `9007199254740993` remains structurally unavailable through current public process setup and is not claimed. D01 remains unauthorized and was not run.
