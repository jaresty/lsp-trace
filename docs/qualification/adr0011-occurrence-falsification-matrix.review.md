# REFERENCES_SYMBOL_V1 — normative occurrence falsification matrix (review draft)

**Status: PROPOSED; not executed or qualified.** This appendix is part of [the production occurrence-admission contract](adr0011-production-admission-gate.review.md), not a report of passing fixtures. Each row is mandatory under pinned implementation/schema/admission-policy digests, including the subordinate document-symbol query-target receipt. Fixtures marked `qualified` require the real clean registered Go/gopls profile; synthetic peers are only controls. No row permits `CALLS`, public exposure, Leiden, or source-context-completeness claims.

## Reading the table

Every row uses one **predeclared** query, `N=1`. `B/T` count begun/terminal *queries*; `E/E_B/E_T` count returned/begun/terminal *elements*; `P` counts only elements from a successful **whole-result** parse; `A` counts **newly** admitted immutable occurrences. `U` means unknowable from complete bounded result evidence, **not zero**. `—` for request outcome means no verified method request outcome may be issued; it is not a new member of the closed request-outcome vocabulary. `NONE` query disposition means B=T=0 or no replayable terminal disposition. A diagnostic prefix never contributes P or A. `V` = independently verified successful method + target/terminal receipts; `D` = independently verified *diagnostic-only* failure record (not a successful method receipt); `N` = none; `H` = previously issued historical receipt no longer replay-eligible. `VERIFIED_SUCCESS`, `VERIFIED_DIAGNOSTIC`, `NOT_ATTEMPTED`, `PRECOMMIT_FAILED`, `PUBLICATION_UNVERIFIED`, `WITHHELD`, `REMOVED`, `DELETE_FAILED_QUARANTINED`, and `REPLAY_UNAVAILABLE_UNEXPLAINED` are publication/lifecycle dispositions, **not** request outcomes. `active` is eligibility for new typed traversal/admission inputs *after* occurrence qualification; `inactive` is ineligible; `tombstoned` is irreversibly ineligible without rewriting historical bytes.

For rows with D, the private diagnostic payload is independently bounded and redacted; if the selected privacy policy withholds it, apply the withheld row instead. `E_B/E_T` count only independently recorded element evaluations, not a parser's speculative prefix. If any prescribed D/V receipt cannot be independently replayed, the row fails qualification rather than silently falling back to another row. Request precedence remains ADR 0011's closed vocabulary; `TARGET_IDENTITY_UNRESOLVED` and `PUBLICATION_UNVERIFIED` are **pre-admission gate** dispositions, not new request outcomes.

## Mandatory terminal and publication fixtures

