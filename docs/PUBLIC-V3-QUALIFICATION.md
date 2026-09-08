# FR23 public V3 qualification

## History and scope

- Exact base history retained: `c35bf9b` → `ea79a9e` → `0d023c6`; no merge or history rewrite.
- No network, install, deploy, product repository, D01 live action, full suite, or full race was run.

## Deterministic outcome matrix

`TestFR23DeterministicPublicFifteenOutcomeMatrix` uses a deterministic fake diagnostics authority at the public V3 wrapping boundary and defines exactly 15 independently meaningful rows.

- Admitted V2 rows: success, protocol error, deadline, process exit, transport closure, capability unsupported, document supply, unavailable diagnostics, and evicted diagnostics.
- Every admitted row produces an offline-admissible V3 artifact and queries diagnostics exactly once.
- Pre-V2 rows for protocol error, deadline, process exit, transport closure, capability, and unavailable document supply preserve the inherited error, return no artifact, and perform zero diagnostic queries.
- Privacy checks inspect only the V3 diagnostics projection (not embedded frozen V2 bytes) and reject raw stderr/error payload fields, RPC params, source, environment, command, arguments, path, URI, secret text, and invented matched/unmatched/late/read-loop/write-start/write-complete vocabulary.
- Result: 15/15 outcomes passed.

## Process and canonical surface byte fixtures

`TestFR23V3RealProcessCLIAndMCPByteParityGenerationOne` launches the built `cmd/lsp-trace` and `cmd/lsp-trace-mcp` executables against the same deterministic fake runtime, workspace, manifest, and generation 1. It compares the emitted graph-provenance/v3 artifact bytes directly, without normalization, validates V3 admission, and rejects secret markers.

`TestFR23CanonicalPublicSurfaceByteParity` separately drives the shared canonical acquisition operation twice with identical deterministic fake runtimes: once from typed CLI-style input and once from raw MCP-style JSON input.

- Real-process parity generation: `1`; canonical typed/raw exact-number generation: `9007199254740993`.
- V3 artifact bytes are identical, each execution queries diagnostics once, and the artifact passes offline V3 admission.
- V2 typed/raw default bytes remain identical.
- Result: 1/1 passed.

Boundary: OS-process CLI/MCP parity is qualified at ordinary generation 1. Exact-number decoding at generation `9007199254740993` remains qualified at the canonical typed/raw boundary and is not claimed as huge-generation OS-process parity.

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

- Normal process parity: `go test ./cmd/lsp-trace-mcp -run '^TestFR23V3RealProcessCLIAndMCPByteParityGenerationOne$' -count=1 -v` — 1 passed in 1 package.
- Normal V3 matrix/admission: `go test ./internal/graphprovenance -run 'TestFR23|TestV3|TestAcquisitionV3' -count=1 -v` — 29 passed in 1 package.
- Normal registry/history/verifier: focused `internal/mcpcontract` and `internal/mcp` selections — 5 passed in 2 packages.
- Race: focused graph-provenance selection plus process-parity selection — 30 passed in 2 package runs.
- Vet: `go vet ./internal/graphprovenance ./internal/schema ./internal/mcpcontract ./internal/mcp ./acquisitionops ./cmd/lsp-trace ./cmd/lsp-trace-mcp` — exit 0, no output.
- Build: `go build ./cmd/lsp-trace ./cmd/lsp-trace-mcp` — exit 0, no output.

## Qualification verdict

Source-level V3 outcome, mutation, privacy, arithmetic, canonical typed/raw parity, and ordinary-generation OS-process CLI/MCP byte parity guards are green. Huge-generation OS-process parity is not claimed. D01 remains unauthorized and was not run.
