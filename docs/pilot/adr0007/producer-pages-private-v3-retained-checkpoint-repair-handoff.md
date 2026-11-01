# Retained checkpoint narrow HIGH + MED repair — pending parent adoption

Base/final HEAD remains `11e863fbf12f05803b924e8a93aafd60eda7aa69`. No commit, staging, push, delegation, live provider, model, actual Census, backend/controller/SDK, configuration/profile/mailbox change or runtime activation occurred. This repair supplements the historical `producer-pages-private-v3-retained-checkpoint-handoff.md`; its prior suite counts and original capsule are not represented as new runs.

## HIGH: verified current ledger before any retained fresh-source acquisition

New `OpenValidatedCurrentLedgerPrivateV3` returns the distinct `ValidatedCurrentLedgerPrivateV3` capability, with an unexported seal bound to the actual ledger state. Both `FreezeRetainedCheckpointSourcePrivateV3` and `AdmitRetainedPageCheckpointPrivateV3` now require it. A plain terminal-only `PageLedgerPrivateV3` cannot authorize retained source Freeze. Original `OpenPageLedgerPrivateV3`, original-family production files, defaults and allowance policy remain unchanged.

The caller supplies an explicit mutable bootstrap `PageCostPrivateV3` budget. Before opening/reading the journal, overflow-safe arithmetic admits and debits a conservative full-frontier verification cost, including root/metadata allowance. Zero/insufficient bootstrap allowance pauses without a capability or source work. Failed bootstrap verification consumes that caller budget but never appends to the unchecked journal. This is explicit bootstrap permission/accounting, not whole-application accounting or a durable retry-total guarantee.

Under the actual terminal record's exclusive flock, the opener checks immediate-next absence and streams records 0..actual generation one bounded record at a time. It verifies canonical JSON, schema/application/generation/allowance, file mode/link identity, previous hash, minimum complete ledger overhead, overflow-safe cumulative sums and the exact caller-supplied actual terminal digest. Only after success does it preserve the verified cumulative spend and append a create-only/fsynced verification reservation under the same frontier lock. That reservation precedes capability escape. Shared-state reservations then extend this chain through the existing mutex/flock/O_EXCL protocol; competing/stale frontiers refuse rather than resetting spend. Admission retains its additional full-chain verification and preserves newer uncheckpointed charges without interpreting them as data progress.

Fresh Freeze retains metadata and source-policy-max reservations BEFORE scope validation/read/hash, including changed/grown source failures. A private read-ops observer proves refused/unsealed capabilities never acquire source; it cannot create an owner. Tests damage the journal BEFORE opening/fresh Freeze with the actual latest terminal intact, unlike the older after-fixture-Freeze negatives.

## MED: preparation field graph admitted before serialization

The retained preparation constructor admits the concrete report/source field graph before serializing either its source-input digest or metadata header. Its detached helper checks context, empty bodies, assessment length, a conservative 8192-byte fixed-field envelope, 256 bytes of final-report slack, and six encoded bytes per byte of every actual string leaf. The graph includes all original source strings, the four selected-object strings, and both new checkpoint/source-family strings. Subtractive checks avoid overflow. This is a concrete-type bound, not reflection or a generic encoder memory claim; future fields require reviewing this bound.

Only after admission does the original `json.Marshal` execute. Valid header bytes remain exact, including nil lists, escaped controls/quotes/HTML characters, UTF-8 and U+2028/U+2029. The existing bounded canonical assessment serializer is unchanged. No encoder/writer is claimed to avoid an internal full allocation. Detached marshal-observer tests do not manufacture any owner; the genuine controlled owned pipeline exercises public minimum-cap refusal followed by successful normal preparation.

## Attributable local evidence

Private repair capsule: `/Users/schwa/.local/state/lsp-trace-evidence/retained-checkpoint-repair-oMZjPx44` (0700; evidence files 0600). `status.before`, `tracked.before.patch` and `pending.before.sha256` precede mutation. Original capsule `/Users/schwa/.local/state/lsp-trace-evidence/retained-checkpoint-v3-XeHrM91I` remains untouched. Go children used umask 022; no environment dump was taken.

