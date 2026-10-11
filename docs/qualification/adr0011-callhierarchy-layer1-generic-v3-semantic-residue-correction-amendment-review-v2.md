# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Semantic-Residue Correction Amendment Review V2

Status: **GO**

Reviewed repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`  
Reviewed HEAD: `fd271defe54ac474d0992a8047354322c314d7f1`  
Reviewed amendment: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-semantic-residue-correction-amendment.md`  
Supersedes: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-semantic-residue-correction-amendment-review.md`

## Review boundary

This is an independent semantic and mechanical re-review of the corrected pending amendment. It treats accepted self-contained v3 `driver` request/transport inputs, the generic `Manager.RoundTrip` contract, and the private generic owner/view shape as the only causal operands. Current/prototype output and current or historical `expected` values are observations to correct or mechanically compare, never semantic operands. World names are offline locators only.

The amendment grants no corpus, RED, test, source, generated-artifact, implementation, C15, activation, commit, or production authority. V1 and v2 remain immutable.

## Prior blocker resolution

The superseded review blocked because the prior exact World 08–11 affected set omitted two changing A paths: `error.present` and `error.data_form`. The corrected amendment now:

1. lists both paths in the exact changing A set;
2. states the complete seven-A set;
3. identifies `error.class` as mandatory rederivation with unchanged scalar value;
4. identifies `derived.external.server_error_bytes` as the one changing B path; and
5. states the exact per-root complement as `7 A + 1 B = 8` changing occurrences and 137 unchanged occurrences.

No residue of the prior five-A claim remains in the corrected amendment.

Disposition: **PASS**.

## Worlds 17–22 commitment-policy correction

Each accepted root driver contains a normal request and a successfully emitted `response-frame` carrying JSON-RPC result `[]`. No driver contains a refusal event. Opaque commitment metadata supplies finite custody and equality only; it supplies no admission, refusal, error, result, terminal, lifecycle, external-execution, or ledger policy.

The corrected derivation is therefore:

```text
refusal.refused                            = false
refusal.reason                             = null
terminal.available                         = true
terminal.value                             = "successful-empty"
round_trip_ledger.terminal.after.available = true
round_trip_ledger.terminal.after.value     = "successful-empty"
```

The exact scalar-change set per root is the four stated paths:

```text
refusal.reason
refusal.refused
terminal.value
round_trip_ledger.terminal.after.value
```

The amendment correctly requires direct rematerialization of attachment access, result, error, external execution, lifecycle, terminal snapshots, and all seven five-member ledger dimensions even where their corrected scalars reproduce existing values. The five `control.*` paths remain C fixture structure and cannot become runtime policy.

Disposition: **PASS** for all six roots, the exact four-path set, mandatory causal-neighbor rematerialization, and the prohibition on commitment-selected outcomes.

## Worlds 08–11 unavailable detailed-error correction

The accepted drivers contain exactly one generic control event each and no `response-frame`:

| Root | Event | Runtime class |
|---|---|---|
| World 08 | `cancel` | `cancellation` |
| World 09 | `deadline` | `deadline` |
| World 10 | `shutdown` | `shutdown` |
| World 11 | `unavailable` | `transport-unavailable` |

The events can establish generic runtime failure classes but cannot establish server-provided JSON-RPC error objects. Exact corpus inspection confirms that the current v3 roots instead contain `error.available=true`, `error.present=true`, `error.data_form="canonical-json"`, populated code/message/data/serialization, and non-null `derived.external.server_error_bytes`. Those are semantic residue, not causal facts.

The corrected values are therefore:

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

### Exact `7 A + 1 B` set and complement

For each root, the seven changing A occurrences are exactly:

```text
error.available
error.present
error.code
error.message
error.data_base64
error.data_form
error.serialized_hex
```

The one changing B occurrence is exactly:

```text
derived.external.server_error_bytes
```

`error.class` is rederived directly from the event/runtime result but its scalar remains unchanged. Thus each root has exactly eight changed occurrences, partitioned `7 A + 1 B`, and exactly `145 - 8 = 137` complementary occurrences. The amendment requires those 137 to be proven unchanged after causal rederivation or structural preservation; it does not infer complement preservation merely from the current expected tree.

The accepted raw-response correction remains intact: no response frame, unavailable/null response frame, frame length zero, zero frame digest, unavailable/null result, available-empty external response bytes `""`, available-known-zero transport bytes/messages, and no reclassification of an error or event class as a response frame.

Disposition: **PASS** for all four roots, the exact seven-A set, exact one-B set, unchanged-class treatment, and the exact 137-occurrence complement.

## World-24 occurrence-sensitive ownership

At the World-24 root, the seven ledger dimensions each contain five members (`before`, `after`, `delta`, `available`, `known_zero`), yielding exactly 35 cross-worker aggregate occurrences. They are B only at the World-24 root. They remain A in all sixteen worker occurrences and in every ordinary root/execution/pair-member occurrence.

The formulas admit only independently executed worker actuals:

```text
available[d]  = AND(w=1..16, worker_actual[w,d].available)
before[d]     = 0
delta[d]      = SUM(w=1..16, worker_actual[w,d].delta), iff available[d]
after[d]      = before[d] + delta[d], iff available[d]
known_zero[d] = available[d] && delta[d] == 0
```

Read-only recomputation over the sixteen retained worker records reproduced:

| Dimension | before | delta | after | available | known_zero |
|---|---:|---:|---:|---|---|
| callbacks | 0 | 0 | 0 | true | true |
| diagnostics | 0 | 0 | 0 | true | true |
| external_bytes | 0 | 608 | 608 | true | false |
| leases | 0 | 0 | 0 | true | true |
| reservations | 0 | 0 | 0 | true | true |
| transport_bytes | 0 | 960 | 960 | true | false |
| transport_messages | 0 | 16 | 16 | true | false |

These retained expected worker values are only a consistency cross-check. A later RED must use actual worker results, propagate unavailability without zero substitution, and reject any production root aggregate storage/view even if its values happen to match.

Disposition: **PASS** for the exact 35 occurrences, formulas over 16 worker actuals, unavailable propagation, and strict no-production-storage rule.

## Set accounting, complements, and preserved contracts

The ownership partition is occurrence-keyed and disjoint:

```text
ordinary root/worker/execution/pair-member:
A=116, B=20, C=9, total=145

