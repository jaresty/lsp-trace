# ADR0007 private candidate publication successor design

- **Date:** 2026-10-08
- **Successor name:** `ADR0007_PRIVATE_CANDIDATE_PUBLICATION_SUCCESSOR_V1`
- **Predecessor identity:** `f6e4315ab242fb3c9eea1c1a30ab9efb093090d8`
- **Predecessor disposition:** `CANDIDATE_PUBLICATION_CUSTODY_BLOCKED`
- **Successor phase:** `DESIGN_ONLY`
- **Successor verdict target:** `CANDIDATE_PUBLICATION_SUCCESSOR_DESIGN_READY`
- **Authority ceiling:** `authority=0`, `accepted=false`, `featureIdentity=UNRESOLVED`, `completeness=UNKNOWN`

This document is an additive private design artifact. It freezes predecessor `f6e4315ab242fb3c9eea1c1a30ab9efb093090d8` as `CANDIDATE_PUBLICATION_CUSTODY_BLOCKED` and defines successor semantics for exactly four defects. It makes no implementation claims, does not change Go code or tests, and does not authorize public API, CLI, MCP, registry, production, release, or stakeholder feature acceptance.

## Scope and preservation rule

The successor preserves existing APIs and qualified semantics unless this document explicitly narrows or qualifies them. Existing call names, result shapes, selector vocabulary, and custody terminology should remain source-compatible where possible. New fields may be additive; changed meanings must be represented as stricter state/custody interpretation rather than silent widening.

The four defects addressed here are exhaustive for this successor:

1. closed selector outcome states and rename-commit semantics;
2. exact immutable collision handling for existing manifest/alias identities;
3. removal of default lookup custody bypass;
4. stable repository-local lock inode.

Everything else is out of scope and remains as in the frozen predecessor or prior qualified ADR0007 artifacts.

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

### State machine

```text
START
  -> PREPARE_TEMP
  -> VERIFY_PRE_RENAME
  -> RENAME_ATTEMPT

RENAME_ATTEMPT
  rename did not occur / failed before destination visibility
    -> NOT_COMMITTED
  rename succeeded or success is indeterminate after destination may be visible
    -> COMMITTED_PENDING_POSTCHECK

COMMITTED_PENDING_POSTCHECK
  post-rename directory sync, close, reread, and verifier all succeed
    -> COMMITTED
  any post-rename directory sync, close, reread, verifier, or receipt-write failure occurs
    -> COMMITTED_VERIFICATION_FAILED
```

`COMMITTED_PENDING_POSTCHECK` is not externally reportable. It is an internal edge label used only to preserve reasoning.

### Commit point

The commit point is the first instant the final selector name may refer to the candidate after the rename operation. Once the rename reports success, or once the process cannot prove the rename did not become visible, the successor must irrevocably set the externally visible outcome family to committed.

After that point:

- directory `fsync` failure cannot return `NOT_COMMITTED`;
- file `close` failure cannot return `NOT_COMMITTED`;
- reread failure cannot return `NOT_COMMITTED`;
- verifier failure cannot return `NOT_COMMITTED`;
- cancellation cannot return `NOT_COMMITTED`;
- panic or helper crash after rename must be recovered, if observable, as committed-family uncertainty rather than ordinary uncommitted failure.

### Retry rules

`NOT_COMMITTED` means a caller may retry the same request after fixing the reported pre-commit cause.

`COMMITTED` means a caller must not retry as a new publication. It may perform idempotent read/verify.

`COMMITTED_VERIFICATION_FAILED` means a caller must not blindly retry. The only permitted follow-up is a reconciliation operation that:

1. pins the selector path without following symlinks;
2. obtains bounded identity and bytes;
3. compares canonical bytes to the expected digest and length;
4. validates predecessor binding and manifest generation;
5. returns either exact idempotent success or tamper/collision failure.

The result message for `COMMITTED_VERIFICATION_FAILED` must explicitly discourage blind retry and identify the reconciliation API or operator playbook.

### Cancellation points and partial results

