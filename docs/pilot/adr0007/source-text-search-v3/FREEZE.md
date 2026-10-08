# Freeze

Freeze covers the v3 docs root, internal v3 package, v3 commands, pinned source-admission bytes, go.mod/go.sum, and predecessor lock. `FREEZE.json` is self-normalized with its own digest all-zero and bytes 0 while computing the root; the stored file is never rewritten by verification.
