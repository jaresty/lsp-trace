# ADR0011 generic exact — additive V2 contract correction (review requested)

**Proposed disposition:** `ACCEPT_GENERIC_LSP_V2_CONTRACT_CORRECTION_FOR_IMPLEMENTATION_ONLY` or `REVISE`. No disposition is asserted by this packet. Checkpoint B is paused. No live acquisition, publication, replay, admission, qualification, provider support, lifecycle authority, runtime wiring, or Checkpoint A pin change is authorized by candidate files or these synthetic tests.

## Supersession boundary

Generic V1 is **superseded before issuance**. Its four immutable originals and selector fixture remain historical reviewed candidates, at SHA-256 `efa909a074fa8b7e8949f7e70395f50312a384f0d0bd95b65168844fa6c8a9bd`, `6fbb54cf37ec6efe86cb42e8717f9610716b8a84e432a57367135b8da9e265ae`, `5d99986050dd33ff0fffaf623bd8a940a9675053361c8d137a749413dff7311a`, `466994a66fadd65ff80692b5d0284de221d0cd1b6907ea3b11a0652400697648`, and fixture `d511db2c3c1454eadbb197aef747ab5441c7bcfc96dd2d960b067e12badec9f0`. Do **not** publish, replay, issue, or admit V1 evidence. V2 is also not issuable pending independent exact-byte review and later, separate implementation gates. Built-in Go/gopls V1 references and historical omitted-selector CALLS bytes are unrelated and unchanged.

## Candidate V2 originals (not accepted)

| Original | Immutable identity | Bytes | SHA-256 |
|---|---|---:|---|
| `originals/adr0011-generic-envelope-v2.schema.json` | `https://jaresty.github.io/lsp-trace/schemas/adr0011-generic-envelope-v2.schema.json` | 13243 | `f843389f811aea940ed5bf1d595f03dcb6de4fa97c4dafafc41ad83b5c1c7f8e` |
| `originals/generic-lsp-exact-transport-v2.json` | `GENERIC_LSP_EXACT_TRANSPORT_V2` | 1206 | `0519ef89b76d966141bece6ca24d9e186b6424113339a39fce68ae3f78ffa67f` |
| `originals/generic-lsp-references-exact-v2.json` | `GENERIC_LSP_REFERENCES_EXACT_V2` | 724 | `cd92d4167abc951432804991e576a52a43236f9061dfe21c412410a6a851ecc5` |
| `originals/generic-lsp-definition-exact-v2.json` | `GENERIC_LSP_DEFINITION_EXACT_V2` | 633 | `f8fa1a0cb9f1379d69e347ef9573f6356d28f448f0cfb2529d5cd5747751a9af` |
| `originals/generic-lsp-v2-selector-vectors.json` | offline fixture; not an admission original | 2220 | `6992776962db2a815c5f2a3ea9d96a89a7330ec40f6b862cd3894a1c13b7614e` |

The four policies' V2 identity fields are additive successors. All non-identity transport and method-policy values, including `source_documents:256`, quotas, result shapes, authority 0, accepted false, and UNKNOWN completeness, retain V1 semantics. The selector preimage domain is `ADR0011-GENERIC-EXACT/2`, otherwise retaining the length-prefixed V1 field order and role-specific fields. Both methods have seven independent vectors apiece, including schema and policy LF substitutions. `scripts/adr0011-generic-v2-vectors.py --check` and Go `TestV2PythonGoSelectorVectorsAndV1Pins` independently compare the candidate vector bytes; neither issues a selector.

## Role and graph contract

A pre-readback transaction has exactly one each of POLICY, SCHEMA, QUERY, CAPABILITY_EVENTS, REQUEST_WRITE, INBOUND_FRAMES, RESULT_READ, TARGET_EVENTS, TERMINAL; 1–256 distinct SOURCE documents; optional PROCESS 0–1. READBACK is separately optional 0–1 at the complete-transaction schema boundary, but not a B replay result. The new `$defs.TRANSACTION` validates role counts. It does not establish selector uniqueness or transaction-wide identity: JSON Schema's per-item `uniqueItems` cannot enforce uniqueness by a nested selector field or equality of separately recorded identities. Independent future replay must reject duplicated SOURCE selectors and any selected original or predecessor from another transaction, using owner-held expected roles and originals rather than claimant self-selection. No V1 replay is authorized.

| Role | Required predecessors | V2 maximum |
|---|---|---:|
| POLICY, SCHEMA, SOURCE, PROCESS | none | 0 |
| QUERY | POLICY, SCHEMA, query SOURCE | 3 |
| CAPABILITY_EVENTS | optional contextual PROCESS | 1 |
| REQUEST_WRITE | QUERY, CAPABILITY_EVENTS, POLICY; optional contextual PROCESS | 4 |
| INBOUND_FRAMES | REQUEST_WRITE | 1 |
| RESULT_READ | INBOUND_FRAMES, REQUEST_WRITE | 2 |
| TARGET_EVENTS | RESULT_READ plus **only** each required target SOURCE | 256 |
| TERMINAL | eight mandatory non-SOURCE originals (POLICY, SCHEMA, QUERY, CAPABILITY_EVENTS, REQUEST_WRITE, INBOUND_FRAMES, RESULT_READ, TARGET_EVENTS) plus **each** selected SOURCE; optional PROCESS may be omitted | **264** |
| READBACK | committed TERMINAL and every selected original, including optional PROCESS when selected | 266 |

The base predecessor maximum is 266 solely to permit the READBACK branch; explicit role maxima retain all unaffected V1 limits. With 256 SOURCEs: TARGET_EVENTS can use RESULT_READ + 255 target SOURCEs = 256; TERMINAL uses 8 + 256 = **264**, even if PROCESS is selected but not linked there; READBACK can use 9 mandatory non-SOURCE originals (including TERMINAL) + 256 SOURCEs + PROCESS = 266. A zero-target/null result selects only its actual query SOURCE, TARGET_EVENTS depends only on RESULT_READ, and no empty or null result invents target documents. `RESULT_READ` true/null still requires the raw `null` token and WHOLE_PARSE_SUCCESS; the result grammar and all other accepted limits remain unchanged.

## Exact boundary witnesses and review gate

`internal/adr0011genericv2proposal/contract_test.go` exercises schema 264 accept/265 reject, TARGET_EVENTS 256/257, READBACK 266/267, both methods' grouped 256 SOURCE accept/257 reject even with only 264 TERMINAL edges, missing RESULT_READ transaction item reject, zero-target/null one-SOURCE selection, and empty, missing-any-of-eight, or duplicate fixed-role TERMINAL predecessor rejection. The schema does **not** compare each TERMINAL SOURCE selector against the separately selected transaction originals; that equality and cross-transaction custody remain later independent-verifier obligations, not claims from schema validation. `vectors_test.go` recomputes both methods' vectors and historical V1 pins. `internal/adr0011genericv2proposal/selection_test.go` supplies contract-only metadata witnesses for 256/257 SOURCE selection, duplicate SOURCE selector rejection, and foreign-transaction SOURCE rejection for both methods; **these are not claims of an implemented issuer or B replay**. Independent review must confirm the V2 original byte differences, bounded transaction semantics, method parity, exact required predecessor roles, identity domain, the role-specific inherited-limit correction, and remaining witness gaps before any `ACCEPT_GENERIC_LSP_V2_CONTRACT_CORRECTION_FOR_IMPLEMENTATION_ONLY`. Checkpoint A's embedded originals must remain pinned to V1 until that independent review and a separate authorized change.
