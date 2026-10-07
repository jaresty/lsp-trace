# ADR 0007 Location v5 input-corpus audit

Verdict: **INPUT_CORPUS_GO**

Approved corpus commit: `64afd44b13c825e9c5f2cc87789f3c6b5bbe47c1`

The corpus contains 26 executable, censused input-only cases. Case 23 isolates 1,001 raw frozen paths with one expanded source; case 25 isolates 10,001 unique witnesses. Typed admission branches reproduce through the pinned admission-only checker. Twenty validator mutation tests pass. No expected result, evaluator, oracle, freeze, or GO artifact appears in executable cases.

The request-specific `W`, `W−1`, `B`, and `B−1` variants remain value-free and deferred until independent oracle derivation. This verdict authorizes only use of the immutable common inputs by isolated design implementations. It does not establish behavioral correctness, `LOCATION_DESIGN_GO`, qualification execution, acceptance, feature identity, completeness, production authority, public enablement, release, or push.
