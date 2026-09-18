# G4 license and revocation review

- **Status:** `OPEN_WITH_REVIEWED_LICENSES`
- **Pilot:** `PILOT_DISABLED`

## Yzma

- Source revision: `347c6ee0b893cc0bcac50ff7fb12ece0611fd01a`
- License: Apache-2.0
- Source license SHA-256: `3b45394652c42955cfbac625d814c86df260dac8a720171bf634e9bc2a4867ba`
- Notice: Yzma identifies included MIT-licensed gollama.cpp portions.

## llama.cpp bundle

- Bundle: `v0.4.0`
- Bundle license: MIT
- License SHA-256: `94f29bbed6a22c35b992c5c6ebf0e7c92f13b836b90f36f461c9cf2f0f1d010d`
- Installer manifest SHA-256: `b95e8680b4d30761492bbc2d4a6fed656f124c5756dd4387cf02c30d27c13d90`

## Qwen model

- Model: `Qwen2.5-Coder-7B-Instruct-Q4_K_M.gguf`
- GGUF metadata: `general.license=apache-2.0`
- Metadata license link: `https://huggingface.co/Qwen/Qwen2.5-Coder-7B-Instruct/blob/main/LICENSE`
- Retrieved license SHA-256: `832dd9e00a68dd83b3c3fb9f5588dad7dcf337a0db50f7d9483f310cd292e92e`
- Retrieved license length: `11343` bytes

## Revocation and update ownership

Local-project owner: `PROJECT_OWNER`.

The owner must revalidate upstream model, Yzma, and llama.cpp release status before any future run. Any withdrawn license, security advisory, compromised artifact, or superseding runtime invalidates the tuple and requires a new digest-bound selection and rebuild.

## Decision

License evidence is now locally retained for Yzma, llama.cpp, and Qwen. Vulnerability evidence is clean for the narrowed rebuilt worker, with the Windows-only advisory remediated. Full SBOM interpretation and ongoing revocation monitoring remain local-owner responsibilities. This record does not itself enable the pilot.
