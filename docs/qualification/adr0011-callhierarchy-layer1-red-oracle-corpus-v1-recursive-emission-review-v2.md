# ADR-0011 Call Hierarchy Layer-1 RED Oracle Corpus v1 — Recursive Emission Review v2

**Verdict: `ADR0011_CALLHIERARCHY_LAYER1_ORACLE_RECURSIVE_EMISSION_GO`**

## Scope and custody

This independent superseding review examined the exact worktree `/Users/schwa/dev/lsp-trace-adr0011-c17` at HEAD `fee5cbe0eb76c3eb15acf8bb1b06b095f69b2c96`. The worktree already contained the corrected uncommitted oracle design and unrelated pre-existing changes. This review did not edit or execute corpus files, runtime code, RED tests, generators, or implementation, and creates no commit. Its only write is this review.

The review performed a fresh in-memory parse of the four-table oracle design, independently applied recursive Table-3 substitution to a fixed point, serialized canonical JSON as UTF-8 with sorted keys, compact separators, and one terminal LF, and recomputed every Table-3 and Table-4 length and SHA-256 pin. No committed corpus bytes were treated as authority.

## Fresh expansion and pins

The fresh expansion reproduced all 63 Table-3 pins and all 88 Table-4 pins with zero length or SHA-256 mismatches. Exactly 25 `C_WORLD` roots were emitted.

| Artifact | Byte length | SHA-256 |
|---|---:|---|
| World 24 recursive bytes | 74,059 | `13350233b3dda56619d4f239585050f96973aff96facee9a5d3bac9576325b3d` |
| World 25 recursive bytes | 241,720 | `f5c571fc8e43064c7b7858f5c530773e35f2cef5851ffd4370461b69eadb0b21` |
| Ordered World 1–25 concatenation | 431,974 | `7fddcd923a9299c52014334d5460cb1a5ce78604e4a732faaef11d02adc30e84` |

The ordered aggregate was recomputed from the 25 freshly emitted root byte strings, not copied from the design.

## World 24

Fresh recursive expansion produced exactly 16 complete worker objects in stored order, `worker-201` through `worker-216`. Every worker has the complete worker-record key set; no worker ID or alias remains in the emitted tree.

The root `round_trip_ledger` is the exact field-wise sum of the 16 embedded worker ledgers. Every dimension has `before=0`, `after=delta`, `available=true`, and `known_zero=(delta=0)`:

| Dimension | before | summed delta | root delta | after |
|---|---:|---:|---:|---:|
| callbacks | 0 | 0 | 0 | 0 |
| diagnostics | 0 | 0 | 0 | 0 |
| external_bytes | 0 | 608 | 608 | 608 |
| leases | 0 | 0 | 0 | 0 |
| reservations | 0 | 0 | 0 | 0 |
| transport_bytes | 0 | 960 | 960 | 960 |
| transport_messages | 0 | 16 | 16 | 16 |

This corrects the superseded review's aggregate-ledger blocker.

## World 25

Fresh recursive expansion produced exactly 22 distinct complete execution objects in stored order, `execution-301` through `execution-322`, and exactly 11 ordered enabled/disabled pairs. Each matrix endpoint is a repeated complete literal object present in the execution list, never an execution ID. Every pair preserves exact external and transport equality while retaining the specified enabled/disabled observation distinction.

## Manifest

The embedded manifest template contains exactly 25 world entries and exactly 25 aggregate-order paths. The paths are the exact ascending set `worlds/world-01.json` through `worlds/world-25.json`; every path is slash-separated, relative, lexically normalized, unique, name-matched, and contained beneath the manifest directory under secure resolution. The world-entry set, aggregate-order set, and expected 25-path set are equal, so the template admits no missing or extra world. Every entry's name, byte length, and SHA-256 matches the fresh root bytes, and the aggregate length and SHA-256 match the fresh ordered concatenation.

The normative policy rejects absolute paths, empty or `.`/`..` segments, normalization-changing paths, symlink escapes, duplicate paths or names, path/name mismatch, missing listed files, and extra files under `worlds/`. This corrects the superseded review's self-contained-path blocker.

## Closure and S1–S5

- **Closure:** recursive expansion terminated with zero `$ref` keys, zero emitted Table-3 IDs, and zero unresolved IDs. The literal provenance values `A_SCHEMA` and `A_HEAD` remain exactly the specified non-dereferenced provenance strings.
- **S1:** World 3 preserves exact JSON `null`; Worlds 4 and 5 preserve distinct nonempty incoming/outgoing results.
- **S2:** Worlds 6–11 preserve six distinct accepted request bodies and their coherent error outcomes.
- **S3:** Worlds 12–14 preserve unavailable, available-zero-length, and available-nonempty payload distinctions; World 15 preserves `no_alias=true` and immutable retained evidence.
- **S4:** recursive substitution yields the required 16 workers and 22 executions with no unresolved ID or canned emitted reference.
- **S5:** Worlds 17–23 preserve refusal-only semantics with no presented error and available external observations.

All closure and S1–S5 checks pass.

## Disposition

The corrected recursive-emission design is internally complete and byte-exact under fresh independent expansion. This GO authorizes only the documented future corpus byte correction to the corrected World 24, World 25, aggregate, and manifest pins. It does not authorize runtime or implementation changes, broader corpus redesign, or a commit.

`ADR0011_CALLHIERARCHY_LAYER1_ORACLE_RECURSIVE_EMISSION_GO`
