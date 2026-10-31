# Private V3 producer pages — resource milestone, NOT product completion

## State and review boundary

Base and final HEAD: `37c6d9a1148d6f01ce39c626daa05c0d3504aef9`.
Sole writer; no commit, push, new application plan, live Census, gopls, model, SDK, service, or runtime activation. Only new successor source/tests and this document were authored. The two pre-existing dirty documents were not edited; their SHA256 values still match the supplied protection pins:

- `private-variable-v2-host-handoff.md`: `13e0b3f37d7eade816a266c2eed2fad6ef50af7cdbd6933561c9d4f2e3d325f9`
- `variable-scope-private-v2-status.md`: `350834897cdec7f053ae7e03791e01e38d0cca8a4016defc4486cc6e00eca128`

**Part 1 acceptance is incomplete. Parts 2 (host composition) and 3 (live pilot) were not started.** The usable one-invocation application inventory remains the product goal; this milestone is not a substitute for it.

## Implemented removable resource-to-publication core

`internal/censuslineagepilot/producer_pages_*private_v3*.go` adds an unactivated family:

1. Original opaque frozen scope, enumeration and exact ordered successful capture capabilities admit a new paged owner. No JSON restoration or legacy packet-profile override exists.
2. Raw bytes, including individual oversized objects, are segmented into bounded binary frames. JSON headers describe exact ordered role/member/batch/offset intervals and a previous-page digest. No complete encoded origin packet or unbounded root index is allocated. The final revision is bounded metadata with full original role/batch denominators and the terminal cursor.
3. Both existing pinned retention roots receive independently verified copies. Fresh paired readback, object-inode distinction, original-byte comparisons, source revalidation, ordering and coverage checks gate verification. A bounded opaque verifier can pause and continue without rescanning the verified suffix.
4. An explicit shared application ledger spans origins, exchanges, reads, failed attempts and continuation. Create-only, fsynced reservation records precede admitted work. Page, exchange and application limits are separate; there is no default allowance or per-page budget reset. Reservations conservatively retain the existing common pair algorithm's 16 logical/32 read-hash passes, plus framing/verification/journal costs. They are not CPU/heap telemetry.
5. Metadata publication is reserved before page work. Budget pause publishes a useful admitted prefix when that operation was admitted. Earlier revisions are immutable and remain valid after later progress or failure.
6. Create-only attempt/checkpoint records spend stream generations. Original-owner checkpoint continuation verifies paired revision bytes and the complete prefix again. A crash without a completion checkpoint resumes only previously admitted progress; uncheckpointed bytes are not silently promoted.
7. A separate opaque per-batch observation owner reads original source/native bytes under the same cumulative ledger. It is explicitly NOT the legacy preparation/Native/AST owner.

All reports retain authority 0, accepted false, feature UNRESOLVED, source UNKNOWN and absence PROHIBITED. The hash-bound index is not authenticated. Same-user malicious journal rollback/editing is outside this private accounting contract; hashes do not authenticate origin or history.

## LIMITS — V3 failed-source accounting MED repair

A narrow follow-up repairs only V3 reservations; no shared source reader, source policy, allowance, pair-retention algorithm, or runtime wiring changed. Before-capsule and new evidence: `/Users/schwa/.local/state/lsp-trace-evidence/producer-pages-v3-medrepair-MtxbWDvZ`. The original resource-core capsule below is unchanged.

The unchanged `FrozenScopePrivateV2.revalidate` reads/hashes/clones **current** files under original policy bounds before comparing with frozen bytes. V3 now reserves those policy maxima, not frozen lengths. For `m` original members, original file cap `F`, and total cap `T`:

- `B = min(m*F, T) + m + 131072`: admitted content maximum, one sentinel per member, and bounded metadata/small-buffer allowance.
- Source-check reservation (up to two revalidations): `24*B` logical bytes, `32*B` read-hash bytes, and the unchanged `2*producerPacketTopologyWorkPrivateV2` work units. ReadAll intermediate/final buffers, clone/compare/hash scans and descriptor/path/result metadata are conservatively covered. These remain named reservations, not exact heap/CPU telemetry.
- Positive bounds are validated before unsigned arithmetic: `1 <= m <= 16`, `F <= 2097152`, `T <= 8388608`. Thus additions, products and sentinel accounting cannot overflow. Maximum reservation: **204472704 logical / 272630272 read-hash bytes**. For the three-member counterexample: **154140744 / 205520992**.
- Existing common pair-retention 16 logical/32 read-hash assumptions and its additional framing reservations remain unchanged. No cheaper-copy claim, automatic allowance increase or budget reset was introduced.

