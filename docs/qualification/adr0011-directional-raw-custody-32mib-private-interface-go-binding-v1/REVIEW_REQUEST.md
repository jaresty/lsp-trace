# Independent review request

Review exact token `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRIVATE_INTERFACE_GO` against exact clean source commit `d19ea0acf0f4ed4a344f60e5f20f75ef1c1b1063`.

Run:

```sh
python3 verify_package.py --harness
```

Confirm independently:

1. committed V3 manifest `02153887105c30cc6eaa3b976ec49e7f32b97afcf35ff6b8aaf185529e029704` and bridge manifest `4233ff8a64ee99888d620bf17c87f478ff664069df7ff9821ddad405843021af`, including every listed committed blob;
2. exact 20 unique IDs and complete map/status/constant/adjacency/definition/total-reference/canonical-call/canonical-occurrence equality derived independently rather than accepted from `BRIDGE.tsv` or another verifier;
3. byte equality of both declaration snapshots across this package, V3, bridge, manifests, and committed blobs;
4. current Manager/account/lease/custody pins, `PLANNED_*`-only future sites, exact 47+1 reconciliation, and no semantic absent-source acceptance or final C15 classification;
5. proposal-only six owned components plus four coordination scopes, with no mutation authority;
6. disposable exact-snapshot compilation ending only `INTERFACE_COHERENCE_ONLY`;
7. negative controls reject count forgery, count-preserving symbol substitution, foreign calls, duplicate IDs, missing rows, and extra rows;
8. no production, test, interface producer, profile, composition, or source mutation.

The token grants interface GO only. It grants no RED, GREEN, implementation, profile, composition, C15 closure, activation, admission, or commit authority.

Requested terminal disposition: `AUTHORED_NOT_GO`.
