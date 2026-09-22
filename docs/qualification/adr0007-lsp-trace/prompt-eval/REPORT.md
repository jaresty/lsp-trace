# ADR0007 model-backed prompt evaluation

Authority: `0`  
Accepted: `false`  
Completeness: `UNKNOWN`

This evaluation selects prompt wording only. It changes no production behavior and semantically accepts no feature identity.

## Controls and scoring

The fixed matrix, pins, no-retry rule, and lexicographic scoring order were predeclared in `PLAN.md` before execution. Valid pretty-printed JSON is accepted by strict semantic parsing and then deterministically re-encoded by the host; model byte-canonicality is not scored. Raw outputs remain session-local outside Git at mode `0600`; this report retains only digests, lengths, and sanitized scores.

The eval worker had a post-EOG harness defect: it emitted a valid `COMPLETE` envelope but did not return, so runs 2–12 appended repeated envelopes until the token cap. No model retry was performed. Scoring deterministically recovers the first EOG-complete envelope from each immutable raw stream; run 1 contained no complete envelope and remains invalid. The worker now returns immediately after EOG for future use.

All four C outputs copied the supplied example prose exactly (identical payload digest). The predeclared citation/coverage and substantive-field criteria therefore score those outputs false: valid citation IDs attached to generic copied claims do not establish packet-specific claim coverage. Criterion order was not changed after observation.

## Variant totals

| Rank | Variant | Strict parse | Citation/coverage | No invented identity | Substantive | Canonicalizable | Latency ms | Tokens |
|---:|---|---:|---:|---:|---:|---:|---:|---:|
| 1 | C | 4/4 | 0/4 | 4/4 | 0/4 | 4/4 | 15470.094 | 536 |
| 2 | B | 4/4 | 0/4 | 3/4 | 3/4 | 4/4 | 15420.547 | 587 |
| 3 | A | 3/4 | 0/4 | 3/4 | 3/4 | 3/4 | 12665.249 | 491 |

## Winner

**Variant C** wins by the predeclared lexicographic ordering.

## Sanitized run ledger

| Run | Variant | Packet | Exit | stdout digest / bytes | stderr digest / bytes | Parse | Citations | No invention | Substance | Canonical | Latency ms | Tokens |
|---:|---|---|---:|---|---|---|---|---|---|---|---:|---:|
| 1 | A | real | 10 | `edc1617c422d8f17d9867f65f6cf95a0c584beed33f683bf7345deddcbcf9931` / 1207 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | false | false | false | false | false | 0.000 | 0 |
| 2 | A | final-01 | 10 | `ce343176ff131560d356849c73d830c91018821bae7762b3f50c6829f862fce7` / 569421 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | true | true | 3198.226 | 143 |
| 3 | A | final-02 | 10 | `2e9014a9495a9e43f8d667c7e09044713340e549080cfd7697769351494adc01` / 520254 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | true | true | 4388.509 | 165 |
| 4 | A | final-03 | 10 | `fdbcc6756e0fe170ab8c38a1b51390bbf3d660a1951fff15d72fea4118f04d68` / 475796 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | true | true | 5078.514 | 183 |
| 5 | B | real | 10 | `a11ca11099586ceb3c5c8cf19b1532e3f6270d6b0587432c803e6cbd219c76b4` / 581892 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | false | false | true | 2630.321 | 122 |
| 6 | B | final-01 | 10 | `5b615362d30c6da1866ec2c2064c8e67e18c2e18cbeafe857c1d98780b70fa77` / 528696 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | true | true | 3664.187 | 164 |
| 7 | B | final-02 | 10 | `12997af6940886bb4ee2f3af3f00977e10fda98ec48bf34884d15ae06b7a0edc` / 453772 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | true | true | 4717.023 | 170 |
| 8 | B | final-03 | 10 | `c7124dd3b1e3c5628e2145b166863228a856e0c5dac6fa374281634fddaedf58` / 612218 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | true | true | 4409.016 | 131 |
| 9 | C | real | 10 | `75a8b9f1adbfa229f836011bb4876bb3e8ead617c25c2926dcceff7de7038ceb` / 553698 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | false | true | 3109.356 | 134 |
| 10 | C | final-01 | 10 | `b888c698a5149046b0f35c2f7f5650af2b5dba362cb24fe89869acddb622d6f8` / 580490 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | false | true | 3312.766 | 134 |
| 11 | C | final-02 | 10 | `c92e8183c01d744462d6c2dc4d39546c2967e967c600d45d56cc7e8c793ac05c` / 586080 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | false | true | 4296.754 | 134 |
| 12 | C | final-03 | 10 | `6bc33b8692aa6485d2f48e45dd3728e57953cb8a34d6e4a0661b70f665be103f` / 587521 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | false | true | 4751.218 | 134 |

## Proposed additive seam (not executed)

After explicit approval, add a new response schema/version containing the minimal model-owned semantic payload. The host should: (1) reject duplicate keys, unknown fields, trailing bytes, non-JSON, invalid citation IDs, and schema violations; (2) deterministically re-encode the accepted payload; (3) enrich it with request IDs, mechanically selected consumer identity/selector and nearest outward consumer, pins, authority, acceptance, completeness, provenance, status, and accounting; and (4) construct the production response record from the enriched object. Keep the existing response version unchanged for compatibility. No model or production change is performed by this proposal.
