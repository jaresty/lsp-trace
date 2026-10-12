# ADR-0011 Call Hierarchy Layer-1 RED Oracle Corpus v1 — Recursive Emission Review

**Verdict: `ADR0011_CALLHIERARCHY_LAYER1_ORACLE_RECURSIVE_EMISSION_BLOCKED`**

## Scope and boundary

Independently reviewed exact worktree `/Users/schwa/dev/lsp-trace-adr0011-c17` at HEAD `fee5cbe0eb76c3eb15acf8bb1b06b095f69b2c96`, including the pre-existing uncommitted design amendment, committed flawed corpus, byte review v2, and unchanged RED. The review was read-only except for this report. It did not edit or execute the corpus, runtime, RED, tests, generators, or implementation, and it creates no commit.

A fresh disposable validator parsed the amended four-table design registry, applied canonical UTF-8 JSON with sorted keys, compact separators, and one terminal LF, recursively replaced Table-3 IDs at the Table-4 boundary, and independently inspected the resulting literal trees.

## Recursive-emission checks

The recursive mapping itself is reproducible:

- World 24 expands to exactly 16 distinct complete worker objects in stored order, `worker-201` through `worker-216`.
- World 25 expands to exactly 22 distinct complete execution objects in stored order, `execution-301` through `execution-322`, plus exactly 11 ordered pairs whose `enabled` and `disabled` endpoints are repeated complete objects.
- World 25's 11 pairs all preserve exact external and transport equality while distinguishing enabled versus disabled observation availability.
- The expanded World 24 and World 25 trees contain zero `$ref` keys and zero `WORKER_*`, `EXEC_*`, or other unresolved registry-ID values. The deliberately literal provenance strings `A_SCHEMA` and `A_HEAD` remain exempt exactly as specified.
- Worlds 1–23 are byte-for-byte unchanged from the committed corpus.
- Only the committed World 24 and World 25 files remain the known flawed compact roots from byte review v2.

Independent canonical pins match the amendment exactly:

| Artifact | Byte length | SHA-256 |
|---|---:|---|
| World 24 recursive bytes | 74,053 | `a167132a3c6e06256ea23a10b5c0bbe2ff1d7f778131978619615865896c0c44` |
| World 25 recursive bytes | 241,720 | `f5c571fc8e43064c7b7858f5c530773e35f2cef5851ffd4370461b69eadb0b21` |
| Ordered World 1–25 concatenation | 431,968 | `c1e87c64c9322afb128bcf2cfb27a7f79421ff755ab1c8cbf6f7ef85285c1703` |

## Blocking defects

### 1. World 24 does not carry the required aggregate ledger

The 16 recursively embedded workers independently sum to these seven ledger deltas:

```text
callbacks=0
diagnostics=0
external_bytes=608
leases=0
reservations=0
transport_bytes=960
transport_messages=16
```

The amended World 24 instead preserves `CORE_24_ROUND_TRIP_LEDGER` unchanged with:

```text
callbacks=0
diagnostics=0
external_bytes=38
leases=0
reservations=0
transport_bytes=60
transport_messages=1
```

Those are one worker/request values, not the aggregate of the 16 embedded workers. This fails byte-review-v2 correction item 3, which requires World 24 aggregate/control material to be self-contained against the embedded workers. The amendment's instruction to retain the stored ledger without recomputation freezes the wrong aggregate semantics; its otherwise exact World 24 and corpus pins therefore pin a defective root.

Required correction: define the exact aggregate ledger/control value from the 16 workers, amend the registry and World 24 expected bytes, then recompute World 24 and aggregate length/SHA pins independently.

### 2. The manifest template does not support self-contained world paths

Every template world entry has exactly `byte_length`, `name`, and `sha256`. All 25 entries omit `path`, and the template supplies no normative name-to-path rule or path-base field. A corpus-directory-only consumer therefore cannot resolve the world files from the manifest alone. This regresses the committed manifest's explicit relative paths such as `worlds/world-24.json` and fails the requested self-contained-path verification.

Required correction: give every world an exact corpus-relative `path` (or define an equally exact, canonical, self-contained name-to-path mapping and base), then pin the canonical manifest bytes if manifest-byte identity is part of the correction.

## Closure and S1–S5

- Reference closure passes for the recursively expanded literal trees, subject to the two semantic/manifest blockers above: no undefined Table-3 reference, cycle, `$ref`, worker ID, execution ID, or unresolved registry ID remains.
- S1 passes: World 3 is exact JSON `null`; Worlds 4 and 5 retain distinct nonempty incoming/outgoing results and frames.
- S2 passes: Worlds 6–11 retain distinct accepted inputs and coherent protocol, malformed, cancellation, deadline, shutdown, and transport-unavailable outcomes.
- S3 passes: Worlds 12–14 retain unavailable/zero-length/nonempty payload distinctions, and World 15 retains immutable-copy/no-alias evidence.
- S4 passes for constructor partitioning and recursive substitution: validation-only fields remain excluded and no canned scenario output or ambient lookup was found. It does not cure the incorrect World 24 aggregate value.
- S5 passes: Worlds 17–23 retain refusal-only semantics with transparent external and transport observations.

## Disposition

The recursive substitution algorithm, W24/W25 complete-record shapes, W25 pair matrix, exact published recursive pins, 23 unchanged worlds, identifier closure, and S1–S5 are supported. Qualification is nevertheless blocked by the non-aggregate World 24 ledger and the path-incomplete manifest template.

No corpus byte correction is authorized.

`ADR0011_CALLHIERARCHY_LAYER1_ORACLE_RECURSIVE_EMISSION_BLOCKED`
