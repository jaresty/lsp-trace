# ADR0007 private candidate publication successor design

- **Date:** 2026-10-08
- **Successor name:** `ADR0007_PRIVATE_CANDIDATE_PUBLICATION_SUCCESSOR_V1`
- **Frozen design predecessor commit:** `f6e4315ab242fb3c9eea1c1a30ab9efb093090d8`
- **Predecessor disposition:** `CANDIDATE_PUBLICATION_CUSTODY_BLOCKED`
- **Successor phase:** `DESIGN_ONLY`
- **Successor verdict target:** `CANDIDATE_PUBLICATION_SUCCESSOR_DESIGN_READY`
- **Authority ceiling:** `authority=0`, `accepted=false`, `featureIdentity=UNRESOLVED`, `completeness=UNKNOWN`

This document is an additive private repository-local design artifact. It freezes design predecessor commit `f6e4315ab242fb3c9eea1c1a30ab9efb093090d8` as `CANDIDATE_PUBLICATION_CUSTODY_BLOCKED` and defines successor semantics for exactly four defects. It makes no implementation claims, does not change Go code or tests, and does not authorize any public API, CLI, MCP, registry, production, release, or stakeholder semantic authority.

## Scope and preservation rule

The successor preserves existing qualified semantics unless this document explicitly narrows or qualifies them. It must not add public compatibility promises. This successor is private and repository-local only. Locators are non-custody discovery aids, not API authority and not stakeholder semantics.

The four defects addressed here are exhaustive for this successor:

1. closed selector outcome states and rename-commit semantics;
2. exact immutable collision handling for existing manifest/alias identities;
3. removal of default lookup custody bypass;
4. stable repository-local lock inode.

Everything else is out of scope and remains as in the frozen predecessor or prior qualified ADR0007 artifacts.

## Identity separation: Git predecessor versus publication predecessor token

The successor has two predecessor concepts that must never substitute for each other.

### Frozen design predecessor commit

`frozen_design_predecessor_commit` is exactly:

```text
f6e4315ab242fb3c9eea1c1a30ab9efb093090d8
```

It identifies the blocked Git design/implementation predecessor whose semantics this document corrects. It is not a publication selector identity, not an expected selector digest, not an expected selector byte length, and not custody evidence for any current selector.

### Publication predecessor token

`publication_predecessor_token` is either:

```text
ABSENT
```

or the exact selector predecessor tuple:

```text
{ selector, digest, byte_length }
```

It identifies the predecessor value of the current selector in the publication domain. It is not a Git commit. It must be verified from precommitted custody material and exact current selector state. A correct `frozen_design_predecessor_commit` never compensates for a missing, wrong, or unverifiable `publication_predecessor_token`; a correct `publication_predecessor_token` never changes the frozen design predecessor disposition.

### API schema separation

Requests must carry both identities in distinct fields:

```text
CandidatePublicationRequest {
  frozen_design_predecessor_commit: string,
  publication_predecessor_token: ABSENT | { selector: string, digest: string, byte_length: integer },
  candidate_id: string,
  selector: string,
  expected_current_selector_digest: string,
  expected_current_selector_byte_length: integer,
  expected_manifest_generation: string,
  expected_manifest_digest: string,
  expected_manifest_length: integer,
  expected_byte_receipt_digest: string,
  expected_byte_receipt_length: integer,
  repository_root: string,
  publication_namespace: string,
  platform_support: DARWIN | LINUX,
  max_manifest_bytes: integer,
  max_alias_bytes: integer,
  max_receipt_bytes: integer
}
```

Results must report both identities separately:

