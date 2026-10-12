# ADR0011 C11 private definition result-token packet v1

This packet carries hermetic source-and-test evidence for the private definition-selected C11 exact original top-level `result` token denominator at 524,288 bytes.

Run the exact command encoded in `manifest.json` from the repository root. It proves that valid definition `Location[]` result tokens of 524,287 and 524,288 bytes are accepted with exact result custody and cleanup, while 524,289 bytes produces typed resource exhaustion with no published result bytes, result lease, successful response observation, or residual custody.

The large token is the actual definition result, with padding inside a valid file URI. The complete decoded message remains below C09’s separate complete-message denominator.

The canonical manifest binds an exact ordered source denominator, pre-evidence baseline revision, selected tests, final five-pass result, contract-complete assertion inventory, and authority ceiling. It hashes neither itself nor this README.

Authority is limited to `C11_HERMETIC_PRIVATE_DEFINITION_RESULT_TOKEN_EVIDENCE`. It provides no C09/C10 authority or recertification, no C12 authority or closure, no C16–C18 recertification, no broader matrix closure, no live-server/runtime proof, no managed-session identity, and no production authority.

Causal RED was independently accepted with `RED_REVIEW_GO` under integration receipt `20261007065118-1685`: four integrated results passed and only the absent packet test failed with `ASSERT_C11_PORTABLE_PACKET_MISSING`.
