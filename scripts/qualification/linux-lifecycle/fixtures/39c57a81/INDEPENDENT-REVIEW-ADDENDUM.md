# Independent paired-control review at 39c57a81

## Disposition: ACCEPT-PAIRED-CONTROL-FOR-HOSTED

No blocking source/harness defect found in the five exact bound variants below. The new paired rendezvous resolves the previously identified early-wait scheduling escape **for these source/test pairs**, subject to acquiring the required runtime evidence. The previous standalone early-wait overlay with the committed original test remains inconclusive and unqualified.

This is acceptance of the qualification artifacts for separately authorized hosted execution, not an observed RED/GREEN result. Ten cross-compilations are actual producer evidence; no variant was executed in this phase or this review. Existing Linux/amd64 normal lifecycle results are separate user-reported/native-candidate evidence, not evidence of these overlays, Linux/arm64, race, or whole-repository success. Funding/admission remain pending and untouched.

**Execution-route finding:** neither of the two permitted CI files supplies a safe existing external-overlay/pure-control execution route. A separately reviewed qualification-workflow proposal is required before hosted execution. No workflow, infrastructure, remote job, installation, source edit, Git mutation, notebook mutation or Go/native execution was created or performed here. This external addendum is the only output written by this reviewer.

## Scope and exact baseline

Repository `/Users/schwa/dev/lsp-trace-variable-v2-TSZxdY`; independently observed HEAD `39c57a81d067b8868710fc88eddafb6f13340e77`.

I read the complete producer REPORT, manifest, inputs, verification records, all five variant patches (candidate-original is empty), the actual complete replacement ordering-test bodies in both positive/negative files, and actual production observer/fence/snapshot bodies in the three mutants. I also inspected the unchanged snapshot-test body used by old-snapshot. The already-reviewed complete production/test baseline was reused only after verifying its current committed bytes are identical to the prior review. Independently reconstructed every complete variant patch from full baseline/replacement bytes, verified both complete harness prefix/suffix boundaries, and parsed/verified all five absolute overlays. Thus no unseen change outside the inspected replacement regions is adopted from a producer summary. No unrelated lifecycle re-audit was performed.

All five lifecycle source files are byte-identical to committed HEAD and the previously reviewed candidate:

| File under internal/managedprocess | HEAD/current SHA-256 |
|---|---|
| process.go | 2f6c15af994f1986e1de29a18f155d9ecf35a193835feea8186bb58e46554c13 |
| private_provider_window.go | c99173a1e9d497792bccd1c81d05353fbb3bec2161b52b118a6f6c8280dfa4d3 |
| supervision_linux.go | fc6c96db10598968096bdcbc70c693ee74d8db10fa92d32ce3c099ae49f4352d |
| process_linux_supervision_test.go | baa697699f5076106551c658039de6182e4acb0ad1f1d9f90169ade0d3cdd101 |
| supervision_linux_unsupported.go | fc55bb1b4f0ca6dec503afc85472d1ab9b3eb02b88b756b22c2d6a724a46aff7 |

Independent metadata check: tracked changes 0; index changes 0; untracked set exactly the 40 prior capsule names. Those paths were excluded by name, with no capsule content opened or hashed. Hash verification was confined to named sources/artifacts and the two requested CI files, not a repository sweep.

## Findings and causal review

### 1. Rendezvous reaches an actual operation, not a manufactured trace outcome

`positive_test.go` and `negative_test.go:435-538` replace only `TestOwnedLinuxFinalSignalReturnFence`. Their sole difference is the local `requireEarlyWaitWitness` constant at line 438. Imports, globals, native fixtures, other tests, and the existing trace assertions are byte-identical to HEAD.

The event callback records gate entry and releases `trace.mu`, closes `atGate`, then blocks until the injected final signal closes `entered` or cleanup closes `abort` (lines 480-492). The controller starts the fence only after observing `atGate`. The injected signal closes `entered` inside its actual function and then blocks on `release` (472-478). The callback therefore cannot return before actual signal entry during a successful rendezvous.

