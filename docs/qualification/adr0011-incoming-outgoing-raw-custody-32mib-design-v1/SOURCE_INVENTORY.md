# Exact source seam and dependency inventory

All hashes are SHA-256 over exact bytes at target HEAD `7f09e5357b117d20449fc16097631e32d4eead7f`.

| Path | Bytes | SHA-256 | Exact role |
|---|---:|---|---|
| `sessionruntime/sessionruntime.go` | 135554 | `ad06c64bf3e267587dc94197a80ece1a03bc7933bd9263b4bb1ac78cad69fb8d` | real Manager round-trip, read loop, `readResult`, request assembly/write and synchronization |
| `sessionruntime/b4_definition_private.go` | 54373 | `ba4d40171650eddc64b7ce0617e14876ab37c4d05defb932e0ada2aee9db6549` | current private 16 MiB admission, transaction install/release, and history retirement integration |
| `sessionruntime/readiness_history_private.go` | 5839 | `d9d32d417ebd17d8e50a8f0c4e7f35fe931ff0f53cb2c144339cea02dc6ba1e0` | readiness-history entries, immutable cuts, identities, ownership counts, retirement; pinned no-change dependency |
| `sessionruntime/owned_method_pair.go` | 21263 | `8685b78ac7883263c7279ae68a543bb365ab7de0ca8610955eb90236ba5b70b8` | predecessor attachment/copy seam; pinned no-change dependency |
| `sessionruntime/b4_explicit_bytes_c15_private.go` | 38132 | `debf86b8b4568d49abd538da14f9a93fb34990936dd13b1156bdbf6de93d21cd` | existing C15 backing accounting and distinct 32 MiB aggregate owner ceiling; not the new ledger |
| `sessionruntime/b4_owned_method_pair_lease.go` | 32794 | `b53e51b89effa526337b96fcf883078da332b9b1a1457fcd1fc44a3db2e585e9` | generic exact frame/result owner, view, publication, terminal, release |
| `sessionruntime/b4_c15_ownership_registry_private.go` | 25463 | `314a9a89f1f483ac9ced32479bccdea1fb8d1649e083c9a7c167f2baca444274` | current generated C15 registry |
| `internal/lspwire/read_frame.go` | 4208 | `9e58a1ae4a79171799134d8640c99731842589125f229088f39196d3d7b9fa58` | exact transport scanner/capture; currently withholds failed-read bytes |
| `internal/lspwire/successor_ingress.go` | 34476 | `df4e53790ba611164b2d1d35751bf56bf788abf447cc7bdd3601a0617ac49fa2` | production scanner/read/result-span/allocation path |
| `internal/lspwire/wire.go` | 18412 | `90640b3100bb89959969d0b997b4beb5bf583fbf722d55f361e013773358184e` | framing and 16 MiB default body limit |
| `internal/provider/provisioning.go` | 5845 | `723180b371c31b4bbcd81297b3f1bfa13491c7edf09467631998794a5c7fcd25` | independent 16 MiB provisioning ceiling; not asserted as universal Manager call edge |
| `docs/qualification/adr0011-c15-allocation-census.provisional.md` | 9826 | `28e626b5434a2857b3d5b2918cda5ed9347f85cb1835f8df598fa0c378c7c479` | correction-safe 54-seam C15 baseline |
| `docs/qualification/adr0011-calls-go-gopls-incoming-capture-seam-design.md` | 19784 | `79547b57a598b3700699fc4e909db01ed35a2205a798ff0980a7732753e25527` | incoming scanner/selection/assembly contract |
| `docs/qualification/adr0011-calls-go-gopls-outgoing-capture-seam-design.md` | 22127 | `66395d49e8c8f9b349e51b2e34a3dc7c810f35397ba58a16e480f9b43960324f` | outgoing scanner/selection/assembly contract |
| `docs/qualification/adr0011-callhierarchy-target-corpus-v1/MANIFEST.sha256` | 1026 | `316afe2501aebe4195894d1334be566ae49eb32c88276ebee7b2e4d86062f44d` | qualified corpus pin; 5900 targets |

## Exact current symbols