- HIGH compiling behavioral RED: `ASSERT_NO_FRESH_SOURCE_ON_DAMAGED_CURRENT_CHAIN`; terminal-only public open followed by public fresh Freeze succeeded despite a missing middle. The first guard attempt had a noncanonical temporary source root and did not establish RED; it was corrected before this recorded failure. GREEN now covers missing/partial/noncanonical/previous-hash/cumulative/record-zero damage, no unchecked-journal append, no source observer calls, 0/1/N frontiers, zero bootstrap, cancellation, unsealed plain ledger refusal, fresh valid Freeze and stale actual tip refusal.
- MED compiling behavioral RED: `ASSERT_HEADER_REFUSED_BEFORE_MARSHAL: calls=1`, including oversized assessment, escaped long checkpoint/source-family/provider fields and cancellation. GREEN has zero observer calls for refusals and exact bytes for valid output. Long-field caps were subsequently strengthened above fixed overhead.
- Full new retained-family normal: **39 passing / 2 skipped test/subtest events**, zero failures, **216.911 s**.
- Full new retained-family race: **39 passing / 2 skipped events**, zero failures, **373.952 s**. Controlled-child entry tests skip outside their subprocess wrappers.
- Direct consumers (`privatedescribeprep`, `retainedpartialprogramc`, `publication`), uncached normal: **716 passing**, zero skipped/failing events.
- Focused ledger/header races: **18 passing**, zero skipped/failing events.
- Original-family ledger/checkpoint normal regression: **7 passing**, zero skipped/failing events.
- After those broad runs, only the pipeline test gained the public owned minimum-header-cap refusal assertion; no production code changed. That strengthened 20-own-body two-process pipeline passed normal (**167.366 s**) and race (**276.045 s**) again. It preserves duplicate occurrences, forward progress, cumulative no-refund, exact paired publication and prior bytes after a later charged source refusal.
- Vet of the four affected packages, gofmt and `git diff --check`: pass. Parent's prior 1692/2334 broad results are historical; they were not rerun here. No real/live/read-only opt-in family was selected or activated.

Final strengthened pipeline synthetic grant remains 1 TiB logical/read-hash. Cumulative conservative reservation: **325,850,489,938 logical / 347,213,889,984 read-hash bytes**, **79,839,086 write bytes**, **2,336,640,900 work units**. These are policy reservations, not measured heap/throughput, practical scalability, whole-application staging or inference/acquisition totals. The 16 MiB unit cap is unchanged.

Only the two named behavioral defects above have claimed RED/GREEN witnesses. Other checks are GREEN evidence, not exhaustive mutant/adequacy certification. Parent retest and independent review remain required before adoption.

## Unchanged custody and limits

`REOPENED_RETAINED_OBSERVATIONS_NOT_ORIGINAL_CAPTURE_OWNERSHIP`; source UNKNOWN; feature UNRESOLVED; authority 0; accepted false; absence PROHIBITED. No Manager/guard/capture/V2 qualified owner is restored. Source Freeze is genuinely fresh and separately admitted. Hashes bind bytes, not history/authentication/permission. Same-user malicious whole-journal rollback remains outside this private contract; ordinary missing/torn middle records fail closed. Borrowed roots/source remain nonconcurrent/non-atomic. Original host composition, application-total staging, acquisition/inference accounting and mid-stage crash recovery remain unfinished and outside this repair.

Protected dirty documents remain exact:

- `private-variable-v2-host-handoff.md`: `13e0b3f37d7eade816a266c2eed2fad6ef50af7cdbd6933561c9d4f2e3d325f9`
- `variable-scope-private-v2-status.md`: `350834897cdec7f053ae7e03791e01e38d0cca8a4016defc4486cc6e00eca128`
