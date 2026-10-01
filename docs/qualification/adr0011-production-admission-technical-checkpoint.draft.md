# ADR 0011 — production admission technical checkpoint (INCOMPLETE)

This is a bounded technical-prerequisite inventory, **not** production admission, authorization, or a row verdict. Rows 015–017 remain unexecuted; the independent preflight `REVISE` remains effective.

## 1. Owner-selected pins

The private synthetic owner has a separately byte-reviewed 90-file implementation-source manifest, raw SHA-256 `f8cf96e649a5fc051fa6475f8b68fe38bcc346e52fd459bb946d99951f57269a` and aggregate `sha256:b06eca5f8b40b96eee624e932e39c215144a7e2c14162e05cd5896eb83bf475c`. `internal/adr0011acquisition/policy_record.go` privately selects exact policy bytes: method `2fb6c0174945a92b34ccffaa751d15eaa737b7349e2178899d8ed2df00a4710f`, admission `483a0db3ac8f56f043aacd00a7cb7e3be0361dbaf9e595ba0a6d942a245de4f3`, privacy `8d5f9bfba89bab65ad4a44bf89e95f4fd427b7fc2227966ef2be6468e363770f`, retention `5e4a817e8dc8aed4bc6053e18e1f4dfda00a159cf909fed37e0ae39f4d1baa68`. The implementation-only successor-schema mapping selects whole-file SHA-256 `86bb214e419f7c69b80ff5dfbdf0f0760c495c9bf0f88d06bdc227b77920fc8e` for its sixteen roles; predecessor objects may not be promoted. These are **private synthetic / implementation-only selections**, not completed production owner-selected pins or a source-to-binary attestation. The draft row index still has `implementation_schema_policy_digests=null`. Do not fill it with private hashes and claim production closure.

## 2. Replayable production-query admission

`internal/adr0011acquisition/owner.go` explicitly says the owner is test-owned; `NewDisabled` is its only production host construction and has no root or final pin. `acquirePrivateFinal` is default-off, has no public caller, and returns only `PrivateFinalReceipt`. The separate `PublishReferences` production path remains fail-closed with three historical success assertions RED. A synthetic final replay does not establish an independently admitted document-symbol query-target or references occurrence in the production host. The accepted contract requires exact READY managed Go/gopls registered clean worktree, owner-observed before/after Git, prepared source/version/encoding, keyed method evidence, independently selected dependencies, terminal/readback/replay, and separately admitted query-target before references. No public behavior change is authorized by this inventory.

## 3. Cleanup and retention

The pinned proposed retention JSON selects `block_on_exhaustion=true`, `committed_unverified=PRIVATE_NON_ISSUABLE_NO_RETRY`, `issued_dependency_deletion=REPLAY_INCOMPLETE`, 0700 root/0600 verified file, 256 objects/128 MiB transaction and 256 entries/512 KiB uncertainty. These bytes **specify**, but do not verify, the admitted-ledger dependency set, active-input replay, retire/remove versus privacy-revoke decisions, cascading tombstones, unexplained loss, failed deletion, and failed/uncertain publication behavior required by the accepted occurrence contract. Never treat optional debug cleanup as authority to delete issued dependencies.

## Independent technical review boundary

A reviewer must inspect an exact frozen production implementation and its completed role-to-file manifest, test a real admitted query-target/references replay against independent expected identities, and falsify retention/removal/revocation paths before marking these three gates complete. This inventory contains no such evidence and requests no authorization. The next permitted implementation step must preserve the historical public RED expectations until an independently reviewed production-admission change is explicitly authorized; no rows, index, or definition implementation follow from this document.