- Manager path: `Manager.RoundTrip`, `roundTripWithPrivate`, `roundTripWithPrivateTransaction`, `readResult`.
- Exact scanner: `framedReadCapture.add`, `Reader.ReadWithExactFrame`, `Reader.ReadWithFrameIfWithin`.
- Successor scanner: `SuccessorIngressReader.ReadFrame`, `readMore`, `consumeCopy`, `topLevelResultSpan`.
- Generic raw custody/lineage: `newCallHierarchyLayer1GenericAttemptLineage`, `genericFrame`, `captureRequestFrame`, `captureResponseFrame`, `captureResult`, `copyGenericObservation`, `newCallHierarchyLayer1GenericOwner`, `genericView`, lease `View`/`Callback`/`Release`, `publishCallHierarchyLayer1GenericOwnerLocked`.
- Predecessor attachment: `callHierarchyLayer1RequestAttachment.privateCallHierarchyLayer1WithPredecessors` in `owned_method_pair.go`.
- Readiness history: `privateReadinessHistory` entry/identity/cut/receipt/borrower/retirement methods in `readiness_history_private.go`, integrated by `sessionruntime.go` and `b4_definition_private.go`.
- Current private admission: `Manager.RoundTripPrivateB4`, `admitPrivateB4SourcesC17Locked`, `installPrivateB4TransactionLocked`, `releasePrivateB4Locked`.

No current production function performs the required bounded failed-read prefix return, prepared-item raw-array selection, or separate incoming/outgoing unchanged-byte assembly. Those are proposed seams, not source facts.

## Frozen minimal future production write-set (not authorized)

1. `sessionruntime/call_hierarchy_directional_raw_v1_private.go` (new): Manager-owned independent reservation ledger; raw-array scanner; selected-ordinal/raw-span selector; `handoffCallHierarchySelectedRawV1`; separate `assembleIncomingCallHierarchyParamsV1` and `assembleOutgoingCallHierarchyParamsV1`.
2. `sessionruntime/sessionruntime.go`: reservation-before-effect integration under Manager synchronization; private transaction-scoped scanner/body bound; exact raw handoff into the real write/read loop.
3. `internal/lspwire/read_frame.go`: bounded incomplete-prefix capture at the exact transport scanner; complete/incomplete typed result.
4. `internal/lspwire/successor_ingress.go`: only if the real Manager path uses this scanner for the opted-in transaction, thread the already-reserved private bound and incomplete-prefix ownership without changing defaults.
5. `sessionruntime/b4_owned_method_pair_lease.go`: extend the existing sole generic owner to retain opaque incomplete-prefix custody and mapping IDs; no second owner/terminal.
6. `sessionruntime/b4_c15_ownership_registry_private.go`: generated only after accepted C15 enumeration; never hand-edit.

`internal/lspwire/wire.go`, `sessionruntime/b4_definition_private.go`, `sessionruntime/readiness_history_private.go`, `sessionruntime/owned_method_pair.go`, and `internal/provider/provisioning.go` are pinned dependencies, not authorized write targets. `b4_owned_method_pair_lease.go` may receive only opaque prefix-custody fields/mapping needed by the sole owner; it may not change predecessor commitments or generic attempt-lineage semantics. If implementation requires changing global/default/public limits or any history/predecessor/lineage behavior, stop `REVISE`.

## Future test/write evidence (not authorized)

Separate private tests must cover Manager ledger atomicity, one slot, exact/plus-one/overflow, failed-read prefix custody, exact release/reuse, the shared scanner/selector, separate incoming/outgoing assemblers, and disabled/`CALLS_ONLY` byte-value equality for every readiness-history and predecessor/generic-lineage field named in `DESIGN.md`; prefix mapping IDs must be absent in controls and must never affect history semantics when enabled. No existing corpus, predecessor package, public contract test, or production fixture may change.

## Managed evidence boundary

READY session `sk1:9056e071454cbbd51661782b2964d4a8d5c266675df42622d95c5d5032abf0f9`, generation 1: neighborhood capture was truncated then failed traversal; target-only projection was complete for the requested source with authority 0 and source-graph completeness unknown. Text and AST inventory do not establish CALLS or dependency completeness.
