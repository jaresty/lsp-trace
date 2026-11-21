# Capacity descriptor safety guard — review candidate

## Disposition

Implemented in the exact dirty worktree `/Users/schwa/dev/lsp-trace-variable-v2-TSZxdY`, as sole writer. **Not independently accepted; parent review remains required.** This repairs maintenance-invalid descriptor forms, not a demonstrated current public-input exploit. No capacity/accounting, admission model, scale model, DTO, V1/V2 or MCP contract change. No network/install/provider/live acquisition/subagents/staging/commit/push. No repository schema regeneration.

A fresh child-local Craft build and `PLAN-capacity-descriptor-guard.md` preceded source edits. Complete token/composition output and local derivation are retained in the executing transcript. No exact READY managed LSP session was established in this thread; bounded source fallback was used. The complete shape, diagnostic, generator, corpus/parity tests and prior canonical-shape report were read. This report does not independently adopt the previous worker's acceptance claims.

## Implementation

`shape.go` now has a small local descriptor grammar check plus the shared `checkedCapacityShape` accessor. Both root admission and exported publication use that accessor before interpretation; every branch is checked, even when unused by the current input. The scalar rule entry points also validate before checking input or generating a map.

Admitted grammar:
- Nonempty homogeneous unique scalar values of exact Go type string, bool or uint64. Empty kind is legal only in this values form. String values must be valid UTF-8.
- Integer kind with inclusive uint64 minimum/maximum, minimum <= maximum, optional nonzero multiple (zero means no multiple restriction).
- Nonempty closed object kind with unique, nonempty valid-UTF-8 names and recursively valid rules; requiredness is the existing optional boolean.

Values/kind/range/object combinations outside those disjoint forms are rejected, not given new interpretation. Unsupported scalar types (including nil, int, float, slices, maps and pointers), empty/unknown kinds, duplicate values, empty/nil objects and malformed nested rules are rejected. Active field-address tracking detects cycles through slice backing arrays and permits shared acyclic subtrees.

Reason branches must be nonempty, unique, named members of the required root reason values, with exact branch coverage. Root metrics must be optional object form. Nil branch metrics means forbidden, as before; non-nil metrics must be nonempty valid field definitions refining existing required scalar fields, not introducing names, optionality or nested object semantics. Constants must be admitted by the parent scalar rule; integer ranges and multiples must refine the parent range/multiple.

`CapacityDiagnosticSchema` now returns `([]byte, error)` and returns nil bytes on invalid descriptors. The generator handles the error before `os.WriteFile`; no successful empty schema fallback or swallowed generation error. The byte-pin test handles the new error return. `diagnostic.go` is byte-identical: semantic authority, DTO, size gate, duplicate raw-key gate and public decode error behavior remain untouched.

Descriptors remain package-private static source data, not a concurrent mutation API. Tests that alter them are serial and restore state. This is not a general JSON Schema engine or a proof that arbitrary future valid descriptor edits preserve the V3 contract; the publication pin and domain corpus continue to guard that separate concern.

## Attributable RED/GREEN evidence

All Go executions used `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOTELEMETRY=off`.

