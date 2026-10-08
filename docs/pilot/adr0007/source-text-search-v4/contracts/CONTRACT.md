# ADR0007 source text search private v4 contracts

This directory defines implementation-independent strict wire contracts for the private v4 source-text-search terminal envelope. It is not production code, not an oracle, not generated oracle output, and not a freeze/go decision.

## Canonical terminal envelope

Every terminal record is one JSON object with `schema_version = lsp-trace.adr0007.source-text-search.terminal.private.v4` and these top-level members only: `schema_version`, `terminal`, `request`, `attempt`, `control`, `sources`, `admission`, `matches`, `positions`, `range_union_candidate`, `policy`, `limits`, `accounting`, `failure`, `custody`, `replay`, `tooling`, `predecessor`, and `payload`.

The JSON Schema is Draft 2020-12 and uses `additionalProperties: false` recursively. Unsigned counters use the bounded uint64 maximum `18446744073709551615`.

## Constants and vocabulary

Private v4 uses schema private.v4 with authority and adjudication constants: `authority0`, `accepted=false`, `UNKNOWN`, and `UNRESOLVED`. Accounting names are exactly `J/Q/P/S/T/M/R/U/B/W` plus eight failure counters: `fail_parse`, `fail_admission`, `fail_resource`, `fail_cancelled`, `fail_deadline`, `fail_overflow`, `fail_internal`, and `fail_policy`.

Failure codes are `PARSE`, `ADMISSION`, `RESOURCE`, `CANCELLED`, `DEADLINE`, `OVERFLOW`, `INTERNAL`, and `POLICY`; each code has a typed code-specific detail object enforced by the standalone validator.

## Terminal rules

Malformed raw input attempt identity is `attempt-raw-sha256-` plus the full lowercase SHA256 of the exact raw bytes.

Every `FAILED` terminal has `matches: []` and `range_union_candidate: null`. Admission is present only when admission completed before the later failure; otherwise `admission.completed=false` and `admitted_source_ids=[]`. `COMPLETE` has `failure: null` and a non-null candidate.

The standalone validator enforces cross-field rules that JSON Schema cannot fully express: byte range `start <= end`, deterministic match ordering, source association, candidate members exactly equal matches, candidate digest, fixed point replay/custody recomputation, `W = J+Q+P+S+T+M+R+U+B`, exactly one failure counter for failed terminals and zero for complete terminals, manifest pins, malformed identity, and custody/replay normalization.

## Canonical JSON profile

Canonical JSON is exact UTF-8, sorted object keys, no insignificant whitespace, and exactly one trailing LF. Raw inputs with duplicate keys, unknown fields, trailing data, invalid UTF-8, or non-canonical terminal encodings are rejected.

## Payload/freeze boundary

The v4 contract boundary is nonrecursive. `payload.members` names frozen payload leaves only; the terminal envelope stores the payload digest and member names but does not recursively validate or interpret payload contents. Freeze, production admission, oracle generation, and implementation-specific search behavior remain out of scope.
