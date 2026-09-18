# G4 supply-chain controls draft

- **Status:** `PREREQUISITES_DRAFT`
- **Pilot:** `PILOT_DISABLED`
- **Scope:** requirements for a future approved runtime/model tuple; no candidate is selected.

## Required tuple

Before any admission, indexing, or inference, an approved bundle must identify and digest-pin:

- worker executable and build inputs;
- protocol and schema artifacts;
- semantic backend and exact backend revision;
- native libraries and ABI-compatible versions;
- model artifact, model source, revision, quantization, tokenizer, and chat template;
- operating system, architecture, accelerator, driver, thread, context, and resource configuration;
- prompt, grammar, sampling, and seed configuration;
- SBOM, licenses, redistribution terms, vulnerability review, and revocation status.

Yzma remains only a replaceable candidate. This draft does not select Yzma, `llama.cpp`, a model, quantization, or an artifact source.

## Acquisition controls

An operator must stage artifacts from approved sources before execution and verify exact digests before admission. The worker runs with automatic download disabled and network access denied. No request, retry, startup path, or build may download, update, or silently substitute an artifact.

Staged artifacts require quarantine before use. Replacement requires a new digest, review, SBOM/license/vulnerability evaluation, and a new bundle identity. Mutable tags, floating URLs, model-family matches, and ABI assumptions are insufficient.

## Revocation

Revocation of a backend, native library, model, license, vulnerability status, or dependency invalidates every dependent generated product. The system must rebuild under a replacement bundle or close the affected products with an explicit unavailable/rejected disposition. Prior records remain immutable and visibly superseded.

## Required evidence

A future G4 bundle must retain:

- source URLs or repository identities and retrieval timestamps;
- archive, extracted-file, and executable hashes;
- native binary format, ABI, `otool`, signing, quarantine, and code-signing evidence where applicable;
- SBOM and dependency identities;
- license and redistribution decisions;
- vulnerability findings and accountable update/revocation owners;
- model-card and artifact provenance;
- environment and runtime identity;
- approval, exception, supersession, and revocation references.

## G4 rejection conditions

Keep the pilot disabled for missing or mutable pins, enabled network/download paths, unresolved license or vulnerability status, incomplete SBOM, incompatible ABI/model, quarantine or signature failure, revoked dependencies, unreviewed replacement artifacts, or any claim that successful staging establishes semantic authority.
