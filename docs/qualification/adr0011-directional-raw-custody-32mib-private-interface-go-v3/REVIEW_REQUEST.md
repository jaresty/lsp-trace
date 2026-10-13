# Independent review request

Requested token: `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRIVATE_INTERFACE_V3_GO`  
Current status: `AUTHORED_NOT_GO`

Review only this package and pinned dependencies. Confirm:

1. v1/v2 remain byte-identical blocked evidence and v3 is noncircular;
2. all declarations are private, complete, compile-ready, gofmt, and constructor/method/planned-call-site checked;
3. Manager contains exactly one dedicated directional-raw subledger in the existing accounting domain, with Manager lifetime and no second recorder/backing/terminal/capacity owner;
4. the six component identities and charge table are exhaustive; aliases bind zero-charge bounded owner ranges;
5. reservation identifies exact attempt/account/component/bytes, uses checked arithmetic, admits exactly 32 MiB, and refuses +1 before effects while recording one reached refusal;
6. owned bytes and complete leases travel together; copy and alias/view types and lifetimes are distinct; bytes cannot escape;
7. transfer moves complete bytes+leases atomically, invalidates the source on success, and preserves exactly one source owner on failure;
8. every terminal and race converges on exact-once release; no double release or orphan lease is possible;
9. the sole lock order is Manager.mu -> owner.mu -> account.mu, with no inversion/callback/wait/asynchronous discharge;
10. incoming/outgoing generic LSP profiles remain independent and target-set completeness/frontier remains relative; no Go/gopls semantics entered;
11. constructors, methods, signatures, fields, nil/zero/invalid/error/refusal behavior are exact and complete;
12. the future write set is exactly six production plus four new tests with no expansion;
13. all 20 committed TSV rows have unique IDs/symbols and exact ID→symbol mappings to adjacent private wrappers, each defined once and explicitly referenced by canonical call sites, with no extras/missing and no wrapper semantic duplication;
14. Stage B actual C15 adjudication remains postimplementation;
15. the disposable /tmp harness compiles/vets explicit per-symbol calls, overflow, broad nil/invalid, transfer failure-source-preservation signatures, callback refusal, lock-order/inversion surfaces, three bounded concurrent race-shaped goroutine calls, and separate sequential duplicate release only;
15. dependencies, checksums, HEAD, package status, and AUTHORED_NOT_GO read back exactly.

Return the requested token only for these exact bytes. It authorizes no RED, behavior implementation, production/test mutation, Stage B adjudication, activation, commit, or GO. Otherwise return `REVISE` or `REJECT` with exact reasons.

AUTHORED_NOT_GO
