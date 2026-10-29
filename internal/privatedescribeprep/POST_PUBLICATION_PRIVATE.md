# Private post-preparation single-envelope publication

This is a prospective private boundary, not ordinary Describe, workflow, Native
admission, historical worker admission, feature acceptance, or authenticated
review. All outputs retain authority 0, accepted false, feature UNRESOLVED, source
UNKNOWN, absence PROHIBITED and ordinary admission NOT_ATTEMPTED.

`RetainPostPreparationPrivate(ctx, h, submissions, reviews, outputs, protected)`
accepts only a constructor-owned `PostPreparation`. It internally invokes
`Reconcile`; there is no StageLedger argument or raw ledger parser. The owner
must supply the complete protected artifact-root set. The topology check verifies
provided capabilities, not completeness inferred from h. Original workspace and
endpoint source paths need not exist and are never opened here.

## Fixed bounds and evidence

* The new fixed output storage profile is 16 MiB per object. Legacy Publish/Read
  remain 2 MiB, partial V2 remains 4 MiB, preparation remains 8 MiB.
* Before Reconcile/build/encoding: preflightCall's conservative ledger allowance,
  exact preparation length and 4096 envelope overhead must fit independently in
  16 MiB, using subtraction. The exact serialized envelope must also fit. A
  legitimate 3,302,509-byte preparation is not clipped into the legacy profile.
* One versioned object contains exact canonical preparation JSON, its SHA,
  internally derived ledger, fixed profile/policy SHA, and zero ceilings.
  No dangling object references or multi-object transaction is introduced.
* Fixed policy: 64 MiB logical including staging, 256 MiB read/hash, two copy
  attempts maximum. These are finite conservative reservations, not heap or CPU
  guarantees. EnvelopeBytes returns detached exact bytes for parent verification.
* Invalid handles, contexts, caller ceilings, rejected caller submissions/reviews,
  count/string/item/output estimates, or topology fail before storage operations.
  Source/resource gaps and pending records remain legitimate ledger accounting.

The topology algorithm is deliberately a narrow arity copy of the existing
algorithm: exactly **2 outputs + 1..16 protected artifact roots**, rather than
3+3 or nine references. Outputs are existing admitted roots outside `/tmp` and
`/private/tmp`, mutually disjoint and disjoint from every protected root in both
ancestry directions. Input-input ancestry is allowed. Canonical chains hold at
most 256 path components per root, use existing descriptor-relative no-follow
operations, recheck every binding, and close all handles even on failure.

Both roots are borrowed NONCONCURRENT capabilities. Owners close and report all
close errors after all calls; the publisher never closes owner roots.

## Concrete pair protocol

Each root gets a close-aware pre-read. Exact existing bytes can qualify only
before a publication attempt. Only finite error trees consisting entirely of
absence permit one publish. Mixed cancellation/close errors stop. Every returned
receipt is cloned, even with errors or a known namespace commit. Any after-attempt
error, including ErrExist from an equal competitor, stops without readback/retry.
All receipt fields must agree before progressing; primary failure prevents backup.
A backup failure preserves primary's committed evidence. After both copies
succeed, four independent reads (two per root) hash and compare exact bytes,
fail-closed on cancellation/close. Only then is Phase PAIR_VERIFIED.

No rollback, deletion, retry, CAS, head mutation, historical CLAIMED-head authority
recovery, atomic multi-object/crash guarantee, attacker guarantee, semantic proof,
or automatic real run is supplied.

## Owned fixture versus real driver

`TestOwnedPrivatePostPublication` uses `preparationOwnedInput`, creates a new
sealed PostPreparation, constructs prospective caller DTOs and a separately tagged
review through Reconcile, then retains once. Protected roots come from retained
configuration record 0's input_roots/output_roots, not its workspace. It checks
unchanged fake-provider traffic and closes owners. All created test directories
are outside temporary roots and cleanup only removes each fixture's owned tree.

`TestReal25PrivatePostPublication` **skips by default**. Set only after independent
parent review:

```
LSP_TRACE_POST_PUBLICATION_REAL_CONFIG=/absolute/path/to/canonical-config.json \
  go test ./internal/censuslineagepilot -run '^TestReal25PrivatePostPublication$' -count=1 -timeout 240s
```

