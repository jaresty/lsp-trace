import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

export const PROVIDER_IDENTITY = 'ember-glint@1';
export const REQUEST_SCHEMA = 'lsp-trace.provider-request.v1';
export const OBSERVATION_SCHEMA = 'lsp-trace.provider-observations.v1';
export const LIFECYCLE_OUTCOMES = Object.freeze([
  'BLOCKED',
  'UNAVAILABLE',
  'FAILED',
  'PARTIAL',
  'BOUNDED',
  'EMPTY',
  'COMPLETE',
]);
export const LIMITS = Object.freeze({
  max_content_bytes: 1_048_576,
  max_observations: 10_000,
  max_timeout_ms: 30_000,
});

const OUTCOMES = new Set(LIFECYCLE_OUTCOMES);
const DIGEST = /^sha256:[0-9a-f]{64}$/;
const TOKEN = /^[A-Z][A-Z0-9_]*$/;

function plainObject(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value) && Object.getPrototypeOf(value) === Object.prototype;
}

function canonicalize(value) {
  if (value === null || typeof value === 'string' || typeof value === 'boolean') return value;
  if (typeof value === 'number' && Number.isFinite(value)) return value;
  if (Array.isArray(value)) return value.map(canonicalize);
  if (plainObject(value)) {
    return Object.fromEntries(Object.keys(value).sort().map((key) => [key, canonicalize(value[key])]));
  }
  throw new TypeError('protocol values must contain only finite JSON data');
}

export function canonicalBytes(value) {
  return Buffer.from(JSON.stringify(canonicalize(value)), 'utf8');
}

export function logicalDigest(value) {
  return `sha256:${createHash('sha256').update(canonicalBytes(value)).digest('hex')}`;
}

export function encodeFrame(value, { maxContentBytes = LIMITS.max_content_bytes } = {}) {
  const payload = canonicalBytes(value);
  if (payload.length > maxContentBytes) throw new RangeError('frame exceeds max_content_bytes');
  return Buffer.concat([Buffer.from(`Content-Length: ${payload.length}\r\n\r\n`, 'ascii'), payload]);
}

export function decodeFrame(input, { maxContentBytes = LIMITS.max_content_bytes } = {}) {
  if (!Buffer.isBuffer(input)) throw new TypeError('frame must be a Buffer');
  const boundary = input.indexOf('\r\n\r\n', 0, 'ascii');
  if (boundary < 0) throw new Error('frame requires CRLF header boundary');
  const header = input.subarray(0, boundary).toString('ascii');
  if (!/^Content-Length: (0|[1-9][0-9]*)$/.test(header)) throw new Error('frame requires exactly one canonical Content-Length header');
  const length = Number(header.slice('Content-Length: '.length));
  if (!Number.isSafeInteger(length) || length > maxContentBytes) throw new RangeError('invalid Content-Length');
  const payload = input.subarray(boundary + 4);
  if (payload.length !== length) throw new Error('Content-Length does not match payload bytes');
  let value;
  try {
    const json = new TextDecoder('utf-8', { fatal: true }).decode(payload);
    value = JSON.parse(json);
  } catch {
    throw new Error('payload must be valid UTF-8 JSON');
  }
  if (!plainObject(value)) throw new TypeError('payload must be a JSON object');
  return value;
}

function strings(value, name, { allowEmpty = true } = {}) {
  if (!Array.isArray(value) || (!allowEmpty && value.length === 0) || value.some((item) => typeof item !== 'string' || item.length === 0)) {
    throw new TypeError(`${name} must be ${allowEmpty ? 'an' : 'a non-empty'} array of non-empty strings`);
  }
  return [...new Set(value)].sort();
}

function positiveInteger(value, name, maximum) {
  if (!Number.isSafeInteger(value) || value <= 0 || value > maximum) throw new RangeError(`${name} must be between 1 and ${maximum}`);
  return value;
}