Cancellation is honored before rename and may return `NOT_COMMITTED` if the implementation proves the final selector was not installed. Cancellation after rename, during directory sync, close, reread, verifier, or receipt recording must return `COMMITTED_VERIFICATION_FAILED` with a partial result containing at least:

- selector;
- expected digest and length;
- candidate identity if known;
- predecessor binding input;
- operation generation if known;
- whether final name visibility was observed;
- the failing post-commit step.

If any of those fields are unavailable, the field is marked `unknown`; absence must not be interpreted as uncommitted.

## Defect 2: exact immutable collision handling

### Problem frozen in predecessor

Existing manifest or alias collisions can be treated as reusable without proving exact immutable identity. That risks accepting a stale, substituted, symlinked, or different-byte object as the candidate.

### Successor identity rule

For an existing manifest or alias to satisfy publication idempotently, the successor must establish exact immutable collision identity:

1. pin the existing path with no symlink following;
2. require a regular file identity;
3. bound bytes before reading;
4. compute canonical byte equality;
5. compare expected digest and expected byte length;
6. bind the object to the same candidate id, selector, manifest generation, and predecessor identity;
7. return exact idempotent success only if every equality holds.

Any deviation is a collision or tamper failure, not success.

### Pinned/no-follow regular identity

The implementation design must use descriptor-based or equivalent pinned identity on Darwin and Linux:

- open with no-follow semantics where available;
- reject symlinks, directories, devices, sockets, FIFOs, hard-link surprises outside policy, and non-regular objects;
- record device/inode or platform-equivalent stable file identity after open;
- compare path lstat and descriptor fstat before and after read when the platform exposes them;
- reject if name-to-object identity changes during verification.

The object read for digest comparison must be the pinned object, not a path reopened after validation.

### Bounded bytes and canonical equality

The successor must define maximum manifest and alias byte lengths. If the existing object exceeds the bound, the result is `COLLISION_UNVERIFIED_BOUNDS` or equivalent fail-closed collision, never idempotent success.

Canonical byte equality means equality of the normalized serialized manifest/alias bytes under the existing canonical encoding. If the predecessor format has multiple encodings for the same logical fields, the successor must compare both:

- raw stored byte digest and length; and
- canonical reserialization digest and length.

If canonicalization itself fails, the collision is not exact.

### Collision state machine

```text
EXISTING_NAME_FOUND
  -> PIN_NOFOLLOW
  -> REGULAR_IDENTITY_CHECK
  -> BOUNDED_READ
  -> RAW_DIGEST_LENGTH_COMPARE
  -> CANONICAL_PARSE_AND_COMPARE
  -> BINDING_COMPARE

all checks pass
  -> EXACT_IDEMPOTENT
any check fails or is unsupported
  -> IMMUTABLE_COLLISION_FAILED_CLOSED
```

`EXACT_IDEMPOTENT` is the only non-error outcome for existing manifest/alias collision.

## Defect 3: no default lookup custody bypass

### Problem frozen in predecessor

A compatibility lookup can locate a candidate and accidentally serve as custody evidence. That bypasses verified manifest generation, byte receipt, selector binding, and predecessor binding.

### Successor custody rule

Default lookup has no custody authority. A compatibility alias may locate a candidate only. Custody requires a verified custody receipt assembled from all required identity components.

### Custody identities

A custody result is valid only when it contains:

- `candidate_id`;
- `selector`;
- `manifest_generation`;
- `manifest_digest`;
- `manifest_length`;
- `byte_receipt_digest`;
- `byte_receipt_length`;
- `predecessor_commit = f6e4315ab242fb3c9eea1c1a30ab9efb093090d8` or the explicitly supplied successor predecessor binding;
- `publication_outcome` in the closed enum from defect 1;
- `custody_status` describing whether the selector is verified, committed, or committed-verification-failed.

Compatibility lookup may return `candidate_id` and locator metadata, but it must not return `custody_status=verified` or equivalent custody-bearing status.

### API inputs and outputs

The successor specifies two distinct API families.

#### Compatibility locator

Input:

