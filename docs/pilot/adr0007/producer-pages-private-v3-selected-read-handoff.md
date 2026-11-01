# Private V3 selected-object continuation — PARTIAL, Part 1 still incomplete

Base HEAD: `3caa23f1e04a944867de1418a82e0e0a31ff6768`. No commit, push, host wiring, live Census, gopls, provider, model, SDK, backend/controller, or historical artifact mutation.

## Added contract

`internal/censuslineagepilot/producer_pages_selected_private_v3.go` adds:

- `ProducerPagesPrivateV3.ObjectReader(ctx, revision, object, exchange)`: new opaque original-owner-only selected-object cursor. Explicit shared-ledger metadata reservation precedes handle allocation. Invalid/foreign revision bindings and object bounds refuse; objects exceeding the existing caller computation-unit policy return `ErrPagesUnitPrivateV3`. No new allowance/default or cap waiver.
- `OriginalPageObjectReaderPrivateV3.Advance(ctx, generation, exchange)`: one monotonically increasing handle generation per attempted exchange. Assembly allocation/copy/final comparison reservations precede allocation. The existing original-byte/pair/source verifier advances its opaque saved cursor and copies only the selected object's exact page intervals. Pauses and cancellation preserve verified progress. No partial bytes escape. Final success transfers the bounded output buffer without a final clone; terminal failed/completed handles cannot replay. Copied internal state fails the self guard; JSON serialization refuses.
- Progress reports include verified page numerator/denominator, actual selected bytes assembled, original binding, status and the same cumulative application ledger spend. This is a selected byte reader, NOT a selected-batch admission or AST qualification.

Selected assembly is contiguous and bounded by the existing explicit computation-unit safety limit (at most 16 MiB). Oversized retained objects still require a separately implemented safe streaming/spool consumer or truthful refusal. This does not solve arbitrary application-total input. No metadata/root index or all-capture clone is built by this reader.

The existing conservative source-policy-max reservation and full source revalidation on each exchange remain. The reader avoids rereading verified page suffixes, NOT repeated source checks/revision checks. Reservations are logical accounting, not exact CPU/heap telemetry. No practical throughput or scalability result is claimed.

## Executed evidence

Fresh capsule: `/Users/schwa/.local/state/lsp-trace-evidence/producer-pages-v3-selected-Cgmva7Bl` (0700; files 0600).

- `go test ./internal/censuslineagepilot -run '^TestProducerPagesPrivateV3' -count=1 -timeout=300s -json`: 41 passing test/subtest events, zero failures, 68.526 s. Includes the prior 37 resource/MED events and four new tests.
- `go test ./internal/censuslineagepilot -run '^TestProducerPagesPrivateV3Selected' -race -count=1 -timeout=120s -json`: four passing tests, 5.251 s.
- `go vet ./internal/censuslineagepilot`, gofmt and `git diff --check`: pass.
- Child test commands used umask 022. Protected document SHA256 values remain `13e0b3f37d7eade816a266c2eed2fad6ef50af7cdbd6933561c9d4f2e3d325f9` and `350834897cdec7f053ae7e03791e01e38d0cca8a4016defc4486cc6e00eca128`.

Evaluated stub REDs: `ASSERT_SELECTED_FORWARD_PROGRESS` and `ASSERT_SELECTED_ZERO_BUDGET_PAUSE`; both reached GREEN. Other negative assertions have GREEN evidence only, not claimed RED witnesses. Initial double-close fixture-cleanup failure and too-early cancellation injection are retained; neither is an implementation RED. The cancellation injection was moved into the next page attempt, after the previous page's full distinct-object validation, without changing the required preserved-prefix assertion. The exact read count includes unsuccessful partial-page reads. Batching tooling failed before launching commands; all reported executions ran directly.

## Remaining acceptance — explicitly NOT delivered

This continuation did not finish the requested page evidence → typed computation → own-AST preparation slice. `OriginalPageBatchPrivateV3.Read` is unchanged and still reparses/reverifies the whole prefix. No new admitted selected batch, sibling Project/assessment/Preparation owner, SourceBinding profile, target-body preparation, prepared publication, or controlled full-20-target consumer test was added. Existing V2 APIs/default schemas/qualification and error families are untouched.

Cross-process resume remains unsupported. Only original-owner in-memory selected-object continuation was added; no checkpoint-derived owner or weaker observational admission was invented. The separately versioned checkpoint proof/admission design remains undone, not demonstrated impossible.

Broader census/consumer/CLI acceptance and full V3 race family were not rerun. No all-20 computation/AST acceptance, real fixture-vector equivalence, useful-body delivery, or product completion is claimed. Parts 2/3 remain unstarted.

Deferred and undone: typed selected admission/computation/AST/prepared publication; checkpoint admission; streaming objects above computation-unit policy; host composition; live pilot. Withdrawal: leave this private family unwired. No deferred choice was activated.
