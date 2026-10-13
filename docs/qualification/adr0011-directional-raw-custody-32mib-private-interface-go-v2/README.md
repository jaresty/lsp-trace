# ADR0011 directional raw-custody 32 MiB private Go interface v2

Status: `AUTHORED_NOT_GO`  
Exact token: `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRIVATE_INTERFACE_GO_V2`  
Target HEAD: `f03672416bbe1342c06e7cbb33bf51127909ce8a`

This documentation-only successor preserves the generalized revision while restoring the halted v1 byte-for-byte. Its noncircular lineage depends on committed reviewed package `adr0011-generic-directional-calls-profile-amendment-go-v1` and immutable predecessor `adr0011-directional-raw-custody-32mib-private-interface-go-v1` at `SHA256SUMS` digest `d36211da45adb55e83f0b112a62140d7bec5ef2076edee12c3facebbd692af7c`; neither dependency refers to v2. It freezes provider-neutral, compile-ready private declarations for independent `CALLS_LSP_INCOMING_EXACT_V1` and `CALLS_LSP_OUTGOING_EXACT_V1` contracts while preserving the accepted opaque Layer 1, sole-owner/C15 lease, Manager reservation, exact-once terminal, D/R separation, and `CALLS_ONLY` architecture.

Normative order:

1. `INTERFACE.md`
2. `sessionruntime_private.go.txt`
3. `lspwire_private.go.txt`
4. `STATE_AND_OWNERSHIP.md`
5. `WRITESET.md`
6. `HARNESS_EVIDENCE.md`
7. `DEPENDENCIES.sha256`
8. `REVIEW_REQUEST.md`
9. `verify_package.py` — package-local structural, lineage, checksum, and write-boundary guard

Profile digests use the exact canonical binary preimage `"lsp-trace.adr0011.calls.profile-identity.digest.v1\x00" || uint32be(len(domain)) || domain || uint32be(len(id)) || id` before SHA-256. Attempted target ordinal/outcome records and the ordered unattempted frontier form the exact-once target-set partition. Release remains separate lifetime/release metadata and is not a disposition.

The `.go.txt` files are exact `gofmt` output from a disposable `/tmp` compile harness. Harness stubs and mirrors prove declaration, constructor, method-set, planned-call-site, and package-privacy typing only. They are not behavior, implementation, production tests, evidence of provider correctness, or copy source.

No Go/gopls semantics occur in this generic interface. Provider bindings remain separate; only an opaque binding identity/receipt is representable. No package file outside this existing untracked package was changed.

Hard stop: any public/dependency/duplicate-owner/duplicate-charge/architecture/default/activation/write-set change requires a new reviewed version.

AUTHORED_NOT_GO
