# Caller-project JavaScript MCP qualification ledger

Baseline: `337d1f1ccf880317b1207413fd6d56932116a3e6`

The tracked `lsp-trace.caller-project-javascript.evidence.v1` family qualifies caller-project JavaScript through an independently packed and offline-installed `ember-glint@1` executable and the real built `lsp-trace-mcp` stdio process. It covers exact `INVOKES_TASK` and `TRIGGERS_RELOAD` positives plus same-spelling, unsafe `any`/`unknown`, missing/malformed configuration, and missing/wrong declaration variants.

The default qualifier is read-only. Only explicit `--retain` writes timestamped evidence carrying package, provider executable, MCP executable, request, response, and transcript digests. The release gate consumes the retained family without rewriting B05 or any historical qualification bytes.

## Derivation

1. Twelve caller-owned JavaScript projects form a closed variant ledger: two positives and ten confusable, unsafe, configuration, or declaration boundaries.
2. Each case is sent through both `lsp_trace_v1_incoming` and `lsp_trace_v1_slice`, twice, yielding exactly 24 logical operations and 48 replay attempts.
3. Positive attempts require exactly one selected relation, graph-v4 validation, provider identity `ember-glint@1`, content/structured parity, original Git/blob custody, and caller-local declaration path/digest custody.
4. Negative and unavailable variants require the exact explicit domain-error schema and recorded code; no empty-success reinterpretation is admitted.
5. Assertion-specific guards independently reject missing variants, wrong counts/outcomes, schema or provider drift, parity loss, replay drift, missing custody, and missing timestamp/digest retention metadata.
6. Release admission adds this family beside B05; it does not rewrite `qualification/retained/b05/` or historical evidence.
