# ADR0011 generic V2 SOURCE selector preimage — unaccepted B3 proposal

**Status: PROPOSED ONLY.** This is a new, additive SOURCE-role preimage candidate. It does not modify or reissue the accepted V2 schema, transport or method-policy originals, the 14 QUERY/TARGET_EVENTS vectors, A2, B1/B2, or historical V1 bytes. No SOURCE selector is currently issuable or replay-admissible from this proposal. B3 SOURCE implementation waits for independent exact-byte review and a separate implementation-only decision. Capability chronology is out of scope.

## Verifier-owned input and transaction domain

The intended verifier input is an ordered, copied collection, **never selected from claimant records**:

```go
type HeldSourceOriginal struct {
    TransactionIdentity string
    SourceSelector      string
    URIBytes            []byte
    VersionBytes        []byte
    ContentBytes        []byte
    CustodyKind         SourceCustodyKind
}
```

The closed custody vocabulary is exactly `OWNER_BUFFER`, `MANAGED_VIRTUAL`, `CLEAN_REGISTERED_WORKTREE`, `IMMUTABLE_SOURCE_SNAPSHOT`, as in the accepted V2 SOURCE schema. `URIBytes` must be nonempty original valid UTF-8 URI string bytes and `VersionBytes` must be valid UTF-8 without normalization; `VersionBytes == nil` is **absent and invalid**, whereas a non-nil empty slice is the valid present empty JSON string. `ContentBytes == nil` is unavailable and invalid; a non-nil empty slice is complete empty source content. No URI parsing, normalization, reserialization or version coercion may replace byte equality. Source content is bounded by the unchanged transport `source_document_bytes:4194304`; transaction SOURCE count remains 1–256. URI/version byte acquisition and total allocation must be bounded by a separately reviewed verifier limit before issuance; this proposal selects no unbounded reader.

Given independently held B2 `(session, generation, transaction)` with nonempty UTF-8 session/transaction and an integer generation in `1..2^64-1` (Boolean and floating-point inputs are invalid), define `LP(b)` as an unsigned 64-bit big-endian byte length followed by the exact byte string `b`. Decimal integers use shortest ASCII base-10 with no sign or leading zeros. Define `TX = "sha256:" + lowercase_hex(SHA256(LP("ADR0011-GENERIC-TRANSACTION/1") || LP(session UTF-8) || LP(decimal generation) || LP(transaction UTF-8)))`. `HeldSourceOriginal.TransactionIdentity` must equal **this verifier-computed TX**, not claimant text. The SOURCE selector preimage binds the same TX. This digest is a transaction-domain identifier, not responder authentication.

The verifier must reject duplicate held SOURCE selectors before indexing; no map overwrite is allowed. Exactly one held entry has the independently held B2 query SOURCE selector, placed first. All remaining held SOURCE entries are sorted strictly by raw ASCII selector bytes; duplicates, omitted and extra held entries, cross-transaction TX, and query/target substitution fail closed against the independent B2 role selection. Deep-copy every slice and the ordered collection before replay; recheck immutability at the verifier boundary. The claimant must not supply the held collection, TX or expected source digest.

## Proposed SOURCE preimage (not an A2 selector extension)

For each method, `profile` is its accepted `GENERIC_LSP_REFERENCES_EXACT_V2` or `GENERIC_LSP_DEFINITION_EXACT_V2` selector, and `policy` is the *embedded, length/hash-pinned* corresponding V2 method original. `schema` and `transport` are likewise the A2-pinned originals. `digest(b)` is lowercase hex SHA-256 **without** the `sha256:` prefix. No caller-supplied schema/policy digest is an expected value.

First form a SOURCE artifact digest from verifier-owned fields only:

```text
artifact_preimage = LP("ADR0011-GENERIC-SOURCE-ARTIFACT/1")
                  || LP(TX ASCII) || LP(URIBytes)
                  || LP("present") || LP(VersionBytes)
                  || LP(decimal len(ContentBytes))
                  || LP(digest(ContentBytes) ASCII)
                  || LP(CustodyKind ASCII)
artifact_digest = digest(artifact_preimage)
```

`LP("present")` is a domain-separated presence marker; absence fails before hashing and must never alias present empty. SHA-256 does not establish byte equality: B3 must **also** compare full claimant source bytes, URI bytes and version bytes against these independently held bytes and derive both length and digest anew from `ContentBytes`.

Extend only the previously unspecified `SOURCE` role of the existing V2 length-prefixed common selector format; do **not** change QUERY or TARGET_EVENTS:

```text
selector_preimage = LP("ADR0011-GENERIC-EXACT/2")
                  || LP(profile ASCII) || LP(method ASCII) || LP("SOURCE")
                  || LP(digest(schema) ASCII) || LP(digest(transport) ASCII)
                  || LP(digest(policy) ASCII)
                  || LP(session UTF-8) || LP(decimal generation)
                  || LP(artifact_digest ASCII) || LP(TX ASCII)
SOURCE_selector = "sha256:" + lowercase_hex(SHA256(selector_preimage))
```

The final TX field is SOURCE-specific and binds the exact independently derived transaction domain in the outer selector as well as its artifact. It is not retroactively added to the two A2 selector roles. A B3 verifier must recompute this selector from held bytes and compare it with `HeldSourceOriginal.SourceSelector` **and** B2's selected SOURCE selector, then compare each claimant SOURCE record's selector, transaction identity, custody, URI/version bytes, complete content bytes, length and digest. The claimed length/digest/URI/version or `verified` flags are never an independent expected value. A matching selector/hash alone is insufficient.

## Offline falsifiers and blocked boundary

The candidate vector fixture must freeze two methods, query and target sources, URI-byte, present-empty versus absent version (absent rejects before selector), version-byte, one-content-byte, custody, transaction, session/generation, schema-LF and policy-LF substitutions. Its seven negative inputs (absent version/content, invalid UTF-8 URI/version, unknown custody, floating-point generation and Boolean generation) must execute as rejections in Python and an independent Go test; labels with null values are not negative evidence. Identical SOURCE bytes across distinct TX values must not share a selector. **Unexecuted B3 obligations:** negative missing/extra/duplicate held-selector and cross-transaction cases must later run against the ordered verifier-owned collection, not a map inferred from claimant graph edges. A constant/stub digest with unequal full bytes must later fail the full-byte comparison even if a test double arranges identical reported digest strings. This proposal's vector generator and Go oracle do not implement or test those collection/byte-comparison claims. The required B3 same-claimant-graph contrast is pending until this preimage is accepted: matching independently held bytes passes, altered independently held version or one content byte leaves B2 graph success unchanged but B3 rejects.

This document and any offline vectors authorize **no B3 code**, SOURCE selector issuance, capability chronology, acquisition, publication, admission, qualification, lifecycle D or public route. V1 remains historical/unissued and A2/B1/B2 retain only their independently accepted private scopes.
