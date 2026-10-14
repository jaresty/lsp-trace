# ADR0011 CALLS Go/gopls workload study v1 — private evidence contract

Status: **CONTRACT AUTHORED; FULL 5900 WORKLOAD NOT RUN**.

This private package defines an evidence-only deterministic runner for the ADR0011 Go/gopls CALLS workload. It is confined to this directory and does not change public CLI, MCP schema, public source, profileGO, production admission, or composition surfaces.

## Presumed / Revised-if / Depends-on ledger

| Claim | Status | Revision trigger |
|---|---|---|
| The workload corpus is commit `7f09e5357b117d20449fc16097631e32d4eead7f`, source revision `4b910fc0c517cb7801fb51d17d295897230d11fa`, source tree `761474bf5d90bbe3468df62b1a68a1fa26dc2c82`, manifest `316afe2501aebe4195894d1334be566ae49eb32c88276ebee7b2e4d86062f44d`, 5900 ordered targets. | Presumed from predecessor qualification artifacts and current correction. | Revise if independent review supplies different exact bytes. |
| LSP positions derive from verified source bytes and the target symbol occurrence within the target byte range, not TSV line/char shortcuts. | Revised. | Revise if target corpus contract defines another canonical mapping. |
| First target range starts at byte `1117` (`{line:35, character:0}`) and the identifier `DecodeManifest` begins at byte `1122`, mapping to zero-based LSP `{line:35, character:5}`. | Revised and unit-tested. | Revise if source bytes or target identity change. |
| Incoming and outgoing ledgers are independent authority-0 streams. | Presumed by contract. | Revise only by a separate accepted bidirectional authority model. |
| Full workload execution remains forbidden here. | Depends on user scope. | Revise only by separate review/go outside this package. |

## Corrected blockers

1. Byte offset to LSP UTF-16 mapping is source-byte verified.
2. Prepared selection requires URI, exact symbol, and target-position containment in `selectionRange` or `range`.
3. Ledger verification rejects any event after a final ordinal disposition and enforces strict attempt ordering.
4. LSP reads enforce initialize/request timeouts and `max_raw_response_bytes`, preserving partial timeout/over-limit bytes.
5. Corpus identity uses full exact commit/revision/tree/manifest values; no prefix fields remain.
6. didOpen is cached per URI/version; duplicate opens are suppressed and didClose is emitted deterministically on shutdown.

## Full workload mode authored but not run

The runner now exposes explicit full workload controls:

- `--dry-full-plan --direction incoming|outgoing` validates the exact 5900-target denominator, writes 118 deterministic 50-target batch manifests and checkpoints, and performs no acquisition.
- `--run-full --direction incoming|outgoing` is the explicit acquisition mode. It is implemented but was not executed in this authoring pass.
- Full mode uses the exact corpus and does not use the smoke hard limit; smoke remains bounded by `smoke_max_targets_hard: 5`.
- Resume uses verified ledgers/checkpoints; open `STARTED` attempts are appended as `ABANDONED` before retry, and any append after a final ordinal disposition is rejected.

Semantic invariants are enforced in verifier code independently of schema/checksums: `max_raw_response_bytes=33554432`, `request_timeout_seconds=30`, `initialize_timeout_seconds=60`, `smoke_max_targets_hard=5`, `batch_size=50`, `target_count=5900`, and `batch_count=118`.

## Allowed execution

Contract check:

```bash
python3 docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/adr0011_workload_runner.py --contract-check
```

Bounded smoke only:

```bash
python3 docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/adr0011_workload_runner.py \
  --smoke \
  --workspace /path/to/private/source/workspace \
  --targets docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/fixtures/targets.smoke.tsv \
  --direction both \
  --max-targets 1 \
  --run-dir docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/runs/smoke-authoring
```

Dry full plan validation without acquisition:

```bash
python3 docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/adr0011_workload_runner.py \
  --dry-full-plan \
  --direction incoming \
  --run-dir docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/runs/full-plan-incoming
```

Expected stable first-target smoke: prepare succeeds; incoming result count 2; outgoing result count 1. If gopls/environment is unstable, record the actual authority-0 failure disposition without treating it as qualification.