```text
CandidatePublicationCustodyResult {
  custody_status: VERIFIED | COMMITTED_VERIFICATION_FAILED | NOT_COMMITTED | FAILED_CLOSED,
  publication_outcome: NOT_COMMITTED | COMMITTED | COMMITTED_VERIFICATION_FAILED,
  frozen_design_predecessor_commit: string,
  frozen_design_predecessor_binding: EXACT | MISMATCH | UNKNOWN,
  publication_predecessor_token: ABSENT | { selector: string, digest: string, byte_length: integer },
  publication_predecessor_binding: EXACT | MISMATCH | UNKNOWN,
  selector: string,
  current_selector_digest: string,
  current_selector_byte_length: integer,
  candidate_id: string,
  manifest_generation: string,
  manifest_digest: string,
  manifest_length: integer,
  byte_receipt_digest: string,
  byte_receipt_length: integer,
  idempotent: bool,
  retry: NEVER_BLIND | SAFE_PRECOMMIT_RETRY | RECONCILE_ONLY,
  partial?: object,
  diagnostics: bounded diagnostics
}
```

A schema, test, implementation, or reviewer must reject any result that uses `frozen_design_predecessor_commit` where a `publication_predecessor_token` is required, or that treats `publication_predecessor_token=ABSENT` as equivalent to the Git commit.

## Defect 1: closed selector outcome states

### Problem frozen in predecessor

Predecessor `f6e4315ab242fb3c9eea1c1a30ab9efb093090d8` is blocked because ordinary post-rename failures can be reported as if the selector were uncommitted. That creates a retry hazard: callers can blindly retry a candidate whose directory entry may already have been published by atomic rename.

### Successor states

Selector publication has a closed outcome enum:

```text
NOT_COMMITTED
COMMITTED
COMMITTED_VERIFICATION_FAILED
```

No other externally reported terminal selector outcome is valid. Internal implementation may have transient states, but API output must collapse into one of these three.

### Precommit custody material

All custody-bearing material must be written and verified before the current selector rename:

- custody-bearing manifest;
- byte receipt;
- verified-generation selector receipt;
- publication predecessor token receipt;
- exact current selector digest and byte length expectation;
- candidate id and manifest generation binding.

The current selector rename is the authoritative commit boundary. No authoritative custody receipt may be written after that boundary. Postcommit work may produce optional non-authoritative diagnostics only. Failure to write optional postcommit diagnostics can yield `COMMITTED_VERIFICATION_FAILED` or a warning, depending on when failure is detected, but it cannot create, repair, or replace custody evidence.

Custody reconciliation after a committed-family result derives or reads the precommitted receipts and the exact current selector. It must not rely on a postcommit authoritative receipt write.

### State machine

```text
START
  -> PREPARE_TEMP
  -> WRITE_PRECOMMIT_MANIFEST_RECEIPTS
  -> VERIFY_PRECOMMIT_MANIFEST_RECEIPTS
  -> VERIFY_PRECOMMIT_PUBLICATION_PREDECESSOR_TOKEN
  -> RENAME_ATTEMPT

RENAME_ATTEMPT
  rename did not occur / failed before destination visibility
    -> NOT_COMMITTED
  rename succeeded or success is indeterminate after destination may be visible
    -> COMMITTED_PENDING_DIAGNOSTICS

COMMITTED_PENDING_DIAGNOSTICS
  optional non-authoritative diagnostics succeed or are skipped by policy
    -> COMMITTED
  optional non-authoritative diagnostics fail or cancellation occurs
    -> COMMITTED_VERIFICATION_FAILED
```

`COMMITTED_PENDING_DIAGNOSTICS` is not externally reportable. It is an internal edge label used only to preserve reasoning. It performs no authoritative custody write.

### Commit point

The commit point is the first instant the final selector name may refer to the candidate after the rename operation. Once the rename reports success, or once the process cannot prove the rename did not become visible, the successor must irrevocably set the externally visible outcome family to committed.

After that point:

- directory `fsync` failure cannot return `NOT_COMMITTED`;
- file `close` failure cannot return `NOT_COMMITTED`;
- reread failure cannot return `NOT_COMMITTED`;
- verifier failure cannot return `NOT_COMMITTED`;
- optional postcommit diagnostic failure cannot return `NOT_COMMITTED`;
- cancellation cannot return `NOT_COMMITTED`;
- panic or helper crash after rename must be recovered, if observable, as committed-family uncertainty rather than ordinary uncommitted failure.

