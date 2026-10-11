# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Causal-Input Contract

Status: **REVIEW_GATE_PENDING**

## Authority and immutable boundary

This additive design is authored against exact repository `/Users/schwa/dev/lsp-trace-adr0011-c17` at starting HEAD `8ff8bdf6`. It creates design authority only for these future paths:

```text
sessionruntime/testdata/callhierarchy-layer1-red-v3/**
sessionruntime/b4_callhierarchy_layer1_generic_v3_test.go
```

V1 and v2 are immutable historical evidence. This contract grants no corpus bytes, RED mutation, source or production edit, implementation, C15 assessment or classification, activation, commit, or production custody. The two pre-existing untracked blocked-v2 ownership documents remain unchanged.

The v3 registry at `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-causal-input-registry.json` is authoring provenance only. It is not a runtime side table and is not an authorized final-corpus dependency.

## Corrected ownership result

V3 retains the exact v2 expected denominator of 145 paths and the exact 17 v2 removals. It changes no generic expected value except the 12 opaque commitment paths, which are mechanically derived from v3 causal inputs rather than projected from v2 expected values. The other 133 generic expectations project unchanged, at the decoded-value level, from the accepted v2 corpus.

The exact, disjoint, exhaustive ownership partition is:

```text
A production generic owner/view fields                     116
B RED-derived assertions from actual executions             20
C fixture/control-only assertions and metadata                9
                                                          -----
exact retained expected paths                              145
removed/deferred paths remain absent                        17
```

This is the blocked v2 proposal's mechanically exact `117/20/8` partition with exactly one reclassification: `external.execution_expected` moves from A to C. Nothing moves into or out of B. No other A or C path changes category. Therefore:

- **A (116)** is exactly the 117-path A inventory in `adr0011-callhierarchy-layer1-generic-v2-oracle-assertion-ownership-amendment.md`, excluding only `external.execution_expected`.
- **B (20)** is exactly the 20-path B inventory in that document, unchanged.
- **C (9)** is exactly its eight-path C inventory plus `external.execution_expected`.

These set definitions are normative and mechanically decidable against the frozen v2 inventories; prose family names do not override them.

### A rule

A paths are actual finite generic observations retained by the private production owner and exposed by its view. The RED compares every A field to fixed expected data field-by-field. The 12 `commitments.*` A paths are populated only from the explicit v3 driver commitment inputs defined below. No A operand may come from `expected`, C metadata, occurrence ID, a world name, a pair label, or ambient lookup.

### B rule

B paths remain expected assertion results in the corpus but are independently computed by the RED from actual A observations and actual acquisition, view, callback, release, transport, and external execution. The formulas are exactly the frozen blocked-v2 B formulas. Expected-fed actuals, expected-to-expected comparison, expected self-comparison, and fabricated observations are forbidden.

### C rule

C paths are structural fixture assertions only. They are never production owner/view fields. The ninth C path, `external.execution_expected`, means only: “this fixture declares that the harness must conduct the external execution represented by this self-contained envelope.” It does not assert a Manager-produced fact, is not passed to production, and cannot select values within the request or transport script. The v3 RED structurally verifies that it is present with the exact fixed fixture value required by the v3 schema and that a real execution was attempted; actual execution outcomes are established only through A and independently computed B evidence.

## Exactly 85 occurrence-specific causal records

The registry contains exactly 85 records and no aliases:

| Structural occurrence class | Stable IDs | Count |
|---|---|---:|
| roots | `root/01` through `root/25` | 25 |
| World-24 workers | `w24-worker/01` through `w24-worker/16` | 16 |
| World-25 executions | `w25-execution/01` through `w25-execution/22` | 22 |
| World-25 pair members | `w25-pair/01/disabled`, `w25-pair/01/enabled` through pair 11 | 22 |
| total |  | **85** |

An occurrence ID identifies only structural location in the frozen cardinality/order. It is assigned before and independently of every expected value, commitment value, semantic name, world label, request ID, pair relationship result, or implementation observation. Renaming fixture `name` values cannot change occurrence IDs. Each final v3 envelope embeds its occurrence ID only as strict C authoring/audit metadata if the final schema explicitly admits it; it is never production dispatch input.

## Registry schema

The registry is one strict object with exactly:

- `schema`: `adr0011.callhierarchy.layer1.generic-v3-causal-input-registry/1`;
- `seed`: the fixed UTF-8 string `ADR0011-CALLHIERARCHY-LAYER1-GENERIC-V3-CAUSAL-INPUT-SEED/1`;
- `generation`: the exact rule identifier `SHA256-DOMAIN-LP-V1`;
- `records`: exactly 85 records in the order shown above.

