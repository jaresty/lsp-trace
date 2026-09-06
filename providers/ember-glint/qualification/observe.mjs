import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import { analyzeSourceConstrainedTypeScript } from '../analyzers/source-constrained-typescript.mjs';

const sha256 = value => createHash('sha256').update(value).digest('hex');
export function createEmberQualificationObserver({ seeds }) {
  return async ({ workspace, commit }) => {
    const relations = [], blockers = [];
    for (const seed of seeds) {
      const path = join(workspace, seed.path), uri = pathToFileURL(path).href, bytes = readFileSync(path);
      const result = await analyzeSourceConstrainedTypeScript({ documents: [{ uri, revision: commit ?? '0000000000000000000000000000000000000000' }], relation_kinds: [seed.relation] });
      if (result.outcome === 'BLOCKED') blockers.push({ seed: seed.path, relation: seed.relation, blocker: result.blocker });
      for (const observation of result.observations) relations.push({ identity: { kind: observation.kind, from: observation.from.node_id, to: observation.to.node_id, anchor: observation.original_anchor.range }, source_sha256: sha256(bytes) });
    }
    return { relations, blockers, complete: blockers.length === 0 };
  };
}
