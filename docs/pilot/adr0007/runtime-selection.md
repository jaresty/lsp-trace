# Minimal Describe pilot runtime selection

- **Status:** `PILOT_SELECTION_DRAFT`
- **Pilot:** `PILOT_DISABLED`
- **Owner:** `PROJECT_OWNER`

## Selected candidate

- **Backend:** Yzma
- **Yzma source revision:** `347c6ee0b893cc0bcac50ff7fb12ece0611fd01a`
- **Source path used by recovered worker:** `/tmp/lsp-trace-adr0007-yzma-spike/source/yzma-347c6ee0b893cc0bcac50ff7fb12ece0611fd01a`
- **Yzma `go.mod` SHA-256:** `9f3a52b729e39fa6f521d53b11a463399790bca0b605ae35fbfa071edb45352e`
- **Qualification harness evidence:** `/tmp/lsp-trace-adr0007-yzma-qualification.oEu4KW/`
- **Harness binary SHA-256:** `3f1b84e5aceaf1d99d3f221a38f3b76efe29fb196b5f6800dd8222ae71b28519`
- **Native bundle:** llama.cpp `v0.4.0`, installed by the pinned Yzma installer into `/tmp/adr0007-llama-lib`.
- **Installer manifest SHA-256:** `b95e8680b4d30761492bbc2d4a6fed656f124c5756dd4387cf02c30d27c13d90`
- **Installed manifest copy:** `runtime/yzma-manifest.json`.
- **Primary library hashes:** `libllama.0.4.0.dylib` = `2ed3e44d06e741c777ab7b8619579ebd2009836014344180a18b994c1e1a604a`; `libggml.0.23.0.dylib` = `98fa46f0bcd8c75a335f733c38cb79f911bc39412ba15ef52fee3ac051c14069`; `libggml-base.0.23.0.dylib` = `ceb4884265248e0260ab142821778d5af57fef07171ad73fd0f165bf6fe84b60`; `libggml-cpu.0.23.0.dylib` = `e63a7b4f8eaa5bbd8e93e524d1a828de22a42bead57ae79cfab46e2339571fb2`; `libggml-metal.0.23.0.dylib` = `89bb5b9ffcae6c19feda3338bc64a57b976f9f70a987e66ee49df7e58636058c`.
- **Worker configuration:** timeout `90s`; maximum generated tokens `384`; context capacity `16384`; greedy sampling; grammar `llama_sampler_init_grammar`.

## Selection rationale

The exact source revision is already referenced by the recovered fixed-topology worker and has a separate harness-only qualification report with deterministic decode, schema, resource, and byte-replay checks. That report does not qualify semantic quality or authorize this pilot; it is runtime diagnostic evidence only.

## Remaining pins before execution

- complete installed-library inventory, Mach-O/ABI metadata, signing/quarantine evidence, and SBOM/license/vulnerability review;
- Go toolchain and compiler identity;
- macOS version, architecture, accelerator, driver, thread, context, and batch limits;
- model/tokenizer/chat-template binding;
- worker build digest from the selected source;
- SBOM, license, vulnerability, quarantine, signing, and revocation evidence;
- external network-denial and automatic-download checks.

Selection does not authorize loading, inference, shipment, public availability, or core integration.
