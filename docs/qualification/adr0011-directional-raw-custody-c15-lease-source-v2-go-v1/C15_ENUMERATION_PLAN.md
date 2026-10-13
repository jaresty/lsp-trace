# Correction-safe C15 enumeration plan over current source

Status: `PLANNED_NOT_ENUMERATED`. No final count or disposition is claimed.

## Scope and baseline

Source pin is HEAD `162d796f7e33fea2c2bdb98f92e41744bb728c2d`; exact files and hashes are in `SOURCE_MANIFEST.sha256`. The historical 54-seam census is baseline evidence only. The mechanical run must first identify every prior uncharged relationship relevant to prepare frame, prepare result, selected handoff, directional frame, directional result, and retained external/evidence custody. A historical row is not automatically current or accepted.

Candidate discovery searches current source for allocations, copies, appends, frame/result captures, byte slices retained across calls, publications, owner fields, lease creation/release, alias/view/span construction, transfer/move/invalidation, callbacks, terminal paths, and generated C15 registry entries. Source locators nominate rows; they do not adjudicate ownership.

## Required finite outputs

Produce exact deterministic TSVs:

- `BASELINE_PRIOR_UNCHARGED.tsv`
- `ADDED.tsv`
- `CHANGED.tsv`
- `REMOVED.tsv`
- `UNCHANGED.tsv`
- `ALLOCATION_SITES.tsv`
- `RELEASE_SITES.tsv`
- `BORROW_ALIAS_SITES.tsv`
- `TRANSFER_SITES.tsv`
- `SOURCE_LOCATIONS.tsv`
- `UNRESOLVED.tsv`
- `COUNTS.json`
- `RUN_MANIFEST.sha256`

Expected meaning, not expected count:

| Delta | Meaning |
|---|---|
| `ADDED` | current/new governed relationship absent from accepted baseline |
| `CHANGED` | same stable row identity, changed source/class/lifecycle/lease semantics |
| `REMOVED` | baseline relationship absent at current source with exact deletion evidence |
| `UNCHANGED` | same identity and adjudicated semantics at exact current source |

All four sets are exhaustive, pairwise disjoint, and their union equals baseline plus current candidate identities under the declared reconciliation equation. No actual counts are frozen here. Any produced counts are labeled `PROVISIONAL_MECHANICAL` until independent adjudication.

## Mechanical row schema

Each row has exactly these fields:

```text
row_id
revision
predecessor_row_id
predecessor_revision
status
source_path
source_start_line
source_end_line
source_sha256
symbol
operation_kind = ALLOCATE|COPY|RETAIN|ALIAS|BORROW|TRANSFER|RELEASE|PUBLISH
component = PREPARE_FRAME|PREPARE_RESULT|SELECTED_HANDOFF|DIRECTIONAL_FRAME|DIRECTIONAL_RESULT|RETAINED_EXTERNAL_EVIDENCE
attempt_scope
account_scope
backing_identity_rule
source_lease_identity_rule
destination_lease_identity_rule
capacity_rule
reservation_site
allocation_site
release_site
borrow_alias_site
transfer_site
terminal_paths
baseline_row_id
delta = ADDED|CHANGED|REMOVED|UNCHANGED
provisional_class = GOVERNED_OWNER|ZERO_CHARGE_ALIAS|SEPARATELY_CHARGED_COPY|ATOMIC_TRANSFER|POLICY_BLOCKED|TEST_ONLY_EXCLUSION|OPAQUE_SCRATCH_EXCLUSION|UNRESOLVED
contrary_evidence
rejection_condition
authority_ceiling
adjudicator_id
adjudication_revision
notes
```

`row_id = sha256("adr0011.c15.lease-source.v2.row\x00" || source_path || "\x00" || symbol || "\x00" || operation_kind || "\x00" || component || "\x00" || backing_identity_rule)`. This is the stable logical identity and therefore remains constant across revisions of one relationship. The canonical revision-record hash is `sha256("adr0011.c15.lease-source.v2.revision\x00" || row_id || "\x00" || uint64be(revision) || "\x00" || predecessor_row_id || "\x00" || uint64be(predecessor_revision) || "\x00" || canonical_tsv_row_without_record_hash)`, and is emitted in the run manifest. Paths use repository-relative slash form; symbols are fully qualified package-local names; no line number enters identity, so line movement alone does not create false add/remove. Ambiguous identities become separate `UNRESOLVED` candidates rather than guessed merges.

## Exact correction lineage

