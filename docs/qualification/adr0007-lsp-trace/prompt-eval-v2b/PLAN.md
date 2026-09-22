# ADR0007 Response V2b corrected prompt campaign

Evaluation-only. Production defaults remain unchanged. The prior harness-blocked campaign remains separately recorded in `../prompt-eval-v2/`.

## Pinned preflight gate

Before campaign packets, the final v2b binary must initialize the exact embedded eval GBNF with the pinned Qwen model and Yzma/llama.cpp runtime under `(deny network*)`, then complete and strictly parse one bounded synthetic generation. The mode-0600 `preflight.green` receipt is mandatory; packet loading is fail-closed before GREEN.

## Fixed campaign

Campaign ID `v2b`: exactly D/E/F × real/final-01/final-02/final-03, 12 calls, zero retries. Greedy/no-rng, 90 seconds, 384 output tokens, 16,384 context tokens. Prompt wording and semantic schema match preserved V2; only GBNF rule-name syntax is corrected (`maybe-string`).

## Score and decision

Lexicographic order: strict V2 parse/canonicalization; substantive packet-specific required fields; no mechanical invention; unresolved handling; semantic consistency; optional citation validity bonus; latency then tokens. Host basis is mandatory. Winner threshold is at least 3/4 strict and substantive plus 4/4 no invention. No winner means no further campaign, no production proposal, and no production switch.

## Custody

Raw prompts, worker envelopes, stderr, preflight records, digests, lengths, and modes remain SESSION-local outside Git at mode `0600`. Git retains only sanitized `REPORT.md` and `summary.json`.
