import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { pathToFileURL } from 'node:url';

import { createGlintAnalyzer } from '../analyzers/glint.mjs';

function fixtureProject() {
  const projectDirectory = mkdtempSync(path.join(tmpdir(), 'glint-project-service-'));
  writeFileSync(path.join(projectDirectory, 'tsconfig.json'), '{}');
  mkdirSync(path.join(projectDirectory, 'src'));
  writeFileSync(path.join(projectDirectory, 'src', 'first.gts'), 'alpha target omega\n');
  writeFileSync(path.join(projectDirectory, 'src', 'second.gts'), 'before\nother value after\n');
  return projectDirectory;
}

function service({ mappingAvailable = true } = {}) {
  const calls = [];
  const analyzer = createGlintAnalyzer({
    loadConfig(projectDirectory) { calls.push(['config', projectDirectory]); },
    analyzeProject(projectDirectory) {
      calls.push(['project', projectDirectory]);
      return {
        transformManager: {
          getTransformedRange(filename, start, end) {
            calls.push(['transform', filename, start, end]);
            if (!mappingAvailable) return undefined;
            return { transformedFileName: `${filename}.ts`, transformedStart: start + 100, transformedEnd: end + 100 };
          },
          getOriginalRange(filename, start, end) {
            calls.push(['original', filename, start, end]);
            const originalFileName = filename.slice(0, -3);
            return { originalFileName, originalStart: start - 100, originalEnd: end - 100 };
          },
        },
        languageServer: {
          getDefinition(uri, position) {
            calls.push(['definition', uri, position]);
            return [{ uri: 'file:///authoritative-definition.ts', range: { start: { line: 4, character: 1 }, end: { line: 4, character: 6 } } }];
          },
          getHover(uri, position) {
            calls.push(['type', uri, position]);
            return { contents: { kind: 'markdown', value: '`string`' }, range: { start: position, end: { line: position.line, character: position.character + 5 } } };
          },
        },
        shutdown() { calls.push(['shutdown']); },
      };
    },
  });
  return { analyzer, calls };
}

const ASSERT_REQUEST = 'ASSERT_GLINT_REQUEST_DERIVES_PROJECT_FILE_POSITION';
const ASSERT_RESULTS = 'ASSERT_GLINT_EXPOSES_MAPPING_DEFINITION_TYPE_RESULTS';
const ASSERT_MAPPING = 'ASSERT_GLINT_MAPPING_UNAVAILABLE_EXPLICIT_BLOCKED';

test(`${ASSERT_REQUEST}: caller-selected coordinates drive each query`, () => {
  const projectDirectory = fixtureProject();
  const { analyzer, calls } = service();
  const requests = [
    { file: 'src/first.gts', position: { line: 0, character: 6 }, range: { start: { line: 0, character: 6 }, end: { line: 0, character: 12 } }, text: 'target' },
    { file: 'src/second.gts', position: { line: 1, character: 6 }, range: { start: { line: 1, character: 6 }, end: { line: 1, character: 11 } }, text: 'value' },
  ];

  for (const request of requests) {
    const result = analyzer.analyze({ projectDirectory, ...request });
    assert.equal(result.status, 'SCOPED_ROLE', ASSERT_REQUEST);
    assert.equal(result.observations[0].original_anchor.text, request.text, ASSERT_REQUEST);
    const uri = pathToFileURL(path.join(projectDirectory, request.file)).href;
    assert.ok(calls.some((call) => call[0] === 'definition' && call[1] === uri && call[2] === request.position), ASSERT_REQUEST);
    assert.ok(calls.some((call) => call[0] === 'type' && call[1] === uri && call[2] === request.position), ASSERT_REQUEST);
  }
});

test(`${ASSERT_RESULTS}: exact mapping and authoritative query values are exposed`, () => {
  const projectDirectory = fixtureProject();
  const { analyzer } = service();
  const result = analyzer.analyze({
    projectDirectory,
    file: 'src/first.gts',
    position: { line: 0, character: 6 },
    range: { start: { line: 0, character: 6 }, end: { line: 0, character: 12 } },
  });
  const observation = result.observations[0];
  assert.deepEqual(observation.virtual_mapping.original, { start: 6, end: 12 }, ASSERT_RESULTS);
  assert.deepEqual(observation.virtual_mapping.virtual, { start: 106, end: 112 }, ASSERT_RESULTS);
  assert.deepEqual(observation.definitions, [{ uri: 'file:///authoritative-definition.ts', range: { start: { line: 4, character: 1 }, end: { line: 4, character: 6 } } }], ASSERT_RESULTS);
  assert.deepEqual(observation.type, { contents: { kind: 'markdown', value: '`string`' }, range: { start: { line: 0, character: 6 }, end: { line: 0, character: 11 } } }, ASSERT_RESULTS);
});

test(`${ASSERT_MAPPING}: unavailable transformed mapping cannot succeed`, () => {
  const projectDirectory = fixtureProject();
  const { analyzer } = service({ mappingAvailable: false });
  const result = analyzer.analyze({
    projectDirectory,
    file: 'src/first.gts',
    position: { line: 0, character: 6 },
    range: { start: { line: 0, character: 6 }, end: { line: 0, character: 12 } },
  });
  assert.equal(result.status, 'BLOCKED', ASSERT_MAPPING);
  assert.equal(result.reason, 'GLINT_MAPPING_UNAVAILABLE', ASSERT_MAPPING);
  assert.deepEqual(result.observations, [], ASSERT_MAPPING);
});
