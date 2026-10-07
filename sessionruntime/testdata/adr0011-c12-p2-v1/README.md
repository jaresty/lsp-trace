# ADR0011 C12 private references result-token packet v1

This packet carries hermetic source-and-test evidence for the private references-selected C12 exact original top-level `result` token denominator at 524,288 bytes.

Run the exact command encoded in `manifest.json` from the repository root. It proves that valid references `Location[]` result tokens of 524,287 and 524,288 bytes return exact noncustodial result bytes with synchronous cleanup, while 524,289 bytes produces typed resource exhaustion with no result bytes, private lease, successful response observation, or residual custody.

The large token is the actual references result, with padding inside a valid file URI. The decoded response remains below C10’s separate complete-message denominator.

The canonical manifest binds an exact ordered source denominator, pre-evidence baseline revision, selected tests, final five-pass result, contract-complete assertion inventory, and authority ceiling. It hashes neither itself nor this README.

Authority is limited to `C12_HERMETIC_PRIVATE_REFERENCES_RESULT_TOKEN_EVIDENCE`. It provides no C09/C10/C11 authority or recertification, no C16–C18 recertification, no broader matrix closure, no live-server/runtime proof, no managed-session identity, and no production authority.

Causal RED was independently accepted with `RED_REVIEW_GO` under integration receipt `20261007072022-0793`: four integrated results passed and only the absent packet test failed with `ASSERT_C12_PORTABLE_PACKET_MISSING`.
