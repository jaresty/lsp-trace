# ADR0007 Location v5 design-freeze custody report

Status: `IMPLEMENTATION_AGREEMENT_GO` for private design freeze only.

This freeze records cross-audit custody for spec `637680d2`, input base plus corrections `64afd44b` and `1546da22`, evaluator source `fccc0a03`, oracle source `1cff6a76`, integration `a9c82f1f`, merge base `d78e54d3`, and skipped patch-equivalent `7354f53c`.

## Boundaries

- Exactly 26 input-only cases are admitted.
- Exactly 26 evaluator results are required.
- Exactly 26 oracle results and 26 oracle derivations are required.
- Exactly four boundary bundles are required: `W`, `W-1`, `B`, and `B-1`.
- Root `FREEZE.json` is self-measured as the zero image digest and byte count in the root census.

## Authority

The independence claim is procedural, not cryptographic. Authority is `0`, accepted is `false`, completeness is `UNKNOWN`, and feature identity is `UNRESOLVED`.

No Location qualification campaign was run or authorized. No execution authorization was created. Dispatch remains `false`.

## Predecessors

v1-v4 remain immutable historical blocked designs. Their files, fixtures, expected results, audit conclusions, and runtime behavior are not normative for v5.
