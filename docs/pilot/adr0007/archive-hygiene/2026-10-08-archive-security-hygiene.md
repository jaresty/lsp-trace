# ADR0007 archive and security hygiene ledger

- **Date:** 2026-10-08
- **Scope:** additive documentation-only hygiene for retained ADR0007 Describe, Search, Group, Location, and Source Text Search campaigns/designs present at worktree head `dd24dd66` and the retained archive/recovery references named below.
- **Status:** archive/security hygiene record
- **Authority:** 0
- **Accepted:** false
- **Completeness:** `UNKNOWN`
- **Feature identity:** `UNRESOLVED`

## Preservation rules

This record is additive. It does not rewrite, delete, redact, move, normalize, or sanitize any frozen artifact, archive branch, recovery branch, generated evidence, binary, log, private object, schema, public surface, ADR0011 material, Program C material, or code. Frozen evidence and predecessor history remain authoritative only within their recorded custody boundaries. Any future quarantine, normalization, or migration must create a separate additive lineage record that binds original and successor artifacts by digest and must not copy credential bytes into documentation.

## Normative basis

The classification below is derived from existing repository decisions, primarily:

- `docs/pilot/adr0007/decisions/2026-10-08-normative-consolidation.md`
- `docs/pilot/adr0007/decisions/2026-10-07-archive-security-classification.md`
- `docs/pilot/adr0007/decisions/2026-10-07-search-group-successor-status.md`
- `docs/pilot/adr0007/decisions/2026-10-07-candidate-group-reconciliation.md`
- `docs/pilot/adr0007/decisions/2026-10-07-group-representative-scope.md`

This record does not independently qualify any campaign. It restates retained classifications for archive hygiene and security review.

## Classification ledger

| Chain area | Retained artifact, campaign, design, or branch | Classification | Basis and hygiene action |
| --- | --- | --- | --- |
| Describe | `describe-custody-successor-2026-10-07`; prepare generation `g-730b55e6a0c4d080d6c371425e7fabcdda3f30f3060fbf20a6cfbf4e901c3743`; review policy2 freeze | qualified | Current bounded Describe successor for the exact 24-case / 96-attempt campaign. Preserve as current qualified evidence; do not infer feature identity, completeness, production authority, Search qualification, or Group qualification. |
| Describe | generation `g-9a5fd445d96fbea3b1c37e9a33fb609b09dda81f765e216bd9ec0108dcb4ac0f` and review freeze `sha256:4f43ed1a381cc07dab11141a7dd50f18d081c18d8225985aa423e431ae78950a` | superseded | Superseded by the Describe successor due to parameter-count semantic change and policy2 review change. Preserve as historical evidence; do not rewrite earlier limitations. |
| Search | Search v11; freeze `sha256:b1207bdd52463dcc81fd0c47eccae0452630cf0e96d2c4d72991d99a4c4fe301`; execution result `SEARCH_CUSTODY_GO` | qualified | Current bounded deterministic Search custody for the exact frozen campaign. Preserve; no inference, production, or public-surface authorization. |
| Search | Search v1 through v10 in `docs/pilot/adr0007/experiment/search-custody-prospective-v11/PREDECESSORS.json` | blocked | Freeze-bound blocked predecessors. Preserve unchanged inside the predecessor manifest; do not reclassify because v11 qualified. |
| Group | Group v1; freeze `group-freeze-f66b64da93d0ec4797d821fb788ec7afabeaca96c68375d9689bc88ad0ce6771`; decisions recording `GROUP_CUSTODY_GO` and `INTEGRATION_GO` | qualified | Current bounded deterministic custody over mechanical candidate groups. Preserve; candidates are not features and do not establish semantic/navigation/representative usefulness. |
| Group | Blocked predecessor set in `docs/pilot/adr0007/experiment/group-custody-prospective-v1/PREDECESSORS.json`: `Group execution`, `public CLI/MCP`, `ADR0011`, and `candidate-group` | blocked | Freeze-bound blocked predecessors. Preserve unchanged; no public CLI/MCP, ADR0011, or candidate-group admission follows from Group v1. |
| Group | candidate-group revision `448a1f4f` and branch `recovery/candidate-group` | blocked | Existing decision says candidate-group remains wholesale blocked pending explicit migration. Preserve; do not partially admit selectors, receipts, `Get`, or `Put`. |
| Group | Prior `NOT_USEFUL` results and rejected corpus | rejected | Retained failures remain historical and are not erased by mechanical custody. Preserve as rejected/not-useful evidence; representative usefulness remains unresolved. |
| Location | Location Intersection v5 successor custody; design root `sha256:1195a420cc2ae215ff1627dbf23b606caaa243aa9fc0ae234242acceddafb48d`; final seal `sha256:f4981045d3489f5ef0633eb4ce6b4a17ab6de1ddc73c4b729f9dd524106b1fd6`; branch `wip/adr0007-location-execution-successor-20261007` | qualified | Current bounded location-relation custody for exact v5 cases and boundaries. Preserve; no broad location truth or production location service authorization. |
| Location | Location v1, v2, v3, and v4 | superseded | Immutable predecessors to v5. Preserve historical design-audit chain; v5 does not rewrite earlier outcomes. |
| Location | Initial `location-intersection-v5-qualification-2026-10-07` artifacts, including `FINAL_SEAL_BLOCKED.json` and `FINAL_AUDIT_BLOCKED.json` | blocked | Blocked historical execution path. Preserve pre-dispatch / zero-effect repair history. |
| Location | `wip/adr0007-location-design-20261007`, `wip/adr0007-location-execution-20261007`, and `wip/adr0007-location-oracle-20261007` branch tips | superseded | Retained design/execution/oracle inputs are predecessor/support branches for the successor custody path. Preserve branch tips; do not move or squash. |
| Source Text Search | Source Text Search v4 successor2 plus correction-generation custody; design successor2 commit `34ed9915`; correction chain through main merge `90ac78d7` and second parent `5e5e32fc` | qualified | Current custody only at later independent terminal `SOURCE_TEXT_SEARCH_QUALIFICATION_CUSTODY_GO`; exact literal mechanical search only. Preserve; no fuzzy, regex, semantic, feature-level, production, or public authorization. |
| Source Text Search | `source-text-search-v4-qualification-34ed9915` | blocked | Blocked predecessor campaign named by correction-generation bindings. Preserve as motivation and predecessor, not qualification oracle. |
| Source Text Search | `source-text-search-v4-zero-effect-successor-02ca9324` and commit `02ca9324` | blocked | Blocked / production-reference predecessor. Preserve by digest-reference; do not use as qualification oracle. |
| Source Text Search | `source-text-search-v4-correction-generation-91f756c7` at head `91f756c72317f09ba8ffac7c0b8933d46494bf0c` | superseded | Historical correction-generation authorization/predecessor in the custody chain. Preserve stakeholder repair provenance; final custody is later correction chain. |
| Source Text Search | correction execution/seal `cef7e0b4` | superseded | Earlier correction evidence, not final custody. Preserve as chain evidence only. |
| Source Text Search | independent-review correction `80b7107d` namespace `source-text-search-v4-independent-review-correction-generation-8f6b2c4d` | rejected | Existing consolidation records disposition `REVISE`. Preserve rejected review/correction evidence. |
| Source Text Search | final-binding correction `5e5e32fc` namespace `source-text-search-v4-final-binding-correction-generation-80b7107d` | blocked | Existing consolidation records artifact pending external review and `go:false`; later independent terminal custody is separate. Preserve as non-final binding evidence. |
| Source Text Search | `wip/adr0007-source-text-search-design-20261008` and `wip/adr0007-source-text-search-execution-20261008` | qualified / superseding chain branch tips | Retained branch tips for current Source Text Search design/execution evidence. Preserve branch tips; do not rewrite or sanitize in place. |
| ADR0007 capture/recovery | `wip/adr0007-capture-lane-20261004`, `.lsp-trace-publication.pre-adr0007-qualification`, `.lsp-trace-publication.pre-reap-qualification-20260921T050028Z`, and recovery branches `recovery/capture-seed-export`, `recovery/generic-acquisition`, `recovery/grouped-slice-candidate`, `recovery/mcp-transport`, `recovery/trace-facade-repair`, `recovery/typed-seeds-v2` | abandoned | Retained archive/recovery material has no current qualification in the normative consolidation. Treat as abandoned/recovery evidence for hygiene purposes; preserve losslessly unless a later additive lineage record explicitly supersedes it. |