The injected consuming-wait function increments an atomic counter and closes `actualWaitEntered` at its actual function entry (466-471). The event is not emitted by a variant name, expected-result field, gate marker, or test controller. Both production mutants retain the actual call site `s.waitChild()`.

For early-wait, the source places that call before receiving `s.fenced` (`early-wait.go:198-217`). For early-fence, the original wait receive remains, but the source closes the channel before executing the held signal (`early-fence.go:248-266`). Both can reach the actual injected wait while signal return is withheld. Correct production cannot do so because its receive remains before the wait and its fence closes after signal return.

### 2. The old scheduling escape is eliminated for the negative pairs

The negative harness waits for `actualWaitEntered` before releasing the signal (512-526). The successfully acquired chain is:

`signal function entered -> gate callback returns -> wait-entry marker -> actual wait call/counter -> actualWaitEntered received -> held-state assertion -> release -> signal return -> final-return marker`.

For early-wait, fence publication follows final return, so actual wait entry necessarily preceded fence publication. The former escape—deschedule the reaper after the gate marker until the controller finishes fencing—is unavailable: that controller now withholds return until actual wait entry. A stalled reaper produces `CONTROL_RENDEZVOUS_UNAVAILABLE`, not an apparently safe ordered pass.

For early-fence, fence publication precedes signal entry/return in the mutated source. The rendezvous additionally requires actual wait entry during the held signal interval. The late final-return marker cannot conceal that earlier publication.

Wait-return itself may occur before or after final signal return. The qualification does not require an unsupported total ordering of every event; the required wait-entry/fence/final-return relationships are sufficient.

### 3. Contrasting test branches do not hard-code the assertion verdict

The good branch does **not** await actual wait entry while holding signal return: that event is impossible with correct production. It applies the held-state safety assertion, releases signal return, joins, and applies the unchanged trace-order/sole-wait assertions. This avoids a good-branch cyclic wait.

The negative branch changes only the acquisition protocol: it demands the actual violating event and validates that signal release remains open, the final-return marker is absent, and the wait count is exactly one. The common safety assertion at 523-525 still examines actual counters/channels/trace; it does not inspect `requireEarlyWaitWitness`. The final `trace.assertOrder` is unchanged and similarly independent of that constant. Setting the constant true with good production cannot fabricate a qualifying failure: it yields unavailable because no actual wait-entry witness can arrive while return is held.

The two positives establish the original baseline and the added rendezvous on correct production. The paired negatives then require a violation that only their altered production path admits. This is an explicitly asymmetric paired qualification protocol, **not** a claim that one unchanged test is universally sensitive to every possible mutant. In particular, positive-harness-plus-mutant sensitivity and the old standalone control are not accepted by this review.

### 4. Synchronization and cleanup are safe for these pure pairs

- `waits` is atomic; channel close/receive publishes entry; trace records/counts use their own mutex. The gate callback waits only after `trace.record` returns and unlocks. It does not hold the trace mutex while awaiting the signal.
- The final signal holds the owner authority lock, but the waiting gate callback does not need that lock. No candidate path needs the callback to finish before starting the final signal. There is no rendezvous lock cycle.
- Deferred cleanup is installed before either worker starts. It closes abort, releases signal return idempotently, then acquires authority through `fence(false)`. This ordering breaks both possible held paths before acquiring the lock.
- Both final joins are attempted separately, not hidden behind a short-circuit OR. Cleanup also attempts both applicable joins. `fenceStarted` is controller-local, so there is no shared flag race.
- The helper uses one absolute deadline capped by the existing one-second helper limit and the test deadline. There is no sleep, yield, retry, or renewed per-wait allowance. Already-published channels are accepted; a missing event/join at expiration is labeled unavailable. Scheduling itself is not promised to meet a wall-clock deadline.
- On an unavailable run, a temporarily unscheduled worker may still finish after the bounded join observation; that is not represented as a successful join or valid RED. A harness/global timeout, stuck cleanup, panic or unexpected race must disqualify the result. Hosted process/job teardown remains the final isolation boundary, not permission to claim a completed local join.
- Only fake observer, wait and signal functions are installed in the selected ordering fixture. The fake numeric target 99 never reaches a kernel operation. The snapshot test likewise uses only memory/channel/result types. Native helper fixtures exist in the compiled package but are not selected by the two exact anchored roots.

