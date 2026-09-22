# Inert ADR0007 next-qualification template

This directory is documentation and fail-closed checking code only. It is not a runnable qualification, does not contain final paths or approved hashes, and **cannot authorize execution**. Preparing or passing synthetic checks does not authorize MCP calls, session changes, binary installation, config mutation, census execution, worker/model execution, or artifact publication.

An operator must create a separate final run directory, choose fresh pairwise-distinct empty roots, pin the final build/config/schema hashes, fully restart Pi, retain one session-list response, and explicitly approve the one-shot request described in `HANDOFF.md`.

Files:

- `HANDOFF.md`: full-restart boundary and exact one-shot request/fingerprint.
- `REQUEST.json`: placeholder session identity with fixed semantic request.
- `validate-staged-request.py`: byte-for-byte copy of the existing request validator; its semantics were not rewritten.
- `preflight.py`: read-only checks over explicitly supplied final paths and the retained pre-result session-list JSON. It normalizes `routing.alias`/`routing.readiness`, permits unrelated sessions, and binds every retained session's `server_instance.executable_sha256` to the pinned MCP digest.
- `check-result.py`: read-only fail-closed acceptance checker for three separately retained inputs: the public delegated-gateway result, a post-result session-list JSON (worker custody), and an execution-accounting JSON/ledger summary (worker/model custody). Counters are never invented in or inferred from the public response.
- `V6-PACKET-CHECKLIST.md`: manual retained-evidence inspection checklist.

The execution-accounting input must identify its retained source and custody and use schema version `lsp-trace.qualification-execution-accounting.v1` with integer `workers` and `model_invocations`. The checker requires both to be zero and independently requires post-result `Census.Workers` to be zero.

The scripts do not create, remove, chmod, rewrite, start, stop, install, invoke MCP, invoke census, invoke a worker, or invoke a model. They read supplied files and print a verdict. Use synthetic fixtures only while developing or testing this template.
