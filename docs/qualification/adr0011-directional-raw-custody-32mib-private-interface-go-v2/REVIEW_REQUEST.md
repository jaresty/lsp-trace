# Independent review request

Requested token: `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRIVATE_INTERFACE_GO_V2`  
Current status: `AUTHORED_NOT_GO`

Review only this package and pinned dependencies. Confirm:

1. all new declarations are private, complete, compile-ready, `gofmt`, and constructor/method/call-site checked;
2. generic identities are exactly independent `CALLS_LSP_INCOMING_EXACT_V1` and `CALLS_LSP_OUTGOING_EXACT_V1` domains, with nonzero SHA-256 digests over the frozen prefix plus big-endian length-prefixed domain and ID;
3. no Go/gopls/provider semantics entered the generic interface; provider binding is opaque identity/receipt only;
4. incoming/outgoing identity, selected ordinal, request, occurrence, accounting, terminal, evidence, and review cannot inherit;
5. preparation is separate and selected zero-based ordinal remains unchanged through synchronized Manager handoff;
6. disposition, terminal, refusal stage/reason, zero/nil/invalid behavior, and precedence are closed and non-conflating; Released is absent from disposition and represented only by lifetime/state/release identity;
7. borrowed/copied/aliased/view/released lifetime and release-race outcomes are explicit;
8. the existing generic owner remains sole byte/C15-lease owner; transfer moves and zeros `*privateB4ByteLeaseV2` atomically with no duplicate charge;
9. Manager owns one-slot reservation and enforces exact 32 MiB / +1 pre-effect behavior with no production effect on refusal;
10. all stateful paths use `Manager.mu -> callHierarchyLayer1GenericOwner.mu`, never reverse;
11. every terminal path converges through Manager exact-once release, with stable release identity and exact slot reuse;
12. target-set-relative denominator equations, exact attempted ordinal/outcome records, disjoint ordered unattempted frontier, terminal counts, bounded traversal, unknown/partial rules, and completeness ceiling prove an exact-once target-set partition;
13. Layer 1 remains opaque/direction-neutral; D/R and `CALLS_ONLY` compatibility is unchanged;
14. exact six-production/four-new-test future write-set is unchanged and no existing test was created, mutated, or run;
15. disposable harness validates both-direction exact/+1/nil/invalid, all disposition/terminal/refusal/lifetime/frontier/race/duplicate-release call shapes at compile time only; package/dependency checksums and HEAD pin read back exactly;
16. v2 depends noncircularly on committed amendment `f036724` and immutable v1 manifest digest `d36211da45adb55e83f0b112a62140d7bec5ef2076edee12c3facebbd692af7c`; neither predecessor depends on v2;
17. no public/duplicate-owner/duplicate-charge/architecture/default/activation/write-set change exists.

Return the token only for these exact bytes. It authorizes no RED execution, implementation, production/test mutation, provider qualification, activation, admission, commit, or GO beyond this private declaration contract. Otherwise return `REVISE` or `REJECT` with exact reasons.

AUTHORED_NOT_GO
