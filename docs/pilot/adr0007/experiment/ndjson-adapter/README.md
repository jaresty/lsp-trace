# Narrow NDJSON adapter

- **Status:** `ADAPTER_DRAFT`
- **Pilot:** `PILOT_DISABLED`
- **Source SHA-256:** `2afadd964adfcb47eb7deab7c4ebcd0ba0e972d0cb4a0a9492f39b63a94af867`
- **Binary SHA-256:** `bd381b09097c2f650df44f9269cec3bf27b2a9ee066082a871f096c978f2f852`

The adapter reads one request JSON object per line and emits one response/error object per line. It validates protocol/message/correlation fields, rejects duplicates, writes caller-supplied prompt bytes to a temporary file, and invokes the pinned worker with fixed model/library paths supplied through environment configuration.

## Smoke result

A valid request with `message_id=m-001` and `correlation_id=c-001` returned:

- message type: `response`
- status: `COMPLETE`
- payload status: `COMPLETE`

The adapter is a narrow experiment boundary, not yet the frozen ADR 0007 protocol. It still requires strict schema validation, admission/lineage fields, bounded temp-file policy, cancellation semantics, resource limits, and sandbox integration before enablement.
