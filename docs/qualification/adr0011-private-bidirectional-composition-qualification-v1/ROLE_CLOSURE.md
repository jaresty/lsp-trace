# Role and non-role closure

## Required roles

| Role | Incoming | Outgoing | Closure |
|---|---|---|---|
| Directional envelope | `incoming` object | `outgoing` object | Present and separately addressable |
| Success result role | Accepted incoming profile/evidence reference | Accepted outgoing profile/evidence reference | Exact success payload pins verified |
| Failure result role | Private fixture-native incoming failure | Private fixture-native outgoing failure | Valid failure envelopes accepted without success claims |
| Profile id | `CALLS_GO_GOPLS_INCOMING_EXACT_V1` for success | `CALLS_GO_GOPLS_OUTGOING_EXACT_V1` for success | Preserved in success payloads only |
| Generic profile id | `CALLS_LSP_INCOMING_EXACT_V1` for success | `CALLS_LSP_OUTGOING_EXACT_V1` for success | Preserved in success payloads only |
| Accepted dependency commit/package hash | Pinned in `INPUT_PINS.json` | Pinned in `INPUT_PINS.json` | Recomputed by verifier |
| Directional success counts | 5900 attempted, 5608 nonempty, 225 empty, 67 failed | 5900 attempted, 4378 nonempty, 1455 empty, 67 failed | Preserved only for success envelopes |
| Directional failure counts | `counts_claimed:false` | `counts_claimed:false` | No fabricated counts |
| Failure identity | Stable incoming failure code/stage/message digest/evidence ref | Stable outgoing failure code/stage/message digest/evidence ref | Direction-specific and private fixture-native; message digest exact lowercase `sha256:[0-9a-f]{64}` |
| Failure reference | Path/hash only for success; fixture evidence ref for failure | Path/hash only for success; fixture evidence ref for failure | Immutable reference |
| Ledger reference | Path/hash only, merge false | Path/hash only, merge false | No concatenation |
| Numeric bounds reference | Path/hash only, measured-not-universal | Path/hash only, measured-not-universal | No ceiling change |
| Raw provenance | Digest/count references only | Digest/count references only | Raw bytes opaque |
| Custody refs | Correction run hashes/counts | Correction run hashes/counts | Separate directional custody |
| Authority/accepted/production | 0/false/refused | 0/false/refused | No shared authority |
| Completeness/frontier | UNKNOWN/UNKNOWN | UNKNOWN/UNKNOWN | Pair remains UNKNOWN/UNKNOWN |
| 32MiB ceiling | 33554432 per direction | 33554432 per direction | No combined ceiling admitted |

## Explicit non-roles

- No production admission or production GO.
- No public, default-on, CLI, MCP, API, schema, publication, release, deployment, or integration role.
- No ADR0011->ADR0007 composition role.
- No Stage 8 role.
- No raw provider byte carrier, raw byte copy, raw byte remarshal, raw byte reconstruction, or ledger concatenation role.
- No graph construction, union, deduplication, inverse synthesis, or CALLS truth role.
- No role allowing one direction to authenticate, erase, relabel, complete, or authorize the other.
- No role changing existing 32MiB raw-custody ceilings.
- No role converting invalid composition input into a valid directional failure.
- No role promoting failure to success or copying peer evidence into failure.

## Closure result

All required roles are closed by immutable metadata references, valid private fixture-native failure envelopes, subtree-preservation assertions, and independent verifier checks. All non-roles are refused by contract text, schema sketch, canonical artifact shape, verifier validation, unknown-key rejection, and adversarial tamper cases.