Each record has exactly `occurrence_id`, `class`, `ordinal`, and `commitments`. `ordinal` is one-based within its class; pair-member records additionally have `pair_ordinal` and `member`. `commitments` has exactly `selected`, `handoff`, and `predecessors`. Selected and handoff are one commitment object each. `predecessors` is an ordered array with registry-fixed cardinality. Every commitment object has exactly:

```text
available : boolean
identity  : opaque ASCII string, or "" when unavailable
sha256    : 64 lowercase hexadecimal characters, or 64 zeroes when unavailable
length    : non-negative integer, zero when unavailable
```

The checked-in registry fixes availability and predecessor cardinality for every occurrence without reading v1/v2 expected values. V3 uses the following deliberately simple independent schedule:

- selected is unavailable exactly when global execution ordinal is divisible by 17;
- handoff is unavailable exactly when global execution ordinal is divisible by 19;
- predecessor cardinality is `global_ordinal mod 3`, yielding 0, 1, or 2 ordered predecessors;
- predecessor index `i` (one-based) is unavailable exactly when `(global_ordinal + i) mod 13 == 0`.

`global_ordinal` is the one-based position in the registry's fixed 85-record order. These availability and cardinality rules are authoring rules, not runtime computations; their results are materialized explicitly in every record and strict validation uses the materialized registry values.

## Domain-separated deterministic input generation

The registry is reproducible without reading any v1/v2 expected field or implementation output. For fixed seed `S`, occurrence ID `O`, role `R` (`selected`, `handoff`, or `predecessor`), and one-based role index `I` (`1` for selected/handoff), define:

1. `LP(x) = uint32_be(len(UTF8(x))) || UTF8(x)`.
2. `D = UTF8("ADR0011-CH-L1-GENERIC-V3/CAUSAL/1") || LP(S) || LP(O) || LP(R) || uint32_be(I)`.
3. `length = 16 + (SHA256(UTF8("ADR0011-CH-L1-GENERIC-V3/LENGTH/1") || D)[0] mod 33)`, hence 16 through 48 bytes.
4. `material = SHA256(UTF8("ADR0011-CH-L1-GENERIC-V3/MATERIAL/1") || D || uint32_be(block_index))` concatenated for block indices starting at zero and truncated to `length` bytes.
5. `sha256 = lowercase_hex(SHA256(material))`.
6. `identity = "v3-" || R || "-" || lowercase_hex(SHA256(UTF8("ADR0011-CH-L1-GENERIC-V3/IDENTITY/1") || D))[0:24]`.

For an unavailable commitment, the registry materializes `available=false`, empty identity, 64 zero digest characters, and zero length; material generation is not used. For an available commitment, all four materialized metadata fields must equal the rule above.

`material` is an opaque test input byte string, not a semantic preimage. Raw material bytes are not stored in the registry or expected subtree and are not interpreted. An authoring tool may regenerate them from the frozen rule solely to construct the v3 driver input and verify metadata. The digest commits only to those opaque generated bytes; it does **not** certify semantic content, call-hierarchy meaning, source completeness, authenticity, producer identity, or production authority.

## V3 envelope and driver schema

Every one of the 85 final v3 corpus envelopes is self-contained and has exactly two top-level siblings: `driver` and `expected`. There are no registry references, occurrence-ID lookups, side-table keys, inheritance rules, or ambient resolvers in final corpus execution.

V3 `driver` retains the complete strict v2 request, transport, and derived-external schema unchanged and adds one required sibling:

```text
driver.commitment_inputs.selected
driver.commitment_inputs.handoff
driver.commitment_inputs.predecessors[*]
```

Each commitment input contains exactly the four metadata fields above plus `bytes_hex`: JSON null when unavailable, otherwise lowercase prefix-free even-length hexadecimal encoding of the exact generated opaque material bytes. `bytes_hex` length must equal `2 * length`, and its SHA-256 must equal `sha256`. Predecessor order and cardinality are literal. Unknown/duplicate keys and every alternative byte encoding are rejected.

The complete driver therefore has exact top-level keys in lexical order:

```text
commitment_inputs
derived
request
transport
```

The v2 `driver.request` and `driver.transport.events` fields remain exact causal execution inputs. V3 adds no semantic request/body/result parser and no runtime world dispatch.

