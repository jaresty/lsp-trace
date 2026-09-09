# Program A Evaluator Repair

This repair addresses the three blocking findings in `/private/tmp/lsp-trace-program-a-evaluators/INDEPENDENT-REVIEW.md` against `d45b469`.

## A2 — effective configuration

The evaluator input no longer has an effective-configuration `Bytes` field. It invokes `provider.Provision`, consumes the returned canonical declarations and selector admission result, and serializes `lsp-trace.program-a-effective-configuration-evidence.v1`. The closed schema binds custody reference, complete canonical provider declarations (including stable identity, exact version, protocol identity/version, executable configuration, capabilities, limits, and conformance state), allowed selectors, and the `PROVISIONED` result. Encoding is decoded with unknown fields rejected, reprovisioned, compared to the original result, re-encoded, and required to roundtrip byte-for-byte before signing.

## A3 — support accounting

The evaluator input no longer has a support-accounting `Bytes` field. It invokes `relations.MinimumDependence` and serializes `lsp-trace.program-a-support-accounting-evidence.v1` directly from the validated observation graph and returned classes. The canonical output binds the SHA-256 input graph digest, `minimum-dependence.v1` computation version, exact observation denominator, every deterministic support group, each group’s qualification, and the exact empty omissions set. Semantic observation changes therefore change the canonical bytes and receipt digest; caller relabeling is unavailable.

## A4 — qualification

A4 is now a required seventh opaque Program A receipt. Issuance requires `lsp-trace.qualification-matrix-profile.v2`; v1 remains accepted by the historical profile validator but cannot issue A4 evidence. The evaluator consumes the actual `AdmissionRequest.Results`, requires a result for every generated cell (including every foundational cell), real-server evidence, exact provider class/version, `PASS`, and no waiver. `UNKNOWN`, `BLOCKED`, `FAIL`, `NOT_QUALIFIED`, missing cells, waivers, synthetic evidence, unknown/duplicate cells, and profile/result provider mismatches reject.

The canonical `lsp-trace.program-a-qualification-evidence.v1` output binds the v2 profile, canonical profile digest, canonical sorted result-matrix digest, exact provider class/version bindings, and every status. It is roundtrip-validated before signing.

## Shared receipt and authority properties

`issue` accepts only package-private retained metadata plus bytes returned by the relevant verifier-output constructor. Every receipt evidence digest is exactly SHA-256 over those canonical output bytes. Program A authority constructors, evaluation, receipt construction, verified provenance, and receipt digests remain package-private/opaque.

This change does not configure production authority, export an evaluator, add a caller, register or activate Program A or Program B, qualify the retained graph fixture, or make actual admission true. Current actual Program A admission remains false; Program B remains unadmitted and unregistered.

## Focused verification

The focused evaluator guards cover caller-byte surface removal, semantic output substitution, deterministic roundtrip, exact support accounting, v1 rejection, missing/waived/UNKNOWN/synthetic/provider-mismatched qualification results, and canonical verifier-byte digest equality. Verification is intentionally limited to focused normal/race tests, package vet, and build; no full suite or full race is run.
