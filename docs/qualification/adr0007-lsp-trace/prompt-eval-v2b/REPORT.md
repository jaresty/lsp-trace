# ADR0007 Response V2b prompt evaluation

Campaign: `v2b`  
Authority: `0`  
Accepted: `false`  
Completeness: `UNKNOWN`  
Production switch: `false`

Exactly 12 deterministic grammar-constrained Qwen campaign invocations (D/E/F × four fixed packets), zero retries. Before packet loading, the final v2b binary passed a separate network-denied pinned-runtime grammar initialization and 67-token synthetic generation; its mode-`0600` receipt is retained outside Git. Raw outputs are session-local outside Git at mode `0600`; only digests and lengths are retained here. Human-safe semantic adjudication is limited to obvious packet mismatch or invention. Host admissible evidence handles grounding; optional citation suggestions are a non-scoring bonus. The prior harness-blocked campaign remains separately recorded in `../prompt-eval-v2/`.

## Variant totals

| Rank | Variant | Strict/canonical | Substantive | No invention | Unresolved | Consistent | Citation bonus | Latency ms | Tokens |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | E | 4/4 | 0/4 | 4/4 | 4/4 | 4/4 | 0/4 | 11853.753 | 358 |
| 2 | F | 4/4 | 0/4 | 4/4 | 4/4 | 4/4 | 0/4 | 17231.530 | 548 |
| 3 | D | 4/4 | 0/4 | 3/4 | 4/4 | 4/4 | 0/4 | 11745.421 | 410 |

## Winner

No winner meets the predeclared gate. No further campaign is authorized: all variants were 0/4 substantive under the frozen score, so no production prompt or grammar proposal is selected.

## Sanitized run ledger

| Run | Variant | Packet | Exit | stdout digest / bytes | stderr digest / bytes | Strict | Substantive | No invention | Unresolved | Consistent | Citation | Latency ms | Tokens |
|---:|---|---|---:|---|---|---|---|---|---|---|---|---:|---:|
| 1 | D | real | 0 | `d48ccae01464d5bb7f93b4ef825ba37d397629f0d1f74684c6a75fa958f1aa8b` / 531 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | false | true | true | false | 2035.349 | 86 |
| 2 | D | final-01 | 0 | `dc66453d1fc1e7f669353ce373495ba746ac324ccad60a33b15b6bc7920a5f5a` / 626 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | true | true | false | 2474.321 | 101 |
| 3 | D | final-02 | 0 | `6e76927ed9b1c9b0e18c84f0a5bd3801cbeb425a47e01d1676ec553d423bb9e0` / 756 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | true | true | false | 3615.046 | 121 |
| 4 | D | final-03 | 0 | `af2ed6cb731066d71bc7623cb2ae1cbb2a0d41c70866534a3b3db0cae2f1c7e9` / 658 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | true | true | false | 3620.705 | 102 |
| 5 | E | real | 0 | `c4e8bb2b4c447cfbe4bb780ccca7050ba9a48d2a2764458ffc21b8287bcf9e0a` / 552 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | true | true | false | 3191.081 | 89 |
| 6 | E | final-01 | 0 | `10e39a2a69b84449f5363394fae44d0b9d6056630929a3514079c49424383f02` / 620 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | true | true | false | 2525.911 | 101 |
| 7 | E | final-02 | 0 | `1a4a73d2ceaa3834ed7b62fd8dbc7e85b21e92d7cd87ef1a21912f05eb3c01fe` / 538 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | true | true | false | 2891.878 | 84 |
| 8 | E | final-03 | 0 | `442409f5624bdc6440ebc51cb3522410389c104f84edb375d2d6a8ffc571802d` / 537 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | true | true | false | 3244.883 | 84 |
| 9 | F | real | 0 | `bca25077759d2b833324c4679f9c478eeb04f280dece8a06ec52b3db865dfc2a` / 509 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | true | true | false | 3071.987 | 84 |
| 10 | F | final-01 | 0 | `f0c023e67280621b19b1cb0c14cbb1839011b2f6295cc8ab216905e1170dd6a4` / 784 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | true | true | false | 3189.781 | 134 |
| 11 | F | final-02 | 0 | `c26743f85daabfa0027362b788aa60af7644e71c45e5a6d49b7c37fa86cbaae9` / 1070 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | true | true | false | 5186.808 | 177 |
| 12 | F | final-03 | 0 | `39b7438de92474160348dc5820db1a0962361592a9bcae754942524c887b76d2` / 861 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` / 0 | true | false | true | true | true | false | 5782.954 | 153 |
