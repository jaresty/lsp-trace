# ADR-0011 call-hierarchy Layer-1 generic corpus RED v2 disabled request-ID amendment review v6

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_RED_AMENDMENT_GO`

Reviewed independently at repository HEAD `1a9a71cb2e2b35bb99800cafeb21c12076a550c6`. This disposition accepts only the availability-aware disabled request-ID amendment contract. It does not accept, activate, or grant authority to the current v2 corpus or RED.

## Independent findings

- **Available identity is causal:** when retained control/attachment observation is available, `driver.request.request_id` is derived before projection from the numeric retained v1 `expected.identity.request_identity.request_key`. The amendment requires it to be a positive JSON integer representable as `uint64` and to equal both fixed complete-frame top-level IDs and the available retained request identity.
- **Unavailable identity is inert:** for a W25 disabled execution with unavailable control/attachment observation, the retained expected request-identity scalar remains preserved among the unchanged expected values but is explicitly non-causal and must not be consulted for derivation, validation, setup, or execution. The causal request ID instead derives mechanically from the equal fixed request/response frame IDs.
- **Driver/frame equality is mandatory:** every envelope must carry its own explicit request ID, and strict derivation validation requires that ID to equal the exact top-level ID in both fixed complete driver frames. This is derivation-time validation only; execution-time frame parsing remains forbidden.
- **W25 pair transparency is preserved:** each disabled/enabled pair must have equal fixed external request frames, equal fixed external response frames, and equal explicit driver request IDs, all equal to the enabled external execution ID. The disabled member must retain `control.enabled=false`, `control.attachment_available=false`, and `control.observation_available=false`. External and transport equality remain required; only private observation availability and its explicitly accounted private lifecycle may differ.
- **No expected consultation or runtime semantics:** execution takes the explicit request ID and opaque Params directly from `driver`, dispatches only on finite driver transport events, and cannot consult the inert retained identity, another expected value, a world/record name, or a hidden table. The amendment introduces no call-hierarchy prepare/outgoing/incoming, symbol, item, range, direction, provider, body, payload, result-subspan, token/form, consultation, or runtime-derived semantics.
- **Expected accounting is unchanged:** `request_id` remains driver-only and excluded from expected traversal and expectation accounting. The inventory remains exactly 162 rows, with 17 removed/deferred and 145 retained (`133` generic plus `12` opaque). No retained expected path is replaced by the driver field.
- **Allocator setup remains test-only:** a fresh `lspwire.NewPending(0)` is advanced solely through matched `Begin(generation)`/`Accept` operations to `request_id - 1`, installed only into an idle, non-protocol-owned runtime session while holding `Manager.mu`, and used only after releasing that lock. No production edit, placeholder rewrite, unmatched pending state, frame parsing, expected consultation, or world dispatch is authorized.

## Current-artifact boundary

The modified corpus is not evidence that the amendment has already been satisfied. Independent inspection found a disabled W25 execution whose current driver request ID follows the inert retained expected identity while both fixed frames carry the paired enabled ID. That current mismatch is precisely what the amendment corrects: the corpus must later be mechanically rederived so every disabled member uses the equal fixed frame/pair ID, with new canonical per-world and aggregate hashes independently reproduced.

The current untracked RED also lacks the amended request-ID field and allocator installation/validation logic. Therefore neither the current corpus nor the current RED is accepted by this review. No corpus, RED, implementation, activation, generated-output, or commit authority follows from this amendment-only disposition.