| Fixture (each independent) | Request outcome / query disposition; B/T | E/E_B/E_T/P/A | Receipt; publication | Eligibility |
| --- | --- | --- | --- | --- |
| Capability absent; **zero method invocation** | `UNSUPPORTED` / `NONE`; 0/0 | U/0/0/0/0 | N; NOT_ATTEMPTED | inactive |
| Server error with code **0 present** | `PROVIDER_FAILURE` / `NONE`; 0/0 | U/0/0/0/0 | D; VERIFIED_DIAGNOSTIC | inactive |
| Server error with code **absent** (same error message) | `PROVIDER_FAILURE` / `NONE`; 0/0 | U/0/0/0/0 | D; VERIFIED_DIAGNOSTIC; diagnostic records absence, never code 0 | inactive |
| Cancel **before output** | `CANCELLED` / `NONE`; 0/0 | U/0/0/0/0 | N; NOT_ATTEMPTED | inactive |
| Timeout **before output** | `TIMEOUT` / `NONE`; 0/0 | U/0/0/0/0 | N; NOT_ATTEMPTED | inactive |
| Cancel **after one complete element prefix, before complete top-level value** | `PARTIAL` / `FAILED`; 1/1 | U/1/1/0/0 | D; VERIFIED_DIAGNOSTIC | inactive |
| Timeout **after one complete element prefix, before complete top-level value** | `PARTIAL` / `FAILED`; 1/1 | U/1/1/0/0 | D; VERIFIED_DIAGNOSTIC | inactive |
| Partial/short **request write**, no completed keyed write | `PROVIDER_FAILURE` / `NONE`; 0/0 | U/0/0/0/0 | N; NOT_ATTEMPTED | inactive |
| Short/unmatched **response read**, no complete keyed response | `PROVIDER_FAILURE` / `NONE`; 0/0 | U/0/0/0/0 | N; NOT_ATTEMPTED | inactive |
| Transport-success response **missing result payload** | `PROVIDER_FAILURE` / `NONE`; 0/0 | U/0/0/0/0 | D; VERIFIED_DIAGNOSTIC (absence encoded) | inactive |
| Exact successful **`null`** under admitted target and full denominator | `COMPLETE_EMPTY` / `EMPTY`; 1/1 | 0/0/0/0/0 | V; VERIFIED_SUCCESS | active empty result, no occurrence |
| Exact successful **`[]`** under admitted target and full denominator | `COMPLETE_EMPTY` / `EMPTY`; 1/1 | 0/0/0/0/0 | V; VERIFIED_SUCCESS, distinct bytes/identity from `null` | active empty result, no occurrence |
| Two **equal valid locations** with distinct original ordinals | `COMPLETE` / `ITEMS`; 1/1 | 2/2/2/2/2 | V; VERIFIED_SUCCESS; two distinct occurrence IDs | active |
| Malformed **first** member in complete two-element array | `MALFORMED` / `MALFORMED`; 1/1 | 2/1/1/0/0 | D; VERIFIED_DIAGNOSTIC, failing ordinal 0; later element unbegun | inactive |
| Valid first, malformed **second** member in complete two-element array | `MALFORMED` / `MALFORMED`; 1/1 | 2/2/2/0/0 | D; VERIFIED_DIAGNOSTIC, failing ordinal 1, diagnostic prefix 1 only | inactive |
| Exactly **1,000** valid returned locations under all byte/work bounds | `COMPLETE` / `ITEMS`; 1/1 | 1000/1000/1000/1000/1000 | V; VERIFIED_SUCCESS | active |
| Exactly **1,001** returned locations, complete array count safely known | `RESOURCE_LIMIT` / `LIMITED`; 1/1 | 1001/0/0/0/0 | D; VERIFIED_DIAGNOSTIC; no truncation to 1,000 | inactive |
| **Pre-evaluation** selected work/byte limit, result E cannot be safely counted | `RESOURCE_LIMIT` / `NONE`; 0/0 | U/0/0/0/0 | D; VERIFIED_DIAGNOSTIC | inactive |
| **Post-evaluation** work limit after one valid element of a complete two-element array | `PARTIAL` / `LIMITED`; 1/1 | 2/1/1/0/0 | D; VERIFIED_DIAGNOSTIC; second unbegun | inactive |
| Privacy policy **withholds before element evaluation** otherwise available mandatory canonical payload | — / NONE; 0/0 | U/0/0/0/0 | N; WITHHELD | inactive; no new admission |
| **Precommit** no-replace publication failure after valid two-item parse | — / NONE; 1/0 (B observed, no verified terminal T) | 2/2/2/2/0 *provisional, not an issued ledger* | N; PRECOMMIT_FAILED | inactive |
| **`COMMITTED_VERIFICATION_FAILED`** after bytes committed | — / NONE; 1/0 (B observed, no verified terminal T) | 2/2/2/2/0 *provisional, not an issued ledger* | N; PUBLICATION_UNVERIFIED; quarantine exact selector/digest | inactive |
| **Duplicate returned Location** (not a duplicate JSON key) | `COMPLETE` / `ITEMS`; 1/1 | 2/2/2/2/2 | V; VERIFIED_SUCCESS; exact ordinal 0 and 1 identities differ | active |
| Duplicate **parameter JSON key**, including escaped/case alias | `POLICY_MISMATCH` / `NONE`; 0/0; zero invocation | U/0/0/0/0 | N; NOT_ATTEMPTED | inactive |
| Duplicate **result JSON key** in later element | `MALFORMED` / `MALFORMED`; 1/1 | 2/2/2/0/0 | D; VERIFIED_DIAGNOSTIC; failing ordinal 1 | inactive |
| Duplicate/unknown/trailing **receipt or ledger field** | — / NONE; no *new* verified verdict | U/0/0/0/0 for new admission | N under substituted bytes; original object may remain VERIFIED_SUCCESS | inactive for substituted object |
| Query-target receipt absent, tied selection, or URI/point/source/revision mismatch **before method invocation** | — / NONE, `TARGET_IDENTITY_UNRESOLVED`; 0/0; zero invocation | U/0/0/0/0 | N; NOT_ATTEMPTED | inactive |

