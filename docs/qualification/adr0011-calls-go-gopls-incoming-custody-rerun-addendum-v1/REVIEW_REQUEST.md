# Review request — incoming custody rerun addendum v1

Requested review disposition: verify fail-closed or return bounded blocker. Do not treat this package as GO evidence.

Review questions:

1. Does public summary construction expose only allowlisted structural metadata/digests/counts and prove forbidden env keys/values cannot appear?
2. Does run reservation refuse all pre-existing run dirs/run-id dirs and avoid same-run-id resume after custody-only crash?
3. Does `--finalize-run` / `--verify-full-run` seal every generated full-run artifact without self-reference and reject changed/missing/extra/symlink/nonregular/unfinalized outputs?
4. Are 5,900 / 50 / 118 / 33,554,432 / incoming-only / retry-free / fresh namespace invariants enforced?
5. Are historical failures/refusals/frontier `UNKNOWN` preserved without authority merge?
6. Are TEST_ONLY artifacts prevented from masquerading as denominator evidence?
7. Does the self-reference correction keep `c3d77247f5a2e06c0d6ac5b1b55a3df7dbea588a` as an immutable ancestor while accepting only addendum-confined descendants with clean tracked state and matching package checksums?
8. Does public/private provenance record the actual execution HEAD only in bounded public token/digest form and full private form?

Expected answer if accepted: package remains `INCOMING_CUSTODY_RERUN_ADDENDUM_AUTHORED_NOT_GO`, with a separate human decision needed before any `--run-full` execution.
