# ADR0011 C10 private references packet v1

This packet carries hermetic source-and-test evidence for the private references-selected C10 decoded-remarshal denominator at 1,572,864 bytes.

Run the exact command encoded in `manifest.json` from the repository root. It proves that decoded notifications of 1,572,863 and 1,572,864 bytes continue to an ordinary valid references result, while 1,572,865 bytes produces typed resource exhaustion before accepted notification, successful response observation, or downstream result publication, with synchronous cleanup.

The large message is a notification followed by a small ordinary references response. This isolates C10 from the separate C12 result-token denominator.

The canonical manifest binds an exact ordered source denominator, pre-evidence baseline revision, selected tests, final five-pass result, contract-complete assertion inventory, and authority ceiling. It hashes neither itself nor this README.

Authority is limited to `C10_HERMETIC_PRIVATE_REFERENCES_INGRESS_EVIDENCE`. It provides no C09 authority or recertification, no C11/C12 authority or closure, no C16–C18 recertification, no broader matrix closure, no live-server/runtime proof, no managed-session identity, and no production authority.

Causal RED was independently accepted with `RED_REVIEW_GO` under integration receipt `20261007061556-9837`: four integrated results passed and only the absent packet test failed with `ASSERT_C10_PORTABLE_PACKET_MISSING`.
