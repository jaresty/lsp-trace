# ADR0011 private bidirectional CALLS composition qualification v1

Status: `ADR0011_PRIVATE_BIDIRECTIONAL_COMPOSITION_AUTHORED_NOT_GO`.

This package is private, default-off, evidence-only, and metadata-only. It composes incoming and outgoing CALLS evaluation results as two independent directional envelopes under an ordered top-level `{incoming, outgoing}` object. It grants no GO, no production admission, no public/API/CLI/MCP/schema exposure, no ADR0011->ADR0007 composition, no Stage 8, no integration, no commit, no deployment, and no release.

## Corrected representation

The canonical composition artifact is exactly an ordered object:

```json
{"incoming":<directional-envelope>,"outgoing":<directional-envelope>}
```

Each directional envelope is a closed sum type:

- `SUCCESS`: immutable reference to the accepted qualified directional profile/evidence plus preserved counts, hashes, custody, numeric-bound, raw-provenance, authority, completeness, frontier, production-refusal, and CALLS-only fields.
- `FAILURE`: bounded private fixture-native directional failure result with direction-specific code/stage/message digest/evidence reference, authority `0`, completeness `UNKNOWN`, frontier `UNKNOWN`, production refused, and no successful counts/claims. Its `message_digest` must be a string exactly matching lowercase `sha256:[0-9a-f]{64}`.

Invalid composition inputs are not represented as failures. Pin/hash/profile/direction/schema drift, cross-source references, missing/swapped direction, unknown keys, and semantic aliases are rejected.

## Stricter-contract inspection

The existing stricter private `ExactCarrierV1` contract found in `docs/qualification/adr0011-adr0007-private-composition-contract.effe3f74.md` is scoped to ADR0011->ADR0007 and requires `ADR0011_ADR0007_COMPOSITION_CONTRACT_GO`. This assignment explicitly excludes ADR0011->ADR0007 changes, so that carrier representation is not applicable here. No accepted generic/private incoming/outgoing bidirectional composition contract imposing a stricter representation was found in small qualification files inspected for this package.

## Dependency pins

The verifier pins and recomputes all four accepted dependencies by commit and package `CHECKSUMS.sha256` hash:

- generic incoming semantics: `c3d77247f5a2e06c0d6ac5b1b55a3df7dbea588a`, package hash `844e2c072474f6b3756ec221ea0b6df2888d775e9b866e1da0bbef030e35b351`.
- gopls incoming profile: `61bfa264d30595016ddb562082dd89d8d3f175da`, package hash `e0b1a9ea0b24e51768fad3bd89194ac50023dab8e370e6cc9a96ff6d78135abf`.
- generic outgoing semantics: `17057a8c53af8e8fe66be9c37788bbf59d10c5cd`, package hash `8c0efb403147ff70ad4def97809c65ec026c4984fdd8c8b5f7e1f9d0e31de6db`.
- gopls outgoing profile: `b196fa22beffc1a0fa18c4fc6bc192f07967721e`, package hash `411e2195723fc5ac5bb6a7af27331b187ca016d0d75767b634454223e5978e80`.

## Raw evidence policy

Canonical metadata serialization is allowed. Raw provider evidence bytes are never read, copied, decoded, remarshal, reconstructed, concatenated, or embedded by this package. Raw custody is retained only by immutable path/hash/count references inside each direction.

## Verification

Run:

```bash
python3 docs/qualification/adr0011-private-bidirectional-composition-qualification-v1/verify_bidirectional_composition.py
```

The verifier checks dependency commit/package hashes, exact directional evidence fields, ordered representation, byte-stable idempotence, valid directional failure retention, exact failure `message_digest` format, canonical subtree preservation, per-direction 32MiB ceiling preservation with no combined ceiling admission, closed-key rejection, cross-direction reference rejection, and the adversarial tamper suite.
