#!/usr/bin/env node
import { createHash } from 'node:crypto';
import { chmodSync, mkdirSync, readFileSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { spawn, spawnSync } from 'node:child_process';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const here = resolve(root, 'qualification/source-constrained-synthetic');
const specification = JSON.parse(readFileSync(join(here, 'target-b05-seeds.v1.json')));
const stable = '/tmp/lsp-trace-target-b05-synthetic-qualification';
const workspace = join(stable, 'workspace'), install = join(stable, 'provider-install'), dist = join(stable, 'dist');
const mcp = join(stable, 'lsp-trace-mcp'), fakeLSP = join(stable, 'fake-lsp');
const output = resolve(root, 'qualification/retained/target-b05-synthetic/qualification-evidence.v1.json');
const retain = process.argv.includes('--retain');
const sha = value => `sha256:${createHash('sha256').update(value).digest('hex')}`;
const canonical = value => Array.isArray(value) ? `[${value.map(canonical).join(',')}]` : value && typeof value === 'object' ? `{${Object.keys(value).sort().map(key => `${JSON.stringify(key)}:${canonical(value[key])}`).join(',')}}` : JSON.stringify(value);
const normalize = response => { const copy = structuredClone(response); copy.id = 0; const normalizeBody = body => { if (body?.request_id) body.request_id = 'replay-normalized'; return body; }; if (copy.result?.structuredContent) normalizeBody(copy.result.structuredContent); if (copy.result?.content?.[0]?.text) copy.result.content[0].text = JSON.stringify(normalizeBody(JSON.parse(copy.result.content[0].text))); return copy; };
function run(command, args, options = {}) { const result = spawnSync(command, args, { cwd: root, encoding: 'utf8', ...options }); if (result.status !== 0) throw new Error(`${command} ${args.join(' ')} failed (${result.status})\n${result.stdout}\n${result.stderr}`); return result; }
function toolCall(id, name, args) { return { jsonrpc: '2.0', id, method: 'tools/call', params: { name, arguments: args } }; }
async function execute(request) { return new Promise((resolvePromise, reject) => { const input = `${JSON.stringify(toolCall(1, 'lsp_session_v1_list', {}))}\n${JSON.stringify(request)}\n`; const child = spawn(mcp, ['--bootstrap-config', join(stable, 'bootstrap.json')], { stdio: ['pipe', 'pipe', 'pipe'] }); let stdout = '', stderr = ''; child.stdout.on('data', value => stdout += value); child.stderr.on('data', value => stderr += value); child.on('error', reject); child.on('close', code => { if (code !== 0) return reject(new Error(`mcp exit ${code}: ${stderr}`)); const lines = stdout.trim().split('\n').map(JSON.parse); if (lines.length !== 2) return reject(new Error(`expected 2 responses, got ${lines.length}: ${stdout}`)); resolvePromise({ request, response: lines[1], input, stdout, stderr }); }); child.stdin.end(input); }); }

rmSync(stable, { recursive: true, force: true });
for (const directory of [workspace, install, dist]) mkdirSync(directory, { recursive: true });
run('go', ['build', '-trimpath', '-o', mcp, './cmd/lsp-trace-mcp']);
run('go', ['build', '-trimpath', '-o', fakeLSP, './cmd/fake-lsp']);
for (const seed of specification.seeds) writeFileSync(join(workspace, `${seed.id}.ts`), readFileSync(join(here, seed.path)));
run('git', ['init', '-q'], { cwd: workspace });
run('git', ['config', 'user.name', 'lsp-trace qualification'], { cwd: workspace });
run('git', ['config', 'user.email', 'qualification@example.invalid'], { cwd: workspace });
run('git', ['add', '.'], { cwd: workspace });
run('git', ['commit', '-qm', 'frozen target-b05 synthetic seeds'], { cwd: workspace, env: { ...process.env, GIT_AUTHOR_DATE: '2000-01-01T00:00:00Z', GIT_COMMITTER_DATE: '2000-01-01T00:00:00Z' } });
const commit = run('git', ['rev-parse', 'HEAD'], { cwd: workspace }).stdout.trim();
const clean = run('git', ['status', '--porcelain'], { cwd: workspace }).stdout === '';
const packed = run('npm', ['pack', '--pack-destination', dist], { cwd: join(root, 'providers/source-constrained-synthetic') });
const tarball = join(dist, packed.stdout.trim().split('\n').at(-1));
const installed = run('npm', ['install', '--ignore-scripts', '--prefix', install, tarball]);
const provider = realpathSync(join(install, 'node_modules/.bin/source-constrained-synthetic-provider'));
chmodSync(provider, 0o755);
const providerID = 'source-constrained-synthetic-provider@1.0.0';
const bootstrap = { version: 1, processes: [{ alias: 'synthetic', profile: { trust_domain: 'target-b05-synthetic-qualification', workspace, profile: 'fake-lsp', environment_reference: 'qualification' }, execution: { path: fakeLSP, directory: workspace } }], providers: [{ schema_version: 'lsp-trace.bootstrap-provider.v1', identity: providerID, version: '1.0.0', protocol: { name: 'lsp-trace.provider-observations', version: '1' }, execution: { path: provider, directory: dirname(provider) }, executable_available: true, conformance_verified: true, capabilities: { relations: ['INVOKES_TASK', 'TRIGGERS_RELOAD'], languages: ['typescript'], frameworks: ['source-constrained-synthetic'] }, limits: { request_bytes: 1048576, response_bytes: 1048576, protocol_messages: 1, stderr_bytes: 4096, wall_time_ms: 30000, termination_grace_ms: 1000 } }] };
writeFileSync(join(stable, 'bootstrap.json'), JSON.stringify(bootstrap));

const attempts = [];
for (const seed of specification.seeds) for (const operation of ['incoming', 'slice']) for (let replay = 1; replay <= 2; replay++) {
  const uri = pathToFileURL(realpathSync(join(workspace, `${seed.id}.ts`))).href;
  const args = { session_id: 'synthetic', generation: 1, uri, line: seed.generated_call_range.start.line, character: seed.generated_call_range.start.character, max_nodes: 100, timeout_ms: 30000, request_timeout_ms: 30000, relations: [seed.relation], providers: [providerID], languages: ['typescript'], frameworks: ['source-constrained-synthetic'], workspace_revision: { kind: 'git', commit, custody: 'CALLER_ASSERTED' }, fail_on_unknown_revision: true };
  if (operation === 'incoming') args.max_depth = 2; else Object.assign(args, { start_mode: 'at', up_depth: 2, down_depth: 2 });
  const request = toolCall(attempts.length + 2, `lsp_trace_v1_${operation}`, args);
  const observed = await execute(request);
  const structured = observed.response?.result?.structuredContent;
  const text = observed.response?.result?.content?.[0]?.text;
  const equivalent = typeof text === 'string' && canonical(JSON.parse(text)) === canonical(structured);
  const graph = structured?.content ? JSON.parse(structured.content) : null;
  const relation = graph?.relations?.find(item => item.kind === seed.relation);
  const exactAnchor = relation?.anchors?.length === 1 && relation.anchors[0].uri === uri && canonical(relation.anchors[0].range) === canonical(seed.generated_call_range);
  const exactEndpoints = seed.relation === 'INVOKES_TASK'
    ? relation?.from?.includes('receiver=TaskForAsyncTaskFunction') && relation?.to?.includes('package=ember-concurrency@5.2.0') && relation?.to?.includes('symbol=AbstractTask.perform') && relation?.to?.includes('evidence=typescript-checker')
    : relation?.from?.includes('receiver=UserImportModel') && relation?.from?.includes('collection=uploads:UserImportModel[]') && relation?.to?.includes('package=@warp-drive/legacy@5.8.1') && relation?.to?.includes('symbol=Model.reload') && relation?.to?.includes('evidence=typescript-checker');
  const documentRevision = graph?.provenance?.custody?.documents?.[0]?.revision;
  const strictPublicCustody = documentRevision?.kind === 'git' && documentRevision.value === commit && documentRevision.custody === 'PROVIDER_PROVED' && documentRevision.blob === relation?.anchors?.[0]?.blob;
  const logical = { schema_version: graph?.schema_version, relation: relation && { kind: relation.kind, from: relation.from, to: relation.to, anchors: relation.anchors }, custody: documentRevision };
  const record = { id: `${seed.id}-${operation}-${replay}`, seed: seed.id, operation, replay, relation: seed.relation, status: equivalent && graph?.schema_version === 'lsp-trace.graph.v4' && exactAnchor && exactEndpoints && strictPublicCustody ? 'PASS' : 'FAIL', custody: { requested: args.workspace_revision, observed_document_revision: documentRevision, workspace_commit: commit, clean }, validation: { graph_v4: graph?.schema_version === 'lsp-trace.graph.v4', relation_kind: relation?.kind, exact_anchor: exactAnchor, exact_endpoints: Boolean(exactEndpoints), checker_provenance: relation?.to?.includes('evidence=typescript-checker') ?? false, strict_public_commit_custody: strictPublicCustody, content_structured_equivalent: equivalent }, request, response: observed.response, transcript: observed.input + observed.stdout + observed.stderr, digests: { request: sha(canonical(request)), response: sha(canonical(normalize(observed.response))), transcript: sha(observed.input + observed.stdout + observed.stderr), logical: sha(canonical(logical)) } };
  attempts.push(record);
}
for (const seed of specification.seeds) for (const operation of ['incoming', 'slice']) { const pair = attempts.filter(item => item.seed === seed.id && item.operation === operation); if (pair.length !== 2 || pair[0].digests.response !== pair[1].digests.response || pair[0].digests.logical !== pair[1].digests.logical) throw new Error(`deterministic replay failed: ${seed.id}/${operation}`); }
if (attempts.length !== 8 || attempts.some(item => item.status !== 'PASS')) throw new Error(`target B05 qualification failed: ${attempts.filter(item => item.status !== 'PASS').map(item => item.id).join(',')}`);
const evidence = { schema: 'lsp-trace.target-b05-synthetic.evidence.v1', immutable: true, evidence_class: 'SOURCE_CONSTRAINED_SYNTHETIC', target: 'B05', source_commit: specification.source.commit, qualifier: { executable: 'qualification/source-constrained-synthetic/qualify-target-b05.mjs', transport: 'actual-built-lsp-trace-mcp-stdio', fake_lsp: true, explicit_provider: providerID, isolated_workspace: true, workspace_commit: commit, workspace_clean: clean, provider_install: 'outside-repository-from-npm-pack', operation_count: attempts.length, deterministic_replay: true }, digests: { mcp_executable: sha(readFileSync(mcp)), provider_package: sha(readFileSync(tarball)), provider_executable: sha(readFileSync(provider)) }, attempts, admission: { direct_javascript: 'UNSUPPORTED', provisional_target_B05_admitted: true, PROGRAM_B_ADMITTED: false }, history: { additive: true, b05_v2_rewritten: false, generic_matrix_rewritten: false } };
if (retain) {
  mkdirSync(dirname(output), { recursive: true });
  writeFileSync(output, `${JSON.stringify(evidence, null, 2)}\n`);
}
console.log(`TARGET_B05_QUALIFIED evidence=${retain ? output : 'fresh-observation-not-retained'} observed_calls=8 replay=deterministic provisional_B05=true program_b=false retained=${retain}`);
