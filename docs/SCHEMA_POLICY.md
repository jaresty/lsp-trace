# Schema compatibility policy

Canonical output declares `schema_version: lsp-trace.graph.v1`. Within v1, field meanings and enum meanings are stable. Additive optional fields may be introduced in a minor release only when existing consumers can ignore them. Removing or renaming fields, changing requiredness, changing edge orientation or identity rules, or changing an existing enum's meaning requires a new schema major version.

New enum values are additive but consumers must treat unknown values as forward-compatible rather than silently mapping them to an existing reason. Canonical ordering and the exclusion of wall-clock metadata remain compatibility obligations. Release review must identify schema-affecting changes and update golden tests, documentation, and the major version together when required.
