# ADR 0007 governance verification plan

## Status

`LOCAL PILOT REQUIREMENTS — PARTIALLY VERIFIED`

This plan defines the local operational controls and remaining evidence gaps. It does not authorize any hosted/service or external surface.

## Work items

### Privacy and retention

- Identify the exact data classes admitted to the pilot.
- Record whether prompts, source projections, outputs, and logs contain sensitive data.
- Define retention, deletion, access, and incident-response rules.
- Verify the rules against the actual output and temporary-file paths.
- Produce an owner-signed privacy/retention decision.

### Monitoring and resource containment

- Freeze CPU, memory, wall-clock, token, temporary-storage, and process-count limits.
- Exercise limit exhaustion and confirm typed terminal failure.
- Record runtime measurements for all four packets.
- Define operational monitoring and alert thresholds for any internal deployment.
- Produce a retained measurement and acceptance record.

### Rollback and revocation

- Define the revocation trigger and accountable operator.
- Revoke the model/runtime/adapter tuple in a disposable local fixture.
- Confirm new requests fail closed after revocation.
- Confirm existing evidence remains immutable and addressable.
- Restore the prior safe state and record timing and terminal outcomes.

### Surface activation

- Exact selected surface: local diagnostic execution only.
- Record the local working-tree, pinned runtime tuple, packet set, output directory, and owner.
- Keep internal service deployment, public, CLI, MCP, core, production, shipment, and census surfaces explicitly prohibited.

## Exit condition

No broader activation is requested. The local diagnostic remains accepted evidence only; any future surface requires a new scope decision and retained verification evidence.
