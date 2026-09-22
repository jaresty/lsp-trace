# ADR0007 12-run prompt evaluation plan

This is an evaluation-only contract. It does not alter the production worker, response schema, package contract, or continuation behavior.

## Fixed matrix

Run in this exact order, once each, without retry:

1. A × real
2. A × final-01
3. A × final-02
4. A × final-03
5. B × real
6. B × final-01
7. B × final-02
8. B × final-03
9. C × real
10. C × final-01
11. C × final-02
12. C × final-03

Packets are the exact retained failing request plus records 1–3 from `docs/pilot/adr0007/experiment/final-four-packet/requests.jsonl`. The real packet exercises the minimal ENDPOINT/HandoffID shape; the final specimens exercise longer resolved-consumer shapes.

## Prompt variants

- **A — production-adapted:** Preserve current production semantic wording, removing only host-owned output fields and adapting the requested JSON shape to `schema.eval.json`.
- **B — concise schema-first:** State the exact model-owned keys and constraints first, then the semantic task.
- **C — schema-first plus one example:** Use B plus one compact valid example whose prose is generic and whose citations use only C1/C2.

## Fixed controls

- Model SHA-256: `1664fccab734674a50763490a8c6931b70e3f2f8ec10031b54806d30e5f956b6`.
- Runtime manifest SHA-256: `b95e8680b4d30761492bbc2d4a6fed656f124c5756dd4387cf02c30d27c13d90`.
- Runtime identity: `llama.cpp-v0.4.0-yzma-347c6ee0b893cc0bcac50ff7fb12ece0611fd01a`.
- Sandbox: pinned `/usr/bin/sandbox-exec` and the retained deny-network profile.
- Decode: grammar-constrained greedy sampling (deterministic; seed is inapplicable and recorded as `greedy/no-rng`).
- Timeout: 90 seconds per invocation.
- Output cap: 384 tokens.
- Context: 16,384 tokens.
- Exactly 12 process invocations; no retries.
- Raw stdout/stderr: `SESSION` retention, outside Git, mode `0600`.

## Predeclared lexicographic scoring

Compare variants in this order; later criteria break ties only:

1. Strict semantic schema parse count.
2. Citation validity and claim coverage count.
3. No invented consumer/identity count.
4. Substantive required-field count.
5. Deterministic canonicalizability by host count.
6. Lower aggregate latency, then lower aggregate generated tokens.

The model is not required to emit byte-canonical JSON. The host first performs strict parsing, rejecting duplicate keys, unknown fields, trailing bytes, non-JSON, and all schema violations. Only accepted semantic payloads are deterministically re-encoded.

## Boundary

Model output excludes request IDs, consumer identity or selector, nearest outward consumer, pins, authority, acceptance, completeness, provenance, status, and accounting. A future host seam may enrich a strictly parsed model payload with those host-owned fields. This run only proposes that additive seam after selecting a winner; it does not implement it.