### 5. Old-snapshot remains a behavioral control, not a compile failure

`old-snapshot.go:398-427` restores completion-after-results in the actual snapshot implementation. The unchanged `TestOwnedLinuxSnapshotCompletionCoherent` releases and joins a real test publisher through the production snapshot seam after the earlier result reads. Its old result can be error-free while the trailing completion check sees closed channels. The expected failure is `ASSERT_LINUX_SNAPSHOT_COMPLETION_COHERENT`. No flag or variant-name branch forces that assertion.

Before crediting this negative on each target/mode, acquire the unmodified snapshot test's positive outcome for the same committed bytes, environment and mode. The five-variant inventory includes two **ordering** positives; it is not a substitute for that snapshot baseline. This can be an exact baseline selection in the future reviewed runner, without another source mutation.

## Exact accepted qualification inventory

External root `/tmp/lsp-linux-controls-39c57a81/`.

Source identity aliases (all independently hashed):

- **P0**, committed production: `fc6c96db10598968096bdcbc70c693ee74d8db10fa92d32ce3c099ae49f4352d`.
- **T0**, committed test: `baa697699f5076106551c658039de6182e4acb0ad1f1d9f90169ade0d3cdd101`.
- **T+**, `positive_test.go`: `b4c012570f498b2f5638c5bc0bc3b041e14d3fa60e130ae4d425e6ef2ac5c4b3`.
- **T-**, `negative_test.go`: `dcff566a3362a6f4afce9a4579b2f22a37b1c170c02d7b5ef6838dc4be1c36a5`.
- **Ps**, `old-snapshot.go`: `fcdef82a8d2b4daaf429f976342b61b7a59df4ec827e312dacb52b4d0046ff5b`.
- **Pw**, `early-wait.go`: `1b1dc93510dae016f8b7579804ed9d611b1f394b622c50e2f40d28325339a894`.
- **Pf**, `early-fence.go`: `26d7f243de78447ed1b37db8a2975236119e5c711722d5c3dfee6df6eca2e686`.

| Variant | Production/test | Exact selection | Expected only |
|---|---|---|---|
| candidate-original | P0/T0 | `^TestOwnedLinuxFinalSignalReturnFence$` | PASS |
| candidate-rendezvous | P0/T+ | `^TestOwnedLinuxFinalSignalReturnFence$` | PASS |
| old-snapshot | Ps/T0 | `^TestOwnedLinuxSnapshotCompletionCoherent$` | named snapshot assertion FAIL |
| early-wait | Pw/T- | `^TestOwnedLinuxFinalSignalReturnFence$` | actual held-entry witness and named ordering FAILs |
| early-fence | Pf/T- | `^TestOwnedLinuxFinalSignalReturnFence$` | actual held-entry witness and named ordering FAILs |

Pair digests bind HEAD, both source digests and the anchored selection using the manifest's sorted-key JSON encoding:

| Variant | Pair SHA-256 |
|---|---|
| candidate-original | 3e7eb7f3c5481ebca27c76879b3c546afda744c06c2be5f08741f8f6202cd21e |
| candidate-rendezvous | f6667ade683387604c2682287a2eb727217a9c645ab13e64e2f9861e86aa3ff1 |
| old-snapshot | 5fda26865782773b7d0a4a9c01cac408a9444be28df3b90b014828b8dea5110b |
| early-wait | 6c66c85406bdbd911e0c4ec40d9a04cb1253e1ea2ba0b70c8bb1db9da2156637 |
| early-fence | 61aad9bc94e7268aecff06d501b35eeb0b4d899c49f345ed647484b305f1f422 |