This is Option B, intentionally conservative even for unchanged tiny files and failures. The held-descriptor reader still limits current bytes to `min(F, remaining T)+1`, including growth after stat; there is no new path-stat shortcut/TOCTOU gap and no new source permission. A future reader/ReadAll implementation change requires renewed accounting review (verified here with Go 1.26.5).

Assertion RED exercised six real public V3 operation paths on an original-owned small frozen source grown to 1048576 bytes within its 2 MiB/8 MiB policy. All six rejected correctly but underreserved; a one-byte-short policy grant also exposed source work before the intended reservation gate. After repair, the counterexample tests pass (8 events), repeated failed checks consume the same ledger until it pauses, and unchanged earlier admitted progress survives a denied reservation. No failed read returns a qualified batch or source/native bytes.

Final focused verification under child umask 022: **37 normal / 37 race passing test/subtest events**, 0 failures; normal 91.125 s, race 128.327 s. Vet, gofmt and diff checks pass. The broad pre-repair suite below was intentionally not repeated: no shared legacy reader/seam changed. Initial follow-up normal/race logs are retained: the previous three-member, one-page-per-exchange verifier fixture correctly exhausted its unchanged 4 GiB allowance after 26/32 pages. That mechanics fixture now uses one original member, with identical source policy and allowances, and still asserts multi-page continuation and exact no-rescan read counts. Three-member grown-source and 20-capture fixtures remain intact. The performance cost is real, not hidden by an allowance increase.

**Parent retest and narrow re-review remain required before adoption. Part 1 acceptance remains incomplete; Parts 2/3 remain untouched and V3 stays unwired.**

## Original pre-MED resource-core evidence

Private logs: `/Users/schwa/.local/state/lsp-trace-evidence/producer-pages-v3-worker-AXn311GX` (0700 directory, 0600 files).

- `go test ./internal/censuslineagepilot -timeout=480s -json`: PASS, 1,641 passing test/subtest events, 389.782 s.
- `go test ./internal/censuslineagepilot -run '^TestProducerPagesPrivateV3' -race -count=1 -timeout=240s -json`, child umask 022: PASS, 28 events, 81.778 s.
- `go test ./cmd/lsp-trace ./internal/retainedpartialprogramc ./internal/privatedescribeprep -timeout=480s -json`, child umask 022: all PASS (CLI 238.536 s; other consumers 0.002/2.439 s).
- `go vet ./internal/censuslineagepilot` and tracked `git diff --check`: PASS; new Go files gofmt-clean.

Three assertion-specific REDs preceded their fixes: forged COMPLETE/empty cursor, changed source, and nonempty journal root. Compilation/fixture corrections were not counted as RED. An initial consumer run under umask 077 failed two pre-existing tests that intentionally create 0644 files; the corrected full consumer run under 022 passed. Failed logs are retained, not erased.

The controlled Go stdio child produced **20 fresh original-owned captures**, 5,331,688 raw artifact bytes, 260 bounded pages and 88 ordered role objects. Retained-failure lengths influenced synthetic payload sizing only; resulting artifact lengths are NOT the exact real 4,012,270-byte vector. This proves no live semantic throughput, feature grouping, or real bootstrap success.

## Remaining work / explicit limitations

- **Preparation integration is missing.** The existing validated snapshot requires an actual qualified whole V2 packet and all parsed batch sources. Manufacturing that owner from page JSON would be false custody. The new batch observation owner deliberately cannot enter `ProjectPrivateV2`/`PreparePrivateV2`. A separate versioned computation/assessment/preparation contract and controlled end-to-end publication test remain necessary.
- Cross-process resume without original opaque capabilities is unsupported. No weaker checkpoint-derived source-qualification owner was invented. A separately reviewed admitted-checkpoint contract remains deferred; withdraw it by leaving these APIs unactivated.
- The verifier's bounded suffix-progress handle is in-memory. Durable restart re-verifies the admitted prefix. Selected object reads currently verify that whole prefix and can pause without returning an object; resumable selected-read assembly remains missing.
- The original producer's per-origin safety limits remain. Arbitrary total application scaling still needs the staged host/application coordination path, not a claim that one finite cap supports all inputs.
- Journal bootstrap/reopen and same-user rollback boundaries require review before stronger whole-application accounting claims. This ledger starts after capture; acquisition/session/inference costs are not integrated. No production application-store/config/role approval binding has been added.
- No host role orchestration, application terminal reports, semantic grouping, revised application coverage or actual 20-target live pilot was implemented or claimed. Existing six leads and spent-generation accounting remain unchanged.

Presumed: explicit cumulative application allowance, not unlimited whole-run spend. Revised if: the user requires a fixed whole-run budget regardless of total input (Unknown, low learning rate until policy review).

Deferred: runtime activation and any weaker checkpoint authority contract. Withdrawn by: do not connect this private family to the CLI/host until parent review and the remaining acceptance gates pass.
