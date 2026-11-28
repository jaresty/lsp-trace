# Qualification-control rendezvous repair at 39c57a81

## Disposition

**Prepared for independent review; compile-only, not runtime-qualified.** This phase changes only external qualification artifacts. No repository source—including the committed test file—was modified. No policy, funding input, workflow, Git state, admission gate, or public behavior changed. No test binary was executed. No observed RED/GREEN is claimed for these variants.

Repository: `/Users/schwa/dev/lsp-trace-variable-v2-TSZxdY`

Exact HEAD: `39c57a81d067b8868710fc88eddafb6f13340e77`

The committed original test and its old standalone early-wait overlay remain scheduling-sensitive together. The replacement qualification artifact is an explicitly bound **production-control + test-harness overlay pair**, not a claim that the old standalone control was repaired by compilation.

## Inputs and attributable evidence

Read the complete 174-line `/tmp/lsp-linux-lifecycle-269e37c-INDEPENDENT-REVIEW-ADDENDUM.md` before design and the complete 71-line `/tmp/lsp-lifecycle-backup-269e37c/REPORT.md`.

Native reviewer acquisition: parent `01a1070d-1392-7797-8214-2c7836f6d572`, reviewer `f697b86a-7a7a-47b`, assignment-inclusive snapshot `6eadc1054fc53779c64d669d6caf77aaa4f7a1d50e14bb0ee634821cd1b1b7dd`; `producer_locator / producer_owned`, one candidate. Both recorded assignments were returned, with no omitted assignment records. The source-established scheduling finding was inspected directly at event `0533badec533fe103bb8945c5e9cd83f9c4767e116de104aca40268dd4d3aae5`, snapshot `0edfa7c7662536e959f9fbb5e97abe05a7f5a96b1d118bca1956a1034dc0c4f4`. Relevant actual source and prior generator were read locally as well.

The review accepted source safety for hosted progression while identifying R1's nondeterministic control. The later hosted report records ten pure and six native roots passing on Linux/amd64 normal at this committed HEAD. That is historical reported hosted evidence, not newly executed/replayed evidence in this phase, not Linux/arm64 or race evidence, and not overlay qualification. The hosted report explicitly states no runtime overlay controls ran. The unrelated CI run had other failing jobs; this phase makes no whole-repository success claim.

Fresh local phase `qualification-rendezvous-39c57a81` used the requested `bar build probe depends atomic`, complete full/depends/atomic token definitions and depends+atomic composition, followed by a closed derivation and pre-mutation build check. No notebook receipt was written because notebook mutation is prohibited.

## Why the former control could pass

Its reaper could announce gate-marker entry and pause before the consuming call. The controller then released the final signal without observing that call. Fencing could finish before the reaper resumed, making the mutant's recorded order look safe. Merely waiting for a marker, yielding, sleeping, or repeating tests cannot exclude that schedule.

Moving the consuming call before the marker would force an ordering violation, as suggested by the review, but would not establish this request's stricter premise that the actual injected consuming-call entry occurs **while final signal return is held**. The paired external harness supplies that rendezvous without changing candidate code.

## Precise contrasting protocol

The two external test copies replace only `TestOwnedLinuxFinalSignalReturnFence`. Everything before and after that function—including imports, globals, native fixtures, and all other tests—is verified byte-identical to HEAD. The positive and negative copies differ by exactly one local boolean constant, `requireEarlyWaitWitness`.

Common protocol:

1. The event callback records `linuxWaitGateEntered`, releases the trace mutex, closes `atGate`, then waits for either actual injected signal entry or cleanup abort.
2. The controller receives `atGate` and starts the fence goroutine. The final signal function closes `entered` and cannot return until `release` is closed.
3. Only after `entered` can the callback return and either implementation proceed toward its consuming call. The fake `waitChild` increments the wait counter and closes `actualWaitEntered` at its actual function entry—not at the earlier trace marker.

**Good branch (`candidate-rendezvous`, constant false):** retain production's receive of `fenced` before the consuming call. Do not await `actualWaitEntered` while the signal is held: that would be impossible. Check the existing held-return safety assertion, release the signal, join both owned goroutines, and check the unchanged ordered trace and sole wait. This branch has no cyclic wait.