A diagnostic record's provisional counts never satisfy `T=N` or authorize `COMPLETE[_EMPTY]`. In malformed rows, `VALID_DIAGNOSTIC` can terminally label the first *element* without making the whole-result P nonzero. The 1001 row tests `MaxResultElements`, **not** `MaxAcquisitionQueries` or downstream `ProgramCMaxInputs`.

### Independent hard-bound and subordinate-target controls

The following rows use the same outcome/counter/receipt/publication/eligibility columns. For target rows, E and element counters refer to the **references request**, which must not be invoked; document-symbol failure is separately retained under its subordinate terminal receipt when privacy permits. `N=17` in the first row is a rejected *acquisition declaration*, not 17 issued requests.

| Fixture | Request outcome / query disposition; N/B/T | E/E_B/E_T/P/A | Receipt; publication | Eligibility |
| --- | --- | --- | --- | --- |
| **17th** acquisition query under `MaxAcquisitionQueries=16` | `POLICY_MISMATCH` / NONE; 17 rejected/0/0, no invocation | U/0/0/0/0 | N; NOT_ATTEMPTED | inactive |
| **65,537** exact parameter bytes (`MaxParamsBytes+1`) | `RESOURCE_LIMIT` / NONE; 1/0/0 | U/0/0/0/0 | N; NOT_ATTEMPTED | inactive |
| **1,048,577** exact result bytes (`MaxResultBytes+1`) | `RESOURCE_LIMIT` / NONE; 1/0/0 for admission | U/0/0/0/0 | D with redacted bound only; VERIFIED_DIAGNOSTIC if within its own cap, otherwise N/WITHHELD; never retain over-bound raw | inactive |
| **1,500,001** encoded canonical bytes (`MaxCanonicalRecordBytes+1`) with a bounded, fully parsed 1,000-element raw result | — / NONE; 1/1/0 (no verified terminal) | 1000/1000/1000/1000/0 *provisional, no issued ledger* | N; PRECOMMIT_FAILED | inactive |
| Selected live-wire/message/aggregate-work limit exceeded **before evaluable output** | `RESOURCE_LIMIT` / NONE; 1/0/0 | U/0/0/0/0 | D redacted; VERIFIED_DIAGNOSTIC or N/WITHHELD by privacy policy | inactive |
| Document-symbol `[]` / no containing selection | — / NONE; 1/0/0, `TARGET_IDENTITY_UNRESOLVED` | U/0/0/0/0 | references N/NOT_ATTEMPTED; subordinate verified-empty target result cannot issue target identity | inactive |
| Document-symbol tied/overlapping most-specific selections | — / NONE; 1/0/0, `TARGET_IDENTITY_UNRESOLVED` | U/0/0/0/0 | references N/NOT_ATTEMPTED; subordinate diagnostic ambiguity | inactive |
| Malformed later document-symbol child, unknown field or duplicate key | — / NONE; 1/0/0, `TARGET_IDENTITY_UNRESOLVED` | U/0/0/0/0 | references N/NOT_ATTEMPTED; subordinate MALFORMED diagnostic, no valid prefix identity | inactive |
| Subordinate target's managed source/version/revision/encoding mismatch or missing canonical payload | — / NONE; 1/0/0, `TARGET_IDENTITY_UNRESOLVED` | U/0/0/0/0 | references N/NOT_ATTEMPTED; target receipt not replay-eligible | inactive |

## Cross-object substitution fixtures — execute **each listed field independently**

Begin with a qualified two-distinct-ordinal V fixture. For each field below, change **only that field** in an independently supplied expectation, retained object, or cross-object reference; do not consistently rewrite all records. Every row must reject the substituted replay: **no newly issued request outcome or query terminal** (`—/NONE`), `E/E_B/E_T/P/A = U/0/0/0/0` for *new* admission, `N=1` already declared, **no newly verified receipt** (the original unchanged receipt may remain V), publication is `NOT_ATTEMPTED` for the substituted object or the original remains `VERIFIED_SUCCESS`, and the substituted path is **inactive**, never silently tombstoned. A pre-invocation check may instead record `REVISION_MISMATCH`, `POLICY_MISMATCH`, or `TARGET_IDENTITY_UNRESOLVED` as specified in the main contract, but it still has B=T=A=0 and zero method invocation. If a field is changed on the actual retained object, the original selector/digest fails closed; no replacement object is admitted. This common verdict is normative for **each** line, not one aggregate test.

