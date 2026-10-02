# ADR 0011 private limit checkpoint — bounded synthetic disposition

**ACCEPT_FOR_PRIVATE_SYNTHETIC_FALSIFICATION_ONLY.** This does not qualify references, authorize production/public issuance, or execute any of the 162 matrix rows. The independent disposition was an isolated judgment of the evidence packet, **not an independent inspection of code or test logs**. The source-byte review below was a separate read-only enumeration.

## Exact source selection

- New no-replace manifest: `adr0011-synthetic-source-pin.f8cf96e649a5fc05.manifest.json`; raw SHA-256 `f8cf96e649a5fc051fa6475f8b68fe38bcc346e52fd459bb946d99951f57269a`.
- Versioned length-prefixed aggregate: `sha256:b06eca5f8b40b96eee624e932e39c215144a7e2c14162e05cd5896eb83bf475c`.
- Independent read-only verification recomputed every listed file length and SHA-256, the exact sorted 90-file non-test Go set in eleven directories, 606162 total bytes, both digests, and absence of omitted/extra source files or symlinks. The prior reviewed manifest was not replaced. The test-only selector in `internal/adr0011acquisition/synthetic_source_pin_review_test.go` binds this manifest and digest.
- This identifies checkout source bytes only, not a commit, executable, runtime, dependency closure, producer, or live analyzed version. Tests are outside this pin. Any subsequent production-source byte change invalidates this selection.

## Bounded witness and negative controls

- Offline focused `go test ./internal/adr0011acquisition -run '^(TestReviewedSyntheticSourcePin|TestADR0011OwnerPrivateFinalAtLimitFrame|TestADR0011OwnerPrivateFinalTwoEqualLocations|TestPrivateFinalSynthetic)$' -count=1 -v`: **156 passed**. Test-only managed peer emits an exact 2,097,152-byte references response frame with padding outside the small result token. The Owner test independently rereads the retained response body and reconstructs the canonical frame byte count. It rereads proposal, candidate and final files, checking final `T=1,A=P=2` and distinct occurrence ordinals. The private Owner path requires verified proposal/candidate/final publication and complete replay followed by fresh final readback before returning its receipt.
- Offline `sessionruntime` Manager framed-write, marked/unmarked, aggregate/message tests: **13 passed**. A 4,194,305-byte keyed request frame fails before write with typed `ResourceExhausted` and no pending-key leak; its oversized params are **Owner-unreachable**. Aggregate overcap and fifth-message cases also return typed failures. The exactly 4,194,304-byte pre-write test is a Manager seam only, not Owner issuance.
- Scoped `go vet ./internal/adr0011acquisition ./sessionruntime` and `git diff --check` passed.
- The historical public-success tests `TestADR0011OwnerManagedTwoEqualLocations`, `TestADR0011ChainTwoEqualManagedLocations`, and `TestADR0011ManagedMalformedDiagnostic` each remain **RED**, without changed assertions. `PublishReferences` remains fail-closed.

## Boundary

The isolated reviewer returned `ACCEPT_FOR_PRIVATE_SYNTHETIC_FALSIFICATION_ONLY` on the supplied packet and explicitly did not inspect repository or external evidence. This accepts the bounded private synthetic checkpoint, **not** the full limit matrix, live raw retention, real Go/gopls evidence, source-to-binary authenticity, occurrence admission, or public issuance. Production/public `T=A=0`; accepted references matrix executions remain **0/162**; authority remains 0 and completeness UNKNOWN. The dirty worktree precludes a clean-Git qualification in this checkout. A separate independently inspected implementation review and references occurrence decision remain necessary.
