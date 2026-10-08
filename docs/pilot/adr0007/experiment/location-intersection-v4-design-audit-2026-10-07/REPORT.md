# ADR 0007 Location v4 design audit

Verdict: **LOCATION_DESIGN_BLOCKED**

Freeze: `sha256:daff605ad35ca59775aed33e60d36395b8304e218f984b43abd1ab3efa8e00cf` (`47,769` bytes; 319 entries)

V4 repaired mandatory root-manifest handling and exact nested `FREEZE.json` census behavior. Its generator is structurally separated from production evaluation and its verifier detects divergence. However, all 24 literal expected results are byte-identical to the blocked v3 evaluator-generated outputs after schema-version substitution.

Literalization establishes structural separation but not semantic independence. V4 therefore remains immutable and blocked. A successor must derive every terminal outcome, row, witness, ranking, counter, byte count, work charge, and precedence rule independently from frozen requests, admissions, source bytes, and explicit formulas—not from production output lineage.

No `LOCATION_DESIGN_GO`, qualification execution, semantic acceptance, feature identity, completeness, production authority, public surface, release, or push is established. Ceilings remain `authority=0`, `accepted=false`, `completeness=UNKNOWN`, and `featureIdentity=UNRESOLVED`.
