# Threat model and conformance tests

Status: `PREREQUISITES_DRAFT`; pilot: `PILOT_DISABLED`. Controls listed here are requirements to be frozen and tested, not claims about an existing worker.

| Threat | Required control | Required conformance evidence |
|---|---|---|
| Authority laundering or hallucinated acceptance | Source/projection split; generated `authority=0`, `accepted=false` | reject forged/upgraded authority and preserve source metadata |
| Scope/completeness inflation | typed mode, scope, denominator and coverage on every result | TARGET/NEIGHBORHOOD cannot claim CENSUS or global absence |
| Cache collision/stale replay | exact canonical preimage and dependency invalidation | perturb every preimage component; every perturbation misses |
| Lineage deletion or stale resurrection | append-only corrections, tombstones, dependent rebuild/deletion/revocation receipts | deleted or revoked admission cannot return from cache, rebuild, retry, restart or backup restore |
| Sensitive-data disclosure | class-specific access, encryption, retention and audited deletion | denied-role, log/temp/cache, backup and expiry tests |
| Supply-chain substitution | controlled sources, complete tuple, distinct builder/archive/extracted hashes, signatures where available, SBOM/license/vulnerability decisions | tamper, mutable-retrieval-only, missing pin, quarantine and revoked-artifact rejection |
| Native ABI or model incompatibility | Mach-O/load-command/ABI-symbol closure and exact GGUF provenance, license, format, tokenizer/template/quantization compatibility | wrong architecture, undeclared dylib, missing symbol, incompatible or substituted model rejection |
| Runtime download or exfiltration | network-denied sandbox, empty external caches and automatic download disabled | deny DNS/connect/listen and verify no startup/request/retry/failure/rebuild download |
| Process-tree escape/resource exhaustion | contain worker plus descendants; CPU, memory, wall time, file/process counts, output and disk limits; kill tree on breach | fork/child escape, OOM, timeout, output flood and crash tests |
| Stop-control ambiguity | explicit accountable stop capability, closed accounting, bounded output, descendant termination and cleanup; representation and lifecycle remain deferred to G1 | race the later-frozen stop control with each capability, timeout, crash and termination; reject late or duplicate closure |
| Filesystem/tool/repository traversal | caller passes only admitted bytes and relationships; deny source reread, arbitrary paths, tools and external retrieval | path traversal, source reread, symlink, environment-secret and tool invocation tests |
| Backend fallback or coupling | backend-neutral request/response records; no in-process `LocalModel`; no backend-specific types or implicit fallback | substitute a non-Yzma test backend; reject Yzma fields and missing-artifact fallback |
| Protocol confusion/injection | later-frozen unambiguous framing, negotiation, bounded messages, versions/types and lifecycle rules | exercise malformed, duplicate, oversized/deep, unknown-version/type, out-of-order negotiation, invalid-lifecycle and post-terminal inputs against the later-frozen contract |
| Denominator suppression | closed enums and exact once-only member closure | duplicate, omitted, extra and nonterminal members fail |
| Evaluation leakage or threshold gaming | immutable splits, independent custodian, numeric freeze before test unlock | attempted test-label access and post-unlock threshold change reject |
| Feature-identity overreach | feature inventory remains downstream and independently adjudicated | groups remain provisional; no generated feature acceptance |

## Supply-chain/runtime pin set

Before any execution could be considered, a manifest must bind worker executable, backend and native libraries, model/tokenizer/chat template/quantization/model card, OS/architecture/accelerator/driver, build tools, environment policy, protocol/schema/policies, SBOM, licenses, vulnerability review, signatures, resource limits and network-denial configuration. Revocation invalidates all dependent generated products and triggers a closed rebuild, deletion, unavailable, externally-retained or rejection disposition. The practical evidence list is in the [Yzma candidate supply-chain dossier](supply-chain-candidate.md).

No-download testing must begin from an empty external cache with network denied and must prove that startup, request handling, retries, failures and rebuilds neither fetch nor update artifacts. Process separation limits blast radius; it does not confer authority, independence, safety, or correctness.

No secure-erasure claim is permitted without independent storage-layer evidence; deletion conformance reports observable disposition and external retention honestly.
