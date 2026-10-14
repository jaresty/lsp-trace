# Recursive presumption ledger — incoming custody rerun addendum v1

Disposition: `INCOMING_CUSTODY_RERUN_ADDENDUM_AUTHORED_NOT_GO`. This ledger records only author-side presumptions and corrections. It grants no GO.

| id | presumption | correction / current disposition |
| --- | --- | --- |
| P0 | Historical incoming evidence can be repaired by editing old evidence. | Rejected. Historical evidence mutation is forbidden; this package defines a fresh run namespace only. |
| P1 | Provider custody can be inferred from retained path/digest/version. | Rejected. The separate provider candidate failed closed on missing byte length, complete ambient env, and toolchain digest inventory; this addendum captures those roles only for fresh reruns. |
| P2 | Public summaries may include redacted-but-structural environment values. | Rejected by reviewer. Public summaries now use allowlist-only construction and expose only structural counts/digests/booleans/dispositions. Raw environment values, environment key names, raw paths, `go_env_json`, argv, cwd, rootUri and workspace folders stay private. |
| P3 | Go toolchain closure requires a whole GOROOT tree hash. | Rejected. The private inventory is a closed, precise executable/file set grounded in exact `go env` `GOROOT`/`GOTOOLDIR`, `go tool`, and selected go-env external tool keys; it explicitly does not claim an uncontrolled recursive GOROOT tree. |
| P4 | A custody-only crash can be resumed with the same run id. | Rejected. Reservation is fresh-only; existing run directories or run-id directories fail closed before writes. |
| P5 | Dry full-plan output is denominator evidence. | Rejected. `TEST_ONLY_DRY_FULL_PLAN_V2` is plan-shape evidence only and is marked unable to masquerade as denominator evidence. |
| P6 | Full-run sealing can be handled by package checksums alone. | Rejected. A package-owned finalize command now seals generated run artifacts outside the run dir without self-reference, and verify-full-run recomputes inventory, checksums, counts, and invariants. |
| P7 | A fresh custody rerun can merge authority into the existing provider candidate. | Rejected. The addendum preserves failures/refusals/frontier `UNKNOWN`, does not modify the provider candidate, and remains authored-not-GO. |
