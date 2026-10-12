# Independent review — ADR 0011 incoming Go/gopls acquisition tranche 1

Disposition: `PARTIAL_EVIDENCE_ACCEPTED`

Profile reviewed: `CALLS_GO_GOPLS_INCOMING_EXACT_V1`

Repository reviewed: `/Users/schwa/dev/lsp-trace-adr0011-c17`

Reviewed HEAD: `082e3f6dfb5aac82fdeef804153f73d07de408e2`

Authority remains `0`. Acceptance remains `false`. Production admission remains `REFUSED_MISSING_EVIDENCE`. The acquisition gate and exact-profile evidence gate remain pending. This review issues neither acquisition evidence GO nor profile GO.

## Accepted PRESENT roles

Exactly these four roles may honestly be `PRESENT` in this incomplete tranche:

1. `repository_commit_and_dirty_state_custody`
2. `workspace_root_identity`
3. `target_file_original_bytes_digest_uri_and_language`
4. `managed_session_and_generation_custody`

The acceptance is bounded to the frozen, authority-zero custody record and its exact profile binding. It is not producer authentication, completeness, semantic qualification, or production admission.

## Independent checks

- `git rev-parse HEAD` matched `082e3f6dfb5aac82fdeef804153f73d07de408e2`.
- Before this review file was created, exhaustive porcelain status contained only the modified incoming evidence manifest and the eight untracked tranche files. No product/source file was dirty.
- The custody record reports `git status --porcelain=v1` with zero entries at acquisition. This correctly describes the pre-evidence capture boundary: the target came from the clean reviewed HEAD; the evidence/manifest writes are the subsequent uncommitted tranche.
- Workspace path and URI consistently bind `/Users/schwa/dev/lsp-trace-adr0011-c17` and `file:///Users/schwa/dev/lsp-trace-adr0011-c17`.
- Target custody independently recomputed from both the worktree file and `HEAD:incomingops/executor.go`: SHA-256 `040622acaf54ea77fe79f3333b10857cfce0bef09bce81f448bd4ee0a619b884`, byte length `36296`, and Git blob OID `15facd039ba84152b0c2b77577e3bec81d0fbc8f`. URI and `language_id=go` are consistent. The zero-based target position line 749, character 24 falls on `IncomingCalls` in those exact bytes.
- Original target bytes are recoverable from the exact commit/path/blob binding; they are not falsely described as an embedded LSP document body.
- Frozen session custody identifies session `sk1:33544354924d71648883b07b3cf158f8ce7564dc8ebcc66b87db42b9d3c961ef`, generation `1`, state `READY`, exact workspace root, `language_id=go`, server profile `gopls`, UTF-16 position encoding, and status request `offline-6`. The acquisition record binds the same session/generation, target URI, profile, and commit. A later independent live registry query returned `APPLICABLE_SESSION_NOT_FOUND`; therefore this review accepts the frozen custody record only and does not claim the session remains live.
- Every non-self entry in `hash-manifest.json` independently matched its SHA-256 and byte length. `custody.json` matched SHA-256 `718d3ba594819891126045823e196adb31ae3f125a11039deb66ee11a87e52cd` and byte length `1892`, exactly as referenced by all four PRESENT roles.
- The prepare and incoming bounded records accurately declare original JSON-RPC frames, original parameter/result bytes, request identity, raw order/cardinality, and null-versus-array form unavailable. Their dependent roles remain `MISSING`.
- No `textDocument/documentSymbol` request occurred. Direct-position selection is disclosed, and full prepared-item identity/ordinal custody remains `MISSING`.
- semantic-vector was not executed or used; its policy remains optional, non-authoritative, and unable to satisfy required roles or issue review/GO.
- Managed gopls binary/process identity and managed Go toolchain custody remain `MISSING`; shell inventories are correctly labeled non-evidence and unbound to the managed profile.
- Initialize/capability custody, exact prepare/incoming wire custody, selection identity, ordering/accounting, normalization, occurrence/multiplicity, refusal vectors, admission identity, ceilings, compatibility, replay, predecessor closure, and all other non-custody roles remain honestly `MISSING` or `PLANNED` as declared.

## Gate conclusion

`PARTIAL_EVIDENCE_ACCEPTED`: the four custody roles listed above are accepted as PRESENT. Required acquisition roles remain absent, so the acquisition gate is pending and admission remains refused. No acquisition evidence GO. No profile GO.
