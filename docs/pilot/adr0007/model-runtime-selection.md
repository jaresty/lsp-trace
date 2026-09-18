# Minimal Describe pilot model/runtime selection

- **Status:** `PILOT_SELECTION_DRAFT`
- **Pilot:** `PILOT_DISABLED`
- **Owner:** `PROJECT_OWNER`

## Selected candidate

- **Model:** `Qwen2.5-Coder-7B-Instruct-Q4_K_M.gguf`
- **Local staging path:** `/tmp/lsp-trace-adr0007-qwen-coder7b-v7-model/Qwen2.5-Coder-7B-Instruct-Q4_K_M.gguf`
- **SHA-256:** `1664fccab734674a50763490a8c6931b70e3f2f8ec10031b54806d30e5f956b6`
- **Byte length:** `4683074336`
- **Backend candidate:** Yzma / local `llama.cpp` runtime
- **Existing harness evidence:** `/tmp/lsp-trace-adr0007-yzma-qualification.oEu4KW/`
- **Selection rationale:** the recovered relative-consumer experiment is associated with the `qwen-coder7b-v7` candidate, and 7B provides the closest continuity with those diagnostic outputs.

## Required pins before execution

- exact model SHA-256 and byte length;
- exact model provenance, license, redistribution, and model-card record;
- exact tokenizer and chat-template identities;
- exact Yzma source/revision and native-library identities;
- exact worker build digest;
- OS, architecture, accelerator, driver, thread, context, and resource limits;
- prompt, grammar, sampling, and seed digests;
- network-denial and automatic-download verification;
- complete SBOM and vulnerability/revocation review.

## Alternatives not selected

- Qwen2.5-Coder-3B-Instruct-Q4_K_M: smaller resource footprint, but weaker continuity with the recovered v7 diagnostic experiment.
- Any hosted or network-fetched model: outside the pilot boundary.

Selection does not authorize model loading, inference, indexing, shipment, public availability, or core integration. The pilot remains disabled until the complete runtime tuple is digest-bound and the local enablement record is accepted.
