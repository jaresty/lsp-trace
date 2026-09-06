import { createHash } from 'node:crypto';
import { existsSync, lstatSync, mkdtempSync, mkdirSync, readFileSync, realpathSync, renameSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, isAbsolute, join, relative, resolve, sep } from 'node:path';
import { spawnSync } from 'node:child_process';

export const STAGES = Object.freeze(['BASELINE', 'ENVIRONMENT_ONLY', 'TYPE_OVERLAY']);
export const AUTHORITY = 'SOURCE_CONSTRAINED_ANALYSIS_INTERVENTION';
export const PURPOSE = 'QUALIFICATION_ONLY';
const SHA = /^[0-9a-f]{40}$/;
const sha256 = value => createHash('sha256').update(value).digest('hex');

function runGit(cwd, args) {
  const result = spawnSync('git', args, { cwd, encoding: 'utf8' });
  if (result.status !== 0) throw new Error(`git ${args.join(' ')} failed: ${result.stderr.trim()}`);
  return result.stdout.trim();
}
function inside(path, root) {
  const rel = relative(realpathSync(root), realpathSync(path));
  return rel === '' || (rel !== '..' && !rel.startsWith(`..${sep}`) && !isAbsolute(rel));
}
function confirm(workspace, expectedHead) {
  return runGit(workspace, ['rev-parse', 'HEAD']) === expectedHead && runGit(workspace, ['status', '--porcelain']) === '';
}
function canonical(value) {
  if (Array.isArray(value)) return `[${value.map(canonical).join(',')}]`;
  if (value && typeof value === 'object') return `{${Object.keys(value).sort().map(key => `${JSON.stringify(key)}:${canonical(value[key])}`).join(',')}}`;
  return JSON.stringify(value);
}
export function relationIdentityMultiset(relations) {
  const counts = new Map();
  for (const relation of relations ?? []) {
    const identity = canonical(relation.identity ?? relation);
    counts.set(identity, (counts.get(identity) ?? 0) + 1);
  }
  return [...counts].sort(([a], [b]) => a.localeCompare(b)).map(([identity, multiplicity]) => ({ identity, multiplicity }));
}
export function compareRelationMultisets(left, right) {
  const before = relationIdentityMultiset(left), after = relationIdentityMultiset(right);
  return { equal: canonical(before) === canonical(after), before, after };
}
function applyProposal(workspace, proposal, stage) {
  if (!proposal || proposal.complete !== true || !Array.isArray(proposal.edits)) throw new Error(`${stage}: incomplete write enumeration`);
  const diagnostics = Array.isArray(proposal.diagnostics) ? proposal.diagnostics : [];
  if (proposal.blocker) return { blocked: true, blocker: proposal.blocker, diagnostics };
  for (const edit of proposal.edits) {
    if (!edit || typeof edit.path !== 'string' || !edit.path || typeof edit.preimage_sha256 !== 'string' || typeof edit.content !== 'string' || !edit.write_origin) throw new Error(`${stage}: invalid preconditioned edit`);
    const target = resolve(workspace, edit.path);
    const lexical = relative(workspace, target);
    if (lexical === '..' || lexical.startsWith(`..${sep}`) || isAbsolute(lexical)) throw new Error(`${stage}: source/path escape`);
    const parent = dirname(target);
    mkdirSync(parent, { recursive: true });
    if (!inside(parent, workspace) || (existsSync(target) && (!inside(target, workspace) || lstatSync(target).isSymbolicLink()))) throw new Error(`${stage}: source/path escape`);
    const before = existsSync(target) ? readFileSync(target) : Buffer.alloc(0);
    if (sha256(before) !== edit.preimage_sha256) throw new Error(`${stage}: stale preimage for ${edit.path}`);
    writeFileSync(target, edit.content);
  }
  return { blocked: false, diagnostics };
}
function commitStage(workspace, stage, predecessor) {
  if (runGit(workspace, ['rev-parse', 'HEAD']) !== predecessor) throw new Error(`${stage}: unconfirmed predecessor`);
  runGit(workspace, ['add', '--all']);
  if (runGit(workspace, ['diff', '--cached', '--name-only']) === '') throw new Error(`${stage}: proposal made no change`);
  runGit(workspace, ['commit', '-m', `qualification: ${stage}`]);
  const commit = runGit(workspace, ['rev-parse', 'HEAD']);
  if (!confirm(workspace, commit) || runGit(workspace, ['rev-parse', 'HEAD^']) !== predecessor) throw new Error(`${stage}: commit verification failed`);
  return commit;
}

