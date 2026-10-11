# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Attachment-Enable Causal-Input Amendment Review

Status: **BLOCKED**

- Reviewed repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `4add2ac3f9cf556789869067b35170dd2c9f6276`
- Reviewed amendment: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-attachment-enable-causal-input-amendment.md`
- Review authority: independent amendment-design review only
- Corpus, RED, test, source, generated-artifact, implementation, C15 execution, writer, commit, activation, and production authority: **not issued**

## Independent checks

### Exact occurrence mapping

A fresh traversal reconstructed the accepted occurrence order directly from the 25 root envelopes, World 24's 16 workers, World 25's 22 executions, and World 25's 11 disabled/enabled pairs. The resulting 85 unique occurrence IDs equal the causal-input registry's ordered IDs exactly.

The proposed mapping is deterministic and exact:

```text
false = w25-pair/01/disabled through w25-pair/11/disabled = 11
true  = every other accepted occurrence                         = 74
total                                                           = 85
```

The registry itself distinguishes each pair member explicitly. No expected value is needed to derive this mapping. Materializing the resulting required boolean in each occurrence's `driver.request` would make the runtime input self-contained and would avoid registry, filename, occurrence-ID, or world dispatch at runtime.

### C ownership and cross-rule blocker

The amendment says all three values

```text
expected.control.enabled
expected.control.attachment_available
expected.control.observation_available
```

remain C, that strict validation checks their agreement **for every occurrence**, that no expected scalar changes, and that the accepted 17 absences and occurrence-sensitive ownership counts remain unchanged.

That contract is not satisfiable against the accepted corpus. Each of the 16 World 24 worker envelopes omits `expected.control` entirely. Those complete worker records contain no `control` member under `expected`. Therefore an all-85 rule requiring the three named control values cannot execute for World 24 without doing one of the following, each forbidden or unspecified by this amendment:

1. add the three C control paths to every W24 worker, changing accepted expected paths, absences, ownership, and bytes;
2. inherit or borrow the World 24 root control, violating self-contained occurrence validation and attributing root evidence to workers; or
3. silently weaken "for every occurrence" to "where present," leaving the W24 driver-bit cross-rule and its C ownership unspecified.

This is a structural contradiction, not a request for production evidence. The amendment must state the exact occurrence-sensitive cross-rule, preserve the accepted absent paths explicitly, and identify how W24's new causal driver bit is validated without manufacturing or borrowing C values. If it instead intends to add W24 controls, that requires a separate exact expected-value/path and ownership amendment; the current amendment expressly forbids such a change.

### W25 external equality and enabled zero-content distinction

The accepted World 25 shape has 22 complete executions and 11 complete disabled/enabled pairs. Every pair declares both `external_equal=true` and `transport_equal=true`. The amendment correctly requires equality to come from the two actual executions and limits the causal contrast to private owner/observation publication.

The registry also contains enabled occurrences with unavailable commitment material, including `w25-pair/11/enabled` with an unavailable selected commitment. Existing root records likewise demonstrate enabled controls despite unavailable selected or handoff material. The amendment therefore correctly distinguishes enabled publication eligibility from commitment availability, zero-normalized metadata, and attachment zero-content. Disabled remains an explicit causal state, not an inference from content.

### Finite comparable production seam

The proposed production seam is finite and reviewable:

- one `enabled bool` field on the comparable private attachment value;
- one exact value-returning setter with signature
  `privateCallHierarchyLayer1WithEnabled(bool) callHierarchyLayer1RequestAttachment`;
- one read at the existing `Manager.RoundTrip` generic-owner admission seam; and
- propagation only through the existing generic-owner path across the named three-file boundary.

The zero value remains disabled, the attachment remains directly comparable, and the amendment forbids alternate objects, side tables, open-ended options, callbacks, content-sensitive setters, and combined predecessor/enable signatures. A separately enumerated and reviewed C15 delta remains mandatory before production authority.

### No runtime dispatch or content policy

The amendment consistently forbids runtime use of expected/control data, registry lookup, occurrence/world/pair names, transport or result content, commitment availability or bytes, semantic inference, and content-derived validity or admission policy. The explicit driver bit is the sole proposed runtime enable operand. No world-runtime dispatch or commitment-content policy is authorized.

## Expected-unchanged plausibility

Preserving the existing expected subtree is plausible for the 25 roots, 22 W25 executions, and 22 W25 pair members only where the three control values already exist and agree with the independent mapping. It is not plausible under the amendment's universal wording for the 16 W24 workers, because their three named C paths are absent. Consequently the promised unchanged expected values, unchanged 17 absences, universal three-control cross-rule, and exact 85-occurrence validation cannot all hold simultaneously.

## Required correction

Revise the amendment before GO to provide one finite, exact rule for W24. The correction must either:

- preserve W24 control-path absence and define an explicit occurrence-sensitive C cross-rule that does not claim those absent values exist and does not borrow the root's controls; or
- separately authorize and account for the exact W24 expected-path additions, byte rematerialization, ownership changes, and replacement absence counts.

The corrected text must update every universal statement and review-gate bullet consistently. All other verified boundaries above should remain unchanged.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_ATTACHMENT_ENABLE_CAUSAL_INPUT_AMENDMENT_REVIEW_GO — NOT ISSUED
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_ATTACHMENT_ENABLE_CAUSAL_INPUT_BLOCKED
```
