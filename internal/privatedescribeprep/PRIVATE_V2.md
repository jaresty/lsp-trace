# Connected private V2 mechanical checkpoint

This is **not ordinary Describe admission, semantic interpretation, feature acceptance, or production authority**. All caller text and source comments are untrusted data. Reviews and different author tags are caller assertions, not authenticated independence or verified semantic support.

## APIs and ownership

1. `PreparePrivateV2(ctx, []InputPrivateV2, Limits)` consumes actual `ProjectionPrivateV2` / `AssessmentPrivateV2` handles. `ValidateInstabilityPrivateV2` checks exact projection-state identity and recomputes comparison metrics from retained partitions; it does not rerun Leiden. The returned `Result` is detached inspection data, not a seal or a post-state constructor.
2. `NewPostPreparationPrivateV2(ctx, inputs, prepLimits, postLimits)` internally runs preparation and version-specific shape validation. Its opaque handle offers detached `PreparedBytes`, `PreparationSHA256`, `Manifest`, and pure `Reconcile`. There is no public bytes, Result, ledger, or callback constructor.
3. `RetainPostPreparationPrivateV2(ctx, handle, submissions, reviews, outputs, protectedInputs)` computes the ledger internally, embeds exact canonical preparation bytes, and retains one envelope in two independent private output roots. New fixed-family methods are `ReadPostPreparationPrivateV2` / `PublishPostPreparationPrivateV2`. V1 methods, request type, prompt, profile, and admission remain unchanged.

Shared internal record assembly, part splitting, canonical whole-packet withdrawal, post accounting, pair retention, root-chain validation, and metadata-only object identity mechanics are reused below versioned wrappers. No V2 source carrier is converted into V1.

## Evidence and coverage

- A namespace hashes the V2 request family and complete sealed source binding: packet, qualification report, batch ordinal, source artifact, original NativeV3, derived canonical V5, policy and profile pins. Assessment bytes additionally bind each prepared packet ID.
- Snapshot discovery target/symbol counts, batch target/symbol counts, and selected batch count are separately labelled. `OriginalSymbols` is batch-local; the sealed origin's own planned batch ordinal and canonical seeds bind exact original discovery targets, which are retained beside the complete snapshot. `OriginID` hashes the shared enumeration origin. Batches from that origin do not count as independent sources. Separate packets/origins retain distinct namespaces.
- Original acquisition order governs target order, even when caller selection is reversed. Every selected target has exactly one index row: `PREPARED`, `SOURCE_GAP`, or `RESOURCE_GAP`; requested equals their sum. Context files are not nominations.
- Bodies come **only** from own `READABLE` capture receipts with the exact policy hash. Packet file bodies are never a fallback. Missing/withheld/ambiguous/unmatched source produces a gap, not qualification.
- Native names, kinds, ranges, request IDs, cached references, original response bytes, null versus arrays, and duplicate/order information remain native evidence. Batch-global request metadata is explicitly not target-owned observation support. Existing expansion-to-request-to-observation linkage governs own call records.
- Retained AST association is labelled `AST_NOT_FULL_DEFINITION_NOT_METHODRESULT`; it does not rewrite provider kind/range or synthesize native full-definition evidence. The shared declaration extractor supports coordinate-bound receiver declarations, including bare `MarshalJSON` / callable kind 12, and encoding-aware offsets.
- Complete assessment records retain all declared runs, partitions/qualified basis metadata, statuses, all pairs, expected counts and private ceilings. No favorable seed or partition is selected; empty/resource/unavailable facts remain explicit and cannot support absence or confidence claims.

## Explicit finite budgets

No zero defaults are accepted for preparation or post-accounting limits.

| Boundary | Private ceiling |
|---|---|
| Constituents | 64 |
| Preparation target work | caller `MaxTargets`, 1..64; remaining requested targets become resource gaps |
| Complete aggregate index | 4096; post limit can narrow it |
| Part bytes | 2 KiB..2 MiB |
| Canonical preparation bytes | at most 8 MiB, at least selected part budget |
| Parts | 1..256 |
| Source bytes per declaration input | 1 byte..2 MiB |
| Post ledger targets / submissions / reviews | 4096 / 256 / 256 |
| Claims per item / citations per claim | 16 / 32 |
| String / caller item bytes | 64 KiB / 1 MiB; source quotes at most 240 bytes |
| Post input / output bytes | 16 MiB / 16 MiB |
| Evidence records / work units | 262144 / 10000000 |
| V2 publication object / logical / read-hash bytes | 16 MiB / 64 MiB / 256 MiB |
| Publication copy attempts | 2 total, at most one per root |
| Actual protected producer roots | 2..16, deduplicated across repeated batches |

