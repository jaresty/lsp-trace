# Threat model and conformance tests

Status: `PREREQUISITES_DRAFT`. Controls listed here are requirements to be frozen and tested, not claims about an existing worker.

| Threat | Required control | Required conformance evidence |
|---|---|---|
| Authority laundering or hallucinated acceptance | Source/projection split; generated `authority=0`, `accepted=false` | reject forged/upgraded authority and preserve source metadata |
| Scope/completeness inflation | typed mode, scope, denominator and coverage on every result | TARGET/NEIGHBORHOOD cannot claim CENSUS or global absence |
| Cache collision/stale replay | exact canonical preimage and dependency invalidation | perturb every preimage component; every perturbation misses |
| Lineage deletion or stale resurrection | append-only corrections, tombstones, dependent rebuild receipts | deleted admission cannot return from cache, rebuild or backup restore |
| Sensitive-data disclosure | class-specific access, encryption, retention and audited deletion | denied-role, log/temp/cache, backup and expiry tests |
| Supply-chain substitution | approved sources, exact digests, signatures where available, SBOM/license/vulnerability decisions | tamper, missing pin, quarantine and revoked-artifact rejection |
| Runtime download or exfiltration | network-denied sandbox and automatic download disabled | deny DNS/connect/listen and verify no retry/build download |
| Process-tree escape/resource exhaustion | contain worker plus descendants; CPU, memory, wall time, file/process counts, output and disk limits; kill tree on breach | fork/child escape, OOM, timeout, output flood and crash tests |
| Filesystem/tool/repository traversal | pass only admitted bytes; deny arbitrary paths, tools and external retrieval | path traversal, symlink, environment-secret and tool invocation tests |
| Protocol confusion/injection | canonical NDJSON, size/depth limits, strict versions/types, bounded outputs | malformed JSON, duplicate keys, oversized/deep, unknown version/type, prompt-injection fixtures |
| Denominator suppression | closed enums and exact once-only member closure | duplicate, omitted, extra and nonterminal members fail |
| Evaluation leakage or threshold gaming | immutable splits, independent custodian, numeric freeze before test unlock | attempted test-label access and post-unlock threshold change reject |
| Backend coupling | backend-neutral records and distinct backend/runtime identity | replace backend without changing protocol authority; cache remains separate |
| Feature-identity overreach | feature inventory remains downstream and independently adjudicated | groups remain provisional; no generated feature acceptance |

## Supply-chain/runtime pin set

Before execution a manifest must bind worker executable, backend and native libraries, model/tokenizer/chat template/quantization/model card, OS/architecture/accelerator/driver, build tools, environment policy, protocol/schema/policies, SBOM, licenses, vulnerability review, signatures, resource limits and network-denial configuration. Revocation invalidates all dependent generated products and triggers rebuild or rejection.

No-download testing must begin from an empty external cache with network denied and must prove that startup, request handling, retries, failures and rebuilds neither fetch nor update artifacts. Process separation limits blast radius; it does not confer authority, independence, safety, or correctness.

No secure-erasure claim is permitted without independent storage-layer evidence; deletion conformance reports observable disposition and external retention honestly.