function validateRequest(value) {
  if (!plainObject(value)) throw new TypeError('request must be an object');
  const allowed = new Set(['schema', 'request_id', 'operation', 'relation_kinds', 'documents', 'limits']);
  if (Object.keys(value).some((key) => !allowed.has(key))) throw new TypeError('request contains unknown fields');
  if (value.schema !== REQUEST_SCHEMA || value.operation !== 'analyze') throw new TypeError('unsupported generic request schema or operation');
  if (typeof value.request_id !== 'string' || value.request_id.length === 0) throw new TypeError('request_id must be non-empty');
  const relationKinds = strings(value.relation_kinds, 'relation_kinds', { allowEmpty: false });
  if (relationKinds.some((kind) => !TOKEN.test(kind))) throw new TypeError('relation_kinds must use canonical tokens');
  if (!Array.isArray(value.documents) || value.documents.length === 0) throw new TypeError('documents must be non-empty');
  for (const document of value.documents) {
    if (!plainObject(document) || typeof document.uri !== 'string' || typeof document.language !== 'string' || typeof document.revision !== 'string' || typeof document.source !== 'string' || !DIGEST.test(document.digest)) {
      throw new TypeError('each document requires uri, language, revision, digest, and source custody');
    }
  }
  if (!plainObject(value.limits)) throw new TypeError('limits must be an object');
  positiveInteger(value.limits.max_observations, 'max_observations', LIMITS.max_observations);
  positiveInteger(value.limits.timeout_ms, 'timeout_ms', LIMITS.max_timeout_ms);
  return Object.freeze({ ...value, relation_kinds: relationKinds });
}

function analyzerMetadata(analyzer) {
  if (!plainObject(analyzer) || typeof analyzer.id !== 'string' || analyzer.id.length === 0 || typeof analyzer.analyze !== 'function') {
    throw new TypeError('analyzer must expose id and analyze(request)');
  }
  return Object.freeze({
    analyzer,
    relationKinds: strings(analyzer.relationKinds, 'analyzer.relationKinds'),
    languages: strings(analyzer.languages, 'analyzer.languages'),
    frameworks: strings(analyzer.frameworks, 'analyzer.frameworks'),
  });
}

function union(records, key) {
  return [...new Set(records.flatMap((record) => record[key]))].sort();
}

function freezeMetadata(records) {
  return Object.freeze({
    identity: PROVIDER_IDENTITY,
    protocol: Object.freeze({ request_schema: REQUEST_SCHEMA, observation_schema: OBSERVATION_SCHEMA }),
    capabilities: Object.freeze({
      relation_kinds: Object.freeze(union(records, 'relationKinds')),
      languages: Object.freeze(union(records, 'languages')),
      frameworks: Object.freeze(union(records, 'frameworks')),
    }),
    limits: LIMITS,
  });
}

async function strictCollectorResponse(request, records) {
  const allowed = new Set(['schema_version', 'provider_id', 'adapter_id', 'session', 'seed', 'relations', 'document_custody', 'limits']);
  if (!plainObject(request) || request.schema_version !== 'lsp-trace.provider-collector-request.v1' || Object.keys(request).some((key) => !allowed.has(key))) {
    throw new TypeError('unsupported strict collector request');
  }
  const uri = request.seed?.uri;
  const relations = strings(request.relations, 'relations', { allowEmpty: false });
  if (request.provider_id !== PROVIDER_IDENTITY || typeof request.adapter_id !== 'string' || !uri?.startsWith('file://')) {
    throw new TypeError('strict collector identity and file seed are required');
  }
  const source = await readFile(fileURLToPath(uri), 'utf8');
  const digest = createHash('sha256').update(source).digest('hex');
  const revision = plainObject(request.document_custody?.workspace_revision) ? request.document_custody.workspace_revision : {};
  const revisionValue = revision.value || digest;
  const document = { document_id: 'original', uri, revision: revisionValue, blob: digest };
  const eligible = records.filter((record) => relations.every((kind) => record.relationKinds.includes(kind)) && record.languages.includes('glimmer-js'));
  let observations = [];
  let failure;
  let coverage = { status: 'UNKNOWN', denominator: [uri], covered: [], CoveredCount: 0 };
  if (eligible.length === 1) {
    const result = await eligible[0].analyzer.analyze({ schema: REQUEST_SCHEMA, request_id: `${request.session.session_id}:${request.session.generation}:${uri}`, operation: 'analyze', relation_kinds: relations, documents: [{ uri, language: 'glimmer-js', revision: revisionValue, digest: `sha256:${digest}`, source }], limits: { max_observations: request.limits.max_nodes || LIMITS.max_observations, timeout_ms: request.limits.request_timeout_ms || request.limits.timeout_ms || LIMITS.max_timeout_ms } });
    observations = result.observations.map((observation) => ({
      ...observation,
      supports: ['source_dependency_relation'],
      does_not_support: ['runtime_execution', 'callback_invocation', 'repaint', 'feature_identity', 'whole_source_completeness'],
    }));
    if (result.outcome === 'COMPLETE' || result.outcome === 'EMPTY') coverage = { status: 'COMPLETE_WITHIN_BOUNDS', denominator: [uri], covered: [uri], CoveredCount: 1 };
    else if (result.outcome === 'BOUNDED' || result.outcome === 'PARTIAL') coverage = { status: 'PARTIAL', denominator: [uri], covered: [], CoveredCount: 0 };
    else failure = result.outcome === 'UNAVAILABLE' ? 'ADAPTER_NOT_AVAILABLE' : 'TRANSPORT_FAILED';
  } else {
    failure = 'RELATION_NOT_SUPPORTED';
  }
  const [adapterName, adapterVersion] = request.adapter_id.split('@');
  return {
    provider: { name: 'ember-glint', version: '1' },
    protocol: { name: 'lsp-trace.provider-observations', version: '1' },
    adapter: { name: adapterName, version: adapterVersion },
    authority: 'PROVIDER_REPORTED',
    coverage,
    ...(failure ? { failure } : {}),
    documents: [{ document_id: 'original', original_uri: uri, content_sha256: digest, revision: { kind: revision.kind || 'content', value: revisionValue, blob: digest, custody: 'PROVIDER_PROVED' }, coordinates: 'ORIGINAL' }],
    observations,
    request_id: `${request.session.session_id}:${request.session.generation}:${uri}`,
  };
}

