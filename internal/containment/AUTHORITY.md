# Containment authority and evidence ceiling

## Local authority

`internal/containment` is the sole production owner of the process-lifetime containment gate. The production `RuntimeGate` accepts no caller-provided observation, fixture, adapter, boolean attestation, serialized result, platform string, setter, or refresh input. Its snapshot is an internal availability input only; external MCP availability remains the separately owned `CONTAINMENT_UNAVAILABLE` projection.

`internal/containment/reference` is a pure classification model for tests, fixtures, and specification examples. Its exported observations, `VERIFIED` classification, and verdicts are distinct non-authorizing types. A reference verdict cannot authorize a production runtime verdict, cannot construct a `RuntimeGate`, and cannot be converted to a production snapshot.

## Evidence ceiling

This reference package and its supporting evidence does not prove native containment. Reference evaluation, unit tests, architecture checks, cross-compilation, source inspection, and documentation are bounded structural evidence only. They establish only the fail-closed type and dependency boundary and deterministic behavior of the reference model.

A native support claim requires separately reviewed, platform-specific adversarial evidence for every designed obligation, produced through a sealed production path before capability publication. No such evidence is present here, so all platforms remain unavailable. This package does not enable Stage 2 or Stage 3 and does not establish owner-death cleanup, descendant completeness, anti-escape behavior, authority isolation, bounded cleanup, survivor enumeration, death observation, or reap on any operating system.
