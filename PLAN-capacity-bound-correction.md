# Completion bound correction (phase18c)

Sole writer; offline; retain existing three-item qualification unchanged. Managed exact-worktree projection sc_c996108b8e32f360b217ddb55a97b9be confirms sealNativeV3's actual charge. Fresh local Craft executed. No ceiling, semantic grammar, custody or denominator change.

## Exact phase18b baseline: 61,132,437 completion bytes

Reconstructed scalar inputs from recorded stage equations: prepared input bytes8391/8382/8385; initial state3675; N=P=3; manifest43924; adapter65536; semantic32768.

Derived bounds: state26203; stateIO484784; description sum246342; base262726; grown464966; seal6026896; groupSeal6334096; description reopen1527204; evidence copies504972; publication outputs1218861.

| Stage | Exact terms (bytes) | Sum |
|---|---|---:|
| Describe1 | stateIO484784 + input read66730 + completion1605632 + returned description82120 | 2239266 |
| Describe2 | stateIO484784 + input read66676 + completion1605632 + returned description82111 | 2239203 |
| Describe3 | stateIO484784 + input read66694 + completion1605632 + returned description82114 + baseBuild8338183 | 10577407 |
| GroupDecision | stateIO484784 + reopen2118192 + decode1064960 + seal6334096 + copy464967 | 10466999 |
| Reviewer | stateIO484784 + reopen3736112 + decode1593344 + seal6334096 + copy464967 + descriptions1527204 + reconcile metadata11840 + corrected copy/decode4188791 + evidence504972 + manifest43924 + report/final962700 + publication4957364 + manifest verification524288 | 25334386 |
| Result | publication artifacts9750888 + manifest524288 | 10275176 |

BaseBuild8338183 = description reopen1527204 + Build metadata16384 + Evidence504972 + seal6026896 + catalog copy262727.

## Formula/source mapping (pre-edit line locators)

- applicationinventory/durable_capacity.go:41,93: stateUpper=initial+N*2048+16384; stateIO=16*stateUpper+65536 covers durable.go state/saveState8x each +32768 each.
- durable_inventory_evidence.go:57-75: read prepared evidence6*input+16384; private_selected_consumer_v3.go:178-216: completion12*adapter+24*semantic+32768; Bytes227-231: actual evidence JSON+1. Body/proof/source/custody persist in original unit/request/selected fields; no source reread.
- durable.go descriptions/base; durable_inventory_evidence.go:80-89: description reopen6*description+16384 each; native_v3.go Build167-186 metadataP*4096+4096; Evidence219-225 charge2*description+4096 each.
- native_v3.go:188-211: seal16384 + sum[24*(body+prompt+record)+32768] + group strings12x + membership1024. Plan incorrectly substitutes entire description for body+prompt+record. Keep24; replace its operand with a sound owner-derived bound.
- native_v3_store.go Reopen:8*catalog+16384; native_v3.go Apply:16*rawDecision+16384; transitions.go applyInventoryDecision review adds8*rawDecision+4096. All64KiB reply ceilings unchanged.
- native_v3.go Bytes:catalog+1. Group strings bounded by nonempty unique membership: <=P groups, ID256/label4096/reason4096 each; decision reason4096; affected<=P prior IDs256. Sixfold escaping remains.
- transitions.go reconcile: N*192+P*1024+8192; corrected Bytes+1; decode8*corrected+4096; each Evidence2*description+4096. These are distinct reservations, not duplicate outer charges.
- transitions.go publish: manifest+2*corrected+32768; publication_v3.go:51-60:4*(16384+four artifact bytes)+16384; durable.go manifest verification8*65536; Result:8*all artifacts+8*65536. Full readback preserved.

## Correction hypotheses and proof obligations