### Retry rules

`NOT_COMMITTED` means a caller may retry the same request after fixing the reported pre-commit cause.

`COMMITTED` means a caller must not retry as a new publication. It may perform idempotent read/verify.

`COMMITTED_VERIFICATION_FAILED` means a caller must not blindly retry. The only permitted follow-up is a reconciliation operation that:

1. pins the current selector path without following symlinks;
2. obtains bounded identity and bytes;
3. compares exact current selector digest and byte length;
4. reads and verifies the precommitted manifest, byte receipt, verified-generation selector receipt, and publication predecessor token receipt;
5. validates `frozen_design_predecessor_commit` and `publication_predecessor_token` as separate identities;
6. returns either exact idempotent success or tamper/collision failure.

The result message for `COMMITTED_VERIFICATION_FAILED` must explicitly discourage blind retry and identify the reconciliation operation or operator playbook.

### Cancellation points and partial results

Cancellation is honored before rename and may return `NOT_COMMITTED` if the implementation proves the final selector was not installed. Cancellation after rename, during directory sync, close, reread, verifier, or optional diagnostic recording must return `COMMITTED_VERIFICATION_FAILED` with a partial result containing at least:

- selector;
- expected current selector digest and byte length;
- candidate identity if known;
- frozen design predecessor commit input;
- publication predecessor token input;
- manifest generation if known;
- whether final name visibility was observed;
- the failing post-commit step.

If any of those fields are unavailable, the field is marked `unknown`; absence must not be interpreted as uncommitted or as custody.

## Defect 2: exact immutable collision handling

### Problem frozen in predecessor

Existing manifest or alias collisions can be treated as reusable without proving exact immutable identity. That risks accepting a stale, substituted, symlinked, hardlinked, wrong-owner, writable, or different-byte object as the candidate.

### Successor identity rule

For an existing manifest or alias to satisfy publication idempotently, the successor must establish exact immutable collision identity:

1. pin the existing path with no symlink following;
2. require a regular file identity;
3. enforce explicit local ownership and mode policy;
4. require `nlink == 1` and reject all hardlinks;
5. bound bytes before reading;
6. compute raw digest and byte length;
7. compute canonical byte equality;
8. bind the object to the same candidate id, selector, manifest generation, byte receipt, and publication predecessor token;
9. bind the design context separately to `frozen_design_predecessor_commit`;
10. return exact idempotent success only if every equality holds.

Any deviation is a collision or tamper failure, not success.

### Pinned/no-follow regular identity

The implementation design must use descriptor-based or equivalent pinned identity on Darwin and Linux:

- open with no-follow semantics where available;
- reject symlinks, directories, devices, sockets, FIFOs, hardlinks, and non-regular objects;
- record device/inode or platform-equivalent stable file identity after open;
- compare path `lstat` and descriptor `fstat` before and after read;
- require descriptor/name inode equality before and after verification;
- reject if name-to-object identity changes during verification.

The object read for digest comparison must be the pinned object, not a path reopened after validation.

### UID, mode, and link policy

The successor must choose one explicit repository-local owner policy and report it in the result:

```text
owner_policy = REPOSITORY_ROOT_OWNER | CURRENT_EFFECTIVE_OWNER
```

For lock, manifest, alias, and receipt files, the file uid must equal the chosen policy uid. The mode must reject group write and world write. The link count must be exactly `1`; all hardlinks are rejected. These checks run before and after the descriptor/name equality check.

### Bounded bytes and canonical equality

The successor must define maximum manifest, alias, and receipt byte lengths. If the existing object exceeds the bound, the result is `COLLISION_UNVERIFIED_BOUNDS` or equivalent fail-closed collision, never idempotent success.

Exact collision identity requires all of the following:

