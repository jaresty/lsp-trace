# ADR0007 Response V2 prompt evaluation

Authority: `0`  
Accepted: `false`  
Completeness: `UNKNOWN`  
Production switch: `false`

Exactly 12 process invocations were attempted (D/E/F × four fixed packets), with zero retries. All terminated before generation with the same eval-harness `GRAMMAR_INIT_ERROR`; therefore no prompt semantics were evaluated. Raw outputs are session-local outside Git at mode `0600`; only digests and lengths are retained here. Human-safe semantic adjudication remains limited to obvious packet mismatch or invention, but was not entered. Host admissible evidence handles grounding; optional citation suggestions remain a non-scoring bonus.

## Harness blocker

The V2 GBNF used `maybe_string`, which the pinned llama.cpp grammar parser rejected at initialization. After the immutable 12-call campaign, the eval-only rule was renamed to `maybe-string` and a focused regression was added; `go test ./...` reports 10 passing tests and `go vet ./...` passes. The campaign was not replayed because its predeclared contract allowed zero retries and exactly 12 invocations.

## Variant totals

| Rank | Variant | Strict/canonical | Substantive | No invention | Unresolved | Consistent | Citation bonus | Latency ms | Tokens |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | D | 0/4 | 0/4 | 0/4 | 0/4 | 0/4 | 0/4 | 0.000 | 0 |
| 2 | E | 0/4 | 0/4 | 0/4 | 0/4 | 0/4 | 0/4 | 0.000 | 0 |
| 3 | F | 0/4 | 0/4 | 0/4 | 0/4 | 0/4 | 0/4 | 0.000 | 0 |

## Winner

No winner meets the predeclared gate. No prompt change is justified because generation never began. Smallest next change: validate the repaired eval-only V2 grammar against the pinned runtime before any separately authorized campaign.

## Sanitized run ledger

| Run | Variant | Packet | Exit | stdout digest / bytes | stderr digest / bytes | Strict | Substantive | No invention | Unresolved | Consistent | Citation | Latency ms | Tokens |
|---:|---|---|---:|---|---|---|---|---|---|---|---|---:|---:|
| 1 | D | real | 7 | `de791792d8ec813048ff27fe1a90b538d3c2c8c307f16d9b5b62c85bb7d0ad9f` / 181 | `2be25b782393f2cd6f1b5774f8e19d50130ef908fe421b8d7c6a60647efd8923` / 2679 | false | false | false | false | false | false | 0.000 | 0 |
| 2 | D | final-01 | 7 | `688f1a8931ca03c0e65c08ee13b990d5677fb5c98357ffc9eaed37f0363ea758` / 182 | `2be25b782393f2cd6f1b5774f8e19d50130ef908fe421b8d7c6a60647efd8923` / 2679 | false | false | false | false | false | false | 0.000 | 0 |
| 3 | D | final-02 | 7 | `03ffba33d0bfd4ac00ef759d2ad402fd42e2b252f3f7048041cc772ca43c8627` / 182 | `2be25b782393f2cd6f1b5774f8e19d50130ef908fe421b8d7c6a60647efd8923` / 2679 | false | false | false | false | false | false | 0.000 | 0 |
| 4 | D | final-03 | 7 | `95facfc392dd2c025df6ac33bc82b27718e4605ead33818773c995b092c429ba` / 182 | `2be25b782393f2cd6f1b5774f8e19d50130ef908fe421b8d7c6a60647efd8923` / 2679 | false | false | false | false | false | false | 0.000 | 0 |
| 5 | E | real | 7 | `5731ce2f9c67e389251529ab4f39a6c42f8e5c056d579e50ed65cb7a0e21bf46` / 182 | `2be25b782393f2cd6f1b5774f8e19d50130ef908fe421b8d7c6a60647efd8923` / 2679 | false | false | false | false | false | false | 0.000 | 0 |
| 6 | E | final-01 | 7 | `c7e6158db1828ecfc6d167a71df036468d450dcb8ff4e96e032bad9e67bfa4a6` / 182 | `2be25b782393f2cd6f1b5774f8e19d50130ef908fe421b8d7c6a60647efd8923` / 2679 | false | false | false | false | false | false | 0.000 | 0 |
| 7 | E | final-02 | 7 | `5f3912015eaed31c075ac4918afe536ade0d02905bb566dfed7e7c71e142d9d1` / 182 | `2be25b782393f2cd6f1b5774f8e19d50130ef908fe421b8d7c6a60647efd8923` / 2679 | false | false | false | false | false | false | 0.000 | 0 |
| 8 | E | final-03 | 7 | `858d0f1405d8aa1ea5210e3a653a9427872d4f23f1d8f70e58346568a2042a39` / 182 | `2be25b782393f2cd6f1b5774f8e19d50130ef908fe421b8d7c6a60647efd8923` / 2679 | false | false | false | false | false | false | 0.000 | 0 |
| 9 | F | real | 7 | `157ad02f837c2a2b951f54ff9a6fb9454d864686f1dc05561dafe8e0d6453022` / 182 | `2be25b782393f2cd6f1b5774f8e19d50130ef908fe421b8d7c6a60647efd8923` / 2679 | false | false | false | false | false | false | 0.000 | 0 |
| 10 | F | final-01 | 7 | `194d865d733b02f7d80d9dfda48bf3e0cadc76a8b4723519bf11e9507bc51736` / 182 | `2be25b782393f2cd6f1b5774f8e19d50130ef908fe421b8d7c6a60647efd8923` / 2679 | false | false | false | false | false | false | 0.000 | 0 |
| 11 | F | final-02 | 7 | `055466de049e20de1e93a439d91213a74cdb63f9c7244c3465fe224d3f35a4a3` / 182 | `2be25b782393f2cd6f1b5774f8e19d50130ef908fe421b8d7c6a60647efd8923` / 2679 | false | false | false | false | false | false | 0.000 | 0 |
| 12 | F | final-03 | 7 | `3aa086b74828c73eab42c57a093605c79a13804732ca3e3d5987a98377fc8459` / 182 | `2be25b782393f2cd6f1b5774f8e19d50130ef908fe421b8d7c6a60647efd8923` / 2679 | false | false | false | false | false | false | 0.000 | 0 |
