# ADR0011 directional raw-custody 32 MiB private Go interface v3

Status: `AUTHORED_NOT_GO`  
Exact token: `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRIVATE_INTERFACE_V3_GO`  
Target HEAD: `acf563e7708095d8a32c809b49d97c7432d49b03`

This documentation-only successor corrects immutable blocked interface v2 from the accepted generic directional profile amendment, C15 lease-source V2 GO package, and two-stage C15 GO contract. V1 and v2 remain byte-identical blocked evidence. V3 freezes compile-ready private declarations only: no behavior, RED, production/test mutation, implementation, Stage B adjudication, activation, or commit authority.

Normative order:

1. `INTERFACE.md`
2. `sessionruntime_private.go.txt`
3. `lspwire_private.go.txt`
4. `STATE_AND_OWNERSHIP.md`
5. `WRITESET.md`
6. `HARNESS_EVIDENCE.md`
7. `DEPENDENCIES.sha256`
8. `REVIEW_REQUEST.md`
9. `verify_package.py`

The Manager-contained directional-raw subledger is metadata in the existing accounting domain. The existing generic owner remains the sole backing and terminal owner; C15 account leases remain the sole capacity authority. Six components are charged exactly once for owned copies; aliases are zero-charge bounded bindings. Bytes and leases travel only in inseparable acquisition values. All nested paths use `Manager.mu -> callHierarchyLayer1GenericOwner.mu -> privateB4ByteAccountV2.mu`.

Incoming and outgoing generic LSP profiles remain independent and provider-neutral. Completeness is target-set-relative only. No Go/gopls semantics enter this package.

All 20 Stage A planned obligation IDs are adjacent to and bound to their exact TSV-named private wrapper symbols. The 18 component wrappers delegate to the generic reserve/copy/alias cores; transfer and release wrappers delegate to their generic cores. Canonical planned call sites reference every wrapper explicitly. The package verifier parses and exactly reconciles the committed TSV, rejecting duplicate, extra, missing, undefined, non-adjacent, or unreferenced mappings. Stage B actual C15 enumeration/adjudication remains postimplementation and absent.

AUTHORED_NOT_GO
