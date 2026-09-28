# ADR0011 request-key V1 — implementation-only decision

**Decision: `ACCEPT_REQUEST_KEY_V1_FOR_IMPLEMENTATION_ONLY`.** This narrow successor closes the request-key string identity gap; it does not accept an owner-read producer, issue references, qualify occurrences, change historical `CALLS_ONLY`, or authorize CLI/MCP exposure.

The selected encoding is `lsp-trace.request-key.v1:g=<generation>;id=<id>` as specified in `adr0011-request-key-v1.addendum.proposed.md`. Independently extracted original request and response JSON-RPC `id` tokens must equal the exact unquoted canonical decimal `id`; caller-converted integers alone do not suffice. The runtime `lspwire.RequestKey`, enclosing generation, `ownerRead.wire_id`, and separately established invocation identity must agree. String IDs need a new version.

## Exact reviewed artifact selection

The successor schema `$id` is `https://jaresty.github.io/lsp-trace/schemas/adr0011-references-issuance-records.request-key-v1.proposed.schema.json`; its **complete-byte SHA-256** is `86bb214e419f7c69b80ff5dfbdf0f0760c495c9bf0f88d06bdc227b77920fc8e`. The exact reviewed addendum, codec, tests, predecessor schema, and predecessor freeze-manifest byte digests are listed in `adr0011-request-key-v1-review.manifest.proposed.md`. This decision selects those values as implementation expectations. Any changed byte requires renewed review; a claimant's schema field does not select the trusted schema digest.

The independent read-only decision was `ACCEPT_REQUEST_KEY_V1_FOR_IMPLEMENTATION_ONLY` after an earlier `REVISE` for preconverted IDs and missing successor digest selection. Parent-side review recomputed the successor schema digest, verified exactly six replacements of `request_key`/`RequestKey` sites (apart from successor `$id`, title, and the shared definition), and reran 66 focused offline codec/schema tests and scoped vet. The independent reviewer explicitly noted that `Identity` accepts raw tokens supplied by a caller; **owner wiring must independently extract those tokens from retained original Manager-bound request and response bodies** before using the codec.

This decision preserves the predecessor freeze at `c817e09f5d7235309f3b0e6a29ebea476f67f5f2` as an historical reference. No accepted predecessor artifact is rewritten. `authority=0`, `completeness=UNKNOWN`, `NO_PRODUCER_AUTHENTICATION`; issued `T=A=0` continues until independently verified final issuance.

**Next authorized checkpoint only:** connect the accepted key encoding and original raw numeric ID-token extraction to the owner-held method frames, with substitution and schema-digest mismatch tests. Keep `PublishReferences` fail-closed. No immutable relation index until final references issuance and admitted occurrence identity stabilize.
