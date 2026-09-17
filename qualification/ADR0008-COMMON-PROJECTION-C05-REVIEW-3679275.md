# ADR 0008 common projection C05 qualification review

- Reviewed revision: `3679275d41fa043213b34728e4693fdec730cd06`
- Execution record: `qualification/adr0008-common-projection.execution.3679275d41fa043213b34728e4693fdec730cd06.json`
- Predecessor: `qualification/adr0008-common-projection.execution.ebf28c738ae7a9ee5278f757fecc1a80670578e0.json`
- Governing matrix: unchanged
- Result: `C02=PASS`, `C03=PASS`, `C04=PASS`, `C05=PASS`; `COMMON_PROJECTION=BLOCKED`; `implementation_qualified=false`

## C05 evidence

One integrated corpus covers public, restricted, withheld, ancillary, unavailable, and unauthorized eligibility classes. It proves:

- metadata-only is the default and returns no body;
- body return requires explicit opt-in and policy eligibility;
- restricted, withheld, ancillary, and unauthorized candidates are excluded before acquisition;
- only the exact public URI reaches the recording managed-document preparer;
- restricted, withheld, and unauthorized projected units receive discoverable `POLICY_WITHHELD` accounting;
- unavailable source receives discoverable `SOURCE_UNAVAILABLE` accounting;
- private-root, arbitrary-path, secret, command, environment, and raw-error markers do not appear in serialized plans, preparation results, metadata projections, withheld projections, or unavailable projections;
- raw supplies remain request-ephemeral and non-JSON.

The focused C05 execution and full repository suite passed at the reviewed revision. Log paths and SHA-256 digests are recorded in the execution record.

## Remaining common-track blockers

- C01: exhaustive identity-field sensitivity.
- C06: exhaustive malformed-input and page-mutation RED corpus.

Historical `L04=FAIL` is unchanged. This review does not qualify the full common track or implementation.
