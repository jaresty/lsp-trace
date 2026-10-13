# V5 — four independent causal witnesses

Binding: **ADR0011_V5_REFRAME**. Verdict ceiling: **AUTHORED_NOT_GO**, never GO.

## Preserved implementation

The complete connected v5 production implementation is retained: production executor SessionClient dispatch, neutral acquisition adapter, private Manager ingress, shared transaction, reserved original request/response transport, lexical aliases, provider ordinals through traverse/slicer, immutable noncopy views, actual SHA256 observations, separate 32 MiB raw/general partitions with combined 64 MiB authority, and existing generic/V2/C15 terminal convergence. Exactly six carrier types remain. Ordinary `Runtime.RoundTrip` is byte-for-byte unchanged and no CLI/MCP/schema activation is added.

Only the required plus-one seam was corrected: allocation refusal now carries a **proposed, not acquired** frame commitment and returns `ResourceExhausted` without poisoning READY or performing request/response transport I/O. The measured pre-effect boundary is directional retained raw-frame allocation and transport effects; the inherited general ownership-table bookkeeping is separately recorded, not falsely reported as zero process allocation.

## The only frozen witnesses

1. **EXACT32:** actual production executor SessionClient prepare acquisition; request + original response retained capacity exactly 32 MiB; physical provider ordinal **1**; exact frame/element commitments; release.
2. **PLUS_ONE:** actual ordinal-1 source acquisition/release, followed by a prospective incoming request frame exactly 32 MiB+1; refused before retained raw allocation or WRITE. An independent probe on Manager stdin records Write calls and offered raw bytes **before any provider frame decoding**, plus actual emitted bytes: all three deltas must be exactly zero. Decoded packets are only supplementary evidence. After refusal/release, a valid small incoming request must succeed on the **same READY Manager, client, session ID and generation**, with positive raw transport bytes matching the decoded request. Reservation/refusal, zero-wire, reuse and release receipts are retained.
3. **PARTIAL_EXTERNAL:** actual incoming transaction consumes a notification, unmatched response, and truncated response; exact acquired prefix commitments; failure; release.
4. **EARLY_TERMINAL:** actual incoming null response takes the production client's early return; generic terminal wins once, invalidates the source view, and releases every created object.

Each witness creates a real Manager, completes its actual initialize/readiness protocol over the fixture's pipes, enters production `incomingops.SessionClient.call` with that Manager (not a replacement Runtime), and obtains ownership from Manager. The protocol provider is deliberately controlled test transport, not production-server/authentication/semantic evidence. Fixture-side bytes form an independent digest/length oracle. No manually constructed generic owner is used.

The bounded PLUS_ONE correction changes only its test instrumentation/assertions and qualification metadata. The 13 connected production files and the other three witness bodies remain unchanged. The added `refusal-incomplete-prefix-write` mutation writes exactly one `C` byte after reservation refusal, without completing a frame; it must compile and fail `ASSERT_C4_PLUS_ONE_ZERO_WIRE`, then be restored before all four independent witnesses pass again. This closes the decoded-packet-only observation gap without expanding the witness count or deferred scope.

The direct frame-release loop remains preserved, but is not claimed as independently critical: its removal left the early-terminal witness passing because the inherited account terminal also freezes/releases remaining C15 objects. The matrix therefore gates the actual generic/account terminal rather than a redundant release route.

The four witnesses execute as four **separate test-binary processes**, with distinct recorded PIDs and independent sessions; the partial-failure session cannot poison another witness. No aggregate 30-test package is frozen. Broader lifecycle, race, sorting, arithmetic-helper, and C15 scenario suites are deferred; existing production behavior is not discarded.

## Review and reproduction

- `union.patch`: 13 preserved production paths and one four-witness test file.
- `PATH_MANIFEST.json`: every actual path, authorized family, base hash, final hash.
- `WITNESS_CONTRACT.json`: exactly four identities and their measurable contracts.
- `CRITICAL_EDGE_MUTATIONS.json`: only critical edges exercised by those four witnesses.
- `SELF_HASHES.json` / `.sha256`: package integrity envelope, not authenticity.

Run `python3 verify.py --source /private/tmp/adr0011-incoming-calls-v1-6yiY4m`.

The verifier clones exact clean base `04a4ca85ffbfd3916010897fb6fcc193ba47252d`, fresh-applies the whole connected patch, checks all hashes, and runs only the four witnesses independently. Each listed edge mutation must compile and fail its targeted causal assertion; compile errors are not witnesses. It restores the patch and independently reruns the same four witnesses. Receipts, logs and hashed `verification/RESULT.json` retain actual provenance. No semantic campaign RED/GREEN, broad regression/race/lifecycle campaign, delegation, commit, or campaign-source mutation occurs. A verdict is emitted only after complete verifier exit 0.