1. Seal's operand contains only original body/prompt byte lengths and response-record bytes, not selection/accounting/envelope JSON. Derive from original owner fields before effects, retaining the same24 multiplier.
2. Bound response record using unchanged adapter ceiling, owned HOST escaped byte lengths, and closed record metadata including all four legal citation handles in every category. Test the fixed overhead against maximal grammar values. No marshal/clone solely to measure; scalar UTF-8 sizing is charged first.
3. State growth is description references and closed report transitions; census strings already exist unchanged. Derive fixed/reference overhead, never reduce N.
4. Catalog fixed overhead is closed keys/statuses/digests plus bounded actor, while all evidence is separately included. Group punctuation/member counts are bounded separately from unchanged6x string escaping.
5. Preserve admission, immutable plan/head binding, monotonic CAS transfer, optional protection and first Result. Do not sum outer slices with their inner reservations or refund unused slices.

Acceptance: original explicit three-item test passes through publication/reopen; bound/maximal-escaping tests and counterfactuals; protected-tail/concurrent/corruption regressions; proportional offline suites and Linux build. Document unqualified crash/scale cases explicitly.

## Delivered formulas and disposition

For the retained sample with the same input/state scalar lengths above:

- state = initial + N*256 +4096 =8539; stateIO=16*state+65536=202160.
- description_i = input_i + adapter +2048; sum=227910; base=sum+4096=232006.
- groupStrings=min(65536,P*8448+4352+P*256); grown=base+6*groupStrings+P*256+1024=416582.
- owner payload_i = original body length + original prompt length + recordUpper. recordUpper=adapter ceiling+2048 closed record/default metadata+512 closed HOST keys/numerics+sum escaped HOST strings. The string walk is charged before execution; invalid UTF-8 bytes cost6, not3.
- seal=24*sum(owner payload_i)+P*32768+16384=5203600; groupSeal=seal+12*min(3*65536,P*8448)+P*1024=5510800. Decoding invalid UTF-8 can triple raw string bytes; the raw cap is unchanged. This correction affects larger P, not the three-item numbers. The sample's reconstructed payload sum is212038; no whole-description substitution.
- description reopen=1416612; evidence copies=468108; baseBuild=7336711. All corresponding operations/multipliers above remain.
- report upper=state; publication outputs=base+2*grown+state=1073709. The durable four-artifact manifest upper is2048, independently checked at maximal closed fields; general PublishV3 is unchanged.
- Final stages=[1950498,1950435,9287167,9066935,22024818,8606056], sum52885909.

Completed: original three-item qualification through both verified Results; full affected library/schema suites; escaped/default/body/proof/closed-state bounds; global-sufficient/local-insufficient refusal; read/submit concurrency (also race); protected optional-read tail; repaired-checksum quote-binding corruption; retained proof/claim crash cutpoints; scoped vet, formatting/diff and Linux build.

The new long invalid-UTF-8 HOST test first failed149024>77042. Corrected scalar sizing uses6-byte JSON `\\ufffd` for an invalid byte, while a valid U+FFFD stays3-byte UTF-8; the same test now passes under race. Counterfactual old manifest reservation and old whole-description seal operand both make the unchanged three-item test fail; omitting HTML escaping makes record-bound tests fail. All mutations restored.

A further parser-boundary RED used eight groups with raw23635 bytes and decoded67584 string bytes: the group-stage component quote4119553 was below4144129. Raising only the decoded-string operand to min(3*rawCap,P*8448), while retaining12x and the unchanged64KiB raw/per-field limits, fixes the same guard. This is a pure parser/quote test, not eight-item native qualification. Final boundary race/library/three-item/vet/Linux gates passed again (`capacity18c-last-*.log`).

Detailed delivered evidence, exact accounting, commands and limitations:
`/Users/schwa/.local/state/lsp-trace-evidence/inventory-progress-ledger-0cfadd72e-01/capacity-bound-correction-20261119-18c/RESULT.md`.

Not claimed: exhaustive crash/scheduler/power-loss matrix, every maximal-body/process trajectory, arbitrary-scale/physical-memory guarantees, live model/provider acceptance, independent actors, Windows runtime or whole-application completeness. The existing proof/legacy-claim kill tests do not constitute a new forced SIGKILL at every multi-item completion-cursor boundary.