**Broken branches (`early-wait` and `early-fence`, constant true):** after actual signal entry, await `actualWaitEntered` before releasing signal return. The early-wait body has its fence receive after the wait; the early-fence body has already released its fence channel. Both therefore can reach the injected wait while the signal remains held. Confirm exactly one wait, an open release channel, and no final-return marker, then emit `CONTROL_ACTUAL_WAIT_ENTERED_WHILE_SIGNAL_HELD`. The original held-return assertion must fail. Release signal return, attempt both joins, and execute the original complete trace/order assertion, which must also fail.

For early-wait, the forced happens-before chain is:

`signal entered -> gate callback returns -> wait-entry marker -> actual fake-wait entry/counter/channel close -> controller receives witness -> held assertion -> release -> signal returns -> final-return marker -> fence publication`.

Wait-return may be recorded on either side of signal return/fence depending on scheduling; no stronger total order is claimed or required. Wait **entry** is necessarily earlier than fence publication, so the ordered assertion cannot pass after a successfully acquired rendezvous. If the reaper stalls after its gate marker, the controller cannot take the former escape path: it still withholds signal return while waiting for actual entry.

For early-fence, the premature fence marker already precedes final signal entry/return, and the paired harness additionally forces actual consuming entry during the held interval. This retains the original ordering violation and strengthens its actual-entry witness.

## Cleanup and unavailable outcomes

- Abort and signal-release cleanup is registered before either goroutine starts.
- Cleanup closes `abort` first, releasing the gate callback even if no signal ever entered; it then idempotently opens `release` before acquiring authority through `fence(false)`.
- Fake wait and signal functions perform no native process, signal, waitid, SELF, or proc operations. The numeric test target never reaches a kernel signal call.
- Both final joins are attempted explicitly; failure of the first does not short-circuit the second. Deferred cleanup also attempts both applicable joins.
- All waits in the revised ordering fixture share one absolute deadline, capped at the existing helper's one-second limit and the test's remaining deadline. There are no new sleeps, yields, retries, or renewed per-wait allowances. This is stricter than granting each wait a fresh helper timeout.
- The shared await first observes already-closed channels and rechecks them at timer expiry. A missing rendezvous/join is `CONTROL_RENDEZVOUS_UNAVAILABLE`, never the intended negative outcome. `CONTROL_RENDEZVOUS_INVALID`, a timeout, panic, unexpected race, incomplete trace, or missing terminal outcome also disqualifies a negative result.
- The negative witness branch must be bound to its broken source pair, not good production. Even an erroneous pairing has the abort/release cleanup path, but its unavailable result is not a valid falsifier.
- The old-snapshot fixture retains its reviewed release-before-cleanup and publisher join behavior; no change was needed. Its source/test pairing is freshly bound to current HEAD.

Two additional channels (`actualWaitEntered`, `abort`) are test-only. No owner worker or production resource is added. No additional execution allowance or approved Lret/Lwork policy is inferred. Host scheduling cannot be guaranteed to progress by a wall-clock deadline; unavailable runs must remain unavailable.

## Exact inventory and artifact binding

All artifacts are under `/tmp/lsp-linux-controls-39c57a81/`:

- `manifest.json`: five exact HEAD/current source identities, five bound variants, full source-pair identities, overlay/patch hashes, counts and exclusions.
- `inputs.json`: fixed baseline, secure-wrapper identity, scoped initializer audit and variant mappings.
- `verification.json`: ten actual compile-only command/result records.
- Five `.overlay.json` files and five `.patch` files named by variant below. The original-candidate patch is intentionally empty; the other four describe external replacements relative to current HEAD. These patches were not applied to the repository.
- Five external Go source files: positive/negative test copies and three broken production copies.
- `prepare.py` and `finalize.py`: local preparation/structural verification; no hosted execution mechanism or CI workflow was added. Preparation produces source, which was formatted before finalization. Do not regenerate raw sources and reuse a stale formatted manifest.

