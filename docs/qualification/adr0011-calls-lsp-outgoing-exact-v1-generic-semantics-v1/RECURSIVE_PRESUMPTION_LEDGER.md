# Recursive presumption ledger

Status: `GENERIC_OUTGOING_SEMANTICS_AUTHORED_NOT_GO`; authority `0`.

## Presumptions made and revision triggers

1. Presumed: The pinned LSP 3.17 `CallHierarchyOutgoingCall` interface is sufficient normative evidence for required `to: CallHierarchyItem`, required `fromRanges: Range[]`, and top-level `CallHierarchyOutgoingCall[] | null`.
   Revised if: a later accepted generic outgoing review identifies a conflicting normative LSP source/schema pin for the same profile and version.

2. Presumed: A present empty `fromRanges: []` is valid and retains an otherwise-valid `A -> B` member with zero occurrence rows.
   Revised if: a normative LSP source or accepted generic outgoing contract imposes a non-empty invariant on `fromRanges` for outgoing calls.

3. Presumed: `to.range` and `to.selectionRange` are endpoint identity/display metadata, not occurrence evidence.
   Revised if: a normative outgoing source explicitly defines either field as a call-site range rather than endpoint range.

4. Presumed: The incoming generic package is safe structural precedent for package shape, verifier style, and checksum convention.
   Revised if: independent review finds a file-role or verifier convention imports incoming authority into outgoing semantics.

5. Presumed: This package may cite the accepted generic directional amendment as historical contract context while retaining `authority=0` and `accepted=false` for this new package.
   Revised if: review finds the amendment was not accepted for the cited independence/no-inheritance boundary.

6. Presumed: Deterministic fixture/oracle verification plus adversarial tamper tests are sufficient for an independently reviewable semantic freeze packet, not for provider qualification.
   Revised if: reviewer requires additional provider-neutral fixture classes before accepting the generic outgoing semantics.

## Non-presumable choices

- Provider qualification is deferred and not presumed.
- Runtime behavior, gopls behavior, fake-LSP behavior, and provider NO_GO package behavior are deferred and not presumed.
- Public schema, registry, composition, custody, ceiling, implementation, test, and production changes are deferred and not presumed.
- Incoming semantic authority transfer is deferred and rejected unless separately authorized by an accepted direction-specific amendment; this package does not perform it.
