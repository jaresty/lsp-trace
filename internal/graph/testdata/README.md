# Historical identity fixtures

`historical_identity_v2.json` and `historical_identity_v3.json` are complete canonical serialization fixtures for the historical V2 and V3 graph identities.

These bytes are immutable compatibility inputs, not updateable snapshots. Do not regenerate them to accept a serializer change. Any additive field, identity rule, canonicalization change, or receipt change that would alter either fixture requires a new schema version and new fixtures. Validation may continue to accept older documents, but V2/V3 production and interpretation must retain their existing behavior.

`TestHistoricalIdentityCanonicalBytes` is the executable boundary. It constructs one representative identity-bearing result for each historical version and compares the complete marshaled document byte-for-byte with these files.
