# Manifests and lineage

## Accepted predecessor package

The wrapper pins the accepted bidirectional qualification package rather than reinterpreting provider payloads:

- Qualification package: `docs/qualification/adr0011-private-bidirectional-composition-qualification-v1/`
- Accepted artifact SHA-256: `07212f4175aeb115dda5828761b3338b56233704673e3a3bbf861373f174e3eb`
- Accepted package checksum SHA-256: `041247bf62ec7b981584971bcaafa24d29081f730fbb2ef55284b73d0db192b2`

## Direction pins

Incoming:

- provider profile: `CALLS_GO_GOPLS_INCOMING_EXACT_V1`
- generic profile: `CALLS_LSP_INCOMING_EXACT_V1`
- evidence/result: `e9c8cce4b77dc91a8ed435166fbf88ce9d6e12bbc28bd4ad0db4fbbb45cc4154`
- provider package checksum: `e0b1a9ea0b24e51768fad3bd89194ac50023dab8e370e6cc9a96ff6d78135abf`
- provider manifest: `675456686a9b073b4462685ad833456ae02942b659f6ecc70bc8504dab401579`
- ordinal identity hash: `273d62e8e91d0b332ada4e1d18e68ef4404f549fed14923879a3e15a8928677d`

Outgoing:

- provider profile: `CALLS_GO_GOPLS_OUTGOING_EXACT_V1`
- generic profile: `CALLS_LSP_OUTGOING_EXACT_V1`
- evidence/result: `e6a60c88f4bd00d2e220d160765c8ee903903523cb86cfb7d814e31a7c433c9f`
- provider package checksum: `411e2195723fc5ac5bb6a7af27331b187ca016d0d75767b634454223e5978e80`
- provider manifest: `7fd171adce13cda709193365fac10c34566a3472e093f347650f2cbf336fa543`
- ordinal identity hash: `7843f1430e5ddb401c8f8ce76eb3a1b3188769ff9f625ee6e6a10b9b7d1544e4`

## Local artifact lineage

- Wrapper source SHA-256: `e300a537b11ba6f3a6f8e42eb6e9911025f314a9cc460e686c564fba0318e201`
- Wrapper test SHA-256: `bff95f21b17a18833d481e64f593cfbeb3baedbf113777a2d43108ec2c422e03`
- Mutation gate script SHA-256: `daa4c02cd888b8a5dc5c4e851bc8f9bce16e330d88aaa635b1978be895c8603b`