The genesis sentinel is exact: `predecessor_row_id=NULL` and `predecessor_revision=0`. It is valid only for `revision=1`. For every later revision, `predecessor_row_id` must equal `row_id`, `predecessor_revision` must equal `revision-1`, and `revision` must increase by exactly one. A correction never overwrites or deletes its predecessor.

Validation builds a graph keyed by `(row_id, revision)` and requires:

- exactly one genesis and exactly one current tip per `row_id`;
- every non-genesis predecessor exists in the same frozen run/evidence closure and its record hash verifies;
- no cycle, self-loop, skipped revision, duplicate revision, missing predecessor, or decreasing revision;
- no branching: more than one record naming the same predecessor is `UNRESOLVED`, not an automatic merge;
- no cross-row predecessor except an explicit split/merge adjudication record; split/merge creates new genesis row IDs whose notes name all source tips, while source tips receive their own next revision marking the finite delta;
- `ADDED` begins at genesis; `CHANGED`, `REMOVED`, and corrected `UNCHANGED` revisions follow the same row chain; a removed row remains a terminal historical tip and cannot be silently resurrected—reintroduction uses a new row identity or an independently adjudicated correction explaining continuity.

Cycle, missing predecessor, branch, ambiguous split/merge, or hash mismatch creates an `UNRESOLVED` row and blocks GO. Independent adjudication verifies both semantic disposition and lineage topology; the mechanical producer cannot adjudicate its own branch resolution.

## Exact source manifest rule

`SOURCE_MANIFEST.sha256` is the input manifest and includes every six-production current source/dependency, the absent required-new production path, all four absent required-new test paths, historical C15 baseline, generic amendment, immutable v1/v2 interfaces, and frozen write-set package. Before enumeration, regenerate it from the exact HEAD and require byte equality. Any drift stops this package and requires a new version.

Generated outputs include their tool/script bytes, argv, environment-affecting values, stdout/stderr, exit status, and hashes. Run twice from identical pins and require byte-identical outputs.

## Reconciliation and lifecycle checks

For every current candidate:

1. identify exact allocation/copy/retention or prove alias/borrow;
2. identify reservation and exact charged capacity before effect;
3. identify owning account/attempt/component and lease source;
4. identify every borrow/alias bound and no-escape rule;
5. identify atomic transfer source/destination and failure preservation;
6. identify every normal/error/refusal/cancel/timeout/STOP/RESTART/Shutdown/race release path;
7. prove one release winner, no double release, no orphan lease, no retained bytes after release;
8. join to exactly one delta row;
9. validate the complete predecessor chain and canonical revision-record hash.

Required equations:

```text
baseline identities = CHANGED + REMOVED + baseline-side UNCHANGED
current identities  = ADDED + CHANGED + current-side UNCHANGED
all current candidates = allocation/copy + zero-charge alias/borrow + transfer/release operation rows
owned live capacity = exact sum(active corresponding lease capacities)
for each row_id: revisions = contiguous 1..tip and predecessor(r) = (row_id, r-1)
lineage roots = exact (NULL, 0) genesis sentinels
lineage cycles + missing predecessors + unresolved branches = 0
UNRESOLVED count = 0 before GO
```

## Independent adjudication

A separate review pass, identified in each row by `adjudicator_id` and `adjudication_revision`, reviews source bytes without inheriting mechanical classifications. It challenges hidden copies, alias escape, lease mismatch, transfer partiality, release gaps, row merges/splits, transitive assumptions, baseline carry-forward, and bounded-zero/absence claims. Independence is a review relationship; no delegation is required.

Every non-`UNRESOLVED` row requires contrary evidence or a falsifier. Corrections create a new monotonically increasing row revision with exact `predecessor_row_id` and `predecessor_revision`, verified canonical revision hash, and finite delta; native source evidence is never rewritten. The independent pass rejects cycles, missing predecessors, branches, duplicate tips, and unjustified split/merge continuity or leaves them `UNRESOLVED`. Structural adjacency, names, CALLS, or generated registry membership cannot accept identity.

Gate prerequisites:

- both deterministic mechanical runs match;
- all manifests/checksums verify;
- every source candidate and baseline identity reconciles;
- every allocation/release/borrow/transfer site is adjudicated;
- `UNRESOLVED.tsv` exists with header only and `UNRESOLVED=0` in counts;
- no write-set expansion or architecture hard stop;
- independent reviewer returns the exact gate for this package checksum.

Until then disposition remains `AUTHORED_NOT_GO`.
