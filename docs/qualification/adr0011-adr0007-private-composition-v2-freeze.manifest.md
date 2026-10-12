# ADR 0011 → ADR 0007 private composition v2 corrected documentation freeze manifest

- **Schema:** `lsp-trace.adr0011-adr0007-private-composition-documentation-freeze.v2`
- **Status:** locally consistent documentation candidate; not accepted; not GO
- **Source revision:** `4a0c527c447d6bd5c9316504ffeba904cece3314`
- **Artifact count:** 7 untracked files, all under `docs/qualification`
- **Implementation or runner/library dependency:** none
- **Public/internal surface effect:** none

## Frozen sources

| Mutable review path | Content-addressed copy | Bytes | SHA-256 |
|---|---|---:|---|
| `docs/qualification/adr0011-adr0007-private-composition-gap-ledger-v2.md` | `docs/qualification/adr0011-adr0007-private-composition-gap-ledger-v2.c82d411a2cee182d7b1475b361bef3076b4250205f5963b6e1c50ea67314b587.md` | 14493 | `c82d411a2cee182d7b1475b361bef3076b4250205f5963b6e1c50ea67314b587` |
| `docs/qualification/adr0011-adr0007-private-composition-contract-v2.md` | `docs/qualification/adr0011-adr0007-private-composition-contract-v2.13197bf16d5463545d5c0b00088f9c2e1aaed00ac11d183f406c63fa7b2ba340.md` | 15507 | `13197bf16d5463545d5c0b00088f9c2e1aaed00ac11d183f406c63fa7b2ba340` |
| `docs/qualification/adr0011-adr0007-private-composition-v2-literal-appendix.json` | `docs/qualification/adr0011-adr0007-private-composition-v2-literal-appendix.fb26ed5ee4e95f3f0857495626cd13eb12ed66919599f4c9148ae65e35ec4c84.json` | 189256 | `fb26ed5ee4e95f3f0857495626cd13eb12ed66919599f4c9148ae65e35ec4c84` |

## Current normative identities

```text
contract_identity=sha256:6d9777305530d08fbbf2e3fba34983556007107726799511a26473eb2f31a69c
positive_request=sha256:b746ddc857f35d78fdcfcad449da529660fe13bf16851a4d17f0cefedb6a73b8
positive_generation_key=sha256:a79dd568e631ffd59e6eadcfc3ef181b06f2f199bdc67ca8856daf5f417ed495
positive_final_generation=sha256:5b405b58bedcb87d72b7d82e14bc6a33db6bb2dc89136cb33202c2ef455c8dac
search_request=sha256:e2677c7ad1f90ff7c76e8e1c17bdff0584577d44bdb0bf02759f8a2444e8bedc
search_result=sha256:5273060c096421af1e3c342132dfaf47232ab74c2f293028d34f50dedc5f7c96
mutation_request=sha256:deb9cc3afa23960c075c33ae19550c5847df6a639e7872280761c53dc3c58f3a
mutation_refusal=sha256:a64d5b4a7841edf50e2dcc75cf7070498e3f045ecbd369c67597d855ec378b65
```

## Search correction receipt

Exact query bytes decode to `returns ok to caller\n`. The normative deterministic policy is: ASCII lowercase; split maximal `[a-z0-9]+` runs; discard empty tokens; deduplicate; sort lexicographically. Independent application yields:

```text
query_distinct_tokens=[caller,ok,returns,to]
corpus_distinct_tokens=[caller,literal,ok,returns,string,target,the,to]
intersection=[caller,ok,returns,to]
score=4/4
```

Search accounting remains `expected=evaluated=successful=terminal=1`, `failed=omitted=missing=0`. Search, Group, downstream stage requests/results, checkpoints, candidate, and final generation identities were recomputed transitively.

## Full consistency receipt

```text
PASS canonical=79 contract_closure=78 literal_query_tokens=caller,ok,returns,to score=4/4 search_accounting=balanced dependency_links=closed checkpoints=6 candidate_citation=closed final_generation=5b405b58 boundary=prose_only
appendix_bytes 189256 appendix_sha256 fb26ed5ee4e95f3f0857495626cd13eb12ed66919599f4c9148ae65e35ec4c84
```

The prior complete checks remain satisfied: 47 request roles/carriers; one contract identity; complete `CAUSAL_TEST_ONLY`, `production_admissible=false` Stage4 packet; ten distinct Stage7A role ranges; reconstructible six-stage semantics; Location+STS candidate citations; six accounting/checkpoint links; one semantic mutation; complete Stage7B/7C mutation reseals; precedence-4 refusal with zero effects.

## Normative and non-normative handoff boundary

Historical omitted-selector `CALLS_ONLY` behavior and canonical bytes remain unchanged. This v2 fixture is normatively `CAUSAL_TEST_ONLY`, non-production-admissible, and does not mint or recover historical production qualification.

Non-normative handoff only: future production composition requires a new additive versioned CALLS qualification/custody regime with method/provider custody, admitted occurrences, relation-family policy, grouping qualification, source/replay custody, instability, coupling, complete accounting, and the unchanged ceiling. This handoff is prose only; it is not a carrier, fixture semantic, selected design, implementation authorization, or GO.

## Qualification boundary

This manifest freezes documentation bytes only. It does not assert implementation, parser, runner, library, internal/public API, CLI, MCP, catalog, capability, schema registration, publication, release, or deployment. It does not modify or supersede ADR0011, ADR0007, frozen v1, the causal specification, or historical CALLS-only bytes. It grants no GO disposition.
