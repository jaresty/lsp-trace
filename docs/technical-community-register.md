# Technical community register

The technical community register is a separate, non-authoritative derived artifact. It is never embedded in the canonical `program-c leiden` result and makes no CALLS, feature, semantic, ownership, service, or organizational claims. Every register has numeric `authority: 0` and `source_graph_complete: "UNKNOWN"`.

## Entry points

Generate Leiden output exactly as before and optionally write a one-partition register beside it:

```sh
lsp-trace program-c leiden --seed 19 --pagerank-top-k 20 --hub-top-k 20 \
  --format json --emit-community-register register.json graph-provenance-v5.json
```

Aggregate repeatable retained Leiden presentation artifacts against one admitted graph:

```sh
lsp-trace aggregate-communities --graph graph-provenance-v5.json \
  --partition leiden-seed-1.json --partition leiden-seed-2.json \
  --output register.json
```

`--output` on `aggregate-communities` writes the register; without it, canonical JSON is written to stdout. The command rejects non-V5 or semantically invalid Graph Provenance, malformed partitions, duplicate `partition_sha256` identities, foreign members, duplicate members, and incomplete node coverage.

## Identity and stability

A community identity is SHA-256 over a domain separator, the exact source Graph Provenance digest, and the canonical sorted member set. Occurrences retain the partition digest, the partition-local community ID, and the Leiden seed. Singletons and sparse/edgeless communities are retained.

Stability uses `EXACT_CANONICAL_MEMBER_SET_ACROSS_RETAINED_PARTITIONS/v1`. Per-community denominator is the number of other retained partitions. A member set present in every retained partition is `STABLE`; a set absent from at least one is `UNSTABLE`. With one partition, every status is `UNKNOWN`, denominator is zero, and reason is `INSUFFICIENT_COMPARABLE_PARTITIONS`; it is never represented as boolean `stable=true`.

Validate the output with:

```sh
lsp-trace validate --family technical-community-register --version v1 register.json
```
