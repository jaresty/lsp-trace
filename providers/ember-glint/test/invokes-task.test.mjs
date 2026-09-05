import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const root = new URL('../', import.meta.url);
const packageManifest = JSON.parse(await readFile(new URL('package.json', root), 'utf8'));
const positiveSource = await readFile(new URL('fixtures/invokes-task-positive.gts', root), 'utf8');
const negativeSource = await readFile(new URL('fixtures/invokes-task-negative.gts', root), 'utf8');

const EMBER_CONCURRENCY_TASK_IDENTITY = 'ember-concurrency:Task';

function qualifiesInvokesTask({ methodName, receiverTypeIdentity }) {
  if (process.env.LSP_TRACE_TEST_NAME_ONLY === '1') return methodName === 'perform';
  return methodName === 'perform' && receiverTypeIdentity === EMBER_CONCURRENCY_TASK_IDENTITY;
}

test('ASSERT_INVOKES_TASK_S1_BLOCKED_WITHOUT_PINNED_TASK_TYPE_BASIS', () => {
  assert.equal(
    Object.hasOwn(packageManifest.dependencies ?? {}, 'ember-concurrency'),
    false,
    'ASSERT_INVOKES_TASK_PINNED_EMBER_CONCURRENCY_DEPENDENCY_ABSENT',
  );
  assert.match(positiveSource, /from 'ember-concurrency'/, 'ASSERT_INVOKES_TASK_POSITIVE_SEED_NAMES_REQUIRED_TYPE_SOURCE');
  assert.equal(
    qualifiesInvokesTask({ methodName: 'perform', receiverTypeIdentity: undefined }),
    false,
    'ASSERT_INVOKES_TASK_UNRESOLVED_POSITIVE_MUST_NOT_EMIT',
  );
});

test('ASSERT_INVOKES_TASK_REJECTS_UNRELATED_PERFORM_METHOD', () => {
  assert.match(negativeSource, /class ReportRunner/, 'ASSERT_INVOKES_TASK_NEGATIVE_SEED_DECLARES_UNRELATED_RECEIVER');
  assert.equal(
    qualifiesInvokesTask({ methodName: 'perform', receiverTypeIdentity: 'source-local:ReportRunner' }),
    false,
    'ASSERT_INVOKES_TASK_NO_METHOD_NAME_INFERENCE',
  );
});

test('ASSERT_INVOKES_TASK_QUALIFIER_REQUIRES_EXACT_TASK_IDENTITY', () => {
  assert.equal(
    qualifiesInvokesTask({ methodName: 'perform', receiverTypeIdentity: EMBER_CONCURRENCY_TASK_IDENTITY }),
    true,
    'ASSERT_INVOKES_TASK_EXACT_QUALIFIED_RECEIVER_ACCEPTED',
  );
  assert.equal(
    qualifiesInvokesTask({ methodName: 'run', receiverTypeIdentity: EMBER_CONCURRENCY_TASK_IDENTITY }),
    false,
    'ASSERT_INVOKES_TASK_NON_PERFORM_MEMBER_REJECTED',
  );
});
