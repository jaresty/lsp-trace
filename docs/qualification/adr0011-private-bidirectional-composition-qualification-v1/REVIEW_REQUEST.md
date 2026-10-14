# Review request

Requested reviewer decision: inspect this private/default-off correction and either return exact blockers or issue a future independent review decision. The producer disposition remains `ADR0011_PRIVATE_BIDIRECTIONAL_COMPOSITION_AUTHORED_NOT_GO`; the producer requests no self-GO and grants no production/public/integration authority.

## Correction focus

This revision addresses the independent NO_GO that failure-retention fixtures were rejected as metadata mismatch instead of accepted as valid directional failure results. The package now uses directional envelopes under ordered `{incoming,outgoing}`:

- valid success envelope: immutable reference to accepted directional profile/evidence and preserved fields;
- valid failure envelope: bounded private fixture-native directional failure with direction-specific code/stage/message digest/evidence reference, authority 0, completeness UNKNOWN, frontier UNKNOWN, production refused, and no successful counts/claims;
- invalid composition input: rejected, not represented as failure.

## Questions for independent review

1. Does the package correctly distinguish success directional results, fixture-native failure directional results, and invalid composition inputs?
2. Does the package correctly fail closed on the existing ADR0011->ADR0007 `ExactCarrierV1` contract as inapplicable to this authorization?
3. Are all four accepted dependency commits and package hashes pinned and independently recomputed?
4. Does `COMPOSITION_ARTIFACT.json` remain exactly the ordered `{incoming,outgoing}` metadata object, now with closed directional envelopes?
5. Does each success envelope preserve profile id, source commit/package/hash, GO evidence reference, target-set-relative completeness, authority, accepted profile qualification state, production refusal, frontier, counts/failures/raw provenance/numeric bounds/custody references?
6. Does each valid failure envelope avoid successful counts/claims, inverse inference, peer evidence copying, direction relabeling, and authority/completeness escalation?
7. Do incoming failure + outgoing success, outgoing failure + incoming success, and both-failure fixtures pass with exact composition hashes and subtree-preservation assertions?
8. Does the verifier reject cross-direction references/claims, unknown keys, semantic aliases, malformed failure `message_digest` values (uppercase, short/long, nonhex, missing prefix, extra suffix/whitespace, nonstring), pin/profile/schema drift, direction swaps, missing peers, merged failures, missing evidence, raw-copy/remarshal, ceiling changes, CALLS-only drift, and public/default-on exposure?
9. Does the package preserve 32MiB per-direction ceilings without admitting a combined ceiling?
10. Does the package avoid raw provider byte reads/copies/remarshal/reconstruction and retain raw bytes only by path/hash references?

## Non-authorizations

A positive review of this package would not authorize production admission, public/default-on exposure, ADR0011->ADR0007 composition, Stage 8, runtime code, API/CLI/MCP/schema changes, integration, commit, release, deployment, graph admission, or CALLS truth beyond the pinned directional success references and private fixture-native failure envelopes.
