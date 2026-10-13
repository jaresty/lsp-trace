# Review request

Verdict requested: `TARGET_CORPUS_V1_GO` or a bounded refusal with exact file/field reason.

Please independently verify:

1. revision/tree and dirty-workspace refusal;
2. all 1,689 tracked `.go` paths are represented exactly once;
3. each exclusion category and every excluded file follows `CONTRACT.md`;
4. all 653 admitted files match their Git blob SHA-256 and parse without error;
5. all 5,900 named `FuncDecl` values, including methods, occur exactly once;
6. byte ranges are exact half-open declaration ranges and positions are zero-based UTF-16-safe;
7. receiver text and length-delimited identity encoding are deterministic;
8. canonical ordering, duplicate/accounting failures, and two-run byte equality;
9. incoming and outgoing manifests bind the same exact corpus bytes but remain separate domains/profiles;
10. authority 0, `accepted=false`, `UNKNOWN`, and `REFUSED_MISSING_EVIDENCE` are preserved;
11. no feature/runtime/ownership/purpose/repository-completeness claim escapes the ceiling.

This request does not ask the reviewer to run gopls, accept relations, implement production/test code, or commit these files.
