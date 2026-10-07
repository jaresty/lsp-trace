# ADR0011 C07 portable P2 execution packet v2

This successor is the selector-designated current-source authority for C07 after the independently scoped C08 startup-epoch correction changed shared runtime source bytes.

Run from the repository root:

```text
go test ./sessionruntime -run '^(TestC07PrivateP2IntegratedPreBlockBoundary|TestC07PortableP2Packet)$' -count=1
```

The semantic C07 boundary remains unchanged: request-limit minus 1 ns reaches the deliberate downstream refusal, while equality and plus 1 ns return request timeout before blocking work or transaction mutation.

`adr0011-c07-p2-selector-v2.json` chooses this packet semantically. This manifest cryptographically binds that selector and the exact current C07 implementation/test source denominator. The v1 manifest, README, and four original pinned sources remain preserved under historical custody and are verified independently.

The selector contains no manifest digest, avoiding cyclic construction. The closure matrix independently pins both final selector and manifest bytes.

Authority is hermetic Go source-and-test evidence only. This packet has no C08 authority, no managed-session or live-runtime authority, and no broader matrix-closure authority.