```text
LookupCandidateByAlias(alias, options)
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
  warnings: ["compatibility alias locates candidate only"]
}
```

This output is never sufficient for publication custody.

#### Custody verifier

Input:

```text
VerifyCandidatePublicationCustody(request)
```

Required request fields:

- `candidate_id`;
- `selector`;
- `expected_manifest_generation`;
- `expected_manifest_digest`;
- `expected_manifest_length`;
- `expected_byte_receipt_digest`;
- `expected_byte_receipt_length`;
- `predecessor_commit`;
- `repository_root`;
- platform support declaration;
- maximum manifest, alias, and receipt bytes.

Output:

```text
CandidatePublicationCustodyResult {
  custody_status: VERIFIED | COMMITTED_VERIFICATION_FAILED | NOT_COMMITTED | FAILED_CLOSED,
  publication_outcome: NOT_COMMITTED | COMMITTED | COMMITTED_VERIFICATION_FAILED,
  selector: string,
  candidate_id: string,
  manifest_generation: string,
  manifest_digest: string,
  manifest_length: integer,
  byte_receipt_digest: string,
  byte_receipt_length: integer,
  predecessor_commit: string,
  predecessor_binding: EXACT | MISMATCH | UNKNOWN,
  idempotent: bool,
  retry: NEVER_BLIND | SAFE_PRECOMMIT_RETRY | RECONCILE_ONLY,
  partial?: object,
  diagnostics: bounded diagnostics
}
```

`VERIFIED` requires manifest generation plus byte receipt plus selector plus predecessor binding. Any missing component is `FAILED_CLOSED` or `COMMITTED_VERIFICATION_FAILED` depending on whether rename may already have committed.

### Predecessor binding

The default predecessor binding for this successor is exactly `f6e4315ab242fb3c9eea1c1a30ab9efb093090d8`. A caller may supply a different predecessor only if the API explicitly names it as a successor input. Silent fallback to current HEAD, branch name, or alias lookup is prohibited.

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
- owner is the expected repository owner or current effective owner under documented local policy;
- mode is not group/world writable unless explicitly supported by repository policy;
- link count is acceptable for the platform policy;
- device/inode or equivalent file identity is captured.

After `flock`, before performing publication, and before releasing the lock, the successor repeats validation and confirms that the descriptor identity, path identity, name, owner, mode, link count, and inode assumptions still match.

### Creation

If the lock file does not exist, creation must be atomic and no-follow. The implementation must reject preexisting symlink or non-regular file. On successful creation, the file remains in place permanently. Cleanup tools may truncate content if necessary, but must not remove the lock file while ordinary publication can run.

### Substitution handling

If any pre-flock or post-flock validation observes substitution, unsupported metadata, link-count anomaly, owner/mode violation, path escape, or name mismatch, the successor fails closed:

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

Compatibility locators may still read non-custody metadata on unsupported platforms if they do not claim custody.

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
    -> PREPARE_TEMP

PREPARE_TEMP
  cancellation or failure before rename, final selector proven absent
    -> NOT_COMMITTED
  pre-rename verification failure
    -> NOT_COMMITTED
  rename attempted
    -> RENAME_COMMIT_BOUNDARY

RENAME_COMMIT_BOUNDARY
  rename proven not visible
    -> NOT_COMMITTED
  rename succeeded or visibility indeterminate
    -> POST_RENAME_VERIFY

POST_RENAME_VERIFY
  all checks pass
    -> COMMITTED
  any check fails/cancelled/unknown
    -> COMMITTED_VERIFICATION_FAILED