- raw digest equality;
- raw byte length equality;
- canonical bytes equality under the existing canonical encoding;
- candidate id equality;
- selector equality;
- manifest generation equality;
- byte receipt digest and byte receipt length equality;
- publication predecessor token equality, including exact `ABSENT` versus `{selector,digest,byte_length}` distinction;
- separate frozen design predecessor commit equality.

If canonicalization itself fails, the collision is not exact.

### Collision state machine

```text
EXISTING_NAME_FOUND
  -> PIN_NOFOLLOW
  -> REGULAR_IDENTITY_CHECK
  -> OWNER_MODE_NLINK_CHECK
  -> PRE_READ_DESCRIPTOR_NAME_INODE_EQUALITY
  -> BOUNDED_READ
  -> RAW_DIGEST_LENGTH_COMPARE
  -> CANONICAL_PARSE_AND_COMPARE
  -> CANDIDATE_SELECTOR_GENERATION_RECEIPT_COMPARE
  -> PUBLICATION_PREDECESSOR_TOKEN_COMPARE
  -> FROZEN_DESIGN_PREDECESSOR_COMPARE
  -> POST_READ_DESCRIPTOR_NAME_INODE_EQUALITY

all checks pass
  -> EXACT_IDEMPOTENT
any check fails or is unsupported
  -> IMMUTABLE_COLLISION_FAILED_CLOSED
```

`EXACT_IDEMPOTENT` is the only non-error outcome for existing manifest/alias collision.

## Defect 3: no default lookup custody bypass

### Problem frozen in predecessor

A compatibility lookup can locate a candidate and accidentally serve as custody evidence. That bypasses verified manifest generation, byte receipt, verified-generation selector receipt, publication predecessor token, and exact current selector verification.

### Successor custody rule

Default lookup has no custody authority. An alias locator may locate a candidate only. Custody requires a verified custody result assembled from all required precommitted identity components and the exact current selector.

### Custody identities

A custody result is valid only when it contains:

- `candidate_id`;
- `selector`;
- exact current selector digest and byte length;
- `manifest_generation`;
- `manifest_digest`;
- `manifest_length`;
- `byte_receipt_digest`;
- `byte_receipt_length`;
- `verified_generation_selector` identity;
- `publication_predecessor_token = ABSENT | {selector,digest,byte_length}`;
- `frozen_design_predecessor_commit = f6e4315ab242fb3c9eea1c1a30ab9efb093090d8`;
- `publication_outcome` in the closed enum from defect 1;
- `custody_status` describing whether the selector is verified, committed, or committed-verification-failed.

Alias lookup may return `candidate_id` and locator metadata, but it must not return `custody_status=verified` or any equivalent custody-bearing status, even if it locates a valid candidate.

### API inputs and outputs

The successor specifies two distinct private repository-local operation families.

#### Non-custody locator

Input:

```text
LocateCandidateByAlias(alias, options)
```

Required fields:

- alias string;
- repository root or publication namespace;
- no-follow preference if available;
- maximum lookup bytes.

Output:

```text
CandidateLocatorResult {
  located: bool,
  candidate_id?: string,
  selector_hint?: string,
  alias_identity?: bounded pinned identity summary,
  custody: "NOT_CUSTODY",
  warnings: ["alias locator locates candidate only; not custody"]
}
```

This output is never sufficient for publication custody and has no public/API/stakeholder semantic authority.

#### Custody verifier

Input:

```text
VerifyCandidatePublicationCustody(request)
```

Required request fields are exactly the separated request schema in the identity section. Missing or wrong selector digest, selector byte length, manifest generation, byte receipt, verified-generation selector, publication predecessor token, or frozen design predecessor commit fails closed. A correct frozen design predecessor commit cannot repair wrong selector digest or length.

Output is exactly the separated result schema in the identity section.

`VERIFIED` requires manifest generation plus byte receipt plus verified-generation selector plus exact current selector plus publication predecessor token plus separate frozen design predecessor binding. Any missing component is `FAILED_CLOSED` or `COMMITTED_VERIFICATION_FAILED` depending on whether rename may already have committed.

