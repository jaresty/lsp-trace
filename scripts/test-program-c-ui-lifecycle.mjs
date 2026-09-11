#!/usr/bin/env node
import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import test from 'node:test';

import { decodeProviderFrame, validateReplay, writeImmutable } from './qualify-program-c-ui-lifecycle.mjs';

function frame(value) {
  const body = Buffer.from(JSON.stringify(value));
  return Buffer.concat([Buffer.from(`Content-Length: ${body.length}\r\n\r\n`), body]);
}

function response(relation = 'RENDERS_FROM') {
  return frame({
    provider: { name: 'ember-glint', version: '1' },
    coverage: { status: 'COMPLETE_WITHIN_BOUNDS' },
    observations: [{ kind: relation }],
  });
}

test('ASSERT_PROGRAM_C_UI_LIFECYCLE_EXACT_BYTES_REPLAY', () => {
  const first = response();
  assert.equal(validateReplay('RENDERS_FROM', first, Buffer.from(first)).count, 1);
  const changed = response('TRIGGERS_RELOAD');
  assert.throws(() => validateReplay('RENDERS_FROM', first, changed), /EXACT_BYTES/);
});

test('ASSERT_PROGRAM_C_UI_LIFECYCLE_PROVIDER_IDENTITY_AND_RELATION', () => {
  const good = response('TRIGGERS_RELOAD');
  assert.equal(validateReplay('TRIGGERS_RELOAD', good, Buffer.from(good)).count, 1);
  const foreign = decodeProviderFrame(good);
  foreign.provider.name = 'foreign';
  const bad = frame(foreign);
  assert.throws(() => validateReplay('TRIGGERS_RELOAD', bad, Buffer.from(bad)), /provider identity/);
  assert.throws(() => validateReplay('RENDERS_FROM', good, Buffer.from(good)), /required relation/);
});

test('ASSERT_PROGRAM_C_UI_LIFECYCLE_EMPTY_OR_INCOMPLETE_FAILS_CLOSED', () => {
  for (const mutation of [
    (value) => { value.observations = []; },
    (value) => { value.coverage.status = 'UNKNOWN'; },
  ]) {
    const value = decodeProviderFrame(response());
    mutation(value);
    const bad = frame(value);
    assert.throws(() => validateReplay('RENDERS_FROM', bad, Buffer.from(bad)), /positive observation|complete coverage/);
  }
});

test('ASSERT_PROGRAM_C_UI_LIFECYCLE_IMMUTABLE_PUBLICATION', () => {
  const directory = mkdtempSync(path.join(tmpdir(), 'program-c-ui-lifecycle-'));
  try {
    const output = path.join(directory, 'receipt.json');
    writeImmutable(output, Buffer.from('one\n'));
    writeImmutable(output, Buffer.from('one\n'));
    assert.equal(readFileSync(output, 'utf8'), 'one\n');
    assert.throws(() => writeImmutable(output, Buffer.from('two\n')), /immutable receipt/);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});