```

Lock substitution, unsupported platform, non-exact collision, and custody bypass attempts fail closed. If they occur after the rename commit boundary, the publication outcome is still committed-family and must not be represented as ordinary `NOT_COMMITTED`.

## RED matrix

The implementation phase must add RED tests before code changes for at least the following matrix. Test names are illustrative; exact package placement is out of scope for this design phase.

| Class | Required RED assertion |
| --- | --- |
| Normal success | Fresh publish returns `COMMITTED`, verified custody fields, exact predecessor binding. |
| Pre-rename failure | Failure before final selector visibility returns `NOT_COMMITTED` and allows safe precommit retry. |
| Post-rename directory sync failure | Rename success plus directory sync failure returns `COMMITTED_VERIFICATION_FAILED`, never `NOT_COMMITTED`, and retry=`RECONCILE_ONLY`. |
| Post-rename close failure | Close failure after rename returns committed-family partial result. |
| Post-rename reread failure | Reread failure after rename returns committed-family partial result. |
| Post-rename verifier failure | Verifier failure after rename returns committed-family partial result and discourages blind retry. |
| Cancellation before rename | Cancellation before final selector exists returns `NOT_COMMITTED`. |
| Cancellation after rename | Cancellation after rename returns `COMMITTED_VERIFICATION_FAILED`. |
| Existing exact manifest collision | Existing pinned regular manifest with exact digest, length, canonical bytes, selector, generation, receipt, and predecessor returns exact idempotent success. |
| Existing stale manifest collision | Same selector with mismatched digest or length fails closed. |
| Existing canonical mismatch | Raw bytes or canonical reserialization mismatch fails closed. |
| Existing symlink alias | Symlink at alias or manifest path fails closed. |
| Existing directory/device alias | Non-regular alias or manifest fails closed. |
| Oversized manifest | Object exceeding byte bound fails closed, not idempotent. |
| Alias lookup custody bypass | Compatibility alias lookup returns locator only with `custody=NOT_CUSTODY`. |
| Missing manifest generation | Custody verifier without generation fails closed. |
| Missing byte receipt | Custody verifier without byte receipt fails closed. |
| Missing selector | Custody verifier without selector fails closed. |
| Missing predecessor binding | Custody verifier without predecessor fails closed. |
| Wrong predecessor binding | Custody verifier with predecessor not equal to request binding fails closed. |
| Stable lock normal | Reused retained lock inode works across two sequential holders. |
| Stable lock restart | Existing retained lock file after process restart is reused, not unlinked. |
| Stable lock race | Concurrent helper processes serialize on the same descriptor identity. |
| Lock symlink tamper | Symlink lock path fails closed before publication. |
| Lock inode substitution before flock | Replacement between precheck and flock fails closed. |
| Lock inode substitution after flock | Replacement after flock but before publication fails closed. |
| Lock owner/mode tamper | Unexpected owner or writable mode fails closed. |
| Lock unlink/recreate attempt | Ordinary operation never unlinks/recreates; test detects inode retention. |
| Unsupported platform | Publication fails closed before custody claim. |
| Helper-process crash post-rename | Parent reconciliation reports committed-family partial, not uncommitted. |
| Race existing collision | Racing publisher that installs exact same bytes yields exact idempotent only after full immutable check. |
| Race conflicting collision | Racing publisher with different bytes fails closed. |

## Compatibility and non-goals

Compatibility goals:

- preserve existing public type names and call sites where possible;
- add closed enums and custody fields without weakening old result interpretation;
- allow compatibility alias lookup for candidate discovery only;
- preserve qualified ADR0007 semantics and authority ceiling.

Non-goals:

- no public publication service;
- no release or production authorization;
- no semantic feature acceptance;
- no automatic migration of predecessor artifacts;
- no broad filesystem portability beyond Darwin/Linux;
- no network/distributed lock semantics;
- no attempt to prove stakeholder feature identity;
- no code or test edits in this design phase.

## Exact boundaries

This successor design is private and repository-local. It governs only candidate publication custody and exact idempotent collision handling for the frozen predecessor lineage. It does not govern unrelated ADR0007 Describe, Search, Group, Location, or Source Text Search custody except by preserving their existing authority ceilings.

A future implementation may claim conformance only if it demonstrates the RED matrix, preserves the closed outcome semantics, and binds custody to verified manifest generation, byte receipt, selector, and predecessor identity. Until then, `f6e4315ab242fb3c9eea1c1a30ab9efb093090d8` remains `CANDIDATE_PUBLICATION_CUSTODY_BLOCKED`.