Manifest SHA-256: `8a543158089fdb0361fa9643d1130b31353c8e70cc3e5ec7bde715d90db06d39`.
Verification SHA-256: `09cd958b563ea29773958d5e1922994ce9efd0999e52aa3435869e06a054023a`.

All five patch/overlay digests in that manifest were recomputed and matched. Every patch exactly equals the full HEAD-to-replacement difference; the empty original patch is correct. Absolute overlay keys/values match the exact two permitted repository paths and manifest-bound external files. The current maps use local macOS paths: **both sides must be remapped and revalidated on a future hosted worker**, without changing pair bytes or selections. The old earlier early-fence digest differs only in formatting from this new artifact; the accepted identity here is Pf, not an inferred alias.

## Required runtime outcome classification

No runtime outcomes are supplied by this review. A future authorized runner must preserve the full log, exit status, selected test identity, pair bytes, architecture/kernel/toolchain and mode.

For each ordering negative, require all of:

1. `CONTROL_ACTUAL_WAIT_ENTERED_WHILE_SIGNAL_HELD`.
2. `ASSERT_LINUX_FINAL_RETURN_REQUIRED_WHILE_HELD`.
3. `ASSERT_LINUX_FINAL_RETURN_FENCE_WAIT_ORDER`.
4. Complete `LINUX_ORDERED_EVENTS=` trace and the selected test's terminal FAIL.
5. No unavailable/invalid rendezvous, timeout, panic, unexpected race, incomplete trace, build failure or missing cleanup outcome.

For old-snapshot, require its named coherence assertion and selected terminal FAIL, with no infrastructure/cleanup failure. An unsuccessful exit alone is never RED. If an intended marker appears and cleanup subsequently becomes unavailable, the run remains unqualified. A passing mutant is not a positive result. The two positive variants must genuinely PASS in each claimed mode/architecture.

The package must be selected with only the exact anchored test shown in the table. Never apply broken production overlays to a broad package test selection, native helper root, native lifecycle test or production command.

## Existing CI capability inspection — no usable external-control route found

Only these two requested files were read, completely, and independently matched to committed HEAD:

- `.github/workflows/ci.yml`, 206 lines, SHA-256 `aeba948e17e9bf4ff511fb2919910279e5261ef764ddf5c75db60ff72a28db94`.
- `scripts/check-ci.sh`, 16 lines, SHA-256 `ec5cca1c74d30131b99154cf1ef9402d5ec5577b608102d18e450a49a6522f7f`.

The workflow has `workflow_dispatch` at line 6, but no inputs for an artifact, overlay, manifest, selection, or command. `test-linux` (77-150) uses five fixed shards on `ubuntu-latest`, resolves package lists through the shard manifest, and runs fixed broad package commands at line 114. It does not ingest these external artifacts or use `-overlay`/the paired pure roots. Artifact handling at 143-150 uploads shard output; it is not an external-control download/verification path. No Linux arm64 matrix is declared in this workflow. Existing unrelated narrow steps do not provide an arbitrary authorized command surface. The quality/vet/build/repository-qualified jobs similarly do not supply that surface.

`check-ci.sh` runs CI contract Python tests/checks and a release-command text check. It is neither a hosted job launcher nor an external-control execution wrapper. I did not follow its references or execute it.

**Conclusion within this bounded inspection:** dispatching the current CI workflow would not run these paired controls, and pushing a broken production/test replacement into its ordinary shard path would violate the required pure-only boundary. There is no safe existing route in the inspected configuration. This does not assert that no other external service or uninspected workflow exists.

