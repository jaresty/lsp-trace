# Review request

Please review `docs/qualification/adr0011-calls-go-gopls-outgoing-profile-qualification-v1/` as the sole outgoing provider qualification finalizer for `CALLS_GO_GOPLS_OUTGOING_EXACT_V1`.

The producer disposition remains `CALLS_GO_GOPLS_OUTGOING_PROFILE_QUALIFICATION_AUTHORED_NOT_GO`. The package is complete for review but does not self-accept: authority `0`, accepted `false`, provider GO `false`, outgoing provider GO `false`, production GO `false`, production admission refused, completeness/frontier `UNKNOWN`.

Reviewer NO_GO correction coverage added in this revision:

- exact recursive authority/accepted/production/completeness/frontier/provider-go assertions;
- exact incoming non-use assertions and rejection of incoming authority/profile/ledger/raw/run dependencies;
- explicit forbidden-effect closure for composition, bidirectional use, ADR0011-to-ADR0007 composition, Stage 8, ceiling, custody-owner, runtime/public/API/schema/CLI/MCP, integration, push, publication, release, deployment, and production admission;
- exact generic prerequisite source/hash/commit and fresh final seal/checksum/evidence HEAD/run pins;
- rejection of conflicting `semantic_statement_duplicate`, `occurrence_source_policy_duplicate`, JSON duplicate keys, and unrecognized semantic-bearing aliases;
- exact `RUN_COMPARISON` target counts, raw outgoing file counts, target-order digest, terminal-event digest, failure digest, semantic-difference indicators, final counts, raw aggregate/inventory, maxima witness/qualifier fields, and historical/fresh identity equality;
- exact allowed keysets for critical objects;
- assertion-bound representative and systematic tamper mutations, not checksum-only protection.

Rereview NO_GO five-omission correction added:

- `ROLE_MATRIX.non_roles` is now an exact allowed keyset with every value canonical false/no-authority.
- `ROLE_MATRIX.provider_binding_roles` is exactly bound to canonical source pins and current files; no unasserted entries are admitted.
- `MANIFEST.accepted_prerequisites_pinned` is exactly bound to canonical `source_pins` and accepted evidence commit/head/run fields.
- `RESULT.comparison_result.known_nonsemantic_differences` is structured and exactly mirrored from `RUN_COMPARISON.semantic_difference_indicators.known_nonsemantic_differences`.
- The tamper suite includes the five reviewer representative mutations directly.

Run:

```bash
PYTHONDONTWRITEBYTECODE=1 python3 docs/qualification/adr0011-calls-go-gopls-outgoing-profile-qualification-v1/verify_outgoing_provider_profile.py
PYTHONDONTWRITEBYTECODE=1 python3 docs/qualification/adr0011-calls-go-gopls-outgoing-profile-qualification-v1/verify_outgoing_provider_profile.py --tamper-tests
```
