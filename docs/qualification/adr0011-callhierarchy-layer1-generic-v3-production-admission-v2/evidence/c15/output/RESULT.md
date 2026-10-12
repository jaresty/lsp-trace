# C15 Delta Enumeration Result

- Generated candidates/classifications: 4154/4154
- Delta: ADDED=309 REMOVED=2 CHANGED=41 UNCHANGED=3804
- Duplicate candidate IDs: baseline=0 generated=0
- Candidate/classification key-set equality: baseline=true generated=true
- Canonical discovery reconciliation: true
- Source-location completeness: true
- Deterministic in-process rerun equality: true
- Accepted patch SHA-256: `ca762452805215750a122ee250e92326387c343e92c36b844fcaf12ad2cefcba`

## Disposition

`C15_DELTA_BLOCKED_PENDING_ROW_ADJUDICATION`

The canonical catalog and exact candidate-ID delta are complete. Every affected row is preserved with canonical old/new classification and provenance, but this mechanical enumerator does not infer NECESSARY, REUSE, or FALSE_POSITIVE from syntax. Independent semantic review is required before a C15_DELTA_GO candidate can be issued.

## Coverage limitations

This evidence covers build-active non-test Go source selected by the canonical C15 v2 helper and the accepted 44-row mapping. It does not authorize classification changes, artifact succession, implementation, commits, or directional behavior. Mapping-row source spans come from the canonical semantic mapping verifier's uniquely resolved operations; ordinary candidate spans are independently correlated back to canonical candidate IDs and AST digests.