Required next step is a **separately reviewed qualification-workflow proposal**, not implicit modification of the existing workflow. That proposal must address trusted artifact transport, exact HEAD/pair digest verification, absolute overlay remapping, separate positive/negative runs on actual authorized Linux architectures, normal/race toolchain readiness, exact test roots, offline/telemetry/initializer preconditions, existing approved remaining allowance, expected-failure classification and cleanup evidence retention. It must keep broken source external and never expose it to ordinary broad shards. No implementation or provisioning of that proposal is authorized by this addendum.

## Attributable writer evidence and verification boundary

Parent `01a1070d-1392-7797-8214-2c7836f6d572`; writer `ac8e826e-e70a-4c1`.

Assignment-inclusive native acquisition: snapshot `676f8b17c989e067ea4fcf3a1ea1366f7eb61372f0cd720ab766b95c3e0f9615`, producer_locator/producer_owned, one candidate, four matched assignment occurrences with none omitted. Current external-only assignment event: `30c5a388d986f8dd22693f67d4094aae26b93d0e7c842b1cf2b60bfea4afb05d`.

Inspected concrete evidence:

- Local derivation call `4748f8adb475eb4867f7314ecc5b5bbb35b6196dfdc03e835b5a996667b8f633`, complete result `a0138c2e60fe34a56d2f9439009f8287b21b52d33c2013fc2d85e0dbe7c92aa1`, snapshot `1b7f820abbb1ef2371a33a3fa1290ce373baaa81a149a27fdf6c6ad3d760b8e9`; phase/build-to-action evidence `b908d286ee460ef2323c950927479a451b58296d0140a1863068922b23aba749` and `7c09846b75e87aeb52c0648d445b559c6d1feb908652bb7dfb4f137d29e56ab0`. This is bounded derivation attribution, not a whole-transcript process certification.
- Ten-compilation call `d93c534d58e94744d1f03d552a88c8ae22ac8c41f2dcbe1d2f378b3315948288`, complete tool result `90d1fdc89f1e135163ebda1f40153a9a41920c239321d9230d0047283d5fa4cd`, snapshot `23103a8281cf05b75e94b4b11c75be2eafeb179dd6f5622c9e507fead088258d`.
- Final producer structural result `9414e3fcf35dfde1a9c40af8cbe0d5013ad85609f31dd032364b120618d4c3b2`, independently checked rather than trusted alone.
- Report write/result `912cd7e0954010259453e5361e92b1ef20c6dbf9e7f563143429c4a73ea774ba` / `53ddf34ca857c7dcab03cfb4dede472fe63933596832e9f3912aeb3ba73d80c5`.

The actual commands use `go test -overlay ... -c -o ...`, five variants times linux/amd64 and linux/arm64, CGO disabled, on Darwin/arm64 with Go 1.26.5. All ten records are complete, untruncated and exit zero. Record inventory and verification digest were independently checked. **No binary execution; runtime evidence IDs for this phase: none.** The separately reported candidate Linux/amd64 normal pass was not replayed or used to infer an overlay pass/fail here.

Fresh reviewer-local Bar probe/depends/atomic derivation preceded substantive review. The prior ownership contract was reused at unchanged source hashes. No source tracing, new LSP session, resource/funding change or notebook integration receipt was needed or created; notebook mutation is prohibited.

## Token application

full: "No blocking source/harness defect found in the five exact bound variants below." — all named positive/negative pairs and their execution boundary were reviewed.

depends: "The negative harness waits for `actualWaitEntered` before releasing the signal (512-526)." — actual entry, not a marker or label, is the release prerequisite.

atomic: "This external addendum is the only output written by this reviewer." — acceptance evidence remained separate from any code/workflow implementation.

## Model interpretation

- The accepted unit is the manifest-bound production/test pair, not the earlier standalone mutant.
- Unavailable is an evidence outcome, not a successful negative. Host scheduling and kernel qualification are not proven by compilation.
- CI route discovery was deliberately limited to the two authorized files. The absent route requires a reviewed proposal, not an invented capability or an automatic workflow edit.

Improved framing: qualify actual observed fence violations through immutable pure source/test pairs, while keeping artifact execution authority and funding decisions separate.
