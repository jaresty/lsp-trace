# ADR0011 directional raw-custody 32 MiB private Go interface v1

Status: `AUTHORED_NOT_GO`

Exact review token: `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRIVATE_INTERFACE_GO`

Target HEAD: `2faac2671987f6e6f7c680fe4f072064cfe48136` (clean before authorship).

This corrected documentation-only package freezes the private Go declaration contract for the accepted directional raw-custody architecture and exact production/test write-set. It binds every byte-bearing component to the existing generic lease/sole owner and moved `privateB4ByteLeaseV2`, uses `Manager.mu → callHierarchyLayer1GenericOwner.mu` for every directional operation, links every existing generic terminal path to one Manager exact-once release method, makes handoff Manager-owned, and records typed refusal stage plus reason. It authorizes no RED execution, repository production/test mutation, implementation, activation, public export, C15 classification, profile admission, commit, or deployment.

Normative order:

1. `INTERFACE.md`
2. `sessionruntime_private.go.txt`
3. `lspwire_private.go.txt`
4. `STATE_AND_OWNERSHIP.md`
5. `WRITESET.md`
6. `HARNESS_EVIDENCE.md`
7. `DEPENDENCIES.sha256`
8. `REVIEW_REQUEST.md`

The `.go.txt` files are exact `gofmt` output from the disposable compile harness. Harness-only `compileAllPlannedCallSites`, `compileReaderCallSite`, stub bodies, and mirrored existing `Manager`, `Reader`, generic-owner/lease, and C15-lease declarations prove compilation only; the new declarations and signatures named in `INTERFACE.md` are normative. The harness cannot be copied into production.

Hard stop: any expansion, exported new symbol, second owner/terminal, C15/directional merge, architecture change, historical-test change, or unlisted production/test path requires a new reviewed version. Current disposition: `AUTHORED_NOT_GO`.
