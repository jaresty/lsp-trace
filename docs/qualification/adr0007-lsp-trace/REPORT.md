# ADR0007 V2 qualification — partial, semantically unqualified

Status: `STOPPED_PARTIAL_IDENTIFIER_ONLY_SOURCE`

Authority: `0`  
Accepted: `false`  
Completeness: `UNKNOWN`

This report records one bounded execution of the current qualification harness against `/Users/schwa/dev/lsp-trace`. It does not claim semantic richness, feature identity, source completeness, semantic acceptance, or a qualified catalog.

## One-shot controls

- Run root: `/tmp/lsp-trace-adr0007-full-v2.9PXZOO`.
- CLI SHA-256: `5e0b98d7921361aec135715da81943077d2981171604b2d6f1f743c9a9b940be`.
- Model SHA-256: `1664fccab734674a50763490a8c6931b70e3f2f8ec10031b54806d30e5f956b6`.
- Worker SHA-256: `b9ad7048c3e1b36ca97006142a0402c450c94a179392cbe97eaea28b18b08a61`.
- Runtime-manifest SHA-256: `b95e8680b4d30761492bbc2d4a6fed656f124c5756dd4387cf02c30d27c13d90`.
- Pinned V2 grammar, V2 invocation/response/semantic schemas, and WorkerV2 wiring were present.
- Network denial was proven (`nc` exit `1`).
- Object cap: `67,108,864` bytes; handoff: `23,295,342` bytes.
- Per-request host timeout: `90,000ms`.
- Run/publication roots were private; every continuation object inspected was mode `0600`.
- The harness was launched exactly once. No automatic retry, resume, or second campaign was run.

## Historical census and preparation

- Historical census exit: `0`; elapsed: `6,265,989,459ns`.
- Catalog path exit: `1`; elapsed: `74,231,320,083ns`.
- Prepared provisional packets: `115`.
- Preparation failures: `16`, all `EXACT_ENDPOINT_SOURCE_UNAVAILABLE`.
- Unresolved records: `1,047`.
- Rendered requests: `135`.

The packets are now known to use identifier-only Graph V5 item ranges mislabeled as display ranges. Therefore the source supplied to the model was not a qualifying full-definition source projection. All semantic outputs below are mechanically retained but semantically unqualified.

## Retained V2 prefix

Exactly 10 model calls completed before the next request failed to produce a terminal record:

- InvocationRecordV2: `10`, all terminal status `SUCCEEDED`.
- ResponseRecordV2: `10`.
- Strict retained-history invariants: canonical history, strict invocation parse, strict response parse, unique identities, and exact invocation-to-response linkage.
- Response verdicts: `COMPLETE=0`, `ABSTAINED=10`.
- Host consumer custody: `RESOLVED=3`, `UNRESOLVED=7`.
- Citation suggestions: present `0`, absent `10`.
- Every retained response: authority `0`, accepted `false`, completeness `UNKNOWN`.
- Remaining requests: `125` not executed after the stop (`135 - 10`).

The terminal checkpoint is `sha256:43581799e64bfde715b0613ded18e07415b63fd8179629dade01214cf750086c`, stage `DESCRIBE_ATTEMPTS`, status `FAILED_WORKER`, diagnostic `WORKER_NO_TERMINAL`. Its retained describe-records object is `sha256:59d5427b0eaa031bd86cedaf77717712d8ceeefbb287baa8a8456c5fe82ad303`.

## Representative retained entries

All examples are `ABSTAINED`; there are no `COMPLETE` examples to show. The identifiers and short labels below illustrate the identifier-only limitation and are not qualified feature descriptions.

1. `artifactSelector` — consumer custody resolved; admissible host basis `C1/C2`, limitation `PACKET_SCOPE`; citation suggestions absent.
2. `catalogPreparationFailures` — consumer custody unresolved; basis uses `UNRESOLVED_CUSTODY`, limitation `PACKET_SCOPE`; suggestions absent.
3. `Get` — consumer custody unresolved; identifier-like behavior only; suggestions absent.
4. `equalJSON` — consumer custody resolved; identifier-like boundary `loadStateFromVerificationStore`; suggestions absent.
5. `ParseComposite` — consumer custody unresolved; identifier repeated as behavior/boundary; suggestions absent.
6. `Accepted` — consumer custody unresolved; generic identifier-derived wording; suggestions absent.
7. `persistCheckpoint` — consumer custody unresolved; identifier repeated as behavior/boundary; suggestions absent.
8. `HandoffID` — consumer custody resolved; generic identifier-derived wording; suggestions absent.

## Catalog, composite, and resume disposition

No CatalogV2, composite, final catalog checkpoint, or public descriptor was produced. Catalog identity/outcome and accounting equality are therefore unavailable, not failed or inferred. RenderReview was not reachable.

Selector-only resume was deliberately not attempted. Consequently there is no zero-census, zero-gopls, zero-model-rerun convergence proof and no catalog/composite identity convergence claim. Resuming would continue model execution from semantically unqualified packets and is prohibited for this qualification.

## Qualification boundary

This run qualifies only the bounded mechanical fact that 10 strict V2 invocation/response pairs were retained under the pinned host controls before a fail-closed `WORKER_NO_TERMINAL` stop. It does not qualify the semantics, the packet source projection, a catalog, semantic richness, or source completeness.
