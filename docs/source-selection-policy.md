# Bounded candidate: declared source selection

The opt-in `census WORKSPACE --scope-job-candidate` route freezes metadata selection in the **existing inventory job**. This does not qualify a provider, perform native acquisition, or publish a semantic inventory. Ordinary census routing and `PrepareDocument` fallback are unchanged.

## Project configuration

```toml
[profiles.typescript]
command = "typescript-language-server"
args = ["--stdio"]
language_ids = ["typescript", "typescriptreact"]

[file_associations]
"*.mts" = "typescript"
"*.cts" = "typescript"
"special/*.go" = "typescript"
```

Run `lsp-trace census /absolute/workspace --scope-job-candidate --profile typescript` with the existing publication-root controls as needed. The profile name declares eligibility; no executable lookup, environment secret resolution, server initialization or capability inference is performed. Named profile fields retain user-then-project merge precedence. Only the project's `.lsp-trace.toml` supplies filename associations; user-config associations are not applied to candidate selection.

The first `language_ids` element remains the document default. The full list is copied and sorted for frozen identity, without changing the separate default. Empty/missing IDs in an explicitly configured candidate profile are an error: eligibility is unknown, not all files. Automatic Go selection and an unconfigured explicit `gopls` retain their known Go declaration. Automatic mode retains the paid Go-root check. Configured custom language IDs are allowed; they are client declarations, not server-confirmed support.

## Association semantics

Builtins: `*.go=go`, `*.ts=typescript`, `*.tsx=typescriptreact`, `*.js=javascript`, `*.jsx=javascriptreact`, `*.cs=csharp`, `*.ex=elixir`, `*.exs=elixir`, `*.py=python`, `*.rs=rust`. `.mts` and `.cts` are **not** new implicit defaults: configure them explicitly as above.

- Project entries replace the builtin with the **identical pattern key** before matching. For example `"*.go"="typescript"` intentionally replaces Go classification.
- Patterns are case-sensitive Go `path.Match` globs (`*`, `?`, character classes). Slashless patterns match a basename at any depth. Patterns containing `/` match the complete workspace-relative slash path; `*` never crosses `/`. No recursive `**`, absolute patterns, backslashes, empty/`.`/`..` components, trailing slash, or leading `!`.
- Multiple matching patterns that assign the same ID are allowed. Different matching IDs produce `FILE_ASSOCIATION_CONFLICT`, not a map-order winner. Thus the `special/*.go` example conflicts with builtin `*.go` when such a file is encountered unless `*.go` is also overridden to the same ID.
- A conflict discovered during traversal closes the paid enumeration as `EnumerationPartial`, with an unknown remainder. It does not report a complete denominator, resolve arbitrarily, or reach a provider.
- Unknown associations remain `NOT_SOURCE` / the historical `unknown_extension_not_source` counter: these mean **unclassified/noneligible**, not proof the file is not source. A known language absent from the profile is `UNSUPPORTED_LANGUAGE`, meaning undeclared for this profile, not a tested server refusal.
- Includes/excludes remain separate existing selection filters. They do not resolve language conflicts. Directory pruning occurs only through the existing explicit prune control; no `.gitignore`, generated/test-file heuristic or Git tracked/dirty/untracked filter is added. Project context files may still matter to a future server without becoming inventory candidates.

## Bounds, persistence and compatibility

At most 64 effective association patterns and 32 distinct language IDs; each string is at most 128 bytes. IDs use lowercase ASCII letters/digits and `_+.-`; duplicates, empty IDs, malformed patterns, unknown TOML fields and duplicate TOML keys are rejected. Candidate input config files and the existing encoded inventory config remain bounded at 32 KiB; ordinary profile loader size behavior is unchanged.

New candidate jobs store the entire normalized mapping, supported-ID set, independent default, profile identity, includes/excludes and explicit prunes in the existing digest-bound config. The selector is `lsp-trace.declared-source-selection.v1`. Policy-bearing jobs use enumeration resource policy v2 and inventory scope segment v2, with per-regular-file `language_id`. Readback recomputes disposition from the **recorded** policy and rejects mismatched recorded languages. Config changes after Create do not affect Enumerate or status. No resumable cursor is introduced.

Absent-policy inventory jobs retain resource/segment v1 and historical Go semantics. Historical standalone scope jobs remain read-only compatible and cannot accept the new selection policy. No migration is performed.

Matching is prepaid in the shared work quote: an additional `2 * (sum(patternBytes+1)*(pathByteLimit+1) + languageCount*128)` work units per step bounds live/replay matching and membership. Pattern validation/canonicalization is bounded configuration work, not repeated per record. Existing creation charges, 9 MiB fixed enumeration allowance, 24 MiB batch quote, 64 MiB/64-packet ceiling, deadlines, admission count and physical-storage limitations are unchanged. Large rule sets or scopes may exhaust the existing allowance; this is not a capacity increase or whole-repository fit claim.

## Explicit document language and deferred bridge

The existing ordinary supplier sends `SourceFile.LanguageID` as `DocumentRequest.LanguageID`; runtime didOpen uses request language before configured default before filename detection. Controlled tests verify TSX is `typescriptreact` despite a `typescript` default, and retain both old fallback branches. These are separate supplier/wire seam tests. The candidate's future paid record-reader/revalidation/acquisition bridge remains unavailable; no test or implementation pretends it already exists.