function responseFor(request, metadata, outcome, observations, coverage, analyzer) {
  const logical = {
    schema: OBSERVATION_SCHEMA,
    provider: metadata,
    request_id: request.request_id,
    outcome,
    analyzer,
    observations,
    coverage,
  };
  return Object.freeze({ ...logical, logical_digest: logicalDigest(logical) });
}

export function createProvider({ analyzers = [] } = {}) {
  if (!Array.isArray(analyzers)) throw new TypeError('analyzers must be an array');
  const records = analyzers.map(analyzerMetadata);
  const metadata = freezeMetadata(records);
  return Object.freeze({
    metadata,
    async handle(input) {
      if (input?.schema_version === 'lsp-trace.provider-collector-request.v1') return strictCollectorResponse(input, records);
      const request = validateRequest(input);
      const eligible = records.filter((record) => (
        request.relation_kinds.every((kind) => record.relationKinds.includes(kind)) &&
        request.documents.every((document) => record.languages.includes(document.language))
      ));
      if (eligible.length === 0) {
        return responseFor(request, metadata, 'UNAVAILABLE', [], Object.freeze({ status: 'UNKNOWN', reason: 'NO_QUALIFIED_ANALYZER' }), null);
      }
      if (eligible.length > 1) {
        return responseFor(request, metadata, 'BLOCKED', [], Object.freeze({ status: 'UNKNOWN', reason: 'AMBIGUOUS_QUALIFIED_ANALYZER' }), null);
      }
      const selected = eligible[0];
      const controller = new AbortController();
      let timer;
      const analysis = Promise.resolve()
        .then(() => selected.analyzer.analyze(request, Object.freeze({ signal: controller.signal })))
        .then((value) => ({ value }), (error) => ({ error }));
      const timeout = new Promise((resolve) => {
        timer = setTimeout(() => {
          controller.abort();
          resolve({ timedOut: true });
        }, request.limits.timeout_ms);
      });
      const settled = await Promise.race([analysis, timeout]);
      clearTimeout(timer);
      if (settled.timedOut) {
        return responseFor(request, metadata, 'BLOCKED', [], Object.freeze({ status: 'UNKNOWN', reason: 'ANALYZER_TIMEOUT' }), selected.analyzer.id);
      }
      if (settled.error) {
        const error = settled.error;
        return responseFor(request, metadata, 'FAILED', [], Object.freeze({ status: 'UNKNOWN', reason: 'ANALYZER_FAILED', message: error instanceof Error ? error.message : String(error) }), selected.analyzer.id);
      }
      const result = settled.value;
      if (!plainObject(result) || !OUTCOMES.has(result.outcome) || !Array.isArray(result.observations) || !plainObject(result.coverage)) {
        return responseFor(request, metadata, 'FAILED', [], Object.freeze({ status: 'UNKNOWN', reason: 'INVALID_ANALYZER_RESULT' }), selected.analyzer.id);
      }
      const ordered = [...result.observations].map(canonicalize).sort((a, b) => canonicalBytes(a).compare(canonicalBytes(b)));
      const limit = request.limits.max_observations;
      const bounded = ordered.slice(0, limit);
      const outcome = ordered.length > limit ? 'BOUNDED' : result.outcome;
      const coverage = ordered.length > limit
        ? Object.freeze({ ...canonicalize(result.coverage), status: 'BOUNDED', omitted: ordered.length - limit })
        : canonicalize(result.coverage);
      return responseFor(request, metadata, outcome, bounded, coverage, selected.analyzer.id);
    },
  });
}
