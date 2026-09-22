# ADR0007 second 12-call Response V2 prompt evaluation plan

Evaluation-only: no production worker, schema selection, package contract, or continuation behavior changes.

## Fixed matrix

Exactly once each, no retries: D × real/final-01/final-02/final-03; E × the same four; F × the same four. Packets are the retained failing request plus records 1–3 from `docs/pilot/adr0007/experiment/final-four-packet/requests.jsonl`.

## Variants

- D: concise schema-first, short field meanings, explicit packet-specific instruction.
- E: task-first then exact JSON schema; explicitly “derive each value from this packet, never copy generic wording”.
- F: internally decompose role/need/behavior/boundary/limits, then emit JSON only with no chain-of-thought.

No prose JSON example is supplied.

## Fixed controls

- Model SHA-256: `1664fccab734674a50763490a8c6931b70e3f2f8ec10031b54806d30e5f956b6`.
- Runtime manifest SHA-256: `b95e8680b4d30761492bbc2d4a6fed656f124c5756dd4387cf02c30d27c13d90`.
- Runtime: `llama.cpp-v0.4.0-yzma-347c6ee0b893cc0bcac50ff7fb12ece0611fd01a`.
- Network denied by retained sandbox profile; grammar-constrained greedy/no-rng decode.
- Timeout 90 seconds, output cap 384 tokens, context 16,384 tokens.
- Exactly 12 process invocations, zero retries.
- Raw stdout/stderr and prompts are SESSION-local outside Git, mode `0600`.

## Predeclared pragmatic lexicographic score

1. strict V2 parse and deterministic host canonicalizability;
2. substantive packet-specific required fields;
3. no invented mechanical identity, consumer, or purpose;
4. correct unresolved handling;
5. semantic consistency with the packet;
6. optional citation-suggestion validity (non-scoring bonus after required semantic criteria);
7. lower latency, then fewer tokens.

Human-safe semantic adjudication may score only obvious packet mismatch or invention. Host admissible evidence handles grounding. Authority is `0`, accepted is `false`, completeness is `UNKNOWN`.

A winner is selected only if it reaches at least 3/4 strict and substantive outputs and 4/4 no mechanical invention. Otherwise report no winner and the smallest next prompt change. Production remains unswitched.
