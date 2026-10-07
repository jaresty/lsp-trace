# ADR0011 C07 portable P2 execution packet

This packet binds the private C07 pre-block deadline path to deterministic Go source and tests.

## Reproduction

From the repository root:

```text
go test ./sessionruntime -run '^(TestC07PrivateP2IntegratedPreBlockBoundary|TestC07PortableP2Packet)$' -count=1
```

Expected result: five passed test/result names and zero failures, exactly as ordered in `manifest.json`.

The integrated witness scripts exactly two manager-clock reads after fixture construction. At request-limit minus 1 ns it passes the deadline gate and reaches deliberate document-supply refusal; at equality and plus 1 ns it returns request timeout before blocking work. Each case preserves child writes, request and worker state, protocol ownership, readiness owners and borrowers, observations, pending-key proxy and tombstones, and the complete supplied zero-valued private transaction reservation.

## Identity

The manifest's `PRE_TEST_PRODUCTION_BASELINE` is the fixed non-self-referential baseline commit `2b3358f560073ffbaf203d573a5938c98a983d2e`, tree `6bf02d8067361e640bc0a715d16004eb21030235`. Exact byte lengths and SHA-256 values independently bind both tests and both production source files.

Manifest bytes are UTF-8 compact sorted-key JSON followed by exactly one LF. The verifier rejects duplicate keys, unknown fields, unsafe paths, symlinks, non-regular files, source drift, command/result drift, and extra packet files.

Design review: `DESIGN_GO`. Causal RED review: `RED_REVIEW_GO`, receipt `20261007042902-8754`.

## Authority ceiling

This is hermetic Go source-and-test evidence only. It does not attest managed-session identity, a live server or runtime execution, production authority, C08 transaction-clock continuity, or closure of any other matrix row or the broader C scope.
