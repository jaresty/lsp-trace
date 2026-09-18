# ADR 0007 final pilot report

## Decision

**RETAIN AS A COMPLETED BOUNDED LOCAL DIAGNOSTIC PILOT.** The exact pinned model was restored and the four-packet run completed successfully. This report does not authorize production, public, MCP, core, census, shipment, or any other broadened use.

## Acceptance

Accepted by the project owner as bounded local diagnostic evidence. This acceptance does not promote the draft enablement record, qualify the worker for production, or authorize any public, core, MCP, shipment, census, or feature-identity use.

## Bound scope

- Operation: `Describe`
- Acquisition: `TARGET`
- Packets: exactly four caller-assembled revision-bound packets
- Runtime controls: pinned adapter and worker digests; llama native bundle; macOS `sandbox-exec` with `(deny network*)`
- Enablement record: `enablement-record.draft.json`, `PILOT_ENABLED_LOCAL_ONLY`
- Model required: `Qwen2.5-Coder-7B-Instruct-Q4_K_M.gguf`, SHA-256 `1664fccab734674a50763490a8c6931b70e3f2f8ec10031b54806d30e5f956b6`

## Final invocation

The revised fail-closed preflight is `experiment/preflight-final-four-packet.sh`; it checked the model, adapter, and worker SHA-256 values before any packet was sent. The prior missing-model attempt terminated `PREFLIGHT_FAILURE` without packet execution.

The final v2 invocation accepted all four valid envelopes. Terminal response counts:

| Packet | Terminal status |
|---|---|
| 01 | `COMPLETE` / `SUPPORTED` |
| 02 | `COMPLETE` / `SUPPORTED` |
| 03 | `COMPLETE` / `SUPPORTED` |
| 04 | `COMPLETE` / `SUPPORTED` |

Each response included bounded citations, limitations, and terminal accounting.

## Immutable run artifacts

Directory: `experiment/final-four-packet-v2/`

- `requests.jsonl`: `fb7e263721bd01af7cc75dd83c972fff027f6ecc9950ebf65a05c178a20aeea4`
- `responses.jsonl`: `ad884a0493c2352e75c1fc79acee77fd97c16e33c7ef8c2911fced6d72158284`
- `stderr.log`: `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`
- Hash manifest: `SHA256SUMS.final`

The adapter binary used hashes to `af22f92bddbaa925cfced6fc93032c398ba671310404a3648c330436a8a171c1`; the worker binary used hash `623ad4341720192d57fe95372343ad7d87450ef8bfb5baa6ecc0c05288d16d83`.

## Assessment

The run demonstrates envelope admission and terminal backend-failure accounting under network denial. It does **not** demonstrate semantic correctness, replay, latency qualification, or successful four-packet Describe behavior. Keep the pilot local-only and outside production/public/core surfaces. The bounded four-packet diagnostic run may be retained as local evidence; it is not a production qualification, public authorization, repository census, or feature-identity claim.
