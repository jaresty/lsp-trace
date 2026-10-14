# ADR0011 -> ADR0007 private causal composition correction qualification v1

Producer disposition: `ADR0011_ADR0007_PRIVATE_CAUSAL_COMPOSITION_AUTHORED_NOT_GO`.

This packet documents the same private/default-off wrapper candidate after the three-blocker rereview correction. ADR0007 private source-text-search remains unchanged at input HEAD; implementation is confined to `internal/adr0011adr0007composition/**` and this qualification package.

## Decision

AUTHORED_NOT_GO. The code is authored for review and qualification only. It is not public API, not CLI/MCP/schema integrated, not Stage8/main integrated, and not authorized for production ADR0011->ADR0007 composition.

## Worktree and base

- Worktree: `/private/tmp/adr0011-incoming-calls-v1-6yiY4m`
- Base HEAD: `336ec84b4688d456813861abcb45ddffaacdd6c4`
- Commit made: no
- Delegation: no

## Corrected blockers

1. **Selected ordinals**: committed artifacts expose `one_final_disposition_ordinals=5900`, directional profile/result identity, and no accepted selected item/ordinal set. The wrapper therefore fails closed on caller-selected ordinals and hard-pins an internal ordinal-identity contract derived from committed artifact fields.
2. **MaxReferences**: `MaxReferences` is enforced with overflow-safe accounting before admitting each envelope/mark reference effect. Tests cover exact boundary 4 pass / 3 fail for two envelopes plus two marks, deterministic accounting, and cancellation/resource precedence.
3. **Frozen write-set filenames**: removed `composition.go` and `composition_test.go`; final implementation filenames are `compose.go` and `compose_test.go`.

## Canonical ordinal source

Source artifacts inspected:

- `docs/qualification/adr0011-private-bidirectional-composition-qualification-v1/COMPOSITION_ARTIFACT.json`
- `docs/qualification/adr0011-private-bidirectional-composition-qualification-v1/INPUT_PINS.json`
- `docs/qualification/adr0011-calls-go-gopls-incoming-profile-qualification-v1/{MANIFEST.json,RESULT.json}`
- `docs/qualification/adr0011-calls-go-gopls-outgoing-profile-qualification-v1/{MANIFEST.json,RESULT.json}`

Observed accepted available identity:

- incoming: `one_final_disposition_ordinals=5900`, result `e9c8cce4b77dc91a8ed435166fbf88ce9d6e12bbc28bd4ad0db4fbbb45cc4154`, no selected ordinal/item list exposed.
- outgoing: `one_final_disposition_ordinals=5900`, result `e6a60c88f4bd00d2e220d160765c8ee903903523cb86cfb7d814e31a7c433c9f`, no selected ordinal/item list exposed.

Wrapper enforcement:

- `OrdinalSemanticsNoSelectedItems = ONE_FINAL_DISPOSITION_PER_TARGET_ORDINAL_NO_SELECTED_ITEMS_EXPOSED`
- `CanonicalOneFinalDispositionOrdinals = 5900`
- incoming ordinal identity hash `273d62e8e91d0b332ada4e1d18e68ef4404f549fed14923879a3e15a8928677d`
- outgoing ordinal identity hash `7843f1430e5ddb401c8f8ce76eb3a1b3188769ff9f625ee6e6a10b9b7d1544e4`
- any nonempty `SelectedOrdinals`, nonzero mark `SelectedOrdinal`, or identity/count drift fails before candidate effect.

## Verification logs

- `logs/focused-wrapper.log`: focused wrapper tests passed.
- `logs/package-wrapper-and-adr0007.log`: wrapper package and unchanged ADR0007 private package passed.
- `logs/race-wrapper.log`: wrapper race test passed.
- `logs/vet-wrapper-and-adr0007.log`: vet produced empty output and exit 0.
- `logs/diff-check.log`: diff check produced empty output and exit 0.
- `logs/mutation-gates.log`: semantic-removal mutations produced expected failures and restored baseline passed.

## Hashes

See `checksums.sha256` for implementation, test, script, log, and documentation checksums.
