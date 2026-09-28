# ADR0011 successor schema role mapping — narrow implementation-only proposal

**Status: PROPOSED; no issuance or occurrence qualification.** This corrects the whole-schema identity ambiguity exposed after the accepted request-key V1 successor. It changes no record shape, canonical serialization, role-domain digest, selector, policy bytes, or accepted predecessor artifact.

## Selection

For **every new ADR0011 references issuance record** defined by the successor schema, select the single complete successor schema file:

- `$id`: `https://jaresty.github.io/lsp-trace/schemas/adr0011-references-issuance-records.request-key-v1.proposed.schema.json`
- complete-byte SHA-256: `86bb214e419f7c69b80ff5dfbdf0f0760c495c9bf0f88d06bdc227b77920fc8e`
- role schema ID: this exact `$id` followed by `#/$defs/<definition>` for each of `methodRecord`, `targetRecord`, `preparedSource`, `sourceIdentity`, `hostGit`, `revisionIdentity`, `ownerRead`, `targetResult`, `responseRead`, `rawResult`, `scanner`, `events`, `proposal`, `candidate`, `final`, and `policy`. The four selected policy versions use the same `policy` definition and separately pinned policy bytes.

The owner-selected `schema_digests` expectation for a **new successor issuance transaction** must bind these roles to the *one* successor whole-file digest above. Independent replay rejects any mixture of original and successor schema IDs/digests within a newly issued dependency closure, even where a definition's fields happen to be identical. A claimant's schema field or selector cannot choose the schema. This mapping does not change the separately pinned implementation identity or the independent selection of method, admission, privacy, and retention policy bytes.

The predecessor schema `$id` `https://jaresty.github.io/lsp-trace/schemas/adr0011-references-issuance-records.proposed.schema.json` and complete-byte digest `bf49de9460fee5a68ba13204f633dc72d2dca566fa5f92d704bc19b5514618ba` remain historical for artifacts produced under the frozen `c817e09f5d7235309f3b0e6a29ebea476f67f5f2` snapshot. No predecessor object is rewritten or silently upgraded. The successor request-key V1 grammar applies to all of its six request-key fields. An existing predecessor cannot be promoted to successor issuance merely by replaying it under the new schema; a successor transaction must independently build and verify its own complete closure. Historical omitted-selector `CALLS_ONLY` bytes are untouched.

## Gate

An independent reviewer must return `ACCEPT_SUCCESSOR_SCHEMA_ROLE_MAPPING_FOR_IMPLEMENTATION_ONLY` or `REVISE`, with exact successor `$id`/digest, role list, non-mixing rule, and predecessor non-upgrade boundary pinned. Until that decision, no new typed predecessor, final issuance, or index may claim this mapping. Acceptance permits synthetic implementation/falsification only: `authority=0`, `accepted=false`, `completeness=UNKNOWN`, `NO_PRODUCER_AUTHENTICATION`, and issued `T=A=0` until separately replayed final publication. Real occurrence qualification and public CLI/MCP exposure remain separate gates.
