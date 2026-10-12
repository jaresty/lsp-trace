# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Semantic-Residue Correction Amendment Review

Status: **BLOCKED**

Reviewed repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`  
Reviewed HEAD: `fd271defe54ac474d0992a8047354322c314d7f1`  
Reviewed amendment: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-semantic-residue-correction-amendment.md`

## Review boundary

This is an independent semantic and mechanical review of the pending amendment. The review recomputes from the accepted self-contained v3 `driver` request/transport inputs, the generic `Manager.RoundTrip` contract, and the private generic owner/view shape only. Current/prototype output and all current or historical `expected` values are observations to be corrected or mechanically compared, never semantic operands. World names are offline locators only.

The generic production contract contains opaque commitment metadata (`identity`, `sha256`, `length`, `available`) and no commitment-validity, admission, refusal, error, terminal, result, external-execution, or ledger predicate over those fields. The generic Manager refusal paths concern context, session, generation, lifecycle, resource, request, and transport state. A commitment field therefore cannot select execution outcome without adding hidden policy.

## Finding 1 — Worlds 17–22 correction is semantically required and its changed set is complete

Each root driver has one normal request and one `response-frame`; none contains a refusal event. The frame carries successful JSON-RPC result `[]`. The six current refusal labels correlate with opaque commitment metadata, but the generic driver/Manager contract supplies no rule that makes unavailable, empty-identity, or zero-digest metadata invalid.

Therefore each root must execute normally and derive:

- `refusal.refused=false`;
- `refusal.reason=null`;
- `terminal.available=true` and `terminal.value="successful-empty"`;
- `round_trip_ledger.terminal.after.available=true` and `.value="successful-empty"`;
- unchanged values elsewhere only after direct execution/rematerialization, never by projection from current expected values.

The exact scalar-change set per root is correctly stated as four paths:

```text
refusal.reason
refusal.refused
terminal.value
round_trip_ledger.terminal.after.value
```

The amendment also correctly requires rederivation of attachment access, result, error, external execution, terminal availability/before/after snapshots, lifecycle, and all seven five-member ledger dimensions. The five `control.*` fixture paths remain C and cannot become runtime policy.

Disposition: **PASS** for Worlds 17–22 semantics, changed paths, and mandatory causal-neighbor rematerialization.

## Finding 2 — Worlds 08–11 unavailable-error semantics are correct, but the asserted exact affected set is incomplete

Each root driver has exactly one control event and no `response-frame`:

| Root | Driver event | Event payload fields |
|---|---|---|
| World 08 | `cancel` | no frame, class, or message |
| World 09 | `deadline` | no frame, class, or message |
| World 10 | `shutdown` | no frame, class, or message |
| World 11 | `unavailable` | no frame, class, or message |

The generic runtime can derive the failure class from the event/Manager outcome. It cannot derive a server-provided JSON-RPC error object, code, message, data, data form, or serialized error bytes. The amendment's corrected unavailable-error rule is therefore semantically sound:

```text
error.available       = false
error.present         = false
error.class           = event/runtime class
error.code            = null
error.message         = null
error.data_base64     = null
error.data_form       = "unavailable"
error.serialized_hex  = null
derived.external.server_error_bytes = null
```

However, exact inspection of the frozen v3 roots shows that all four currently contain:

```text
error.available = true
error.present = true
error.data_form = "canonical-json"
```

along with populated `code`, `message`, `data_base64`, `serialized_hex`, and identical `derived.external.server_error_bytes` fabricated from inherited detailed payloads.

Consequently, the amendment's statement that `error.present=false` and `error.data_form="unavailable"` “already have the correct scalar values” is false. Its five-path exact A change set omits two changing A paths. The complete exact A change set per World 08–11 root is seven paths:

```text
error.available
error.present
error.class is rederived but its scalar remains unchanged
error.code
error.message
error.data_base64
error.data_form
error.serialized_hex
```

`error.class` is a mandatory direct rematerialization operand but not a scalar change. Excluding it, the exact scalar-change set is:

```text
error.available
error.present
error.code
error.message
error.data_base64
error.data_form
error.serialized_hex
```

The B change remains:

```text
derived.external.server_error_bytes
```

The accepted raw-response facts remain coherent and must stay unchanged: response frame unavailable/null/length 0/zero digest, result unavailable/null, external response bytes empty-but-available as `""`, transport message and byte measures available-known-zero, and no error object reclassified as a response frame.

Disposition: **BLOCKER**. The amendment must replace the five-path A set with the exact seven-path scalar-change set and remove the false claim that `error.present` and `error.data_form` already have corrected values.

## Finding 3 — World-24 occurrence-sensitive ownership and formulas are correct

The root's seven ledger dimensions each have five members (`before`, `after`, `delta`, `available`, `known_zero`), yielding exactly 35 occurrences. These root values compare the sixteen workers collectively and are not fields of the root execution's own production owner/view. They are therefore B only at the World-24 root and must be computed from the sixteen independently executed worker A ledgers. They remain A in every worker and every ordinary occurrence.

The formulas are complete and do not admit expected-fed operands:

```text
available[d]  = AND(w=1..16, worker_actual[w,d].available)
before[d]     = 0
delta[d]      = SUM(w=1..16, worker_actual[w,d].delta), iff available[d]
after[d]      = before[d] + delta[d], iff available[d]
known_zero[d] = available[d] && delta[d] == 0
```

