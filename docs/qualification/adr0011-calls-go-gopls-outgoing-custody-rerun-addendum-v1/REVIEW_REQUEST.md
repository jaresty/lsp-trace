# Review request — outgoing custody rerun addendum v1

Please review this package as `OUTGOING_CUSTODY_RERUN_ADDENDUM_AUTHORED_NOT_GO` only. Do not treat it as approval to execute a full run.

Questions:

1. Does the package preserve the exact outgoing historical denominator/corpus: target count `5900`, batch size `50`, batch count `118`, direction `outgoing`, retry-free final ledger, raw ceiling `33554432`, and frontier `UNKNOWN`?
2. Does it invoke the accepted workload runner through the outgoing path for dry planning, and does the full-run wrapper force `forwarded.direction = "outgoing"`?
3. Does prelaunch custody capture exact complete env, gopls path/length/hash/version/argv/cwd, Go/toolchain inventory, Python/source, and initialize bytes/root/workspace/capability before launch?
4. Is the exact captured environment object explicitly passed to `subprocess.Popen(..., env=captured_env, ...)`?
5. Are public artifacts allowlisted and free of private paths, raw env values/key names, argv/cwd/rootUri/workspaceFolders?
6. Does the immutable-base/package-only descendant policy avoid exact HEAD self-reference and avoid dependency on any untracked provider package?
7. Do finalization and verification sidecars reject changed, missing, extra, symlink, FIFO, socket, device, directory, dotdot, and absolute-path tamper cases without following or blocking on nonregular paths?
8. Are outgoing-specific ledger/final-disposition checks present, with no inherited incoming authority other than explicitly cited custody-mechanics precedent?

Expected answer if accepted: package remains `OUTGOING_CUSTODY_RERUN_ADDENDUM_AUTHORED_NOT_GO`, with a separate human decision required before any `--run-full` execution.
