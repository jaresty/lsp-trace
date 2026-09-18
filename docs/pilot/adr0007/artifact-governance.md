# G3 artifact governance draft

- **Status:** `PREREQUISITES_DRAFT`
- **Pilot:** `PILOT_DISABLED`
- **Scope:** documentation-only inventory and control requirements.

## Governing rule

Every raw, derived, operational, and deletion-related artifact is potentially source-sensitive. No artifact class may be created, retained, copied, backed up, or deleted during a pilot until it has an accountable owner, access policy, retention period, deletion behavior, and backup treatment.

## Artifact classes

| Class | Examples | Required controls |
|---|---|---|
| Admission | assembled bytes, selectors, ranges, custody and revision metadata | encrypted storage where applicable; restricted access; immutable identity; retention and deletion owner |
| Semantic output | descriptions, limitations, abstentions, errors | source-sensitive access; authority boundary; lineage; invalidation and deletion |
| Embedding/index | vectors, index records, manifests, cache entries | access control; exact identity; rebuild and revocation behavior; no authority elevation |
| Query/result | queries, rankings, filters, returned members, group proposals | query privacy; provenance; denominator and coverage disclosure; retention policy |
| Runtime | model files, tokenizer, native libraries, SBOM, logs | digest pinning; quarantine; license/vulnerability review; network denial |
| Temporary | scratch files, pipes, sockets, staging data, crash dumps | bounded lifetime; cleanup verification; no uncontrolled spill |
| Audit/lineage | admission attempts, corrections, tombstones, deletion receipts | append-only integrity; actor and authority identity; backup treatment |
| Backup/snapshot | filesystem snapshots, archives, recovery copies | inventory; retention; deletion/tombstone propagation; external-retention disclosure |

## Minimum policy fields

A later frozen policy must assign, for each class:

- accountable owner and independent approver;
- storage location and custody boundary;
- encryption and key-management requirements;
- read, write, export, and administrative access roles;
- retention duration or retention condition;
- deletion trigger and verification evidence;
- backup and snapshot treatment;
- revocation and rebuild behavior;
- incident and access-audit requirements;
- permitted environments and network constraints.

## Deletion contract

Deletion by admission ID must:

1. append a deletion request and actor identity;
2. invalidate dependent descriptions, embeddings, indexes, rankings, groups, caches, logs, and temporary files;
3. rebuild or close affected products with explicit dispositions;
4. process backups and snapshots or record `EXTERNALLY_RETAINED` with owner and reason;
5. write tombstones preventing stale reappearance;
6. emit a receipt enumerating `DELETED`, `REBUILT`, `UNAVAILABLE`, and `EXTERNALLY_RETAINED` artifacts.

No secure-erasure claim may be made without independent storage-layer evidence.

## G3 rejection conditions

Keep the pilot disabled for any unclassified artifact, missing owner, missing retention rule, unbounded temporary storage, untracked backup, unresolved external retention, absent access control, deletion without dependency invalidation, or generated artifact that can be mistaken for accepted authority.
