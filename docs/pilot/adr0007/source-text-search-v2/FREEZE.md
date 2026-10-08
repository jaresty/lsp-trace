# Freeze convention

Freeze includes normative docs, schemas, corpus inputs, expected oracle records, generator/evaluator/verifier source, and tooling identity. It excludes v1 and public-surface files. The freeze manifest schema is `lsp-trace.adr0007.source-text-search.freeze.private.v2`.

Self-measuring rule adopted from the qualified Location `SPEC_MANIFEST`: the manifest contains a self entry for `FREEZE.json` whose `sha256` is `sha256:` followed by sixty-four zeroes and whose `bytes` is `0` while computing the root. The stored manifest then contains the resulting root. Verifier recomputes with the self entry zeroed, compares exact census and root, and rejects any tamper. Generating two empty roots must be byte-identical.
