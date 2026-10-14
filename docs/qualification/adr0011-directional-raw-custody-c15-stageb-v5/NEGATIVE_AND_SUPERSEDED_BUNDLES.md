# Negative and superseded bundles retained

- `adr0011-directional-raw-custody-c15-enumeration-v1` remains a pre-implementation, all-UNRESOLVED enumeration bundle. Its absent-source proposed rows are not promoted here.
- `adr0011-directional-raw-custody-c15-two-stage-contract-v2` remains the StageA/StageB contract. StageA row IDs are predecessor lineage only.
- The earlier 47+1 model is reconciled in `MODEL_47_PLUS_1_RECONCILIATION.tsv`; proposed `call_hierarchy_directional_raw_v1_private.go` rows are superseded by current v5 source-derived IDs, not normalized into source facts.
- Existing nearby borrows from `b4_owned_method_pair_lease.go` remain negative/nonreusable predecessor context. StageB v5 uses response-frame aliases/views and C15 zero-charge aliases instead of treating those borrows as retained allocations.
- `View.SharesBackingPrivate` is intentionally classified `FALSE_POSITIVE`: it is a private qualification helper for alias proof, not production custody or retained allocation behavior.
