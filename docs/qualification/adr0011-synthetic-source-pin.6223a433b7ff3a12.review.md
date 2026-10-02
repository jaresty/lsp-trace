# ADR0011 synthetic source pin candidate review correction — 6223a433

## Disposition

**REJECT_FOR_SELECTOR_UPDATE.**

The candidate exactly described 92 predecessor-selected paths and their current bytes, but the review assignment omitted the verifier's independent current-directory census. `VerifySyntheticSourcePin` rejects every non-test Go implementation file in its eleven selected directories that is absent from the manifest. The focused selector test therefore failed.

The candidate manifest remains an immutable record of the incomplete 92-path attempt. It must not be selected, used for private final issuance, or treated as a current-source pin.

Correction receipt: `20261001235751-5651`.

## Bounded facts retained

- Candidate manifest SHA-256: `6223a433b7ff3a12ed08543dfeaf239b9800e31796696e7de66d04385f5f2873`
- Candidate byte length: `14,691`
- Candidate aggregate over its 92 declared entries: `sha256:de910c106f298095e30f9c2cdf5ab0b840c8821d03e2027219e7702e0390cb9a`
- Historical predecessor manifests remain unchanged.

These facts do not repair the omitted-file defect or authorize selector adoption.
