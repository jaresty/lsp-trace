# ADR0007 Source Text Search v4 unfrozen freeze design

This candidate is unfrozen. It defines a nonrecursive future freeze split:

- `PAYLOAD_MANIFEST.json` covers active normative payload bytes: production package, evaluator, checker, corpus generator, mutation runner, cases, matrix, tooling census, and this design document.
- The payload manifest excludes generated terminal/oracle bytes and excludes any future outer `FREEZE_ENVELOPE`.
- Terminal replay binds only `PAYLOAD_ID`, the digest of the normalized payload manifest, and never embeds a future outer freeze root.
- A later outer freeze envelope may bind expected outputs and the payload manifest without changing attempt/result replay preimages.

Upstream qualified Location pins are distinct from this candidate's future freeze identity and are fixed in every semantic attempt.
