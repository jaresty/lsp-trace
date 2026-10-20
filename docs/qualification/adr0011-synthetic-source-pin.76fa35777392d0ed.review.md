# ADR0011 synthetic source pin review candidate — 76fa35777392d0ed

Status: AUTHOR-PREPARED REVIEW CANDIDATE; INDEPENDENT ACCEPTANCE NOT CLAIMED

## Candidate

- Predecessor manifest: `adr0011-synthetic-source-pin.25166c996fd6ade.manifest.json`
- Predecessor manifest SHA-256: `25166c996fd6adecc4cf21e15b7e7321f9172568b755e6e19616d500220bc505`
- Predecessor review: `adr0011-synthetic-source-pin.25166c996fd6ade.review.md`
- Predecessor review SHA-256: `760997f6a4ca3da7b636dfff2b5d1c0da88c9317b5e44f5cb73873fa0cac6984`
- Successor manifest: `adr0011-synthetic-source-pin.76fa35777392d0ed.manifest.json`
- Successor manifest SHA-256: `76fa35777392d0ed84237c1b6e532612cd81146b95320498203ffca16c38fc61`
- Successor aggregate digest: `sha256:f50cbb3e24b0b44b09e408b96c94e0cc6019e60c724ad4e67fc6ab35fe57a25a`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: 128 files
- Current selected source bytes: 1173828

## Predecessor classification

The predecessor and every earlier manifest/review remain immutable historical source-byte evidence. This candidate supersedes none of them and does not alter the selected reviewed pin. Relative to `adr0011-synthetic-source-pin.25166c996fd6ade.manifest.json`, it records 10 additions, 28 modifications, and 0 removals.

- Added: `internal/adr0011work/account.go`, `internal/lspwire/c15_site_bindings.go`, `internal/publication/boundfile_cas.go`, `internal/publication/boundfile_cas_other.go`, `sessionruntime/adr0011_c17_c15_observer_contract_private.go`, `sessionruntime/b4_c15_ownership_registry_private.go`, `sessionruntime/b4_c15_site_bindings_private.go`, `sessionruntime/c18_pair_coordinator_private.go`, `sessionruntime/call_hierarchy_directional_raw_v5_private.go`, `sessionruntime/provider_message_composition.go`
- Modified: `internal/adr0011acquisition/owner.go`, `internal/adr0011acquisition/proposal_context.go`, `internal/adr0011acquisition/proposal_record.go`, `internal/adr0011acquisition/raw_scanner.go`, `internal/adr0011acquisition/scanner_record.go`, `internal/adr0011acquisition/synthetic_source_pin.go`, `internal/lspwire/read_frame.go`, `internal/lspwire/successor_ingress.go`, `internal/publication/selector.go`, `internal/publication/verifiable.go`, `sessionruntime/b4_capability_event_c17_private.go`, `sessionruntime/b4_definition_private.go`, `sessionruntime/b4_event_account_c17_private.go`, `sessionruntime/b4_explicit_bytes_c15_private.go`, `sessionruntime/b4_owned_method_pair_lease.go`, `sessionruntime/b4_retained_notification_c15_private.go`, `sessionruntime/b4_retained_response_c15_private.go`, `sessionruntime/b4_server_error_lease_c15_private.go`, `sessionruntime/b4_server_request_c15_private.go`, `sessionruntime/diagnostic_backing_c15_private.go`, `sessionruntime/diagnostic_history_storage_c15_private.go`, `sessionruntime/diagnostic_runtime.go`, `sessionruntime/method_frame_correspondence.go`, `sessionruntime/owned_method_pair.go`, `sessionruntime/private_encoded_request.go`, `sessionruntime/readiness_history_private.go`, `sessionruntime/sessionruntime.go`, `sessionruntime/startup_attempt.go`
- Removed: none

## Author-prepared reproducibility record

The successor was derived from the verifier's twelve authoritative directories, direct regular non-symlink `.go` files only, excluding `_test.go`, sorted bytewise by slash-relative path. The aggregate uses `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00` followed per path by uint64-be path length, UTF-8 path bytes, uint64-be source byte length, and raw SHA-256 digest. Canonical compact sorted-key JSON includes one trailing newline; the filename is the first 16 hexadecimal characters of the manifest SHA-256.

## Independent reviewer checklist

- [ ] Confirm all predecessor source-pin artifacts remain byte-identical to their committed versions.
- [ ] Independently enumerate the verifier's exact source selection and compare every ordered path.
- [ ] Independently recompute every entry byte length and SHA-256.
- [ ] Independently recompute the aggregate digest, manifest hash, and filename.
- [ ] Run `TestReviewedSyntheticSourcePin` and downstream final-issuance guards only after independently selecting this successor.
- [ ] Record an independent disposition separately; this candidate is not that disposition.

## Authority boundary

This author-prepared candidate records source-byte reproducibility only. It grants no independent acceptance, capability, final-issuance authority, production authority, Census execution, provider authentication, binary identity, dependency closure, runtime behavior, or qualification.