## Security scan summary

Detailed redacted scanner output is in `docs/pilot/adr0007/archive-hygiene/secret-scan-summary.json`.

- Reputable local secret scanners checked in `PATH`: `gitleaks`, `trufflehog`, `detect-secrets`, and `ggshield`; none were available.
- Fallback scanner: bounded local Python regular-expression scanner, no network access, no secret bytes printed or retained.
- Current scanned scope completed: `docs/pilot/adr0007` at worktree head `dd24dd66`.
- Selected ADR0007/recovery branch-tip scanning was attempted twice but timed out before producing a complete result; this is a limitation, not a clean bill of health.
- Current scanned files: 4,144.
- Skipped large/restricted current files: 3.
- Credential-pattern findings: 1 potential `generic_assignment_secret` match, recorded only as a redacted fingerprint and location in `docs/pilot/adr0007/runtime/osv-yzma.json` line 2190. Sanitized context classifies it as vulnerability advisory prose about OpenTelemetry diagnostic logging, not a committed credential value.
- Confirmed restricted archival material categories in current scope: large raw request object and retained binaries. These are restricted archival material, not credentials by this scan.

## Exclusions and limitations

- This record does not scan network resources, external storage, uncommitted files outside the current worktree, every historical commit, or every branch blob.
- Branch-tip scanning over selected ADR0007/recovery refs timed out before completion; no credential bytes were printed or retained during the attempts.
- Pattern scanning can miss secrets and can over-match benign advisory prose. Findings require human security review before remediation.
- Large files and binaries were classified as restricted archival material when not scanned byte-for-byte in the fallback pass.
- No `.gitignore` change is made; no independent need was found for ignore-rule mutation.

## Remediation recommendations

1. Install and run an approved local scanner such as `gitleaks` or `trufflehog` offline against the selected ADR0007 and recovery branch tips, with output configured to redact secrets.
2. If a credential is confirmed, revoke/rotate it outside this repository first, then add an additive remediation lineage record. Do not sanitize frozen artifacts in place.
3. Keep logs, binaries, private source objects, path/UUID-bearing records, and large raw requests restricted under existing custody unless a separate approved publication review reclassifies them.
4. Preserve blocked, rejected, superseded, and abandoned predecessors exactly as historical evidence; never reclassify predecessor bytes merely because a successor qualified.
5. Keep ADR0011, Program C, public schemas, public CLI/MCP/API, code, and qualification behavior unchanged for this hygiene pass.

## Verdict

`ADR0007_ARCHIVE_HYGIENE_READY` for additive documentation hygiene at the stated bounded scope, with the explicit limitation that retained branch-tip secret scanning remains incomplete until a reputable local scanner or a separately bounded branch scan completes without timeout.
