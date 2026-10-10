# Independent review — ADR 0011 outgoing Go/gopls acquisition v1 stop

- Reviewed ref: `pi-agent-d3f5f679-track-b-outgoing-stop`
- Reviewed commit: `61d0e168826518fdc023aa0e27f1b14a4c3f20f4`
- Accepted parent: `bafd546b98a284df5916b24a5549fc508d93b82b`
- Disposition: `STOP_EVIDENCE_ACCEPTED`
- Review scope: bounded stop packet only
- Evidence GO: not issued
- Profile GO: not issued
- Authority: `0`
- Completeness: `UNKNOWN`

## Packet and ancestry verification

The reviewed ref resolves exactly to the reviewed commit, and the reviewed commit has the accepted parent as its sole parent. The parent-to-reviewed diff adds exactly three files under this packet directory:

- `SHA256SUMS`
- `STOP.md`
- `packet.json`

No source, test, qualification-plan, vector-inventory, prior-review, or manifest file changed in that diff.

The two packet payload hashes recomputed from the reviewed commit match `SHA256SUMS` and the packet metadata:

| artifact | bytes | sha256 |
| --- | ---: | --- |
| `STOP.md` | 6691 | `8525c196b4fd330d8576d57c677f688738fa91bb634639e92f991b255e6c6b89` |
| `packet.json` | 2077 | `27ee45b7eaab7d97b0e7b02cac87ec84a5db22b6c003b9454fb5f22361a82c60` |

The five source qualification artifacts named by `STOP.md` retain identical Git blob identities between the accepted parent and reviewed commit. Their recomputed byte lengths and SHA-256 values match the stop packet, including the unchanged evidence manifest at 12230 bytes with SHA-256 `b1df161c6352ca111072037a0b133ec7539806849392ed0d7a8dc1209be59e0f`.

## Stop-boundary verification

The packet consistently records zero semantic requests, zero initialize requests, zero prepareCallHierarchy requests, and zero outgoingCalls requests. Its three-file documentation-only diff contains no executable path capable of starting a managed process or issuing such a request. This accepts the producer's bounded stop record; it does not claim independent runtime observation of events outside the retained commit.

The exact seam claim is supported by the pinned source:

1. `sessionruntime.RoundTripResult` internally retains request/write custody, method request-frame bytes, response/read custody, and selected method-specific response-frame or owned-pair state.
2. `CompletedMethodRequestFrame` can expose an eligible completed request frame, but `CompletedMethodResponseFrame` permits only definition and references.
3. `eligibleOwnedMethodRequest` permits definition, references, and documentSymbol, not `textDocument/prepareCallHierarchy` or `callHierarchy/outgoingCalls`.
4. `internal/acquisition/sessionclient.New` consumes managed `RoundTrip` but returns only `result.Result` through the acquisition wire adapter.
5. The acquisition coordinator records params and responses by encoding Go values through `boundedJSON`. Those decoded/re-encoded records are useful semantic records but cannot establish identity with the original framed wire transaction.

Therefore manager-internal frame custody exists, but paired prepare/outgoing response-frame exposure required by this exact profile is unavailable at the acquisition boundary. Stopping before semantic effects is justified.

## Test-only seam assessment

The proposed seam is minimal in placement and isolated in authority if a later, separately authorized implementation keeps it as a private manager-owned projection that:

- opts in only for `textDocument/prepareCallHierarchy` and `callHierarchy/outgoingCalls`;
- binds the exact request key, session, generation, owner sequence, completed write/read observations, request frame, matching response frame, and raw result token under fixed limits;
- preserves top-level `null` distinctly from `[]`;
- rejects unsupported methods, mismatched or stale identity, incomplete custody, failure where successful custody is required, reordering/duplication, and exhausted accounting; and
- has no CLI/MCP exposure and does not alter ordinary production acquisition behavior or any authority ceiling.

Reusing or narrowly extending the existing private owned-pair machinery is smaller and safer than adding a new general response-frame accessor or broadening the public acquisition client. The proposal should remain a test-only exact-method specialization rather than a method-neutral production facility. This assessment authorizes no implementation and does not select a concrete API.

## Disposition boundary

`STOP_EVIDENCE_ACCEPTED` means only that the three-file stop packet is internally consistent, hash-correct, ancestry-correct, manifest-preserving, and source-supported at the stated seam. It does not qualify evidence, activate the profile, establish source or runtime completeness, or issue any evidence/profile GO.
