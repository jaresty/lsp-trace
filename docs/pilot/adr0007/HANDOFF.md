# ADR 0007 pilot handoff

## Current state

The local narrow Describe pilot is enabled only in scope, not publicly or in production.

- Scope: four caller-assembled revision-bound structural packets.
- Acquisition: `TARGET` only.
- Operation: Describe only.
- Model: Qwen2.5-Coder-7B-Instruct-Q4_K_M.gguf, SHA-256 `1664fccab734674a50763490a8c6931b70e3f2f8ec10031b54806d30e5f956b6`.
- Yzma: commit `347c6ee0b893cc0bcac50ff7fb12ece0611fd01a`.
- llama.cpp: `v0.4.0`, installer manifest SHA-256 `b95e8680b4d30761492bbc2d4a6fed656f124c5756dd4387cf02c30d27c13d90`.
- Worker SHA-256: `623ad4341720192d57fe95372343ad7d87450ef8bfb5baa6ecc0c05288d16d83`.
- NDJSON adapter SHA-256: `af22f92bddbaa925cfced6fc93032c398ba671310404a3648c330436a8a171c1`.
- Enablement record: `docs/pilot/adr0007/enablement-record.draft.json`.

## Evidence completed

- Four prior fixed-topology results recovered and hashed.
- Four-packet network-denied worker run: all `COMPLETE / SUPPORTED`.
- NDJSON validation, duplicate, wrong-mode, bad-digest, malformed, deadline, cancellation, and process-group tests passed.
- Full current adapter conformance output: `docs/pilot/adr0007/experiment/full-conformance.jsonl`, SHA-256 `41e494724bbb325e60a90770e30ad386ce31bbceb48711182d22dc88b5ac7040`.
- G4 license, SBOM, vulnerability, native inventory, and revocation records are retained; local exceptions are documented.
- G7 local owner self-review exception accepted.

## Completed continuation

- Verified the local-only enablement record and pinned adapter/worker/runtime identities.
- Added fail-closed preflight: `experiment/preflight-final-four-packet.sh`.
- Located and verified the pinned model artifact at `/tmp/lsp-trace-adr0007-qwen-coder7b-v7-model/Qwen2.5-Coder-7B-Instruct-Q4_K_M.gguf`.
- Ran the final four packets through the unchanged NDJSON adapter under network denial; all returned `COMPLETE / SUPPORTED`.
- Captured immutable v2 outputs and hashes under `experiment/final-four-packet-v2/`.
- Wrote `FINAL-PILOT-REPORT.md` and updated the bundle README.
- Accepted the result as bounded local diagnostic evidence only; no authorization boundary changed.

## Next session tasks

1. Review the final diff and documentation checks.
2. Preserve the enablement record as local-only and draft unless a separately documented governance decision changes it.
3. Do not add public CLI/MCP exposure, core integration, production use, shipment, census behavior, or feature-identity authority.

## Resume prompt

Continue ADR 0007 from `docs/pilot/adr0007/HANDOFF.md`. This is a local-only four-packet TARGET Describe pilot. First inspect the handoff and `git status`; then verify the enablement record, run the final four packets through the pinned NDJSON adapter under network denial, capture immutable output hashes, and write the final pilot report. Do not broaden scope or claim production/public authorization.