## Defect 4: stable lock inode

### Problem frozen in predecessor

A lock file that is unlinked and recreated during ordinary operation permits holder substitution and can defeat assumptions made by `flock`-based serialization.

### Successor lock rule

There is one repository-local regular non-symlink lock file per publication namespace. It is created safely once and retained across holders and restarts. Ordinary operation must never unlink or recreate it.

### Lock identity

Before `flock`, the successor validates:

- repository-local path under the expected publication namespace;
- exact lock file name;
- no symlink in the final component;
- regular file type;
- uid equals the chosen explicit local policy uid: repository root owner or current effective owner;
- mode is not group writable and not world writable;
- `nlink == 1` exactly;
- descriptor/name inode equality when the platform exposes inode identity;
- device/inode or equivalent file identity is captured.

After `flock`, before performing publication, and before releasing the lock, the successor repeats validation and confirms that the descriptor identity, path identity, name, uid, mode, link count, and inode assumptions still match.

### Creation

If the lock file does not exist, creation must be atomic and no-follow. The implementation must reject preexisting symlink, hardlink, or non-regular file. On successful creation, the file remains in place permanently. Cleanup tools may truncate content if necessary, but must not remove the lock file while ordinary publication can run.

### Substitution handling

If any pre-flock or post-flock validation observes substitution, unsupported metadata, link-count anomaly, uid or mode violation, path escape, or name mismatch, the successor fails closed:

```text
LOCK_SUBSTITUTION_FAILED_CLOSED
```

It must not continue with a newly created replacement lock in the same operation. Operator repair is explicit: stop all publishers, inspect the repository-local lock path, restore a regular retained lock file, and restart.

### Platform support

Supported platforms are Darwin and Linux only, where descriptor identity, no-follow checks, regular file checks, and advisory `flock` or equivalent repository-local serialization can be implemented with bounded assumptions.

Unsupported platforms fail closed before publication with:

```text
UNSUPPORTED_PLATFORM_FAILED_CLOSED
```

Non-custody locators may still read non-custody metadata on unsupported platforms if they do not claim custody.

## Combined publication state machine

```text
REQUEST
  -> PLATFORM_CHECK
  -> LOCK_PRECHECK_OR_CREATE
  -> FLOCK
  -> LOCK_POSTCHECK
  -> EXISTING_SELECTOR_CHECK

EXISTING_SELECTOR_CHECK
  existing exact immutable collision
    -> EXACT_IDEMPOTENT / COMMITTED
  existing non-exact collision
    -> FAILED_CLOSED
  no existing selector
    -> WRITE_AND_VERIFY_PRECOMMIT_CUSTODY_MATERIAL

WRITE_AND_VERIFY_PRECOMMIT_CUSTODY_MATERIAL
  missing or wrong frozen_design_predecessor_commit
    -> NOT_COMMITTED / FAILED_CLOSED
  missing or wrong publication_predecessor_token
    -> NOT_COMMITTED / FAILED_CLOSED
  missing or wrong current selector digest/length expectation
    -> NOT_COMMITTED / FAILED_CLOSED
  all custody-bearing receipts written and verified
    -> RENAME_COMMIT_BOUNDARY

RENAME_COMMIT_BOUNDARY
  rename proven not visible
    -> NOT_COMMITTED
  rename succeeded or visibility indeterminate
    -> POSTCOMMIT_NONAUTHORITATIVE_DIAGNOSTICS

POSTCOMMIT_NONAUTHORITATIVE_DIAGNOSTICS
  diagnostics succeed or are skipped
    -> COMMITTED
  diagnostics fail/cancel/unknown
    -> COMMITTED_VERIFICATION_FAILED
```

Lock substitution, unsupported platform, non-exact collision, and custody bypass attempts fail closed. If they occur after the rename commit boundary, the publication outcome is still committed-family and must not be represented as ordinary `NOT_COMMITTED`.

## RED matrix

