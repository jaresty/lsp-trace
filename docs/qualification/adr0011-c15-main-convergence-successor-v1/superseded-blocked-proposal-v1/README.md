# ADR0011 C15 main-convergence successor v1

Writer-authored candidate-specific successor package. All row classifications are **PROPOSED, NOT ACCEPTED**.

Counts: baseline 3847; current 4155; ADDED 310; REMOVED 2; CHANGED 3; UNCHANGED 3842; duplicates 0; proposed unresolved 0.

Identity reconciliation uses stable buckets `(path, enclosing, AST kind, display)` and normalized AST digests. It never pairs by ordinal. Unique exact digest matches preserve historical identity and classification while regenerating current location; unique residual pairs are CHANGED; unmatched rows are ADDED/REMOVED; ambiguity would be UNRESOLVED.

The 44 allocation mapping rows retain their verified site identities and provenance. Historical C15 v2 and all prior qualification packages remain immutable. Negative and superseded evidence remains at the prior versioned package paths and is not deleted or rewritten.
