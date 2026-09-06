import assert from 'node:assert/strict';
import test from 'node:test';
import { createProvider } from '../src/protocol.mjs';

for (const outcome of ['BLOCKED', 'UNAVAILABLE', 'FAILED', 'PARTIAL']) {
  test(`ASSERT_STRICT_DIAGNOSTIC_CONTRACT_${outcome}`, async () => {
    const reason = `distinct ${outcome} diagnostic`;
    const analyzer = { id: 'test@1', relationKinds: ['INVOKES_TASK'], languages: ['glimmer-js'], frameworks: [], analyze: () => ({ outcome, observations: [], coverage: { status: outcome === 'PARTIAL' ? 'PARTIAL' : 'UNAVAILABLE', reason } }) };
    const response = await createProvider({ analyzers: [analyzer] }).handle({ schema_version: 'lsp-trace.provider-collector-request.v1', provider_id: 'ember-glint@1', adapter_id: 'test@1', session: { session_id: 'test', generation: 1 }, seed: { uri: new URL('../fixtures/plain.ts', import.meta.url).href }, relations: ['INVOKES_TASK'], limits: {} });
    assert.equal(response.coverage.status, outcome === 'PARTIAL' ? 'PARTIAL' : 'UNKNOWN');
    assert.deepEqual(response.coverage.covered, []);
    assert.match(response.diagnostics[0].message, new RegExp(reason));
    assert.match(response.diagnostics[0].message, new RegExp(outcome));
  });
}