1. **Before production edits:** new independently authored `TestDescriptorCounterexamples` and `TestDescriptorMalformedBranches` ran against the existing implementation and failed. `/tmp/capacity-descriptor-guard-red.jsonl` records the three schema publication failures, enum/range input admission, and malformed unused-branch acceptance/publication. The temporary test adapters supported the old no-error API so this was runnable assertion RED, not a compilation failure. Those adapters were removed after error plumbing.
2. `/tmp/capacity-descriptor-guard-green.jsonl`: first focused GREEN, including existing publication byte pin.
3. Additional independent-schema-engine characterization against the **saved original source**, via an external overlay, is retained in `/tmp/capacity-descriptor-guard-original-probe.log`: unsupported string input `"x"` produced checker=false/schema=true; empty rule with true produced false/true; integer kind plus uint64(1) value and maximum 0 produced true/false. This supplemental compiler characterization was run after implementation using the immutable baseline overlay; it is not represented as the earlier chronological RED run.
4. Expanded valid-form tests initially exposed a test-helper mistake: passing Go `[]string` directly to the independent schema compiler instead of parsed publication JSON yielded `invalid jsonType []string`. `/tmp/capacity-descriptor-guard-expanded.jsonl` preserves that failed harness run. The helper now marshals/parses its generated schema before independent compilation. This was not a production descriptor failure.
5. `/tmp/capacity-descriptor-guard-packages.jsonl`: full affected package set passes: capacitydiagnostic, capacitydiagnostic/genschema (compiles; no test files), censusresult, schema. **2,354 passing test records, zero failures**, counting named parents/subtests, not independent scenarios. Includes the unchanged **2,120-case** mutation corpus (aliases, numeric spellings/bounds, duplicates, semantic distinctions) and publication pins.
6. `/tmp/capacity-descriptor-guard-race.jsonl`: full capacitydiagnostic/censusresult/schema race run passes, **2,354 passing test records, zero failures**; corpus reports 2,120 cases.
7. Counterfactual common-gate reductions are external overlays only. Removing the accessor only from decode yields `malformed descriptor admitted at decode entrypoint` in `/tmp/capacity-descriptor-guard-reduction-decode.log`. Removing it only from publication yields `malformed descriptor published at exported entrypoint: bytes=4059 error=<nil>` in `/tmp/capacity-descriptor-guard-reduction-publish.log`. Both run `TestDescriptorMalformedBranches/empty_reason`, fail independently, and leave repository source unchanged.
8. Actual generator binary ran twice in `/tmp/capacity-descriptor-guard-generated/capacitydiagnostic`, producing byte-identical baseline schema both times. An invalid-root-descriptor overlay generator exits 1 before writing; `/tmp/capacity-descriptor-guard-generator-rejection.log` records the error, and `cmp` confirmed the existing temporary valid publication was unchanged.
9. `go vet ./internal/capacitydiagnostic ./internal/capacitydiagnostic/genschema ./internal/censusresult ./internal/schema`, owned-file gofmt, and `git diff --check` pass.

The tests cover malformed nested rules, unused invalid branches, duplicate/unknown names, invalid requiredness/refinements, cycles, and shared acyclic valid objects. They exercise the actual decode and exported publication entry points rather than only the grammar helper.

## Byte preservation and exact write set

Generated and committed V3 schema SHA-256 remains:
`eb4ae29fe1b5970c48c141fe3dfffb491d201f5e537632778350ccf29dcda806`.

Exactly six repository files were written by this phase:
1. `PLAN-capacity-descriptor-guard.md` — new plan.
2. `docs/capacity-descriptor-guard-report.md` — this new report; prior report preserved.
3. `internal/capacitydiagnostic/shape.go` — local grammar/common gate and publication errors.
4. `internal/capacitydiagnostic/shape_test.go` — new focused descriptor and independent-engine tests.
5. `internal/capacitydiagnostic/genschema/main.go` — stop on error before writing.
6. `internal/censusresult/capacity_shape_test.go` — only adapt publication call to error-returning API; corpus unchanged.

Narrow pre-edit originals and hashes: `/tmp/capacity-descriptor-guard-baseline/`, including `hashes.json`. Final existing-file hashes: `/tmp/capacity-descriptor-guard-final-hashes.json`. Baseline-relative source diff: `/tmp/capacity-descriptor-guard-source.diff`. This is a narrow baseline, **not** a whole-tree byte inventory or authentication claim.

| Existing file | Before SHA-256 | After SHA-256 |
|---|---|---|
| shape.go | 3ed6a26f6f96a71d053b517145a5212b950627e497bf8a3e9a980e3f819dc141 | a10b96852bd6e68436dd2d78f070cf396bb0bcf3cbce6af8b724a7bb66762913 |
| genschema/main.go | 793f9b063e5944cbc5788061347ab94b134b6d4347130ed5d62dc8335791b259 | ce2a7554a231b3f04431288e85be365068e6e9039ef7a17302a7f0df9480fa8e |
| capacity_shape_test.go | eca8258b6ba08ce50dfd43a787f2f55dd4b3778a4eb5dd820f0acf83492e4630 | fa14212cd512432016014517ca67127e733fbc0c937e3dbfe726a32eff928507 |

Unchanged sampled boundary hashes:
- diagnostic.go: `7e6eb75fbd118c9b851a5502d5e39a207c6c54ebc6ad064665525850fea0c087`
- prior canonical-shape report: `3107754935913a6c426028bdb09b74682d8308f4b0935da3ea036310654d9bc4`

## Limits / next authority

No whole-repository or controlled scale matrix rerun; no changed path required capacity/scale requalification. No claim of universal descriptor/input-domain equivalence, exhaustive per-assertion perturbation coverage, arbitrary-depth resource containment, or concurrent descriptor mutation support. Independent parent inspection of the actual source, baseline diff, retained transcript and logs remains the acceptance gate. Existing standalone schema lexical/duplicate-key/arithmetic limits are unchanged.
