# Private V3 assessment encoded-size repair

Delivered for parent independent retest and narrow review; V3 remains unwired. HEAD is still `3caa23f1e04a944867de1418a82e0e0a31ff6768`; no commit/staging/push.

## Repair

`retainedpartialprogramc/assessment_serialization_private_v3.go` owns one private bounded serializer. Both `ComputeInstabilityPrivateV3` and `ValidateInstabilityPrivateV3` call it instead of directly marshaling the report. It admits the complete concrete Source/Math field graph before `json.Marshal(report)`: every string value, the four selected-object digests, seeds, runs, partitions, communities/member strings, pairs and metrics. The cap remains **16 MiB**, with subtraction/division before potentially overflowing arithmetic. Six bytes per string byte plus quotes covers HTML/control/quote/backslash escaping, UTF-8 (including replacement of invalid bytes), and U+2028/U+2029. Fixed object budgets cover actual keys, scalar widths and punctuation; lists include nil/empty/container/element costs. Tests independently check the concrete schema and fixed budgets so new fields/custom marshalers cannot silently evade the guard.

The non-recursive count pass checks context throughout and reserves minimum list work before entering each list, using one monotone **1,000,000-unit** local work bound. It uses the existing original-ledger stage reservation, not a new/reset exchange or ledger. After admission, production still uses exactly `json.Marshal`, propagates its error, checks the actual byte cap defensively and checks cancellation. A private callback can only observe/refuse before marshal; it cannot provide encoded bytes or an owner. Production supplies `nil`.

Original opaque admission/ownership, profiles, mathematical computation/metrics, V2, unit caps, source evidence, AST/preparation, runtime/configuration and the original-worker fixture are unchanged. Existing math hashing/cloning before report serialization is unchanged: this is **not a whole-computation heap bound**.

## Executed proof

Fresh private evidence (0700 directories/0600 files):

`/Users/schwa/.local/state/lsp-trace-evidence/producer-pages-v3-med-rlvhfsen/repair-8wg5mfc0/`

- `red-marshal-first.log`: two compiling assertion REDs, oversized detached report and already-canceled context; each reached its observe/refuse callback once before admission (`calls=1`). This was a marshal-first serializer stub, not an oversized original-owned public-path witness or a measured-allocation experiment.
- `unit-normal-final.jsonl`: **171** passing test/subtest events, **11** named top-level tests, zero failures.
- `unit-race-final.jsonl`: **171** passing events, the same 11 named tests, zero failures.
- `package-normal-final.jsonl`: **201** passing retained-partial package events, zero failures.
- Coverage includes six repeated oversized identity strings in six partitions (one member per partition, 15 pair rows), no marshal-boundary callback on refusal; **160** exported string leaves independently forced past the conservative admission bound; exact normal JSON; escaping; nil/empty lists; maximum uint64/int/float widths; conservative cap/next-byte and overflow boundaries; work admission; cancellation before/during the walk; observer refusal; JSON errors; unchanged neutral two-node mathematical assessment. Detached reports do not mint a Projection/assessment seal or source authority.
- `owned-pipeline-admission-normal.jsonl`: **13** passing events; all **10** required top-level pipeline/admission/selected tests executed. The unchanged original-owned controlled pipeline prepared all **20** exact own bodies and published their bound references. A source-AST dependency guard separately confirms both actual production functions call the serializer and contain no direct `json.Marshal`.
- Vet passed for retainedpartialprogramc and censuslineagepilot. Gofmt and `git diff --check` passed. Go test/vet children used umask 022.
- `preservation.json` and `private-v3.delta.patch`: all 15 other baseline files unchanged, including both required protected hashes, both old V3 handoffs and the preexisting original-worker fixture delta. `private_v3.go` differs only at the two serializer call sites and the relocated size check. New files are the private serializer, its tests and this handoff.

## Explicit limitations

**OVERSIZED PUBLIC OWNER PATH NOT DIRECTLY EXERCISED.** Oversize refusal is detached-serializer proof; original-owned integration is normal-path proof. The two primary refusal assertions have RED/GREEN observations; auxiliary tests are GREEN-only, not exhaustive mutant/adequacy evidence. No measured heap/CPU or whole-computation allocation proof is claimed.

The upper bound is deliberately conservative and may refuse JSON that would actually fit below 16 MiB. That underutilization is an explicit limit, not a scalability claim. The unchanged controlled test grants 1 TiB logical/read-hash allowance; the observed cumulative logical reservation was **266,744,933,604 bytes** and read/hash **274,724,407,638 bytes**, not practical scalability evidence or measured memory.

No live provider/gopls/model, delegation, backend/SDK/configuration, source/claim mutation or runtime activation was added. Cross-process restoration, ordinary admission, historical authentication and production authority remain outside this repair. No broad 557-second lineage suite or full existing V3 race family was rerun; the focused gates above are the executed evidence for this narrow delta.