Do not run this command during implementation. The driver does not create or
delete output roots or invoke a provider. It reads only explicit retained artifact
paths. Reports are O_EXCL/absent-only, outside temporary/protected/output trees.
All root-close errors are recorded before report serialization. Report-file
write/sync/close errors are additionally emitted by the test (they cannot be
retroactively added to immutable absent-only report bytes).

Declared phases: hash/pair-verify pinned inputs and all 25 producer/review CAS
pairs; NewExactRetainedPartialQualificationInput; QualifyRetainedPartialNativePrivate;
close all five owner-held roots (the qualifier's borrowed-root ops.close is
intentionally a no-op; no separate exported proof-input owner is returned);
DeriveRetainedPartialNativePrivate; Project; explicit finite ComputeInstabilityPrivate
with seeds 42..47; ValidateInstabilityPrivate with that fresh sealed expectation
against the full retained assessment; NewPostPreparation using exact original
limits; compare exact preparation bytes/SHA; CALLER_CONVERSION_V1; Reconcile for
canonical submission SHA; separately attributed caller review import; Retain once;
close every remaining root; absent-only report. Recomputations are recorded
separately from acquisition replay. Any mismatch stops, never selects new seeds,
normalizes partitions, changes scope, or rehydrates an assessment DTO seal.

CALLER_CONVERSION_V1 hash-checks the original candidate file and both copies of
all raw producer/review bytes first. The old CAS uses **bare hex filenames**.
Every input artifact pair (qualification pins, assessment, preparation and every
producer/review CAS pair) rejects identical canonical paths before I/O and rejects
physical aliases, including hardlinks, using os.SameFile on identities captured
from the very descriptors supplying the bounded reads. CAS directory roots also
receive lexical and descriptor-identity alias preflight during config validation.
Close errors remain fail-closed. These are boundary observations, not attacker,
namespace-lease or power-loss guarantees; reading one physical file twice is not
a two-copy proof.

Review claim_ceiling is retained as json.RawMessage, accepting the pinned mixed
JSON string/object profile without stringifying objects. It must be valid UTF-8,
nonempty valid JSON within 16 KiB, with a nonblank string or meaningful object;
null, arrays, scalars, blank strings and empty metadata are refused. Imported
Findings also uses RawMessage for OriginalClaimCeiling, preserving JSON value and
type. This metadata is untrusted DATA, not new authority. Strict review top-level
keys and null required_revision compatibility remain unchanged.

It preserves claim text and reference order. Quotes attach only to their existing
own AST reference (240 UTF-8 bytes maximum), never to an added citation. Missing
or failed evidence (array or the pinned object) and alternatives are bounded
JSONStrings, not semantic rewrites. Author/reviewer tags identify new conversion
and import plus original raw SHA. Original review ceilings/findings and raw SHA
remain in bounded Findings; independence is only CALLER_ASSERTED. The report
retains candidate→new lead crosswalk, the whole ledger (including gaps/rejections),
partial publication receipts, operation errors and close errors. No count is
forced to 25. Pinned original candidates remain untouched.

## Configuration example (field checklist)

Serialize the `realPostConfig` Go struct with encoding/json, in its declared field
order, no whitespace or final newline. Unknown, reordered, duplicated, case-folded
or missing config fields are not canonical. Use full canonical paths, no aliases,
source fallback or arbitrary queries. This illustrative checklist is **not** a
ready-to-run config; the parent must choose/prepare existing empty output roots,
a separate existing report parent and config parent.

Let B = `/Users/schwa/Library/Application Support` (expand it in every field):

