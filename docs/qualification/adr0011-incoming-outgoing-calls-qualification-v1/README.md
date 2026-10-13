# ADR0011 Incoming/Outgoing CALLS Qualification Architecture v1

**Status: BLOCKED_UNJUSTIFIED_NUMERIC_BOUNDS.** Documentation-only correction at repository revision `4b910fc0c517cb7801fb51d17d295897230d11fa`. Independent review exposed that proposed exact-transport values were incorrectly treated as numeric authority. At least twelve bounds lack accepted governing requirements or retained measurements, so the architecture cannot be frozen. It authorizes no implementation, provider, profile, runtime occurrence, public API, production route, graph admission, or CALLS truth.

## Decision ceiling

This package closes architecture questions only. Historical files remain unchanged and retain their original authority. Generic Layer 1 is the sole terminal/custody authority. Incoming and outgoing are separate immutable directional transfers above Layer 1. Neither direction can authenticate the provider or authorize the other.

## Package contents

- `architecture.md` — ownership, directional models, state machine, limits, seams, write-sets, alternatives, C15 and public-surface analysis.
- `causal-red-plan.md` — causal RED plans through real production paths; no tests are added.
- `evidence.md` — bounded source/runtime evidence, commands, typed failures, probe result, hashes, and unresolved evidence.
- `review-request.md` — independent review questions and fail-closed acceptance boundary.
- `numeric-bound-audit.log` — immutable row-by-row measurement/authority audit and hard-stop.
- `outgoing-structural-census.log` — exact HEAD outgoing production seam and minimal write-set.
- `DEPENDENCIES.sha256` — exact dependency hashes and dispositions.
- `MANIFEST.sha256` — package checksums (self-excluding).

## Eight blocker dispositions

1. **Empty incoming `fromRanges`: RESOLVED FOR DESIGN.** Non-null `[]` creates one retained zero-site member record, zero range records, zero occurrences, and zero edges. Missing/null/scalar/object remains malformed and the transaction admits no prefix.
2. **Layer 1 custody: RESOLVED FOR DESIGN.** Generic Layer 1 alone allocates, owns, refuses, terminalizes, drains, zeroes, releases, and accounts for retained transport custody. Directional consumers receive immutable views only.
3. **Existing V3 integration/default-off: RESOLVED FOR DESIGN.** Extend private generic machinery only; no public activation. Disabled execution and selected CALLS-only execution must retain byte-identical provider requests/responses and legacy artifact bytes.
4. **Numeric limits: BLOCKED.** Proposed exact-transport originals are informative only. `numeric-bound-audit.log` records source/measurement, observed maximum, margin, owner, exact-limit/+1 evidence, and shadowing guard for every candidate. At least twelve rows are unjustified and therefore hard-stop architecture qualification.
5. **Availability state machine: RESOLVED FOR DESIGN.** Closed state machine and precedence are frozen in `architecture.md`.
6. **Exact-profile runtime/provider evidence: OPEN AND GO-BLOCKING.** A prospective session status and live probe are retained, but language-server binary/version/argv/env, initialize transcript, exact workspace revision custody, complete request/response originals, and independent provider authority are absent.
7. **Outgoing production seam: RESOLVED STRUCTURALLY AT HEAD.** `outgoing-structural-census.log` freezes `traceExecutor.Execute` → acquisition orchestration → `slicer.Discover[Prepared]` and `slicer.Client.OutgoingCalls`, exact ownership, tests, hashes, and minimal write-set.
8. **Provider identity/authority: RESOLVED FAIL-CLOSED.** Incoming and outgoing have distinct provider identity records and independent authority reviews. Evidence or GO cannot be transferred across directions.

## Final disposition

`BLOCKED_UNJUSTIFIED_NUMERIC_BOUNDS`: hard-stop. The preserved nonnumeric models, rejected alternatives, sole-owner rule, no-public-surface boundary, dependency pins, and resolved outgoing census remain review inputs, but no corrected architecture decision can issue until every numeric row has non-proposed authority or retained measurement plus causal exact-limit/+1 evidence.