| Variant | External replacements | Exact pure test | Expected, not observed |
|---|---:|---|---|
| candidate-original | 0 | TestOwnedLinuxFinalSignalReturnFence | PASS of original test |
| candidate-rendezvous | 1 test file | TestOwnedLinuxFinalSignalReturnFence | PASS of positive rendezvous branch |
| old-snapshot | 1 production copy | TestOwnedLinuxSnapshotCompletionCoherent | ASSERT_LINUX_SNAPSHOT_COMPLETION_COHERENT |
| early-wait | production + negative test copy | TestOwnedLinuxFinalSignalReturnFence | actual held-entry witness + both ordering assertions |
| early-fence | production + negative test copy | TestOwnedLinuxFinalSignalReturnFence | actual held-entry witness + both ordering assertions |

The ordering-control required negative markers are `CONTROL_ACTUAL_WAIT_ENTERED_WHILE_SIGNAL_HELD`, `ASSERT_LINUX_FINAL_RETURN_REQUIRED_WHILE_HELD`, `ASSERT_LINUX_FINAL_RETURN_FENCE_WAIT_ORDER`, and a complete `LINUX_ORDERED_EVENTS=` trace. Exit failure alone is insufficient.

| External source | SHA-256 |
|---|---|
| positive_test.go | b4c012570f498b2f5638c5bc0bc3b041e14d3fa60e130ae4d425e6ef2ac5c4b3 |
| negative_test.go | dcff566a3362a6f4afce9a4579b2f22a37b1c170c02d7b5ef6838dc4be1c36a5 |
| old-snapshot.go | fcdef82a8d2b4daaf429f976342b61b7a59df4ec827e312dacb52b4d0046ff5b |
| early-wait.go | 1b1dc93510dae016f8b7579804ed9d611b1f394b622c50e2f40d28325339a894 |
| early-fence.go | 26d7f243de78447ed1b37db8a2975236119e5c711722d5c3dfee6df6eca2e686 |

The early-wait production mutation itself intentionally retains its old bytes. Deterministic sensitivity now depends on the **pair**, including the new test copy. Pair identities include HEAD, production hash, test hash and anchored selection:

- candidate-original: `3e7eb7f3c5481ebca27c76879b3c546afda744c06c2be5f08741f8f6202cd21e`
- candidate-rendezvous: `f6667ade683387604c2682287a2eb727217a9c645ab13e64e2f9861e86aa3ff1`
- old-snapshot: `5fda26865782773b7d0a4a9c01cac408a9444be28df3b90b014828b8dea5110b`
- early-wait: `6c66c85406bdbd911e0c4ec40d9a04cb1253e1ea2ba0b70c8bb1db9da2156637`
- early-fence: `61aad9bc94e7268aecff06d501b35eeb0b4d899c49f345ed647484b305f1f422`

Local overlay keys and values are verified absolute paths to the exact checkout and external files. Hosted mapping is **not provisioned**. A future authorized mechanism must remap both sides to its exact absolute candidate/control roots, verify current HEAD and all pair bytes, preserve the anchored pure selection, and use only the existing remaining job allowance. Never run a native fixture or broad test selection against a broken overlay. Do not use the local macOS path mappings unchanged on a hosted worker. No workflow or hosted execution is authorized or created by this report.

## Actual verification and preserved boundary

Before any Go invocation: verified Darwin, HOME 0700, actual macOS/XDG telemetry files 0600/off, env-i, offline proxies, local toolchain, GOENV off, readonly modules and buildvcs off. `go telemetry` returned `off`; `go env` returned `darwin`, `arm64`, `go1.26.5`. The modified external test region introduces no init/TestMain/import/global changes; no target initializer or test body was executed. Only standard existing time/sync/atomic/testing support is used.

**Ten compile-only results passed:** five variants × linux/amd64 and linux/arm64, CGO disabled. Every record is untruncated and exit zero. No new Linux test, local native test, negative control, or race runtime was executed. No RED/GREEN claim follows. Runtime IDs for this phase: none.

Final checks: five reviewed candidate files byte-identical to committed HEAD; tracked changes 0; index changes 0; untracked paths exactly the 40 original capsules, excluded by name and unread. No production edit, test-file repository edit, module change, funding input change, notebook mutation, network/install, agent, Git mutation or commit. Public Linux admission remains disabled. Proposed allocation policy remains UNAPPROVED and untouched.

Independent review should inspect the paired rendezvous and cleanup protocol, then separately arrange authorized hosted normal/race results for both Linux architectures if a mechanism becomes available. Stop here; do not interpret this handoff as funding, admission, or runtime qualification.
