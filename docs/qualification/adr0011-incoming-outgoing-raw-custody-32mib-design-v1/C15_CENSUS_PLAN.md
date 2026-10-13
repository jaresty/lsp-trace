# Correction-safe C15 and directional-ledger relationship/delta plan

Status: proposal only. No row is claimed added, accepted, complete, or measured.

## Two scopes that must remain separate

1. **C15 backing custody** classifies concrete allocations, owners, copies, aliases, transfers, and releases. Baseline: 54 logical backing seams (15 governed owners, 7 aliases, 22 separately charged copies, 4 policy-blocked, 5 test-only exclusions, 1 opaque scratch exclusion), with aggregate acceptance withheld.
2. **Directional reservation ledger v1** is Manager-owned admission/accounting policy with one active slot, per-attempt ceiling `33554432`, and aggregate-live ceiling `33554432`. It is not C15 backing, a C15 owner, a child C15 lease, or a debit from `privateB4ByteAccountV2`/`privateB4MaxOwnedBytes`.

## Exhaustive candidate backing rows

Enumerate, without presuming classification: prepare request frame/params; prepare response frame/result; failed-read prefix; selected-item backing; directional request params/frame; directional response frame/result; observation backing; mapping/ledger record backing; any scanner scratch; all aliases, transfers, and release operations. Each source allocation/copy site and each distinct retained backing must appear exactly once in the C15 candidate set. Variables/views without backing do not create rows.

## Directional rows

Enumerate reservation admission, component-map insertions, slot/live/cumulative transitions, refusal, release, duplicate-release no-op, and fresh-identity reuse. These are directional ledger/operation rows, never C15 backing rows.

## Relationship table and invariant

Produce `BACKING_DIRECTIONAL_RELATION.tsv` with direction, attempt ID, backing ID, source fingerprint, C15 classification/row, directional component, capacity, alias/transfer flags, release operation ID, and a newly namespaced prefix mapping ID where applicable. Prefix mapping IDs are correlation-only: they cannot equal, replace, append to, reorder, or feed any history identity/entry, predecessor ID/commitment/digest, generic lineage identity, or lifecycle decision.

Require:

```text
one live backing identity -> exactly one C15 classification
one (attempt, backing identity) -> at most one byte-bearing directional component
aliases contribute zero directional bytes
transfers never create two backing charges
sum(distinct mapped capacities) = directional attempt total
no directional reservation row appears as C15 owner/copy backing
no C15 debit is nested under or charged to the directional ledger
all retained directional backings have a relationship row
prefix mapping IDs produce zero history/predecessor/lineage deltas
```

## Procedure and artifacts

1. Pin exact source/generator bytes and enumerate the complete accepted write-set.
2. Produce finite C15 before/after sets: `ADDED.tsv`, `CHANGED.tsv`, `REMOVED.tsv`, `UNCHANGED.tsv`, source locations, candidates, classifications, operation delta, counts, and unresolved ledger.
3. Separately produce directional ledger before/after rows and transition delta.
4. Join scopes only in `BACKING_DIRECTIONAL_RELATION.tsv`; reconcile C15 counts, directional bytes, and non-duplication independently.
5. Independently adjudicate every C15 candidate as governed owner, accepted alias, separately charged copy, policy-blocked, test-only exclusion, opaque-scratch exclusion, or unresolved.
6. Verify scanner prefix ownership, alias non-escape, transfer atomicity, reservation-before-effect, clear-before-release, exact-once release, and fresh identity.
7. Run enumeration twice from identical pins and require byte-identical outputs.

Unknowns remain unresolved. Structural locators nominate candidates only and cannot accept ownership identity. This plan cannot issue C15 or directional-ledger acceptance.
