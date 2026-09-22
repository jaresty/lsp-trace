PROVISIONAL — NOT ACCEPTED · Authority 0 · Completeness UNKNOWN

Qualification status: STOPPED_PARTIAL_IDENTIFIER_ONLY_SOURCE
Run root: /tmp/lsp-trace-adr0007-full-v2.9PXZOO
Terminal checkpoint: sha256:43581799e64bfde715b0613ded18e07415b63fd8179629dade01214cf750086c
Stage/status/diagnostic: DESCRIBE_ATTEMPTS / FAILED_WORKER / WORKER_NO_TERMINAL

## Bounded accounting

Packets 115 · Preparation failures 16 · Unresolved 1047 · Rendered requests 135 · Retained invocations 10 · Retained responses 10 · Not executed 125

Retained verdicts: COMPLETE 0 · ABSTAINED 10
Host consumer custody: RESOLVED 3 · UNRESOLVED 7
Citation suggestions: present 0 · absent 10

## Source limitation

The packets use identifier-only Graph V5 item ranges mislabeled as display ranges. They do not provide qualifying full-definition source. The model outputs are therefore semantically unqualified even though the 10 retained V2 record pairs are mechanically strict.

## Representative retained entries

| # | Verdict | Identifier-like output | Host admissible basis | Citation suggestions |
|---|---|---|---|---|
| 1 | ABSTAINED | `artifactSelector` | C1/C2; `PACKET_SCOPE` | absent |
| 2 | ABSTAINED | `catalogPreparationFailures` | C1; `UNRESOLVED_CUSTODY`; `PACKET_SCOPE` | absent |
| 3 | ABSTAINED | `Get` | C1; `UNRESOLVED_CUSTODY`; `PACKET_SCOPE` | absent |
| 4 | ABSTAINED | `equalJSON` / `loadStateFromVerificationStore` | C1/C2; `PACKET_SCOPE` | absent |
| 5 | ABSTAINED | `ParseComposite` | C1; `UNRESOLVED_CUSTODY`; `PACKET_SCOPE` | absent |
| 6 | ABSTAINED | `Accepted` | C1; `UNRESOLVED_CUSTODY`; `PACKET_SCOPE` | absent |
| 7 | ABSTAINED | `persistCheckpoint` | C1; `UNRESOLVED_CUSTODY`; `PACKET_SCOPE` | absent |
| 8 | ABSTAINED | `HandoffID` | C1/C2; `PACKET_SCOPE` | absent |

There are no COMPLETE entries to render. The table is a mechanical record view, not a feature inventory or semantic acceptance.

## Unreached stages

No CatalogV2, composite, catalog accounting, final descriptor, or RenderReview exists. Selector-only resume was not attempted, so no zero-rerun or identity-convergence claim is made. No retry or continuation is authorized from this checkpoint.
