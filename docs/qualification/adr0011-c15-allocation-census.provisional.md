# ADR0011 C15 allocation census — provisional

Status: `ACCEPT_C15_BOUNDARY`; the ledger is accepted as a correction-safe boundary, while aggregate C15 acceptance is withheld.

## Evidence boundary

- Filesystem source set: reviewed manifest `adr0011-synthetic-source-pin.eb859ce7fd43d19c.manifest.json`.
- Manifest SHA-256: `eb859ce7fd43d19c674a1f22925d1cb3a29c456828571d6bb0d77a9bdfe57d4c`.
- Aggregate: `sha256:fb36a36ffb71bb4d58c5cdea3f247b5fbc749a987aca11f62683c076de7437bd` (114 files, 872462 bytes).
- Managed structural attempt: exact READY session `sk1:949e027243da188035cc5c6532c4cb10df4210f2bdde80acc58b7b7b9af5f283`, generation 1; 551 candidates, 500 selected, 51 omitted; client output truncated; authority `0`; source-graph completeness `UNKNOWN`.
- Structural evidence is locator evidence only. It does not accept ownership identity or establish completeness.
- Counts below are logical ownership seams, not dynamic allocation-event counts.

## Classification vocabulary

- `GOVERNED_OWNER`: accepted owner with an exact release/transfer route.
- `ACCEPTED_ALIAS`: callback/dynamic-extent view of governed backing.
- `SEPARATELY_CHARGED_COPY`: independent backing with a distinct pre-reservation.
- `POLICY_BLOCKED`: intentionally unresolved or outside the marked custodial policy.
- `TEST_ONLY_EXCLUSION`: default-off package-private instrumentation.
- `OPAQUE_SCRATCH_EXCLUSION`: inaccessible standard-library/runtime scratch that does not escape.
- `UNRESOLVED`: project-visible backing without an accepted ownership policy.

## Reconciled seam ledger

### Governed owners (15)

| ID | Seam | Owner / release boundary |
|---:|---|---|
| 1 | V2 transaction table | `privateB4ByteAccountV2`; terminal request plus all lease releases (`20261005192502-3026`) |
| 4 | Manager source-ingress table | Manager owner; `Shutdown` invalidates held states, clears bytes/entries, and releases table (`20261006030408-1456`) |
| 5 | Per-source ingress state | ingress lease release or descriptor transfer (`20261005173021-0599`, `20261005175249-6916`) |
| 8 | Source descriptor transfer | dedicated V2 descriptor; released at reservation retirement |
| 9 | Request-frame capture charge | temporary request-frame lease (`20261004222557-4857`) |
| 10 | Request-frame retained charge | temporary request-frame lease (`20261004222557-4857`) |
| 11 | Caller request-frame owner | caller lease exact-once release (`20261004222557-4857`) |
| 13 | Outbound canonical header/body charge | outbound transport lease (`20261004224204-4520`) |
| 14 | Successor prefetch | reader close (`20261004231943-0245`) |
| 15 | Successor header | per-attempt deferred release (`20261004232336-5883`) |
| 16 | Successor consume scratch | per-copy deferred release (`20261004233247-1583`) |
| 17 | Successor body | per-frame deferred release (`20261004232611-2989`) |
| 22 | Provisional result-token replacement | previous manager holding released before replacement (`20261006023814-5955`) |
| 25 | Diagnostic event backing | atomic transfer to Manager history, then eviction/shutdown release (`20261005201105-5277`) |
| 27 | Fixed diagnostic history | fixed Manager slots/order; generation-qualified release (`20261005214628-6095`) |

### Accepted aliases (7)

| ID | Seam | Dynamic-extent boundary |
|---:|---|---|
| 29 | Notification callback view | `WithNotifications`; terminal/release drains callbacks (`20261006014845-6698`) |
| 31 | Unmatched/late response callback view | `WithResponses` (`20261006023814-5955`) |
| 33 | Matched server-error callback view | `WithServerError` (`20261006021001-8737`) |
| 36 | Owned method-pair callback view | `WithPair` (`20261005230220-3632`) |
| 43 | Borrowed lexical result | `Prepare/CommitPrivateB4DefinitionBorrowed` callback (`20261005000642-3335`) |
| 44 | Borrowed capture value | callback-scoped struct/slice views; must not escape |
| 45a | Server-request ID token | exact string/integer token aliases the governed original frame only through synchronous rejection output (`20261006040600-1604`) |

### Separately charged copies (22)