World-24 root only:
A=116-35=81, B=20+35=55, C=9, total=145
```

The corpus denominator remains exactly 85 occurrences:

```text
25 roots + 16 World-24 workers + 22 World-25 executions + 22 World-25 pair members = 85
```

The amendment preserves the exact 145-path expected schema and all exact 17 removed/deferred paths. It preserves driver-to-expected equality for all 12 commitment paths while assigning those bytes no policy semantics. It also preserves the accepted World 07–11 raw-response correction, exact traversal/cardinalities, array order, and immutable v1/v2 bytes and dispositions.

The exact-set rules are coherent and non-overlapping:

- Worlds 08–11: exactly `7 A + 1 B` changed and 137 complementary occurrences;
- Worlds 17–22: exactly four changed scalar paths, with named mandatory rederivation neighbors;
- World 24 root: exactly 35 A-to-B ownership migrations and no scalar rewrite absent contrary actual recomputation;
- every other retained occurrence/path remains outside these finite correction sets.

No skip list, wildcard exemption, expected-fed actual, duplicate assertion, or path-name-only ownership classification is permitted.

Disposition: **PASS** for exact sets, complements, `116/20/9`, `81/55/9`, 145 paths, 85 occurrences, 17 absences, commitment equality, and preserved boundaries.

## Canonical rematerialization and RED rules

The amendment correctly limits scalar rematerialization eligibility to Worlds 08–11 and 17–22 plus causally changed manifest entries and aggregate fields. World 24 changes assertion ownership only unless independent actual execution disproves a listed value.

Required controls are complete: strict decoding; duplicate/unknown-key rejection; exact driver preservation; direct driver execution; exact-set and complement proof; canonical UTF-8 JSON; lexical object-member order; retained array order; one terminal LF; complete-world length/SHA regeneration; independently reproduced hashes; actual-only A acquisition; actual-derived B formulas; structural-only C checks; and immutable v1/v2.

The RED rules preserve all 85 real executions and prohibit world/name/C/expected dispatch, prototype or current-output operands, expected self-comparison, fabricated observations, fallback, skip lists, wildcard exemptions, record/update mode, and golden rewrite.

Disposition: **PASS**.

## Independent review conclusion

All criteria from the pending review gate and the superseded review have been checked. The sole prior blocker is corrected exactly, including the requested `7 A + 1 B` set and 137-occurrence complement. No new blocker was found.

This GO accepts the pending amendment as a correction specification only. It does not authorize corpus rematerialization, RED edits, implementation, activation, publication, or production use.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_SEMANTIC_RESIDUE_CORRECTION_GO
```
