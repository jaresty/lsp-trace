# Predecessors and independent finding

## Blocked identities

The following identities are recorded only as blocked predecessor design identities; they are not imported authorities and were not consulted for v5 semantics:

- `lsp-trace.adr0007.location-intersection.private.v1` — blocked: under-specified.
- `lsp-trace.adr0007.location-intersection.private.v2` — blocked: under-specified.
- `lsp-trace.adr0007.location-intersection.private.v3` — blocked: under-specified.
- `lsp-trace.adr0007.location-intersection.private.v4` — blocked: under-specified.

No v1–v4 fixture, oracle, expected result, audit conclusion, or runtime behavior is normative for v5.

## v5 status/custody plan

v5 is an additive private design-freeze root with status `IMPLEMENTATION_AGREEMENT_GO` and no Location qualification execution. Custody records spec `637680d2`, input base plus corrections `64afd44b` and `1546da22`, evaluator source `fccc0a03`, oracle source `1cff6a76`, integration `a9c82f1f`, merge base `d78e54d3`, and skipped patch-equivalent `7354f53c`.

The v5 freeze requires exactly 26 input-only cases, 26 evaluator results, 26 oracle results plus derivations, four boundary bundles (`W`, `W-1`, `B`, `B-1`), manifests, cross-custody, and a root `FREEZE.json` whose census self-measurement is the zero image digest and zero bytes. Independence is procedural-not-cryptographic; authority is 0; accepted is false; completeness is UNKNOWN; feature identity is UNRESOLVED; execution is designGO; dispatch is false.

## Independent under-specification finding

Starting only from the requested feature shape, types/schemas, and `sourceadmissionv2`, an interoperable implementation cannot be derived unless the design fixes canonical bytes and digests; validation/outcome precedence; path and selector semantics; UTF-16/LF geometry; relation direction and quantification; identity, duplicate, and per-member classification; witness/ranking order; exact counters; source/output/work accounting; limit checkpoints; cancellation; and backend failure behavior. Therefore every such choice is made prospectively in `DESIGN.md` and `ALGORITHM.md`.
