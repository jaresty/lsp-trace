# Minimal Describe pilot scope

- **Status:** `PILOT_SCOPE_DRAFT`
- **Enablement:** `DISABLED`
- **Owner:** `PROJECT_OWNER`
- **Governance:** local-project self-review exception accepted

## Scope

This pilot is limited to one operation: Describe one caller-assembled structural projection. It does not build an index, compute embeddings, search, nominate groups, crawl repositories, or expose a public CLI/MCP surface.

## Corpus and item type

- Corpus: revision-bound code and structural evidence.
- Item type: caller-assembled source projection containing one target definition and, when available, one mechanically selected immediate incoming `CALLS` predecessor.
- Acquisition mode: `TARGET`.
- Inputs: the four existing revision-bound projected packets used by the bounded relative-consumer experiment.
- Coverage: exactly four admitted target packets; no repository or source census claim.

## Processing contract

The caller must complete `Select → Resolve → Assemble` and provide immutable target/consumer definitions, admitted bytes or selectors, ranges, graph subjects, revision/generation identity, custody, policy, and digests. The worker may describe only the target's contribution to the mechanically supplied immediate consumer and must preserve uncertainty and limitations.

The worker must not discover consumers from relevance, add relationships, infer dynamic dispatch, claim product purpose, or promote the description into authority, acceptance, ownership, runtime use, completion, or feature identity.

## Required pilot outputs

For each admitted packet:

- one terminal Describe member outcome;
- bounded description or explicit abstention/failure;
- source and input identity references;
- model/runtime/prompt/policy digests;
- authority `0` and accepted `false`;
- coverage and limitation fields;
- immutable result and lineage identity.

## Evaluation

Compare the mechanical-consumer configuration against the rejected relevance-ranked consumer configuration and a non-semantic baseline. Evaluate:

- consumer-topology preservation;
- distinguishing target behavior;
- unsupported terminology;
- boundary overreach;
- abstention and failure accounting;
- latency and resource limits;
- exact replay under the frozen runtime tuple.

The four existing experiment artifacts are diagnostic evidence only. They are not qualification results and cannot substitute for frozen labels, thresholds, runtime pins, or conformance vectors.

## Enablement blockers

Execution remains disabled until the project owner freezes:

1. the exact NDJSON envelope and lifecycle schema;
2. the worker implementation and digest;
3. the exact model/backend/runtime tuple and supply-chain record;
4. the four assembled input records and their digests;
5. numeric evaluation thresholds and labels;
6. conformance-test results;
7. a local pilot enablement record binding all identities.

No implementation or model selection is implied by this scope document.
