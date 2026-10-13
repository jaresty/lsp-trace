# Exact-byte binding

## Immutable producers

| producer | committed path | manifest SHA-256 | commit |
|---|---|---|---|
| private-interface V3 | `docs/qualification/adr0011-directional-raw-custody-32mib-private-interface-go-v3/SHA256SUMS` | `02153887105c30cc6eaa3b976ec49e7f32b97afcf35ff6b8aaf185529e029704` | `d19ea0acf0f4ed4a344f60e5f20f75ef1c1b1063` |
| two-stage bridge | `docs/qualification/adr0011-directional-raw-custody-c15-two-stage-bridge-v1/SHA256SUMS` | `4233ff8a64ee99888d620bf17c87f478ff664069df7ff9821ddad405843021af` | `d19ea0acf0f4ed4a344f60e5f20f75ef1c1b1063` |

Every listed producer file must equal its committed blob and its manifest entry. Producer bytes are inputs, never rewritten.

## Exact 20-row bridge

`BRIDGE.tsv` is an exact-byte snapshot of the committed bridge. The verifier does not trust that snapshot as authority. From the committed two-stage `STAGE_A_PLANNED_OBLIGATIONS.tsv` and exact V3 sessionruntime declaration snapshot it independently requires:

- exactly 20 rows, 20 unique 64-hex IDs, and 20 unique symbols;
- the complete ID-to-symbol map and `PLANNED_*` status for each row;
- the exact V3 constant name and constant value;
- exact adjacency: ID constant, exactly one blank line, immediately followed by its `func (m *Manager) SYMBOL(` definition;
- exactly one definition per symbol;
- the complete-snapshot count of every exact `m.SYMBOL(` call reference;
- one exact symbol-bound canonical call line in `compilePlannedV3`, with that selected line occurring exactly once;
- equality of every bridge column, with no missing, duplicate, foreign, or extra row.

The verifier's negative controls mutate data in memory only and must reject forged counts, count-preserving symbol substitution, foreign canonical calls, duplicate IDs, extra rows, and missing rows. It invokes no producer verifier as a proxy.

## Declaration equality and source pins

Both declaration snapshots must be byte-for-byte equal across this package, the V3 package, the bridge package, and the corresponding committed blobs. V3 manifest entries must equal their bytes.

`SOURCE_EVIDENCE.tsv` is an exact committed bridge snapshot. Each current Manager/account/lease/custody source path must equal its pinned SHA-256 and committed blob. These are `CURRENT_SOURCE_PIN_ONLY`, not semantic acceptance.

## Future and accounting boundary

Every Stage A status remains `PLANNED_RESERVATION`, `PLANNED_COPY`, `PLANNED_ALIAS`, `PLANNED_TRANSFER`, or `PLANNED_RELEASE`. No absent source is accepted or finally classified. Exact committed evidence remains 47 preserved V1 identities plus one new planned architecture obligation, denominator 48. The model-reported 48 provenance remains unpinned and inconsistent.

The six proposed owned components are `PREPARE_FRAME`, `PREPARE_RESULT`, `SELECTED_HANDOFF`, `DIRECTIONAL_FRAME`, `DIRECTIONAL_RESULT`, and `RETAINED_EXTERNAL_EVIDENCE`. The four proposed coordination scopes are Manager subledger, generic backing/terminal owner, C15 account/lease authority, and terminal coordinator. This exact six-plus-four statement is proposal scope only: it creates no owner, lease, account, source, declaration, or write authority.

## Authority boundary

A successful review may issue only `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRIVATE_INTERFACE_GO`, meaning exact private-interface byte coherence. It is not RED, GREEN, implementation, production/test mutation, profile refinement, profile authority, composition authority, runtime behavior, C15 closure, semantic absent-source acceptance, activation, admission, or commit authority.

AUTHORED_NOT_GO
