# G4 runtime verification record

- **Status:** `PREREQUISITES_DRAFT`
- **Pilot:** `PILOT_DISABLED`
- **Artifact:** `/tmp/adr0007-llama-lib`
- **Bundle:** llama.cpp `v0.4.0`
- **Bundle manifest:** `b95e8680b4d30761492bbc2d4a6fed656f124c5756dd4387cf02c30d27c13d90`

## Observed verification

- All inspected dynamic libraries are Mach-O 64-bit `arm64`.
- `libllama.0.4.0.dylib` depends on the version-matched `libggml` family and system `libc++`/`libSystem` libraries.
- `libggml-metal.0.23.0.dylib` depends on Apple Metal/Foundation frameworks.
- `codesign --verify --deep --strict libllama.0.4.0.dylib` exited successfully.
- Code signatures are ad hoc/linker-signed with no Team ID; this is not vendor identity or notarization evidence.
- `com.apple.provenance` extended attribute was present on the primary library.
- Host: Darwin `25.5.0`, `arm64`.
- Go: `go1.26.5 darwin/arm64`.

## Primary hashes

- `libllama.0.4.0.dylib`: `2ed3e44d06e741c777ab7b8619579ebd2009836014344180a18b994c1e1a604a`
- `libggml.0.23.0.dylib`: `98fa46f0bcd8c75a335f733c38cb79f911bc39412ba15ef52fee3ac051c14069`
- `libggml-base.0.23.0.dylib`: `ceb4884265248e0260ab142821778d5af57fef07171ad73fd0f165bf6fe84b60`
- `libggml-cpu.0.23.0.dylib`: `e63a7b4f8eaa5bbd8e93e524d1a828de22a42bead57ae79cfab46e2339571fb2`
- `libggml-metal.0.23.0.dylib`: `89bb5b9ffcae6c19feda3338bc64a57b976f9f70a987e66ee49df7e58636058c`

## Remaining G4 work

This verifies local format, architecture, dependency shape, signatures, and environment only. It does not complete SBOM, license/redistribution, vulnerability/revocation, or full-library inventory review. It does not authorize inference or pilot enablement.
