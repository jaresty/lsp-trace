# Bounded declared source selection

Sole writer; exact dirty checkout. Filesystem-only evidence: available managed session belongs to another checkout. No server/session started.

## Decisions
- Add optional `selection_policy` v1 to existing scope config; absent policy retains historical Go interpretation/serialization. New CLI candidate freezes the complete effective association list, sorted unique declared IDs and independent first-ID default.
- Project `.lsp-trace.toml` `[file_associations]` maps glob to language ID. Exact pattern overrides builtin before matching. Builtins retain existing known suffixes; `.mts`/`.cts` require explicit declarations. Case-sensitive basename patterns (no slash) or workspace-relative path patterns (slash), `path.Match` syntax; no recursive `**` association DSL. Includes/excludes retain existing semantics separately.
- Up to 64 effective patterns, 32 language IDs, 128-byte patterns/IDs. Canonical IDs are nonempty ASCII lowercase letters/digits plus `_+.-`; custom IDs accepted. Duplicate IDs rejected; sorted identity does not change first default.
- Conflicting overlapping matches are discovered during paid enumeration and close PARTIAL with explicit association-conflict reason, never false READY. Unknown remains NOT_SOURCE; known undeclared language remains UNSUPPORTED_LANGUAGE. New records carry resolved language.
- Automatic and unconfigured explicit gopls retain known Go declaration. An explicitly configured profile without IDs fails candidate admission. Candidate reads profile declarations without resolving secrets or starting providers. User config may supply named profile; only project config supplies associations.
- Existing config digest includes policy. Stored replay uses recorded policy, not new builtins/project config. Existing creation/enum owner and ceilings unchanged. Matching coefficient includes bounded pattern bytes/path bytes and declared-ID membership; no full config validation per record.
- Candidate acquisition remains unavailable. Verify ordinary supplier explicit language and runtime didOpen independently using existing in-memory seams; do not invent a bridge.

## Steps
1. Record scoped preimages and accepted dependency hashes; add executable RED candidate fixture.
2. Implement policy and profile preservation, wire new candidate only, retain historical semantics.
3. Add policy/replay/freeze/conflict/funding and supplier/wire tests; run exact offline normal/race tests plus focused build/vet/format/diff checks.
4. Write unique external report with actual commands, hashes, write set and limits; parent independent review required.
