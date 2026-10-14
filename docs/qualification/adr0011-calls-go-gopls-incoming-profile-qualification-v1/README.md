# ADR0011 Go/gopls incoming CALLS provider-profile qualification package v1

Profile: `CALLS_GO_GOPLS_INCOMING_EXACT_V1`  
Binding target: `CALLS_LSP_INCOMING_EXACT_V1`  
Producer disposition: `AUTHORED_NOT_GO`  
Provider qualification GO: **not producer-issued**; independent reviewer must decide.  
Authority ceiling: `authority=0`, `accepted=false`, `completeness=UNKNOWN`, production admission `REFUSED_NOT_REQUESTED_NOT_GRANTED`.

This package is a source-visible provider-binding correction for the exact incoming Go/gopls profile only. It consumes the accepted generic incoming semantics package and the accepted incoming-custody correction v2 run evidence. It does not edit runtime code, memory limits, admission gates, schemas, selectors, publication surfaces, raw-custody mechanics, custody policy, composition behavior, or any other direction/profile.

## Bounded result

The retained workload and custody-rerun evidence supports only this bounded statement:

> Under the pinned corpus/revision/environment and gopls v0.23.0 / go1.26.5 environment recorded by the accepted evidence packages, all 5900 incoming target ordinals have exactly one final disposition: 5608 non-empty successes, 225 successful-empty results, 67 LSP-error failures, and 0 refusals. The correction v2 run preserves those final counts exactly and closes the previously missing provider/runtime custody roles for gopls executable byte length, complete prelaunch environment capture, and Go executable/toolchain inventory. Numeric maxima remain measured maxima over the accepted historical workload package only; the frontier remains `UNKNOWN` because neither retained run provides a frontier bound.

It does not establish universal bounds, repository completeness, source-graph completeness, behavior completeness, production readiness, or correctness for any other provider, version, revision, target set, policy, or run.

## Accepted prerequisite pins

Generic semantics prerequisite:

- Token: `CALLS_LSP_INCOMING_EXACT_V1_SEMANTICS_GO`
- Commit: `c3d77247f5a2e06c0d6ac5b1b55a3df7dbea588a`
- Package: `docs/qualification/adr0011-calls-lsp-incoming-exact-v1-generic-semantics-v1`
- `CHECKSUMS.sha256` SHA-256: `844e2c072474f6b3756ec221ea0b6df2888d775e9b866e1da0bbef030e35b351`
- `MANIFEST.json` SHA-256: `8657876ab14ad6c4939218331d8a58e3c222da61b21ae55200644e5c43674f7d`

Incoming custody correction prerequisite:

- Token: `INCOMING_CUSTODY_CORRECTION_EVIDENCE_GO`
- Run package commit: `b4a86914702fd595f66bd1e0085c7063fe517144`
- Run directory: `docs/qualification/adr0011-calls-go-gopls-incoming-custody-rerun-addendum-v1/runs/incoming-custody-correction-v2/full-incoming`
- Final checksums SHA-256: `ba003061790bb4033bcabf85480e92ec97aba4522ab9877d99ccabbf24db60a8`
- Final seal SHA-256: `a15ddcb57a0f908e13e7742dd0697dff717fd08e66e2aee28a478020f1c3ba34`
- Final seal artifact count: `53380`
- Final seal status: `RUN_VERIFICATION_PASSED`

Provider text preserves the accepted generic decisions verbatim and supplies no provider-owned semantic override:

- top-level response null and [] remain distinct successful-empty source forms when admitted by terminal policy
- missing member fromRanges is malformed for semantic admission and preserved only in raw custody
- null member fromRanges is malformed for semantic admission and preserved only in raw custody
- present empty fromRanges: [] retains an otherwise-valid relationship/member and emits zero CALLS occurrence rows and zero occurrence range rows
- nonempty supplied fromRanges are the only CALLS occurrence ranges; supplied range order, target ordinal, member ordinal, and duplicate multiplicity are preserved without synthesized ranges
- returned from.range and from.selectionRange are endpoint identity/display spans, never occurrence evidence; CALLS occurrences are derived solely from supplied fromRanges

## Provider/runtime custody closure from v2

The correction v2 evidence closes the mandatory provider/runtime roles that the prior candidate failed closed:

- `gopls_executable_byte_length`: `41038194`, with gopls SHA-256 `82600f8b7f6ad337120b27ab738a5e44897ff1701919bc4620836ff7d98c2f7a`
- `complete_prelaunch_environment`: captured privately before gopls launch; public digest `21fe317ba06ab7dc9fbc0758dd1561945860a2471ffd133835db8d2b62f1d72c`, key count `64`, private record SHA-256 `32789e7ec3efde32db09e83c7e7dd5b080fc0564f3b2bb28fa6134454a62f48d`
- `go_executable_and_toolchain_inventory`: Go executable byte length `14481762`, Go executable SHA-256 `3f947495f00cb7f8088a5cfd694da8dc43869b33f5e7377b048fb18922ffb7e0`, closed inventory complete with `14` required records over `4` inventory scopes
- `launch_env_binding`: source `captured_env`, argv digest `2c18ebe2caf8fd4dd7ab9973de8faab265ab6019c55703050d0044818452a401`, workspace digest `f81c8f84ab2a3c92fc4c0a14a613186ae001dd6dc3be39c2067f52165c5defba`