## Mechanical v3 expected derivation

For each occurrence, derivation is ordered:

1. validate the corresponding registry record and independently regenerate each available commitment's opaque bytes and metadata from seed, occurrence ID, role, and index;
2. copy the complete accepted v2 `driver.request`, `driver.transport`, and `driver.derived` decoded values for the same structural occurrence without consulting v2 expected values for causal behavior;
3. embed the complete commitment inputs, including opaque `bytes_hex`, in `driver.commitment_inputs`;
4. project the 133 non-commitment generic expected paths unchanged from accepted v2;
5. derive all 12 commitment expectation schema paths mechanically from `driver.commitment_inputs`: copy availability, identity, digest, length, predecessor cardinality, and predecessor order, but never copy `bytes_hex` into expected;
6. strict-validate the revised exact `A=116`, `B=20`, `C=9` partition and all 17 absences; and
7. canonicalize and hash the complete self-contained envelope.

This authoring projection is the only permitted driver-to-expected derivation. Runtime RED execution must not derive an actual from expected. The harness constructs the request attachment directly from `driver.commitment_inputs`, and production receives only the resulting request attachment.

## Canonical bytes and hashes

Registry and future corpus JSON use canonical UTF-8 JSON: duplicate keys rejected; recursively ascending lexical object-member order by Unicode member-name strings; arrays retain specified order; integers use shortest decimal form; strings use minimal required JSON escaping; no insignificant whitespace; and exactly one terminal LF. SHA-256 is over the complete canonical file bytes including that LF and is rendered as 64 lowercase hexadecimal characters.

The registry has no self-digest field. A review must independently reproduce and publish its exact byte length and SHA-256. The future v3 manifest must list all 25 world paths in lexical order, exact per-file lengths and SHA-256 values over complete canonical envelope files, and an aggregate SHA-256 over their ordered complete bytes. Hash acceptance cannot replace strict schema, derivation, or semantic-boundary validation.

## Required v3 RED behavior

A later separately authorized `sessionruntime/b4_callhierarchy_layer1_generic_v3_test.go` must:

1. strict-load and canonical-byte/hash validate every self-contained envelope;
2. perform all **85 real executions**: 25 roots, 16 W24 workers, 22 W25 executions, and 22 W25 pair members;
3. build each request solely from that envelope's driver, including its explicit commitment inputs;
4. compare all 116 A paths field-by-field against actual production owner/view observations;
5. independently compute all 20 B paths from actual executions and compare only the computed results to expected;
6. validate all nine C paths structurally only, including fixture-only `external.execution_expected`;
7. preserve W24 aggregate arithmetic and W25 pair-equality checks over independently executed actuals;
8. preserve causal negative, defensive-copy, lifecycle, panic-containment, drain, release-winner, external-transparency, canonicality, and no-world-dispatch checks; and
9. emit exact per-record and global counters with no skips, wildcard exclusions, fallback, record/update mode, or golden rewrite.

The RED must not feed expected commitments into production inputs, compare expected to itself, fabricate an actual observation, treat successful decoding as execution, reconstruct commitment inputs from expected, consult the authoring registry at runtime, or dispatch by occurrence/world/record/pair name. A missing A field is implementation RED; a failed B formula is assertion RED; invalid C structure is fixture RED.

## Review gate

An independent review must verify all of the following before any v3 corpus or RED mutation:

- only these two design files were created and v1/v2/test/source bytes were unchanged;
- the registry contains exactly 85 unique structural occurrence IDs with class counts `25/16/22/22`;
- IDs, availability, and predecessor cardinality are independent of expected values;
- every available commitment exactly reproduces identity, length, opaque bytes, and digest under the fixed domain-separated rule;
- unavailable normalization and predecessor order are exact;
- the corrected partition is exactly `116/20/9`, union 145, with only `external.execution_expected` reclassified from the blocked proposal;
- all 12 commitment expected paths derive only from explicit driver commitment inputs and the other 133 paths project unchanged from accepted v2;
- every future envelope is self-contained and has no registry reference;
- canonical lengths and SHA-256 values are independently reproduced;
- all 85 executions are real and A/B/C assertion ownership is enforced without expected-fed actuals, self-comparison, or fabrication; and
- no implementation, C15, source, corpus, RED, activation, or commit authority is inferred from this pending design.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_CAUSAL_INPUT_CONTRACT_REVIEW_GO — NOT ISSUED
STATUS — REVIEW_GATE_PENDING
```
