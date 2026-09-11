#!/usr/bin/env node
import { execFileSync, spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const PROFILE = 'ui-lifecycle-v1';
const COORDINATE = ['typescript', 'ember', 'ember-glint', '1.0.3'];
const RELATIONS = ['RENDERS_FROM', 'TRIGGERS_RELOAD'];
const RECEIPT_NAME = 'ui-lifecycle-v1-typescript-ember-ember-glint-1.0.3.receipt.json';

function fail(message) {
  throw new Error(`ASSERT_PROGRAM_C_UI_LIFECYCLE_QUALIFIER: ${message}`);
}

function sha256(bytes) {
  return `sha256:${createHash('sha256').update(bytes).digest('hex')}`;
}

function digestRecords(records) {
  const hash = createHash('sha256');
  for (const [name, bytes] of records) {
    hash.update(name); hash.update('\0'); hash.update(String(bytes.length)); hash.update('\0'); hash.update(bytes);
  }
  return `sha256:${hash.digest('hex')}`;
}

export function decodeProviderFrame(bytes) {
  const separator = bytes.indexOf('\r\n\r\n');
  if (separator < 1) fail('provider response missing frame boundary');
  const header = bytes.subarray(0, separator).toString('ascii');
  if (!/^Content-Length: (0|[1-9][0-9]*)$/.test(header)) fail('provider response has noncanonical Content-Length');
  const body = bytes.subarray(separator + 4);
  if (body.length !== Number(header.slice('Content-Length: '.length))) fail('provider response Content-Length mismatch');
  return JSON.parse(body.toString('utf8'));
}

export function validateReplay(relation, first, second) {
  if (!first.equals(second)) fail(`${relation} replay is not EXACT_BYTES`);
  const response = decodeProviderFrame(first);
  if (response.provider?.name !== 'ember-glint' || response.provider?.version !== '1') fail(`${relation} provider identity mismatch`);
  if (response.coverage?.status !== 'COMPLETE_WITHIN_BOUNDS' || response.failure !== undefined) fail(`${relation} complete coverage required`);
  const observations = response.observations?.filter((item) => item?.kind === relation) ?? [];
  if (observations.length < 1 || observations.length !== response.observations.length) fail(`${relation} positive observation for required relation only`);
  return { count: observations.length, response };
}

export function writeImmutable(output, bytes) {
  if (existsSync(output)) {
    if (!readFileSync(output).equals(bytes)) fail(`immutable receipt already exists with different bytes: ${output}`);
    return;
  }
  writeFileSync(output, bytes, { flag: 'wx', mode: 0o444 });
}

function encodeRequest(request) {
  const body = Buffer.from(JSON.stringify(request));
  return Buffer.concat([Buffer.from(`Content-Length: ${body.length}\r\n\r\n`, 'ascii'), body]);
}

function executeProvider(provider, request, cwd) {
  const run = spawnSync(provider, [], { cwd, input: encodeRequest(request), maxBuffer: 2 * 1024 * 1024 });
  if (run.status !== 0 || run.stderr.length !== 0) fail(`provider process failed status=${run.status} stderr=${run.stderr}`);
  return run.stdout;
}

function positionOf(source, needle) {
  const offset = source.lastIndexOf(needle) + needle.lastIndexOf('.') + 1;
  if (offset < 0) fail(`missing position needle ${needle}`);
  const lines = source.slice(0, offset).split('\n');
  return { line: lines.length - 1, character: lines.at(-1).length };
}

function args(argv) {
  const values = {};
  for (let index = 0; index < argv.length; index += 2) {
    const key = argv[index];
    if (!key?.startsWith('--') || argv[index + 1] === undefined) fail('arguments must be --name value pairs');
    values[key.slice(2)] = argv[index + 1];
  }
  for (const key of ['revision', 'output']) if (!values[key]) fail(`missing --${key}`);
  return values;
}

export function qualify(options) {
  const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
  const expectedOutput = path.join(root, 'qualification', 'program-c', RECEIPT_NAME);
  if (path.resolve(root, options.output) !== expectedOutput) fail('output must be the exact ui-lifecycle receipt path');
  if (!/^[0-9a-f]{40}$/.test(options.revision)) fail('revision must be an exact lowercase SHA-1');
  const actualRevision = options.revision;
  const qualifierPath = 'scripts/qualify-program-c-ui-lifecycle.mjs';
  const qualifierBytes = readFileSync(path.join(root, qualifierPath));
  const revisionQualifierBytes = execFileSync('git', ['-C', root, 'show', `${actualRevision}:${qualifierPath}`]);
  if (!qualifierBytes.equals(revisionQualifierBytes)) fail('qualifier bytes do not match exact repository revision');

  const manifestPath = path.join(root, 'qualification/program-c/projection-profiles.v1.json');
  const matrixPath = path.join(root, 'qualification/program-c/profile-qualification.tsv');
  const packagePath = path.join(root, 'providers/ember-glint/package.json');
  const renderPath = path.join(root, 'providers/ember-glint/fixtures/renders-from-positive.gts');
  const reloadPath = path.join(root, 'providers/ember-glint/fixtures/source-constrained-synthetic/triggers-reload-positive.js');
  const manifestBytes = readFileSync(manifestPath);
  const matrixBytes = execFileSync('git', ['-C', root, 'show', `${actualRevision}:qualification/program-c/profile-qualification.tsv`]);
  const packageBytes = readFileSync(packagePath);
  const providerPath = path.join(root, 'providers/ember-glint/bin/ember-glint.mjs');
  const providerBytes = readFileSync(providerPath);
  const renderBytes = readFileSync(renderPath);
  const reloadBytes = readFileSync(reloadPath);
  const packageManifest = JSON.parse(packageBytes);
  if (packageManifest.name !== '@lsp-trace/ember-glint-provider' || packageManifest.version !== COORDINATE[3]) fail('provider package identity mismatch');
  const manifest = JSON.parse(manifestBytes);
  const profile = manifest.profiles.find((item) => item.name === PROFILE);
  if (!profile || JSON.stringify(profile.relations.map((item) => item.kind)) !== JSON.stringify(RELATIONS)) fail('profile relation identity mismatch');

  const rows = matrixBytes.toString('utf8').split('\n');
  const oldRow = [PROFILE, ...COORDINATE, 'yes', 'MISSING', '-', '-'].join('\t');
  if (rows.filter((row) => row === oldRow).length !== 1) fail('matrix must contain exactly one untouched MISSING target row');
  const newRow = [PROFILE, ...COORDINATE, 'yes', 'PASS'];

  const versionOutput = `lsp-trace program-c-ui-lifecycle-qualifier/1 revision=${actualRevision} modified=false`;

  const common = {
    schema_version: 'lsp-trace.provider-collector-request.v1', provider_id: 'ember-glint@1',
    adapter_id: 'lsp-trace-observation-adapter@1',
    document_custody: { workspace_revision: { kind: 'git', value: actualRevision, custody: 'PROVIDER_PROVED' } },
    languages: ['typescript'], frameworks: ['ember'], limits: { max_nodes: 100, request_timeout_ms: 30000 },
  };
  const renderSource = renderBytes.toString('utf8');
  const requests = {
    RENDERS_FROM: { ...common, session: { session_id: 'program-c-ui-lifecycle-renders-from', generation: 1 }, seed: { uri: pathToFileURL(renderPath).href, ...positionOf(renderSource, 'this.visibleTotal') }, relations: ['RENDERS_FROM'] },
    TRIGGERS_RELOAD: { ...common, session: { session_id: 'program-c-ui-lifecycle-triggers-reload', generation: 1 }, seed: { uri: pathToFileURL(reloadPath).href }, relations: ['TRIGGERS_RELOAD'] },
  };

  const outputs = [];
  const evidence = [];
  for (const relation of RELATIONS) {
    const first = executeProvider(providerPath, requests[relation], root);
    const second = executeProvider(providerPath, requests[relation], root);
    const checked = validateReplay(relation, first, second);
    outputs.push([`${relation}.request`, encodeRequest(requests[relation])], [`${relation}.result`, first]);
    evidence.push({ relation, status: 'PASS', count: checked.count, custody: 'PROVIDER_PROVED', replay: 'EXACT_BYTES' });
  }

  const receipt = {
    schema_version: 'lsp-trace.program-c-profile-qualification-receipt.v1', result: 'PASS', current: true,
    repository_revision: actualRevision, selection: { status: 'CURRENT', supersedes: null },
    profile: { name: PROFILE, logical_digest: profile.logical_digest, relations: RELATIONS },
    coordinate: { language: COORDINATE[0], framework: COORDINATE[1], provider: COORDINATE[2], provider_version: COORDINATE[3], observed_provider_identity: 'ember-glint', observed_provider_version: packageManifest.version },
    policy_matrix_identity: { profile_manifest: 'qualification/program-c/projection-profiles.v1.json', profile_manifest_sha256: sha256(manifestBytes), qualification_matrix: 'qualification/program-c/profile-qualification.tsv', qualification_matrix_prepublication_sha256: sha256(matrixBytes), matrix_row: newRow },
    tool_identity: { name: 'lsp-trace', revision: actualRevision, version_output: versionOutput },
    exact_command: ['node', 'scripts/qualify-program-c-ui-lifecycle.mjs', '--revision', '$REVISION', '--output', '$OUTPUT'],
    command_bindings: { REVISION: actualRevision, OUTPUT: `qualification/program-c/${RECEIPT_NAME}` },
    run_identity: { run_id: `program-c-ui-lifecycle-${actualRevision.slice(0, 12)}`, generation: '1', replay_runs_per_relation: '2' },
    input_sha256: { profile_manifest: sha256(manifestBytes), qualification_matrix_prepublication: sha256(matrixBytes), provider_package_manifest: sha256(packageBytes), provider_executable: sha256(providerBytes), qualifier_script: sha256(qualifierBytes), renders_from_fixture: sha256(renderBytes), triggers_reload_fixture: sha256(reloadBytes), requests: digestRecords(outputs.filter(([name]) => name.endsWith('.request'))) },
    result_sha256: digestRecords(outputs.filter(([name]) => name.endsWith('.result'))), relation_evidence: evidence,
    claim_ceiling: { scope: 'EXACT_COORDINATE_AND_INPUTS_ONLY', whole_source_complete: false, source_authenticated: false, cross_coordinate_transfer: false },
  };
  const bytes = Buffer.from(`${JSON.stringify(receipt, null, 2)}\n`);
  writeImmutable(expectedOutput, bytes);
  return { receipt, bytes, matrixBytes, oldRow, newRow: [...newRow, `qualification/program-c/${RECEIPT_NAME}`, sha256(bytes)].join('\t') };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const result = qualify(args(process.argv.slice(2)));
    process.stdout.write(`ASSERT_PROGRAM_C_UI_LIFECYCLE_QUALIFIER result=PASS relations=${result.receipt.relation_evidence.map((item) => `${item.relation}:${item.count}`).join(',')} result_sha256=${result.receipt.result_sha256} receipt_sha256=${sha256(result.bytes)} matrix_prepublication_sha256=${sha256(result.matrixBytes)}\n`);
  } catch (error) {
    process.stderr.write(`${error instanceof Error ? error.message : error}\n`);
    process.exitCode = 1;
  }
}
