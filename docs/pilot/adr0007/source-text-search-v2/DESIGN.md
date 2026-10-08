# ADR0007 source text search prospective v2 design candidate

Status: `DESIGN_GO` for exactly one later private campaign only. This writer does not execute semantic qualification, does not self-verdict, and creates no public surface.

Normative scope is exact literal nonempty case-sensitive UTF-8 search over a multi-file admitted envelope. The design preserves overlaps and forbids regex, fuzzy, token, rank, model, backend, and feature semantics. Authority is `0`, accepted is `false`, completeness is `UNKNOWN`, and featureIdentity is `UNRESOLVED`. Fail closed.

All records are canonical JSON, validated by closed Draft 2020-12 schemas, and frozen with the self-measuring freeze convention in `FREEZE.md`. Terminal custody is exact-once: every attempt has exactly one result and exactly one custody/replay/accounting terminal record.
