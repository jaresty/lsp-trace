# ADR0011 C08 portable P2 execution packet

This packet proves the private generation-bound 60-second startup-attempt epoch from startup preflight through lifecycle retirement.

Run from the repository root:

```text
go test ./sessionruntime -run '^(TestC08PrivateP2AttemptStartsAtStartupPreflight|TestC08PrivateP2StopRetiresAttemptEpoch|TestC08PrivateP2RestartCreatesDistinctEpoch|TestC08PrivateP2FailedRestartAdmitsNoEpoch|TestC08PortableP2Packet)$' -count=1
```

The witnesses establish:

- the epoch begins at startup preflight rather than session installation;
- minus 1 ns reaches the deliberate downstream refusal while equality and plus 1 ns prohibit new blocking work;
- completed STOP retires the generation and epoch;
- successful RESTART retires the old generation and creates a distinct successor startup attempt and epoch;
- stale-generation work is refused before deadline admission;
- failed RESTART admits no successor epoch and cannot begin private blocking work.

The manifest binds exact production and test source bytes, the non-self-referential pre-implementation baseline, exact command/results, and filesystem containment.

Authority is limited to hermetic Go source-and-test evidence for C08. This packet has no C07 authority or recertification role, no managed-session or live-runtime authority, no production authority, and no broader matrix-closure authority.

Causal RED review: `RED_REVIEW_GO`, receipt `20261007051231-2962`.
