# ADR0011 C09 private definition packet v1

This packet carries hermetic source-and-test evidence for the private definition-selected C09 decoded-remarshal denominator at 1,048,576 bytes.

Run the exact command encoded in `manifest.json` from the repository root. It proves that decoded messages of 1,048,575 and 1,048,576 bytes continue through a valid definition response, while 1,048,577 bytes produce typed resource exhaustion before accepted observation or downstream definition publication, with synchronous cleanup.

The large message is a notification followed by an ordinary valid definition response. This isolates C09 from the separate C11 result-token denominator.

The manifest binds the exact ordered source denominator, pre-evidence baseline revision, selected tests, expected passing names, assertions, and authority ceiling. It does not hash itself or this README.

Authority is limited to `C09_HERMETIC_PRIVATE_DEFINITION_INGRESS_EVIDENCE`. It provides no C10 closure, no C11/C12 or C16–C18 recertification, no broader matrix closure, no live-server/runtime proof, no managed-session identity, and no production authority.

Causal RED was independently accepted with `RED_REVIEW_GO` under integration receipt `20261007054705-9006`: four integrated results passed and only the absent packet test failed with `ASSERT_C09_PORTABLE_PACKET_MISSING`.
