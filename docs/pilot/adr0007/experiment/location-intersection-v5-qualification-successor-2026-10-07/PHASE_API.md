# Successor phase API

Successor identity: `location-intersection-v5-qualification-successor-2026-10-07`.

Ordered phase API:

1. Plan
2. Simulate
3. Producers
4. Reviewers
5. Boundaries
6. Reconcile
7. Verify

`Plan` and `Simulate` are the only phases allowed without `PREDISPATCH_GO.json`. `Producers`, `Reviewers`, `Boundaries`, `Reconcile`, and `Verify` require an independently committed gate whose successor identity, tooling digest, authorization digest, assignments digest, freeze identity, authorization actor, and allow-real flag match exactly.

The checked-in `ASSIGNMENTS.json` binds exactly 26 ordered producer assignments and exactly 26 ordered reviewer assignments to `attempt-successor-01`. Producer roles forbid oracle, derivation, reviewer output, external inference, semantic retry, semantic repair, predecessor attempts. Reviewer roles forbid external inference, semantic retry, semantic repair, producer code changes, predecessor attempts.