If any worker actual required by a dimension is unavailable, that root aggregate is unavailable under existing normalization; expected worker values and zero substitution are forbidden.

A mechanical cross-check of the frozen sixteen worker A ledger values reproduces the listed aggregate scalars:

| Dimension | before | delta | after | available | known_zero |
|---|---:|---:|---:|---|---|
| callbacks | 0 | 0 | 0 | true | true |
| diagnostics | 0 | 0 | 0 | true | true |
| external_bytes | 0 | 608 | 608 | true | false |
| leases | 0 | 0 | 0 | true | true |
| reservations | 0 | 0 | 0 | true | true |
| transport_bytes | 0 | 960 | 960 | true | false |
| transport_messages | 0 | 16 | 16 | true | false |

Those expected-worker values are only a consistency cross-check; the later RED must recompute from actual workers. No production root aggregate API or storage is admissible, even if it returns equal scalars.

Disposition: **PASS** for the exact 35 occurrences, A/B ownership migration, sixteen-worker formulas, unavailable propagation, and no-production-storage rule.

## Mechanical accounting

The global schema remains 145 unique paths. The ordinary occurrence partition remains disjoint and exhaustive:

```text
A=116, B=20, C=9, total=145
```

At the World-24 root only, migrating the exact 35 dimension-member occurrences from A to B gives:

```text
A=116-35=81
B=20+35=55
C=9
81+55+9=145
```

Because the migration is occurrence-sensitive, the same path names remain A in each of the sixteen worker occurrences. The assertion rule must use an occurrence-plus-path key, not path name alone. For each of all 85 occurrences, the 145-path union must be disjoint and each occurrence-path pair must be asserted exactly once. No skip, duplicate, wildcard exemption, fallback, or expected-fed actual is permitted.

The 85-occurrence denominator is exact: 25 roots + 16 World-24 workers + 22 World-25 executions + 22 World-25 pair members. Mechanical inspection found all 17 removed/deferred paths absent in all 85 occurrences (zero present violations).

Disposition: **PASS** for `116/20/9`, World-24 `81/55/9`, exact-once occurrence accounting, 145 paths, 85 occurrences, and 17 absences.

## Commitments, raw custody, and immutable versions

Across all 85 occurrences, selected and handoff commitment metadata have zero driver-to-expected mismatches; predecessor cardinality and predecessor metadata also have zero mismatches. This establishes finite equality/custody only and does not authorize semantic interpretation.

The amendment correctly preserves the accepted roots World 07–11 raw-response custody correction. In particular, control events produce no response frame, empty external response sequences remain available as `""`, and a runtime failure class is not serialized into a fabricated JSON-RPC error object.

The amendment grants no mutation authority and keeps v1/v2 immutable. Repository status before this review showed only the untracked amendment; no tracked source, test, corpus, manifest, generated artifact, v1, or v2 byte was modified by this review.

Disposition: **PASS** for commitment equality, opaque custody, raw-response rules, and v1/v2 immutability boundaries.

## Canonical rematerialization and RED rules

The proposed canonical rematerialization boundary is otherwise sound: only Worlds 08–11 and 17–22 plus their manifest/aggregate consequences are scalar-rematerialization candidates; World 24 changes ownership/assertion logic without scalar rewrite unless independent actual execution disproves the formulas. Strict decoding, exact complement proof, canonical JSON, complete-byte length/hash regeneration, independent reproduction, direct actual A comparisons, actual-derived B formulas, structural-only C checks, and prohibited expected/prototype/golden operands are all required.

The World 08–11 blocker prevents that rule from being mechanically complete as written. Before authorization, the amendment must correct both its exact affected set and every downstream complement/manifest statement that relies on that set.

## Required correction before re-review

Amend the pending document so that, for each World 08–11 root:

1. the exact changing A set contains seven paths, adding `error.present` and `error.data_form`;
2. `error.class` is explicitly identified as rederived-but-scalar-unchanged;
3. the statement that `error.present` and `error.data_form` already have correct scalars is removed;
4. exact-complement proofs use the corrected seven-A-plus-one-B occurrence set.

No other semantic expansion is required by this review.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_SEMANTIC_RESIDUE_CORRECTION_BLOCKED
```

## Model interpretation

The review treats the amendment as a pending correction specification, not as authority to mutate corpus or RED artifacts. Generic Manager and driver behavior is authoritative for semantics; frozen expected values are used only to identify present residue and mechanically cross-check counts/equality. The World-24 worker expected ledgers corroborate listed arithmetic but cannot become later aggregate operands.

Key assumptions:

- HEAD `fd271defe54ac474d0992a8047354322c314d7f1` is the exact review revision.
- The untracked amendment is the residue amendment named by the request.
- The requested verdict is blocked by any incorrect claim of an exact affected set.

Gaps and verification items:

- Re-review the corrected World 08–11 exact set after amendment revision.
- In the later authorized RED, prove the 35 World-24 root paths have no production storage/view source.
- In later rematerialization, independently reproduce changed complete-world and aggregate bytes/hashes.

Improved framing: define every correction set as occurrence-keyed `(structural occurrence, expected path)` membership before stating unchanged complements or category totals.