| Field | Exact value / artifact suffix |
|---|---|
| schema | `post-publication-real-caller.private.v1` |
| qualification_pins.paths | both `B/lsp-trace-private-partial-programc-ssoapecm/{primary,backup}/canonicalpins.780f77b9d75c013ce99d1e9d1e142e7eca27761b3fbd07e738b5fd3bcaa11a9c.json` |
| qualification_pins.sha256 | `sha256:780f77b9d75c013ce99d1e9d1e142e7eca27761b3fbd07e738b5fd3bcaa11a9c` |
| original_roots | `B/lsp-trace-inspect-filter-pilot-r9_1vnuv/{primary,backup,journal}` |
| partial_roots | `B/lsp-trace-scoped-partial-4m-_kggk8wc/{primary,backup}` |
| instability.paths | both `B/lsp-trace-private-partial-instability-jt_dd0xl/{primary,backup}/instabilityartifact.8afbd5c5bb7e1d9a8242e5346b68a5a5fbee56e24735951186f2ba0c4561f9e3.json` |
| instability.sha256 | `sha256:8afbd5c5bb7e1d9a8242e5346b68a5a5fbee56e24735951186f2ba0c4561f9e3` |
| preparation.paths | both `B/lsp-trace-private-target-preparation-bound-dv27oj35/{primary,backup}/preparationresult.5d6af15eb96c4c33a00b8aea70705dba85f34d1c42ad040e5abb319db4c26f5a.json` |
| preparation.sha256 | `sha256:5d6af15eb96c4c33a00b8aea70705dba85f34d1c42ad040e5abb319db4c26f5a` |
| candidates_path | `B/lsp-trace-private-target-interpretation-n4wbs2pu/provisional-candidates-v1/candidates.a803c5ec633d58ff03bd0db70e41972f90195fb9abe86b02eeb3eb92c6b02339.json` |
| candidates_sha256 | `sha256:a803c5ec633d58ff03bd0db70e41972f90195fb9abe86b02eeb3eb92c6b02339` |
| caller_cas_roots | `B/lsp-trace-private-target-interpretation-n4wbs2pu/{retained-primary,retained-backup}` |
| output_roots | two fresh existing private directories outside TMP |
| protected_artifact_roots | exact 3 original + 2 partial + 2 pins parents + 2 instability parents + 2 preparation parents + candidates parent + 2 CAS roots + config parent (15 if distinct) |
| report_path | absent full path in a separate existing private directory outside TMP and protected/output roots |

Canonical qualification pins retain CodeHEAD
`af7515ddad7ebea6a9fbf0155b0e36e7fc204bbf` and their original eight IDs; the driver
never changes those pins or the embedded scope/metadata/head observations.
The inspected retained assessment profile is
`DERIVED_RETAINED_PARTIAL_INSTABILITY_PRIVATE_V1`: 6 COMPLETE runs, 15 COMPLETE
pairs, outcome UNSTABLE. Its artifact SHA above is distinct from its internal
assessment Digest. The original preparation limits are exactly 64 targets,
65536-byte parts, 8388608 total bytes, 256 parts, 2097152 source bytes.
Prospective post limits explicitly retain all existing hard caps, with 16384-byte
strings, 1048576-byte items and 16777216-byte input/output allowances.

## Verification boundary

Targeted publication/preparation tests and owned integration are local evidence,
not full-repository CI or mutation-adequacy evidence. Pure encoder and conversion
fixtures are not sealed producer proofs. The opt-in real operation has not been
executed during implementation; independent parent review and parent execution
must precede any subsequent commit or acceptance.

### Correction verification (no real operation)

The separate `TestRealPostReadOnlyPinnedConversionSizing` is gated by
`LSP_TRACE_POST_PUBLICATION_READONLY_CONVERSION=1`; it skips by default and is
**not** the real driver. It only reads/hash-verifies the fixed candidate artifact,
paired preparation bytes and all 25 paired producer/review CAS objects. Decoded
preparation DTO manifests are explicitly MECHANICAL ONLY, never capability seals.
It uses the driver's exact conversion and imported-Findings encoder. It invokes
no NewPostPreparation, qualification, projection, instability computation,
Reconcile, retention, acquisition, provider or report write.

Observed read-only Go accounting: 25 reviews decoded, **4 string ceilings and 21
object ceilings**, 25 mechanical conversions, zero conversion/size rejections;
maximum imported Findings **15,036 bytes**, below the unchanged 16,384-byte limit.
The measured 4/21 distribution differs from the review handoff's 5/20; pinned
bytes were not changed to reconcile that discrepancy. Qualifications were not
dropped or clipped. These results do not establish sealed producer admission,
semantic support, live recomputation equality or successful real publication.
Pure guards additionally exercise both ceiling types, invalid forms/UTF-8 and the
size boundary, own-reference/quote preservation, same-path and hardlink alias
refusals, distinct equal-content acceptance, corrupt-copy refusal and config CAS
alias preflight. Rerun results: publication/preparation suites passed 319 test
events (124 top-level tests), both normally and with race; focused owned,
conversion, ceiling and alias race checks passed 30 test events (5 top-level),
with both real-operation and read-only-artifact tests skipped under default
configuration. The separate explicitly gated read-only size check passed once.
These are test-event counts including subtests, not mutation-adequacy claims.
Parent independent recheck and parent execution remain required.