Records and declarations are indivisible. Budget pressure withdraws whole packets and repairs coverage/index accounting. If the complete index alone cannot fit, the invocation refuses; it never returns a clipped index or body. Conservative complete-envelope estimates can refuse before storage.

## Retention boundary

The constructor obtains required root path/device/inode bindings from each opaque projection's sealed qualification report. Callers must supply exactly that complete actual producer-input set; foreign, missing, duplicate, aliased, closed, or replaced roots refuse. Repeated batches may share required roots. At most eight distinct qualified packet-pair bindings are admitted; shared root sets cannot widen this finite pair ceiling. Larger required root or pair sets explicitly refuse rather than losing protection. No hidden-root monitoring is claimed.

Output roots must be private, outside `/tmp` and `/private/tmp`, physically distinct, and neither equal to nor ancestors/descendants of actual protected roots. Root chains are checked before writes and at final validation. Exact named CAS objects are checked by metadata for output/output and output/input physical aliases; input contents are not reread by this check.

Only pure absence authorizes a new publication attempt; exact pre-existing copies may instead be verified. The shared pair protocol performs no overwrite/delete/rollback/retry, stops on late `ErrExist`, preserves per-copy receipts after partial namespace commitment, and reopens both copies twice with close-aware exact-byte/hash validation. Final topology and object checks must also succeed before `VERIFIED_PAIR_PRIVATE_WITH_LIMITS`. Namespace commitment is not an atomic pair transaction, authenticated custody history, or a power-loss guarantee. Roots are borrowed NONCONCURRENT; their owner must keep them alive and report close errors.

## Owned verification and remaining work

External-package owned tests use a real Manager with a fake pipe provider, capture every batch, shut the Manager down, retain/qualify/derive the producer packet, compute six-seed assessments, prepare AST packets, account prospective caller submissions/reviews, publish one envelope to the two output roots, and independently reopen/hash/compare both copies. The matched fixture variant generates declaration names, ranges, source bytes and hashes together **before capture**. The historical unmatched `Callable*` fixture is unchanged and produces honest source gaps. Successful preparation is also exercised after owned live source files are removed while the borrowed source root remains intact.

Tests cover 1/0, 2/1, 5/2 nomination/context counts; repeated batches and independent origins; aggregation of 72 requested targets with over-64 resource gaps; the 16-root bound and explicit refusal of nine real packet pairs; foreign same-byte projection/assessment state; clone mutation; selection ordering; bounds; whole-packet withdrawal; missing/foreign/duplicate actual roots; foreign quotes/handles; wrong pins/parts; oversize callers; pending/abstained/rejected accounting; and the private publication profile/physical aliases. A named actual-root guard bypass is used as the controlled RED counterfactual, then restored for GREEN.

Verification boundary for this implementation checkpoint: affected preparation/publication/graph packages and selected owned V1/V2 consumers passed; preparation/publication races and the connected/refusal/multi-origin V2 race family passed (`GOMAXPROCS=4`). The all-V2 race invocation including the nine-origin aggregate test exceeded a 360-second harness bound after preparation/publication races passed; aggregate coverage is claimed from its successful normal run, not from that timed-out race invocation. Parent verification additionally passed 348 preparation/publication cases, 19 connected normal cases, and separated connected/refusal/multi-origin race cases. The separately selected 72-target aggregate race test also exceeded a 240-second bound; its captured main stack was actively parsing a packet during fixture/projection setup. This is not an aggregate race pass or a scalability qualification. Vet and `git diff --check` passed. No real-25 driver or broad CI run was performed.

Subsequent measured correction (`07de8de`): full admission validation is retained,
while selected projection/preparation reads reuse constructor-owned immutable
snapshots. The original aggregate test file is unchanged. Its named race gate now
passes within the same 240-second bound in both writer and parent execution;
the writer measured 171.87 seconds. Normal profiled runtime decreased from 45.73
to 14.84 seconds. Earlier timeout evidence remains historical; no all-V2 race,
maximum-63-batch, or general scalability result follows from this correction.

Still **not done**: execution over actual varied user-owned real source; actual semantic interpretation, independent authenticated review/adjudication, feature identity acceptance, product meaning, or production validation. The connected mechanics do not discharge those tasks.
