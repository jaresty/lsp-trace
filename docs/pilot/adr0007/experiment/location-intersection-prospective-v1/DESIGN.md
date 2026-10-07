# ADR0007 Location Intersection Prospective v1 — DESIGN

Status: DESIGN ONLY. This tree is private, bounded, harness-neutral, non-dispatching, and not qualification evidence.

The design evaluates exact source-bound member locations against one frozen selector using `INTERSECTS`, `CONTAINED_BY`, or `CONTAINS`. It emits exactly one terminal member row for every input ordinal; ranked results are only a projection of eligible rows. Every eligible row carries a concrete selector/candidate intersection witness.

No model invocation, CLI/MCP/server registration, public API, candidate-group contract, ADR0011 surface, or production dispatch is introduced. `LOCATION_DESIGN_GO` is intentionally absent and may only be supplied by a separate audit.