The implementation phase must add named RED tests before code changes for at least the following matrix. Test names are normative suggestions; exact package placement is out of scope for this design phase.

| Test name | Required RED assertion |
| --- | --- |
| `TestPublishSeparatesFrozenDesignPredecessorFromPublicationPredecessorToken` | A correct `frozen_design_predecessor_commit` does not satisfy a missing or wrong `publication_predecessor_token`, and a correct publication token does not satisfy a wrong frozen Git predecessor. |
| `TestPublishRejectsWrongSelectorDigestWithCorrectFrozenCommit` | Wrong current selector digest fails closed even when frozen design predecessor commit is correct. |
| `TestPublishRejectsMissingSelectorDigestWithCorrectFrozenCommit` | Missing current selector digest fails closed even when frozen design predecessor commit is correct. |
| `TestPublishRejectsWrongSelectorByteLengthWithCorrectFrozenCommit` | Wrong current selector byte length fails closed even when frozen design predecessor commit is correct. |
| `TestPublishRejectsMissingSelectorByteLengthWithCorrectFrozenCommit` | Missing current selector byte length fails closed even when frozen design predecessor commit is correct. |
| `TestPublishWritesAndVerifiesCustodyBeforeRename` | Manifest, byte receipt, verified-generation selector, and publication predecessor token are written and verified before current selector rename. |
| `TestPublishDoesNotWriteAuthoritativeReceiptAfterCommit` | No authoritative custody receipt write occurs after rename commit boundary. |
| `TestPostcommitDiagnosticFailureReturnsCommittedVerificationFailed` | Failure of optional postcommit diagnostics after successful rename returns `COMMITTED_VERIFICATION_FAILED`, never `NOT_COMMITTED`, and does not create custody. |
| `TestFreshPublishCommittedWithPrecommittedCustody` | Fresh publish returns `COMMITTED`, verified precommitted custody fields, exact current selector digest/length, separate frozen Git predecessor, and publication predecessor token. |
| `TestPreRenameFailureReturnsNotCommitted` | Failure before final selector visibility returns `NOT_COMMITTED` and allows safe precommit retry. |
| `TestPostRenameDirectorySyncFailureCommittedFamily` | Rename success plus directory sync failure returns `COMMITTED_VERIFICATION_FAILED`, never `NOT_COMMITTED`, and retry=`RECONCILE_ONLY`. |
| `TestPostRenameCloseFailureCommittedFamily` | Close failure after rename returns committed-family partial result. |
| `TestPostRenameRereadFailureCommittedFamily` | Reread failure after rename returns committed-family partial result. |
| `TestPostRenameVerifierFailureCommittedFamily` | Verifier failure after rename returns committed-family partial result and discourages blind retry. |
| `TestCancellationBeforeRenameNotCommitted` | Cancellation before final selector exists returns `NOT_COMMITTED`. |
| `TestCancellationAfterRenameCommittedVerificationFailed` | Cancellation after rename returns `COMMITTED_VERIFICATION_FAILED`. |
| `TestExistingExactManifestCollisionIdempotent` | Existing pinned regular manifest with exact raw digest, length, canonical bytes, candidate id, selector, generation, byte receipt, publication predecessor token, and frozen Git predecessor returns exact idempotent success. |
| `TestExistingStaleManifestCollisionFailsClosed` | Same selector with mismatched digest or length fails closed. |
| `TestExistingCanonicalMismatchFailsClosed` | Raw bytes or canonical reserialization mismatch fails closed. |
| `TestExistingSymlinkAliasFailsClosed` | Symlink at alias or manifest path fails closed. |
| `TestExistingDirectoryDeviceAliasFailsClosed` | Non-regular alias or manifest fails closed. |
| `TestExistingOversizedManifestFailsClosed` | Object exceeding byte bound fails closed, not idempotent. |
| `TestAliasLocatorNoCustodyEvenForValidCandidate` | Alias locator returns `custody=NOT_CUSTODY` and no verified status even when it locates a valid candidate. |
| `TestCustodyVerifierRejectsMissingManifestGeneration` | Custody verifier without generation fails closed. |
| `TestCustodyVerifierRejectsMissingByteReceipt` | Custody verifier without byte receipt fails closed. |
| `TestCustodyVerifierRejectsMissingVerifiedGenerationSelector` | Custody verifier without verified-generation selector fails closed. |
| `TestCustodyVerifierRejectsMissingPublicationPredecessorToken` | Custody verifier without publication predecessor token field fails closed; explicit `ABSENT` is accepted only when expected and verified. |
| `TestCustodyVerifierRejectsWrongPublicationPredecessorToken` | Custody verifier with wrong selector/digest/length predecessor token fails closed. |
| `TestStableLockRetainsInodeAcrossSequentialHolders` | Reused retained lock inode works across two sequential holders. |
| `TestStableLockRetainsInodeAcrossRestart` | Existing retained lock file after process restart is reused, not unlinked. |
| `TestStableLockSerializesHelperProcessesSameInode` | Concurrent helper processes serialize on the same descriptor identity. |
| `TestLockRejectsSymlinkTamper` | Symlink lock path fails closed before publication. |
| `TestLockRejectsInodeSubstitutionBeforeFlock` | Replacement between precheck and flock fails closed. |
| `TestLockRejectsInodeSubstitutionAfterFlock` | Replacement after flock but before publication fails closed. |
| `TestLockRejectsHardlinkNlinkNotOne` | Lock, manifest, alias, or receipt with `nlink != 1` fails closed. |
| `TestLockRejectsWrongUIDPolicy` | UID not equal to chosen repository root/current effective owner policy fails closed. |
| `TestLockRejectsGroupOrWorldWritableMode` | Group- or world-writable lock, manifest, alias, or receipt fails closed. |
| `TestLockNeverUnlinksOrRecreatesDuringOrdinaryOperation` | Ordinary operation never unlinks/recreates; test detects inode retention. |
| `TestUnsupportedPlatformFailsClosed` | Publication fails closed before custody claim. |
| `TestHelperCrashPostRenameCommittedFamily` | Parent reconciliation reports committed-family partial, not uncommitted. |
| `TestRaceExistingExactCollisionIdempotentOnlyAfterFullCheck` | Racing publisher that installs exact same bytes yields exact idempotent only after full immutable check. |
| `TestRaceExistingConflictingCollisionFailsClosed` | Racing publisher with different bytes fails closed. |