| Independently substituted field (one test per line) | Required mismatch observation |
| --- | --- |
| Method name or exact query occurrence ID | method/query identity rejection |
| Query URI | query/target mismatch |
| Query line or character (test each) | query/target mismatch |
| UTF encoding | encoding/source mismatch |
| Exact raw params **formatting** with semantically equal JSON | exact-byte digest mismatch; do not normalize |
| Params field value or `includeDeclaration` | grammar or query mismatch |
| Managed session ID or generation (test each) | session/generation mismatch |
| Owned request key or collision-safe invocation ID (test each) | key/invocation mismatch |
| Completed request-write or selected response-read observation (test each) | keyed wire-consistency mismatch |
| Capability advertisement or provider/adapter as reported (test each) | capability/provider policy mismatch |
| Host Git root, before/after commit or cleanliness evidence (test each) | `HOST_GIT_PROBE_V1`/revision mismatch, never provider verified |
| Revision commit or revision custody (test each) | revision mismatch, never upgraded to provider verified |
| Prepared document URI, version or digest (test each) | document/target mismatch |
| Document-symbol source payload or its full result bytes (test each) | subordinate target receipt mismatch |
| Document-symbol selection range or uniquely chosen target symbol identity (test each) | subordinate target derivation mismatch |
| Document-symbol method, terminal or query-target selector/digest (test each component) | subordinate cross-object link mismatch |
| References method, result payload or result presence (test each) | references receipt and cardinality mismatch |
| Reference returned URI, range or ordinal (test each) | occurrence identity/multiplicity mismatch |
| References method receipt selector/digest (test each) | transaction cross-object link mismatch |
| Terminal member ledger selector/digest or N/B/T/E/E_B/E_T/P/A value (test each) | terminal balance/custody mismatch |
| Occurrence ledger selector/digest, kind, custody role, source/target direction or ID (test each) | typed-admission mismatch; never retype as CALLS |
| Privacy/method/admission policy ID or implementation/schema digest (test each) | selected policy/version mismatch |
| Unknown family version/kind, duplicate key or trailing canonical content (test each) | closed-schema replay rejection |

## Retention and lifecycle fixtures — no historical ledger rewrite

Here `E/E_B/E_T/P` means **historical stored** 2/2/2/2 for the originally admitted two-location fixture, not fresh replay after deletion. `A_new=0` for every row; historical `A=2` bytes remain immutable. No new request outcome or query disposition is issued (`—/NONE`) by a deletion event. A `PRIVACY_REVOKE` cascades immutable tombstones to every dependent; a retirement cannot run with active dependencies. Distinguish physical bytes from *eligible* bytes.

| Fixture | Request/query; E/E_B/E_T/P/A_new | Receipt; publication | Eligibility |
| --- | --- | --- | --- |
| Reviewed `RETIRE_DERIVATIVES_AND_REMOVE`, zero active dependents, successful deletion | —/NONE; historical 2/2/2/2/0 new | H; REMOVED with exact removal manifest | tombstoned, no active input |
| Reviewed `PRIVACY_REVOKE` while active dependents exist; cascade all tombstones then remove | —/NONE; historical 2/2/2/2/0 new | H; REMOVED with exact revoke/dependency manifest | tombstoned, no active input |
| Retire attempted while live occurrence/derivative still pins payload | —/NONE; historical 2/2/2/2/0 new | original V; VERIFIED_SUCCESS; removal disposition `REMOVAL_DENIED_ACTIVE_DEPENDENCY`, no deletion | original active; removal denied |
| Reviewed removal or revoke, **deletion fails** and bytes may remain | —/NONE; historical 2/2/2/2/0 new | H for new admission; DELETE_FAILED_QUARANTINED, exact failed selector inventory | inactive/quarantined pending reviewed resolution; revoke dependents tombstoned |
| Mandatory payload **missing without reviewed removal manifest** | —/NONE; historical 2/2/2/2/0 new | N; REPLAY_UNAVAILABLE_UNEXPLAINED | inactive/quarantined, **not** a legitimate tombstone |

## Acceptance of the matrix

Each row and each independently listed substitution must have pinned input bytes, owner and fixture profile, observed request/terminal/element counters, exact publication/readback statuses, expected versus observed verdict, diagnostic redaction result, and executable guard identity. A missing receipt or unmeasured counter is **INCOMPLETE**, never an inferred pass. Pin the matrix digest with the contract and qualification packet before execution. Reviewer acceptance of this *draft* permits implementation of these tests; it does not assert they passed.
