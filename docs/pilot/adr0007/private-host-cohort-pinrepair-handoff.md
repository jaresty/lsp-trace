# Private cohort: pinned-root / returned-spend repair — pending parent review

HEAD remains `ffc99380fad002b2ce586fb67ac9d76456e3d0fe`. Sole writer; no delegation, live provider/model/application execution, SDK/service work, staging, commit, push, installation or activation. This handoff supplements, not rewrites, the baseline and four-finding repair handoffs.

## Implemented boundaries

**Pinned identity at use.** The concrete file-IPC capability retains its opened mailbox root from initial pinning through Run's lifetime. Each disclosure checks that handle's device/inode, private directory mode, exact READY-byte hashes and configuration/grant binding before writing any request part. A renamed original directory may be operated through its original handle; its replacement path receives no request. Canonically valid replacement launch IDs are not sufficient. The private context capability passes through the existing reservation wrapper; generic backends acquire no caller-selected recovery path.

Recovery opens once, validates identity on that root, then reads registered bundles, parts and replies through root-taking internal readers for both mandatory and navigation families. Public path APIs and wire families remain unchanged. Root ownership closes on deadline, cancellation, panic and ordinary return. These checks establish local consistency, not authentication against a same-UID actor.

**Known spend before reconciliation.** Both exchange paths charge nonempty returned physical bytes and persist the returned receipt before reconciliation or semantic validation. Matching mailbox/host copies remain one event. Conflicts never select the cheaper copy or adopt ambiguous semantics. Reconciliation/persistence refusal returns `AccountingValidation=UNKNOWN_HISTORY_KNOWN_RETURNED_BYTES`: counters retain known returns/prefix, not exact whole-history certification. Such immediate reports advertise no machine/human pairs. Final snapshot failure also retains known spend in the returned qualified DTO. Consistent invalid protocol/digest replies remain charged. No claim refund, generation reset or automatic replay was added.

## RED/GREEN and executed gates

Before production edits, compiling public Run assertions failed for both AFTER-PIN / BEFORE-Exchange substitutions and mandatory/navigation immediate understated counters. Additional compiling REDs covered failed final snapshots, an intentional post-validation root-reopen mutation returning foreign replies in both bundle families, and omission of root close: descriptors grew from 7 to 18 across twelve deadlines. Those mutations were restored. The first descriptor fixture's stat error was rejected as RED.

Final uncached normal **and** race terminal test/subtest counts: product **137 pass**, preparation **299 pass**, focused CLI **43 pass / 1 controlled-child skip**, relevant lineage **3 pass**; zero failures. Six-package vet, gofmt and diff checks pass. CLI tests use actual processes with controlled stdio sources: immediate mandatory/navigation failures retain **11,489 / 22,995** response bytes, exact prior histories and durable receipts, with UNKNOWN accounting and no advertised pairs. Broken output is tested separately from spending. Initial outer 1,200-second CLI interruption is excluded; complete selector reruns with 1,800-second test timeouts passed in approximately 1,044 seconds each. An incorrect preparation-package setup attempt is likewise excluded.

The unchanged actual twenty-item scripted driver remains **8 exchanges, 84,514 request / 30,232 response bytes**. Wait publication, bounded Readdirnames acquisition and whole-navigation admission repairs remain intact.

## Evidence and preservation

Private evidence: `/Users/schwa/.local/state/lsp-trace-evidence/host-cohort-pinrepair-20261101-sole/`; directories 0700, files 0600, Go children 022. `manifest.json`, assertion logs, final JSON logs, SELECT-only `counts.sql`, `repair.delta.patch` and individual SHA-256 manifests support review. Pre-mutation HEAD/hash baseline is retained; predecessor source reconstruction was later checked against those pins. Five existing source files changed and six source files were added. The combined 38-path `SOURCE-whitelist.sha256` digest is `af00f2900133848bc1b6403d435e6ba1c770c8f766a32b5fe03e44ebe9394fbb`. Older whitelists, both protected document pins, historical grants/capsules and earlier handoffs remain unchanged. This handoff is pinned separately.

A finite local Craft derivation preceded this repair. The predecessor's disclosed process gap remains separate; these tests do not retroactively repair it.

## Remaining limits

No runtime ceilings increased: 16 calls; 1 MiB request; 64 KiB response; 8 MiB wire; 4 MiB output; 60/180/240/270-second exchange/backend/operation/HOST bounds; original two-second cleanup. Direct UTF-8 IPC accounting excludes host/export framing, acquisition/session/bootstrap retries, inference billing and full-store IO. Whole-application paging/accounting remains future work; source-unit-one and per-progress/report 4 MiB bounds remain. None_Model / SCRIPTED_CONTRACT is not actual Agent judgment, isolation, authentication or cleanup evidence. No JSON resurrects a PostPreparationV2 owner. Source UNKNOWN; feature UNRESOLVED; accepted false; authority 0; absence PROHIBITED. Parent tests and independent review remain required before commit or activation.
