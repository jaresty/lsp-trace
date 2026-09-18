# Full narrow-pilot adapter conformance

- **Status:** `PASS_WITH_EXPECTED_TERMINALS`
- **Pilot:** `PILOT_DISABLED`
- **Output:** `full-conformance.jsonl`
- **Output SHA-256:** `41e494724bbb325e60a90770e30ad386ce31bbceb48711182d22dc88b5ac7040`

Verified against the concurrent adapter and rebuilt worker:

- valid TARGET request: `COMPLETE`;
- duplicate message ID: `DUPLICATE_INPUT`;
- CENSUS request: `POLICY_MISMATCH`;
- incorrect digest: `POLICY_MISMATCH`;
- malformed JSON: `INVALID_INPUT`;
- blank line: ignored;
- 1 ms deadline: `TIMEOUT`.

Cancellation and descendant process-group cleanup are covered by the separate adapter records. All negative outcomes are typed and terminal. This passes the current narrow adapter vector set but does not itself create a formal G1/G8 enablement record.
