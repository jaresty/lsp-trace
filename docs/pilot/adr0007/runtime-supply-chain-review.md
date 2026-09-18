# G4 supply-chain review record

- **Status:** `PREREQUISITES_DRAFT`
- **Pilot:** `PILOT_DISABLED`
- **Bundle:** llama.cpp `v0.4.0`
- **Installer:** Yzma commit `347c6ee0b893cc0bcac50ff7fb12ece0611fd01a`
- **Bundle manifest SHA-256:** `b95e8680b4d30761492bbc2d4a6fed656f124c5756dd4387cf02c30d27c13d90`

## License

The fetched bundle includes an MIT license covering the ggml/llama.cpp software, copyright The ggml authors 2023–2026. The license permits use, modification, distribution, sublicensing, and sale subject to preserving the notice and disclaimer.

This is a local license observation, not legal approval for redistribution of the complete pilot tuple. Yzma, the Qwen model, model-card terms, and transitive dependencies require separate review.

## Inventory evidence

The copied installer manifest records the upstream asset digest and source tag. The runtime directory contains versioned and unversioned aliases for the native libraries plus auxiliary executables and implementation libraries. The primary semantic loading set is:

- `libllama.0.4.0.dylib`
- `libllama-common.0.4.0.dylib`
- `libmtmd.0.4.0.dylib`
- `libggml.0.23.0.dylib`
- `libggml-base.0.23.0.dylib`
- `libggml-cpu.0.23.0.dylib`
- `libggml-blas.0.23.0.dylib`
- `libggml-metal.0.23.0.dylib`
- `libggml-rpc.0.23.0.dylib`

All files in the installed directory must remain bound by the complete generated hash inventory before execution.

## Revocation and vulnerability status

No independent vulnerability scan, revocation check, or SBOM generator was available in this verification pass. Those are **OPEN**. The bundle must not be treated as approved for pilot execution until an SBOM, vulnerability result, revocation owner, and update/replacement policy are recorded.

## G4 decision

`OPEN_WITH_GAPS`: license evidence and upstream manifest are present; SBOM, vulnerability/revocation review, model licensing, and complete inventory binding remain required. No inference or pilot enablement is authorized.
