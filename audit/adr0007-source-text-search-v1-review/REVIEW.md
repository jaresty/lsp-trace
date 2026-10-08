# ADR0007 source text search v1 independent review evidence

Verdict: `SOURCE_TEXT_SEARCH_DESIGN_REVISE`

Scope: additive review evidence for existing v1 files only. This directory is audit evidence and does not alter or qualify v1. V1 files, commits, and freezes are to remain byte-identical.

## Findings

1. Schemas are not a complete closed Draft 2020-12 family for every ADR-required record; discriminating const identifiers, enums, ranges, patterns, and `additionalProperties: false` coverage are incomplete.
2. Canonical JSON records and examples are absent or insufficient for every required request, result, policy, limits, accounting, custody, replay, source binding, Location composition, and freeze record.
3. Corpus coverage is too small and largely name-based; it does not provide at least 40 executable input-only case directories with independent expected assertions.
4. Source-admission reuse is not pinned to the exact v2 admission source by commit, path, size, sha256, git blob sha1, and exported symbols with byte verification.
5. Location composition pins are abbreviated and incomplete; execution/final seal roots and `executedLocation=false` candidate-only constraints need exact records and tests.
6. Cancellation and deadline fields, polling points, precedence, and tests are absent.
7. Multi-file admitted envelope semantics, duplicate/path/revision/digest checks, and deterministic UTF-8 path-byte then byte-offset ordering are underspecified.
8. Accounting is not uint64 checked/precharged with the required formula `W=50+3J+5Q+7P+S+11T+13M+17R+19U+31B`, inclusive limits, `+1` failure tests, and fixed-point output-byte measurement.
9. UTF-8 and position behavior is incomplete: strict valid UTF-8, no clamping, exact byte plus LSP half-open line/UTF-16 character positions, CRLF/LF/bareCR/non-BMP tests are required.
10. Freeze is not a stable self-measuring convention with a qualified Location `SPEC_MANIFEST` rule, zeroed self-entry hash/bytes, exact census/root comparison, tamper failure, and twice-empty-root equality.
11. Destructive mutation tests do not cover every schema, pin, custody, replay, accounting, precedence, overflow, cancel, deadline, and freeze tamper boundary.
12. Public-surface exclusion is not explicitly enforced by package/command names and protected-path tests.

## Required correction direction

Create a separate prospective v2 root with private v2 package and command names. Preserve v1 byte-identically. The v2 candidate must remain design-only: authority `0`, accepted `false`, completeness `UNKNOWN`, feature identity `UNRESOLVED`, no public surface, no execution of semantic qualification, and `DESIGN_GO` authorizing only one later private campaign.
