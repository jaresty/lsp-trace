# ADR0011 synthetic context schema-digest labels — implementation-only proposal

The accepted successor schema maps sixteen definitions to one pinned full-file digest (`86bb214e419f7c69b80ff5dfbdf0f0760c495c9bf0f88d06bdc227b77920fc8e`). Its `$defs/context.schema_digests[].role` permits only lowercase ASCII letters, digits, underscore and hyphen (`^[a-z][a-z0-9_-]{0,63}$`). The CamelCase `$defs` names are **not** valid values. Do not alter either the accepted schema or `$defs` identities.

For a private synthetic context, independently select exactly the following sorted unique label→definition pairs, all with the same full-file digest:

| Context label | Exact successor `$defs` |
| --- | --- |
| `candidate` | `candidate` |
| `events` | `events` |
| `final` | `final` |
| `host-git` | `hostGit` |
| `method-record` | `methodRecord` |
| `owner-read` | `ownerRead` |
| `policy` | `policy` |
| `prepared-source` | `preparedSource` |
| `proposal` | `proposal` |
| `raw-result` | `rawResult` |
| `response-read` | `responseRead` |
| `revision-identity` | `revisionIdentity` |
| `scanner` | `scanner` |
| `source-identity` | `sourceIdentity` |
| `target-record` | `targetRecord` |
| `target-result` | `targetResult` |

These context labels are indexing aliases only, not substituted `$defs`, schema IDs, selector roles, or new accepted schema bytes. The builder and replay select the exact array above independently of any claimant record and compare exact order, label and pinned digest. The four separate policy receipts all use the same `$defs/policy` and remain separately verified dependencies; the single `policy` label does not merge their identities. This is **implementation-only synthetic**; it does not pin current binary/runtime bytes, admit a query, establish producer custody, or qualify a live reference.