export async function runCampaign({ source, pinnedCommit, adapter, observe, output, validate, tempRoot = tmpdir() }) {
  if (!SHA.test(pinnedCommit ?? '')) throw new Error('pinnedCommit must be a full Git commit');
  if (!source || !adapter || typeof observe !== 'function' || typeof validate !== 'function' || !output) throw new Error('source, adapter, observe, validate, and caller-selected output are required');
  const sourceRoot = realpathSync(source), repositoryRoot = realpathSync(resolve(dirname(new URL(import.meta.url).pathname), '../..'));
  if (runGit(sourceRoot, ['rev-parse', `${pinnedCommit}^{commit}`]) !== pinnedCommit) throw new Error('pinned source commit is unavailable');
  const workspace = mkdtempSync(join(resolve(tempRoot), 'lsp-trace-type-overlay-'));
  if (inside(workspace, sourceRoot) || inside(workspace, repositoryRoot)) throw new Error('disposable workspace must be outside source and lsp-trace');
  try {
    runGit(workspace, ['init', '--quiet']);
    runGit(workspace, ['config', 'user.name', 'lsp-trace qualification']);
    runGit(workspace, ['config', 'user.email', 'qualification@example.invalid']);
    runGit(workspace, ['fetch', '--quiet', '--no-tags', sourceRoot, pinnedCommit]);
    runGit(workspace, ['checkout', '--quiet', '--detach', pinnedCommit]);
    if (!confirm(workspace, pinnedCommit)) throw new Error('BASELINE: unconfirmed pinned source');
    const stages = [{ name: 'BASELINE', commit: pinnedCommit, observation: await observe({ stage: 'BASELINE', workspace }) }];
    let predecessor = pinnedCommit;
    for (const [name, method] of [['ENVIRONMENT_ONLY', 'proposeEnvironment'], ['TYPE_OVERLAY', 'proposeOverlay']]) {
      if (!confirm(workspace, predecessor)) throw new Error(`${name}: unconfirmed predecessor`);
      const proposal = await adapter[method]({ workspace, predecessor });
      const applied = applyProposal(workspace, proposal, name);
      if (applied.blocked) {
        stages.push({ name, predecessor, status: 'BLOCKED', blocker: applied.blocker, diagnostics: applied.diagnostics });
        break;
      }
      const commit = commitStage(workspace, name, predecessor);
      stages.push({ name, predecessor, commit, status: 'CONFIRMED', diagnostics: applied.diagnostics, observation: await observe({ stage: name, workspace, commit }) });
      predecessor = commit;
    }
    const comparisons = stages.slice(1).filter(stage => stage.observation).map((stage, index) => ({ from: stages[index].name, to: stage.name, relations: compareRelationMultisets(stages[index].observation.relations, stage.observation.relations) }));
    const report = { schema_version: 'lsp-trace.analysis-only-type-overlay-qualification.v1', authority: AUTHORITY, purpose: PURPOSE, automatic_continuation: false, admission_mutated: false, inventory_mutated: false, pinned_source: { commit: pinnedCommit, immutable: true }, stages, comparisons };
    const bytes = Buffer.from(`${JSON.stringify(report, null, 2)}\n`);
    await validate(bytes, report);
    mkdirSync(dirname(resolve(output)), { recursive: true });
    const candidate = `${resolve(output)}.candidate-${process.pid}`;
    writeFileSync(candidate, bytes);
    renameSync(candidate, resolve(output));
    return report;
  } finally { rmSync(workspace, { recursive: true, force: true }); }
}
