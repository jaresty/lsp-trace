# ADR-0011 call-hierarchy Layer-1 generic corpus RED v2 opaque Params amendment review v7

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_RED_V2_AMENDMENT_REVIEW_GO`

This independent disposition accepts only the current amendment's exact opaque-Params correction. It does not accept, activate, or grant authority to the current v2 corpus, RED, implementation, generated artifacts, or later rematerialized hashes.

## Independent findings

- **Accepted fixture provenance is exact:** at accepted v1 RED commit `751b0276c46dbccc4899cfec32adf513c123b57f`, `sessionruntime/b4_owned_method_pair_lease_test.go` is blob `daea113100184745998cde6961d3164c0a3431ce` with file SHA-256 `e766074f72b2ff7d43a59d9532b52b3d5c150e4d30f3b61f431c9fab6a540dc2`. Its request constructor copies the supplied complete `params []byte` directly into `json.RawMessage` with `append(json.RawMessage(nil), params...)`. The amendment correctly makes those exact request-input bytes, or the exact generic request Params preimage fixed by that fixture, the derivation source.
- **Params remain opaque and nonsemantic:** `params_state` plus `params_hex` encode the complete `json.RawMessage` byte sequence supplied to `Manager.RoundTrip`, including every wrapper byte. They do not encode a selected request-body/payload subspan, do not restore any of the 17 removed rich paths, and cannot be reconstructed from an expected frame, expected semantic value, production execution, or implementation output. The illustrative `{"payload_hex":"7b7d"}` value is correctly encoded as the hex of that entire JSON value, not as the inner `7b7d` bytes.
- **Offline encoder validation is one-way:** during corpus derivation, the accepted generic wire encoder must combine the fixed Params bytes with the fixed method and request ID and reproduce the unchanged complete expected request frame. This validates the independently derived input; it does not make the frame an input to Params derivation and introduces no execution-time frame parsing.
- **Execution is direct:** execution hex-decodes `driver.request.params_hex` once, preserves nilness according to `params_state`, converts/copies the resulting bytes as opaque `json.RawMessage`, and passes them directly to `Manager.RoundTrip`. Parsing, wrapping, rewriting, semantic reconstruction, and reserialization are forbidden.
- **Nil, empty, and present remain distinct:** the accepted fixture exercises and preserves `nil`, non-nil empty, and nonempty Params, including a nilness equality check. The amendment's strict rules map these respectively to JSON null, `""`, and nonempty lowercase even-length hex, so unavailable/nil cannot collapse into available-empty and present values retain every byte.
- **Expected accounting is unchanged:** this correction changes only driver Params fields and consequent canonical lengths/hashes on rematerialization. Every retained decoded `expected` value remains fixed; the inventory remains exactly 162 paths, exactly 17 remain removed/deferred, and the expected denominator remains exactly 145 (`133` generic plus `12` opaque). Driver paths remain outside that denominator.

## Boundary

The current corpus and untracked successor RED are not accepted evidence that this correction has been satisfied. Corpus rematerialization, strict driver validation, independent reproduction of new canonical per-world and aggregate hashes, and RED execution review remain later gates. This review authorizes no corpus, RED, source, implementation, activation, generated-artifact, or commit change.
