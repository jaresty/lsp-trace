# ADR 0011 incoming Go/gopls acquisition tranche 1

Profile: `CALLS_GO_GOPLS_INCOMING_EXACT_V1`.

This directory contains only the first incoming acquisition tranche: repository/workspace/document/session custody, strict managed-gopls and toolchain gaps, a direct-position prepare bounded record, a target-bound selection receipt, and one incoming bounded result. Acquisition stopped after the incoming raw-result boundary; no normalization, admission, Stage 3, implementation, test/vector/review edit, commit, or GO occurred.

## Strict evidence boundary

- Exact JSON-RPC prepare and incoming frames were not available from the managed operation. The bounded records say so explicitly and do not claim wire identity.
- The composed managed result preserves projected callers and `fromRanges`, but does not establish raw response order, duplicates, `null` versus array form, complete `data`, or full prepared-item identity.
- No `textDocument/documentSymbol` request was made; target selection was direct-position only.
- Session status identifies profile `gopls` but does not expose managed gopls executable/argv/environment. Shell gopls and Go inventories are retained only as non-evidence contrasts and cannot satisfy managed-profile roles.
- Capability remains `MISSING` because initialize request/result custody is absent.
- `authority=0`, `accepted=false`, and `production_admission=REFUSED_MISSING_EVIDENCE` remain unchanged.

## semantic-vector boundary

semantic-vector was not executed and was not used for capture, occurrence derivation, prepared-item selection, normalization, multiplicity, manifest population, or qualification. Future adoption, if separately authorized, must follow this sequence: freeze evidence → independently author expected vectors → independent byte review → handwritten checks → semantic-vector secondary runner → compare. Any disagreement blocks. A later secondary runner may test canonical worlds, mutation, output-failure effects, contrast, provenance, and ceilings; it may never determine correctness of gopls evidence, rules, or policy.

## Gate

Acquisition gate remains pending. No GO is issued.
