# ADR0011 revised C closure scope v2

This document applies the accepted scope amendment `ADR0011-C02-SUPERSEDED-BY-C01-V1` without modifying its predecessor:

- predecessor: `docs/qualification/adr0011-remaining-c-consolidated-plan.proposed.md`
- predecessor SHA-256: `b71e2e1947d2b33cc1fac5b37cd38176b80486b1da44a9367c11a63a30fd8b9e`
- amendment: `docs/qualification/adr0011-c02-supersession-amendment.json`

## Revised inventory

The historical inventory remains C01–C18 plus six work counters. Future C exit evaluates:

- 23 active rows: C01 and C03–C18 plus all six work counters;
- one retained superseded inventory row: C02, whose only valid disposition is `SUPERSEDED_BY_C01` bound to the exact amendment identity.

C02 is no longer an independent pre-decode admission requirement. C01 is authoritative for generic complete-original-frame size admission before body allocation. Frames admitted by C01 continue through existing post-decode capability-family validation.

## Rejected streaming-classifier alternative

A streaming classifier was rejected because it would introduce a second bounded JSON-RPC parsing/correlation path solely to determine capability-family identity before normal decode. C01 already enforces the generic pre-allocation frame-size boundary. The duplicate parser and lifecycle state would increase disagreement and cleanup risk without changing admitted production behavior.

## Checker rule

A checker may treat C02 as non-blocking only when all of these hold:

1. row ID is exactly `C02`;
2. status is exactly `SUPERSEDED_BY_C01`;
3. successor is exactly `C01`;
4. amendment ID is exactly `ADR0011-C02-SUPERSEDED-BY-C01-V1`;
5. amendment path and SHA-256 match the tracked accepted amendment;
6. predecessor path and SHA-256 remain byte-identical;
7. the complete retained historical C02 row matches its canonical predecessor-row SHA-256;
8. every other active row satisfies the normal closure contract.

No other row may use a superseded status. Supersession is a scope decision, not implementation evidence and not a waiver that can be generalized.

## Claim ceiling

This revision does not authorize production behavior changes, D work, occurrence admission, indexing, path demonstration, public/API/schema changes, push, or release. Future reports must state that they assess the revised scope rather than the original C01–C18 active scope.
