# ADR 0011 incoming Go/gopls prospective acquisition v2

Profile: `CALLS_GO_GOPLS_INCOMING_EXACT_V1`.

Disposition: `STOPPED_BEFORE_SEMANTIC_EFFECTS_OBSERVABILITY_SEAM`.

This is one new prospective attempt against the existing managed capability surface. Discovery found no exact-workspace session. The only READY parent session did not expose the mandatory pre-execution managed gopls executable bytes/version/digest/launch argv/cwd/environment, Go toolchain/environment, initialize request/result and negotiated call-hierarchy capability, or exact original JSON-RPC frames/payloads. Therefore no child session was derived and no prepareCallHierarchy, incomingCalls, documentSymbol, normalization, occurrence, admission, publication, implementation, test/vector/review, Track B, semantic-vector, GO, or commit action occurred.

`authority=0`, `accepted=false`, `completeness=UNKNOWN`, and `production_admission=REFUSED_MISSING_EVIDENCE` remain unchanged. The four pre-existing PRESENT manifest roles remain untouched.

Files:

- `managed-discovery.json`: exact managed session discovery results used for the stop decision.
- `capability-boundary.json`: exact relevant operation names, request IDs, advertised limits, and absent custody fields.
- `observability-seam-report.json`: precise missing preconditions, zero-effect accounting, and smallest test-only capture proposal.
- `SHA256SUMS`: immutable hashes and lengths for this directory, generated after all other files.
- `BOUNDARIES.json`: pre/post repository and manifest boundaries, generated last before final verification.
