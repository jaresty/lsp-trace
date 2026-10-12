# ADR 0011 outgoing Go/gopls acquisition v1 — observability STOP

- Profile: `CALLS_GO_GOPLS_OUTGOING_EXACT_V1`
- Track: B only
- Disposition: `STOP_BEFORE_SEMANTIC_EFFECTS`
- Authority: `0`
- Accepted: `false`
- Completeness: `UNKNOWN`
- Production admission: `REFUSED_MISSING_EVIDENCE`
- Evidence GO: not claimed
- Profile GO: not defined or claimed

## Immutable starting point

The isolated worktree was reset to the exact accepted branch/ref:

- ref: `pi-agent-76161000-8f12-49d`
- commit: `bafd546b98a284df5916b24a5549fc508d93b82b`
- tree: `77008a16f18c86bab79e3db1de7bdad83bd97b5e`
- accepted plan-review token: `ADR0011_CALLS_OUTGOING_PLAN_GO`
- review scope: four-file qualification-plan review only

Verified source artifacts at that commit:

| artifact | bytes | sha256 |
| --- | ---: | --- |
| `docs/qualification/adr0011-calls-go-gopls-outgoing-profile-plan.md` | 10567 | `ed8ec9d30f7a8b668e47401b2e979d122cae0422e85a8ea69b00a73c3f7d9878` |
| `docs/qualification/adr0011-calls-go-gopls-outgoing-causal-tests.md` | 11111 | `2e1463b745fed699ff8ba7f9d96eaa64a0a934d6abf5f950adc90f8b0005d7d1` |
| `docs/qualification/adr0011-calls-go-gopls-outgoing-evidence-manifest.json` | 12230 | `b1df161c6352ca111072037a0b133ec7539806849392ed0d7a8dc1209be59e0f` |
| `docs/qualification/adr0011-calls-go-gopls-outgoing-vector-inventory.md` | 9380 | `07cd7304503a7accfd0e672f5300182059be86e197a44c2f3faf63f2f8bfedc9` |
| `docs/qualification/adr0011-calls-go-gopls-outgoing-review-v2.md` | 4115 | `ea2db18793f5ef9e9cbc7aff48476cac012036f34658bdc92222f2040278a376` |

## Prospective precondition evaluated

Before any `textDocument/prepareCallHierarchy` or `callHierarchy/outgoingCalls` request, the accepted plan requires exact managed transaction custody: separate exact request/result custody, request/session/generation identity, ordering, limits, framing/accounting, terminal outcome, and preservation of original result identity including successful top-level JSON `null`.

That precondition is not available through the existing managed acquisition surface.

## Exact observability seam

Source-visible observations at the pinned commit:

1. `sessionruntime.RoundTripResult` internally carries `requestWrite`, `methodRequestFrame`, `responseRead`, and method-specific response-frame fields (`sessionruntime/sessionruntime.go:648-680`).
2. `CompletedMethodRequestFrame` can return an exact completed request frame for eligible ordinary results (`sessionruntime/method_request_frame.go:3-12`), but this alone does not supply the required paired result custody.
3. `CompletedMethodResponseFrame` permits only `textDocument/definition` and `textDocument/references`; every other method returns unavailable (`sessionruntime/method_response_frame.go:13-26`). It therefore cannot expose prepareCallHierarchy or outgoingCalls response frames.
4. Existing `CompletedOwnedMethodPair` eligibility is limited to definition, references, and documentSymbol (`sessionruntime/owned_method_pair.go:147-153`); prepareCallHierarchy and outgoingCalls are excluded.
5. `internal/acquisition/sessionclient.New` calls managed `RoundTrip` and returns only `result.Result` after translating failures (`internal/acquisition/sessionclient/client.go:29-38`). It does not transfer the manager-owned framed request/response observations.
6. `internal/acquisition.WireClient` receives only unframed result JSON and decodes it (`internal/acquisition/wire.go:34-64`).
7. The coordinator retains `RequestRecord.Params` and `RequestRecord.Response` by calling `boundedJSON` on Go values (`internal/acquisition/coordinator.go:240-246,248-325`). This is deterministic re-encoding, not original-wire identity. The accepted plan explicitly forbids re-encoding from claiming original-wire identity.

Consequently, running the current acquisition would create semantic effects without the exact paired transaction custody required by this profile. No managed gopls process was started, no initialize request was sent, no document was supplied, and no prepareCallHierarchy or outgoingCalls request was issued in this tranche.

## Unproduced evidence

Because execution stopped before semantic effects, this tranche does not claim or reconstruct:

- managed gopls binary bytes/version/digest/argv/workdir/environment;
- Go toolchain or `go env` inventory;
- initialize request/result or capability exchange;
- session/generation/workspace/source/URI/encoding custody;
- prepare request/result;
- prepared-caller selection;
- outgoing request/result, successful-null classification, normalization, ranges, multiplicity, ordinals, duplicates, parallel edges, or self-loops;
- live limits/privacy/accounting/replay/refusal results.

Historical payloads and decoded/re-encoded records were not promoted as prospective evidence.

## Smallest test-only proposal (not implemented)

Add a private, test-only, bounded manager-owned transaction-pair projection for exactly:

- `textDocument/prepareCallHierarchy`; and
- `callHierarchy/outgoingCalls`.

The projection should require explicit opt-in and exact session/generation, retain immutable copies of the exact completed framed request and matching framed response plus raw params/result slices, request key and owner sequence, completed write/read observations, declared message/byte/time limits, terminal classification, and a fixed-storage byte account. It must reject missing/mismatched keys, stale or foreign generation, incomplete write/read, server/transport failure where success custody is requested, over-limit bytes/messages, duplicate or reordered ownership, and unsupported methods. It must preserve a successful top-level raw result token `null` distinctly from `[]`.

The test-only proposal must not add a CLI/MCP surface, alter production acquisition behavior, activate the profile, share incoming authority, add semantic-vector execution, or weaken existing refusal/ceiling rules. Counterfactual tests should prove refusal when either frame, key, generation, method, raw result identity, or byte account is changed. A later separately authorized implementation phase would be required.

## Manifest disposition

The outgoing evidence manifest is intentionally unchanged. No role has an immutable new evidence locator plus byte length, SHA-256, provenance, and exact-profile binding sufficient for an honest transition to `PRESENT`.

## Review boundary

This producer does not self-review and does not claim evidence GO. A separate later reviewer must inspect this packet and its immutable digest, verify that no semantic request occurred and no forbidden file changed, and issue only a bounded stop-packet disposition. Such review cannot qualify or activate the profile.
