# Recovered relative-consumer experiment package

- **Status:** `DIAGNOSTIC_INPUTS_ONLY`
- **Pilot:** `PILOT_DISABLED`
- **Source:** recovered from `/tmp/lsp-trace-adr0007-network-v1/`
- **Authority:** non-authoritative; `authority=0`, `accepted=false`

This package preserves the existing fixed-topology experiment without rerunning it. The worker source and four final result files match the digests recorded in ADR 0007.

## Contents

- `worker-fixed-topology.main.go` — worker source, SHA-256 `98bf44170300f84f7d48c0c8694ebc2bfe2ce09660bc59fc89554a7ac1efb09a`
- `worker-fixed-topology.go.mod` — module metadata, SHA-256 `c5f1e03a31459edfdcd86d69a4418756a58beba0427cb8718555cd62b336c5d6`
- `worker-fixed-topology.go.sum` — dependency checksums, SHA-256 `d802b35d06cfa78cb2b12343064a38bfcac74244cb96cca760c8219fdd2ea583`
- four `fixed-topology-supported.result.json` files — result digests recorded in ADR 0007

The source and results are retained as prior diagnostic evidence. They are not a frozen runtime tuple, qualification report, pilot enablement record, or proof of semantic quality. Input packets, model artifacts, full supply-chain evidence, and numeric evaluation labels remain separate prerequisites.
