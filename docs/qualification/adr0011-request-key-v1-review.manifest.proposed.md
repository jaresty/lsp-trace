# ADR0011 request-key V1 review manifest — PROPOSED, version 1

SHA-256 below is lowercase hexadecimal over exact file bytes. These values describe this local review candidate, not accepted implementation authority. No accepted manifest or hash assertion is replaced.

| Role | Path | SHA-256 |
| --- | --- | --- |
| Frozen base issuance schema (unchanged) | `docs/qualification/schemas/adr0011-references-issuance-records.proposed.schema.json` | `bf49de9460fee5a68ba13204f633dc72d2dca566fa5f92d704bc19b5514618ba` |
| Existing freeze-manifest context (unchanged) | `docs/qualification/adr0011-references-contract-freeze-artifacts.manifest.md` | `483824f178b79484a701fbee36a051d01add4a996921912f58d34aca0d3664e2` |
| Versioned proposed successor schema | `docs/qualification/schemas/adr0011-references-issuance-records.request-key-v1.proposed.schema.json` | `86bb214e419f7c69b80ff5dfbdf0f0760c495c9bf0f88d06bdc227b77920fc8e` |
| Proposed normative addendum | `docs/qualification/adr0011-request-key-v1.addendum.proposed.md` | `4c08030265098f8906579f910dcb27c8bb54652cd165f041fe993a3ea9826a57` |
| Private codec | `internal/adr0011requestkey/key.go` | `df9f7ce5a930780efa4966d6bea195df21aef9f9111edecdb2e0a4042b01f995` |
| Codec tests | `internal/adr0011requestkey/key_test.go` | `00adac325b41290ebb74f544b038d7d6dffc49842fbb6fa863ccc34fcc81ed53` |
| Proposed schema tests | `docs/qualification/schemas/references_request_key_v1_proposed_test.go` | `2eb7b5112eee0d673a6b201bc4e3f83f45e7f74920116a804a643549360a17a9` |

The proposed successor's selected candidate digest is `86bb214e419f7c69b80ff5dfbdf0f0760c495c9bf0f88d06bdc227b77920fc8e` over complete schema bytes; it is not trusted implementation authority until an independent acceptance decision explicitly pins it against the distinct successor `$id` and reconciles replay with the accepted predecessor at `c817e09f5d7235309f3b0e6a29ebea476f67f5f2`. The original schema digest above differs from older historic freeze-manifest assertions because those manifests record their own past artifact revisions. Do not rebaseline any of them.