| ID | Seam | Charge / release boundary |
|---:|---|---|
| 2 | Private request params ingress copy | temporary params lease (`20261004230052-1374`) |
| 3 | Prepared source content | Manager source lease (`20261005175249-6916`) |
| 6 | Source-lease handle slice | exact `len * sizeof(handle)` pre-reservation and reservation retirement (`20261006030408-1456`) |
| 7 | Transaction source bytes | one exact lease per admitted source |
| 12 | Encoded request body | outbound transport owner (`20261005023155-0654`) |
| 18 | Original assembled response frame | Manager/caller response-frame owner (`20261005032901-8738`) |
| 21 | Exact result token | Manager/caller result owner (`20261005000642-3335`) |
| 23 | Accepted decoded-message clone | decoded owner released before return (`20261005043636-1438`) |
| 26 | Diagnostic metadata strings | exact metadata lease; replacement/eviction/shutdown release (`20261005214628-6095`) |
| 28 | Retained notification set | aggregate owner/lease (`20261006014845-6698`) |
| 30 | Retained unmatched/late response set | transactional aggregate owner/lease (`20261006023814-5955`) |
| 32 | Matched server-error Message/Data | error owner/lease (`20261006021001-8737`) |
| 34 | Owned pair Params | combined pair charge (`20261005230220-3632`) |
| 35 | Owned pair Result | combined pair charge (`20261005230220-3632`) |
| 37 | Persistent reservation capture params | exact pre-reservation before copy (`20261006030408-1456`) |
| 39 | Consumer capture RequestFrame | exact final-capture lease |
| 40 | Consumer capture RequestParams | exact final-capture lease |
| 41 | Consumer capture ResponseFrame | exact final-capture lease |
| 42a | Consumer capture TargetSources slice | exact final-capture reservation |
| 42b | Consumer capture source bytes | one exact capacity per source |
| 45b | Unsupported-response body | exact final-output allocation, atomically reserved and released after write/failure (`20261006040600-1604`) |
| 45c | Unsupported-response canonical header | exact logical header capacity reserved before writer allocation and released after write/failure (`20261006040600-1604`) |

The former reservation `capture.Result` defensive copy was removed. Result bytes now remain solely under the accepted result owner/borrow boundary; it is not a current seam.

### Policy-blocked seams (4)

| ID | Seam | Boundary |
|---:|---|---|
| 46 | Ordinary/public notifications | ordinary behavior intentionally unchanged |
| 47 | Ordinary/public unmatched/late responses | ordinary behavior intentionally unchanged |
| 48 | Ordinary `RoundTrip` and noncustodial P2 | outside marked custodial boundary |
| 50 | Legacy ordinary owned-pair copies | outside private lease owner |

Former remarshal IDs 20 and 24 no longer allocate: exact canonical `Message` size is calculated as bounded scalar metadata from decoded fields/raw spans (`20261006040600-1604`). Former policy-blocked server-request ID 45 is replaced by accepted alias/copy seams 45a–45c.

### Test-only exclusions (5)

| ID | Seam |
|---:|---|
| 51 | `methodCandidateTestHook` / defensive test observation |
| 52 | `ownedMethodPairTestHook` |
| 53 | outbound/after-write/params accounting probes |
| 54 | decoded/owned-pair reserve/refusal probes |
| 55 | C wire/decode/retain probes |

All are package-private, default-off, and excluded from production custody.

### Opaque scratch exclusion (1)

| ID | Seam | Boundary |
|---:|---|---|
| 19 | `encoding/json` decoder/runtime internals | inaccessible transient backing; excluded unless project-visible backing escapes |

### Lifecycle join (cross-cutting)

| ID | Seam | Boundary |
|---:|---|---|
| 56 | STOP/RESTART retirement join | terminal admission closure and post-Manager-unlock wait for pair, notification, server-error, and response owners |

ID 56 is cross-cutting and is included in the governed-owner count through the owners it joins; it is not an additional backing allocation.

## Count reconciliation

```text
logical backing seams = 54
54 = 15 GOVERNED_OWNER
   + 7 ACCEPTED_ALIAS
   + 22 SEPARATELY_CHARGED_COPY
   + 4 POLICY_BLOCKED
   + 5 TEST_ONLY_EXCLUSION
   + 1 OPAQUE_SCRATCH_EXCLUSION
   + 0 unclassified backing in the reconciled filesystem census
```

Cross-cutting lifecycle join ID 56 and removed historical result-copy ID 38 are not counted as current backing seams.

## Gap ledger

| Gap | State | Revisit condition |
|---|---|---|
| Incoming server requests | `ACCEPTED_PARTIAL` | governed synchronous MethodNotFound rejection; no caller publication (`20261006040600-1604`) |
| Complete-message and diagnostic remarshal | `ACCEPTED_PARTIAL` | allocation-free exact canonical size calculation (`20261006040600-1604`) |
| Managed structural evidence | `BLOCKED_CAPABILITY_GAP` | all 551 identities are required; current API exposes only 500 identities plus aggregate omitted=51; authority remains 0 |
| Dirty worktree identity | `ACCEPTED_BOUNDARY` | exact reviewed source manifest above identifies selected implementation bytes; it is not build identity |
| Aggregate receipt | `NOT_ACCEPTED` | independent consolidated semantic review of this census and terminal accounting |

## Falsifier

This census is false if any marked custodial production path allocates, copies, aliases, transfers, or retains project-visible backing not represented above; retains governed backing after its listed terminal release; lets a callback view escape; publishes a governed server request instead of synchronously rejecting it; or permits ordinary/public paths to enter private custody.

## Evidence ceiling

This document is a provisional ownership ledger, not C15 completion, runtime proof, production authority, source-graph completeness, C16/C17 qualification, or public enablement.
