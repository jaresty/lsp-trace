# ADR 0011 historical CALLS normalized-occurrence compatibility (proposal)

**Status: PROPOSED, UNFROZEN, UNEXECUTED.** This is a review candidate for an additive projector from **already accepted historical CALLS** to a future common normalized occurrence envelope. It is not a frozen schema, accepted projection, admission receipt, migration, reacquisition plan, public endpoint, grouping-policy qualification, or permission to run Leiden. See [ADR 0011](../adr/0011-versioned-definition-reference-grouping-evidence.md), the [grouping plan](adr0011-definition-reference-grouping-plan.md), and the [admitted-relation index proposal](adr0011-admitted-relation-index-architecture-b.proposed.md).

## Boundary and proposed input

Normalization follows generic definition/reference semantic replay and precedes D/R publication/qualification and common-index construction in the grouping plan. The adapter reads an exact **already accepted** historical CALLS evidence selector and its admitted immutable artifact, under the historical reader and acceptance contract. It neither reissues LSP requests nor revises historical snapshots, packets, selectors, checkpoints, CLI/MCP bytes or omitted-selector `CALLS_ONLY` behavior. An unaccepted candidate, missing immutable payload, unavailable exact selector, or unverified source is not adapter input. A digest alone cannot reconstruct absent call-site facts.

Candidate input binding (field names are descriptive, **not** an approved wire schema): `{historical_selector, historical_schema_and_version, exact_artifact_bytes_and_length, verified_digest, accepted_admission_identity, original_custody_and_revision, original_claim_ceiling}` plus the accepted historical occurrence ledger and endpoint bindings actually supplied by that artifact. If an asserted binding cannot be verified against the accepted evidence, stop; do not read ambient source, query a server, infer a caller from text, or substitute a newer artifact under the same logical name.

## Candidate output and exact mapping for review

For each independently identified, accepted historical call occurrence `h`, propose one immutable typed projection `p(h)` with the following mapping. The mapping is conditional on those facts being present and verifiable in the historical evidence; it does not assert every old artifact contains every field.

| Candidate output field | Required mapping from accepted historical CALLS | Invariant |
| --- | --- | --- |
| `relation_kind` | Literal `CALLS` after verifying the selected historical family | Never relabel D/R or source text as CALLS. |
| `source_evidence` | Exact original selector, schema/version, byte length and verified digest; accepted admission identity | Preserve the original selector, not a newly issued or substituted selector. |
| `source_endpoint`, `target_endpoint` | Original caller identity and original callee identity respectively | Canonical caller → callee direction; never reverse to match another family. |
| `occurrence_identity`, `call_site` | Original independently identified call-site occurrence and exact server-reported call-site range/URI where admitted | Do not collapse equal endpoint pairs or shared spans; retain exact half-open range and encoding. |
| `relation_local_ordinal`, `multiplicity` | Original ordinal and one contribution for each admitted historical occurrence; preserve repeated occurrences separately | No re-numbering based on projector iteration, sorting, pair aggregation, or D/R ordinals. |
| `version_and_custody` | Historical family/version, provider/session/revision/document or retained custody, encoding and applicable source digest **only as actually supplied and verified** | No invented generation, document version, producer identity, or source completeness. |
| `claim_ceiling` | Historical claim labels and limitations unchanged | `authority=0`, `accepted=false`, `completeness=UNKNOWN`; no producer authentication or runtime-execution proof is conferred by this projection. |

The proposed projector is a partial function on verified accepted historical inputs: one output per admitted historical occurrence, with a verifiable one-to-one identity/ordinal mapping and exact per-source denominator. An absent historical fact required by the proposed envelope is a typed **ineligible projection**, not a default value. Candidate canonical ordering may order output records deterministically, but cannot replace original relation-local ordinals or multiplicity. The exact field names, version, canonical encoding, digest domain, selector format, failure vocabulary and denominator equation remain **for review and freeze**, not normative accepted schema.

## Proposed fail-closed invariants

- Missing selector, payload, accepted admission, caller/callee, call-site identity/range, original ordinal, required custody or version: stop without a partial admitted projection; explicitly report the missing evidence rather than guessing it.
- Digest/length/selector substitution or custody mismatch: stop before projecting. An equal endpoint or display span is not proof of equal evidence.
- Duplicate occurrence identity with conflicting ordinal, range, endpoints, selector or custody: stop; exact repeated calls with **distinct** admitted occurrence identities preserve multiplicity. Invalid, reversed, out-of-bounds, wrong-encoding, or otherwise unverifiable call-site range: stop, never clamp or reconstruct from text.
- `REFERENCES_SYMBOL` and `RESOLVES_TO_DEFINITION` are separate typed occurrences with their own method, query-target, source and admission gates. No conversion to CALLS, cross-family deduplication, shared-identity upgrade, or implied D/R acceptance is permitted. A zero-CALLS input remains zero CALLS; a mixed input retains every family separately.
- This compatibility projection grants no common-index publication, typed traversal, `PairWeight` composition, grouping, Leiden, representative selection, or feature identity. Each later stage retains independent admission, policy, qualification and reviewer gates. Historical omitted selection still takes the historical CALLS-only path and retains its byte baseline.

## Private compatibility fixtures and review gates (proposed, not run)

| Fixture/assertion | Expected result |
| --- | --- |
| Pinned accepted historical CALLS ledger with two equal endpoint pairs but distinct call-site identities/ordinals | Two typed CALLS outputs in canonical order, unchanged caller/callee direction, exact site ranges, original ordinals and multiplicity; stable bytes/identities under input permutation without ordinal rewrite. |
| Historical omitted-selector CALLS process-byte baseline captured before any selector change | Existing public schemas, selectors, snapshot/packet/checkpoint/replay and CLI/MCP bytes unchanged; adapter output is separate, never a replacement historical artifact. |
| Mixed CALLS + admitted references/definitions and no-CALLS inputs | No cross-kind dedup or kind conversion; no-CALLS yields no projected CALLS; index/grouping still unavailable absent their own gates. |
| Missing accepted evidence, substituted digest/length/selector, invalid site range or conflicting duplicate identity | Typed fail-closed stop before any admitted normalized output; no source-text repair, LSP reacquisition or guessed historical value. |
| Text resembling a call versus server-reported accepted CALLS occurrence | Only the accepted server-reported call evidence can project CALLS; text alone contributes zero. |

Freeze requires a reviewed exact input/output mapping, canonical schema and replay identity, accepted historical fixture provenance and byte baseline, independently checked permutation and negative fixtures, and an **independent reviewer decision** binding the adapter version and evidence. Any missing fixture, unbalanced denominator, incompatible historical version, or unresolved field is `INCOMPLETE`, not implicit acceptance. Even an accepted adapter would not itself qualify D/R, publish the admitted-relation index, qualify a grouping policy, or enable Leiden/public surfaces. This document supplies none of those receipts or results.
