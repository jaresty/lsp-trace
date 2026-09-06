import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const schemaPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../schema/schemas/lsp-trace.analysis-only-type-overlay-qualification.v1.schema.json');
const schema = JSON.parse(readFileSync(schemaPath));

export function validateQualificationReport(_bytes, value) {
  const required = schema.required;
  if (!value || typeof value !== 'object' || required.some(key => !Object.hasOwn(value, key))) throw new Error('qualification report does not satisfy registered schema');
  if (value.schema_version !== schema.properties.schema_version.const || value.authority !== schema.properties.authority.const || value.purpose !== schema.properties.purpose.const) throw new Error('qualification authority/schema mismatch');
  for (const field of ['automatic_continuation', 'admission_mutated', 'inventory_mutated']) if (value[field] !== false) throw new Error(`${field} must remain false`);
  if (!/^[0-9a-f]{40}$/.test(value.pinned_source?.commit ?? '') || value.pinned_source.immutable !== true) throw new Error('immutable pinned source is required');
  const names = value.stages?.map(stage => stage.name);
  if (!Array.isArray(names) || names.some((name, index) => name !== ['BASELINE', 'ENVIRONMENT_ONLY', 'TYPE_OVERLAY'][index])) throw new Error('stages must be the fixed confirmed prefix');
  for (const comparison of value.comparisons ?? []) for (const side of ['before', 'after']) {
    if (!Array.isArray(comparison.relations?.[side]) || comparison.relations[side].some(entry => typeof entry.identity !== 'string' || !Number.isInteger(entry.multiplicity) || entry.multiplicity < 1)) throw new Error('relation comparison must contain canonical identity multisets');
  }
}