The public package intentionally records only digests/counts for private environment values. It does not transfer custody ownership, expose raw private environment values, or alter production admission.

## Historical vs correction-run comparison

- Final disposition counts are exactly equal: historical accepted workload and correction v2 both report 5900 ordinals, 5608 non-empty successes, 225 successful-empty results, 67 LSP-error failures, and 0 refusals.
- Custody roles differ by design: the historical workload did not retain gopls byte length, complete ambient environment, or a closed Go toolchain inventory; correction v2 records those roles before launch and seals them.
- Raw inventories are not hidden as equal: historical raw aggregate digest is `dfedbc400dcdb7315143fd0479e791a67dda79331112bd922446e4f770c717bc`; correction v2 final artifact inventory digest is `2e59bdcc6ff3b1dab8d8303a18f544c1e8c7af6956802b2da14ec5c54c7939cc`, run-dir SHA-256 is `ddc7a39f2322ed5d3e0730f376563ef69e8ffd31c982c1f0928d3ea7f0174633`, and final checksums line count is `53380`.
- Numeric maxima remain inherited from the accepted historical workload package because correction v2 does not publish a replacement numeric-bounds artifact. They remain measured-not-universal and do not change runtime ceilings.

## Numeric envelope: measured witnesses only

- `max_elapsed_seconds` = `1.950021`
- `max_message_count_per_target` = `157`
- `max_prepare_request_bytes` = `309`
- `max_prepare_response_bytes` = `10861`
- `max_prepare_returned_members` = `1`
- `max_prepare_returned_nodes` = `1`
- `max_prepare_returned_ranges` = `2`
- `max_request_bytes` = `144704`
- `max_response_bytes` = `67906`
- `max_result_request_bytes` = `572`
- `max_result_response_bytes` = `67906`
- `max_result_returned_members` = `114`
- `max_result_returned_nodes` = `114`
- `max_result_returned_occurrences` = `328`
- `max_result_returned_ranges` = `328`
- `max_ledger_result_count` = `114`

These are measured maxima, not universal bounds, and do not alter any runtime ceiling. The frontier remains `UNKNOWN`.

## Recursive presumption lineage

1. First provider candidate NO_GO: `CALLS_GO_GOPLS_INCOMING_EXACT_V1_NO_GO`; hard stop for missing accepted generic `fromRanges` prerequisite and overstated provider custody.
2. Generic prerequisite GO: `CALLS_LSP_INCOMING_EXACT_V1_SEMANTICS_GO` at commit `c3d77247f5a2e06c0d6ac5b1b55a3df7dbea588a` closed provider-neutral incoming semantics only.
3. Failed pre-effect v1 provider correction: generic closure was consumed, but provider/runtime custody still carried stale missing-role blockers and could not qualify.
4. Second NO_GO: stale provider package remained `AUTHORED_NOT_GO` with fail-closed custody gaps before correction v2 evidence was consumed.
5. Incoming custody rerun corrections: addendum GO and head correction GO produced independently reviewed `INCOMING_CUSTODY_CORRECTION_EVIDENCE_GO` with final run checksums/seal pinned above.
6. Accepted v2 correction basis: counts remain exactly equal to the accepted workload run; mandatory custody roles are closed by v2; producer remains `AUTHORED_NOT_GO` and cannot self-issue provider GO.
7. Final verifier/tamper coverage NO_GO: reviewer rejected solely because the verifier did not independently assert every accepted generic semantic statement and authority numeric typing in both MANIFEST and RESULT.
8. Verifier/tamper coverage correction: this package now records exact MANIFEST/RESULT/role semantic preservation, numeric authority `0`, refusal/accepted-false checks, and assertion-bound tamper cases; producer remains `AUTHORED_NOT_GO`.
9. Occurrence-source policy escape NO_GO: reviewer found RESULT.semantic_artifacts.occurrence_source_policy could be mutated to endpoint-span-derived occurrences without failing.
10. Occurrence-source policy correction: one canonical policy is now required exactly in RESULT, MANIFEST, and role artifacts; mutation/deletion tamper cases fail `RESULT_OCCURRENCE_SOURCE_POLICY_EXACT`.

## Review boundary

Reviewers should verify the correction package and decide whether to issue `ADR0011_QUALIFIED_CALLS_INCOMING_PROFILE_GO` or return one exact bounded blocker. The producer does not issue GO. Production admission remains refused, authority remains separate, and this package conveys no public/runtime/custody ownership or ceiling change.
