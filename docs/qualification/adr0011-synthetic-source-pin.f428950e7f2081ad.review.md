# ADR0011 synthetic source pin review — f428950e7f2081ad

**Verdict: REPIN_VERIFY_GO — independently corrected and reproduced within the source-pin boundary; authority 0.**

## Identity

- Source HEAD: `07344fe6`
- Manifest: `docs/qualification/adr0011-synthetic-source-pin.f428950e7f2081ad.manifest.json`
- Manifest SHA-256: `f428950e7f2081ad6c86c4c80ce118418067a7800cc586eb57c2a0a1770b6d96`
- Aggregate: `sha256:1a98a918299ef63e438700ad7cb266ced68da15b81fc017abf0e6e290892c627`
- Selection: 170 paths, 1,741,323 bytes

## Independent ordering correction

The first proposed manifest, SHA-256 prefix `981f`, was rejected and is absent. Its ordering did not satisfy the implementation's canonical three-segment verifier order. The corrected manifest was independently reconstructed in the exact required order:

1. 128 direct regular non-symlink `.go` files from the twelve authoritative directories, excluding `_test.go`, in strictly increasing lexical path order;
2. 38 additional Go paths in the exact declaration order of `syntheticSourceAdditionalGoPaths`;
3. four additional schema paths in the exact declaration order of `syntheticSourceAdditionalSchemaPaths`.

The corrected bytes hash to the filename-bound manifest SHA-256 above. The correction changes ordering only as required to make the manifest conform to the verifier's canonical implementation semantics; it does not broaden the selected source set or its authority.

## Implementation semantics

`VerifySyntheticSourcePin` verifies repository source bytes only. The verifier requires canonical UTF-8 slash-relative paths; the exact segment ordering above; regular non-symlink leaves with no symlink ancestors; exact per-file byte lengths and SHA-256 values; exact framed aggregate identity; bounded path, per-file, and total-byte limits; complete enumeration of eligible non-test Go files in the authoritative directories; and either the historical additional-path profile or the complete successor profile.

The successor profile is valid only when all 38 declared additional Go paths and all four declared schema paths are present in declaration order after the 128 directory-selected Go paths. Reordering, omission, unexpected selection, path mismatch, source-byte drift, incomplete successor membership, limit breach, or aggregate mismatch fails closed.

These semantics establish an exact source-byte pin. They do not establish build or binary identity, dependency closure, runtime execution, source completeness outside the bounded policy, provider authentication, deployment fitness, feature identity, or production behavior.

## Lineage

Against predecessor manifest SHA-256 `71f9d77c4860911e6e9fcf7e413573d6e665e2df42839536d7653039bfa3870c`, paired with review SHA-256 `b5516dbead0df7e0d7e3943773677ec6d299abee446111a0d94ba049bd145a72`, the corrected manifest records:

- 165 unchanged paths;
- three changed paths;
- two added paths;
- zero removed paths.

The predecessor remains immutable lineage evidence. This review does not rewrite or retroactively strengthen it.

## Verification gates

The independently supplied verification packet passed all source-pin gates at source HEAD `07344fe6`:

- exact manifest SHA-256 `f428950e7f2081ad6c86c4c80ce118418067a7800cc586eb57c2a0a1770b6d96`;
- exact aggregate `sha256:1a98a918299ef63e438700ad7cb266ced68da15b81fc017abf0e6e290892c627`;
- exact canonical verifier order `128 + 38 + 4`;
- exact accounting of 170 paths and 1,741,323 bytes;
- exact predecessor delta `165 same / 3 changed / 2 added / 0 removed`;
- rejected `981f` candidate absent;
- focused tests passed;
- focused vet passed.

The predecessor-bound selector is expected to remain stale and fail closed. Selector mutation, commit, publication, and push are outside this review.

## Authority ceiling and unresolved disposition

```text
authority=0
accepted=false
featureIdentity=UNRESOLVED
completeness=UNKNOWN
effects=NONE
implementationDecision=UNISSUED
production_admission=REFUSED_MISSING_EVIDENCE
```

`REPIN_VERIFY_GO` accepts only the corrected manifest's bounded reproducibility against the stated source-pin implementation and verification gates. No external acceptance contract is supplied. This review therefore grants no feature identity, completeness, semantic acceptance, runtime claim, implementation acceptance, selector admission, or production admission; all such propositions remain unresolved or refused for missing evidence.