## Compatibility and non-goals

Compatibility is private repository-local compatibility only:

- preserve previously qualified ADR0007 authority ceilings;
- preserve non-custody locator behavior for local discovery;
- add closed enums and custody fields without creating public type/API authority;
- keep locator results explicitly non-custody.

Non-goals:

- no public type compatibility promise;
- no public API, CLI, MCP, registry, or release surface;
- no production authorization;
- no stakeholder semantic authority or feature acceptance;
- no automatic migration of predecessor artifacts;
- no broad filesystem portability beyond Darwin/Linux;
- no network/distributed lock semantics;
- no attempt to prove stakeholder feature identity;
- no code or test edits in this design phase.

## Exact boundaries

This successor design is private and repository-local. It governs only candidate publication custody and exact idempotent collision handling for the frozen predecessor lineage. It does not govern unrelated ADR0007 Describe, Search, Group, Location, or Source Text Search custody except by preserving their existing authority ceilings.

A future implementation may claim conformance only if it demonstrates the RED matrix, preserves closed outcome semantics, writes and verifies custody-bearing material before current selector rename, binds exact collision identity to raw digest, length, canonical bytes, candidate id, selector, generation, byte receipt, and publication predecessor token, and keeps `frozen_design_predecessor_commit` separate from `publication_predecessor_token`. Until then, `f6e4315ab242fb3c9eea1c1a30ab9efb093090d8` remains `CANDIDATE_PUBLICATION_CUSTODY_BLOCKED`.
