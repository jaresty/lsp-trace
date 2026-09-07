#!/usr/bin/env python3
"""Independent stdlib-only table oracle; no source reads and no admission claim.
Input: exact retained JSON on stdin. Output: metrics artifact using the v1 policy.
--vectors emits fixed hash vectors over deliberately non-admissible tiny inputs.
"""
import base64
import hashlib
import json
import sys
from collections import Counter

VERSION = 'lsp-trace.bounded-retained-metrics.v1'
POLICY = 'retained-CALLS-unit-group-structural-metrics/v1'

def canonical(value):
    text = json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=False)
    for char in '<>&\u2028\u2029':
        text = text.replace(char, '\\u%04x' % ord(char))
    return text.encode('utf-8')

def digest(domain, value):
    return 'sha256:' + hashlib.sha256(domain.encode() + b'\0' + canonical(value)).hexdigest()

def basis(raw):
    return digest(VERSION + ':basis', {'policy': POLICY, 'parameters': {}, 'input_bytes': base64.b64encode(raw).decode()})

def metrics(raw, tables):
    groups = tables['groups']
    ids = sorted(node['id'] for node in tables['endpoints'])
    rows = []
    for node in ids:
        incoming = [g['caller_node_id'] for g in groups if g['callee_node_id'] == node]
        outgoing = [g['callee_node_id'] for g in groups if g['caller_node_id'] == node]
        rows.append(dict(id=node, in_group_degree=len(incoming), out_group_degree=len(outgoing),
                         in_distinct_neighbors=len(set(incoming)), out_distinct_neighbors=len(set(outgoing))))
    n = len(ids)
    q = len({(g['caller_node_id'], g['callee_node_id']) for g in groups if g['caller_node_id'] != g['callee_node_id']})
    result = dict(schema_version=VERSION, policy=POLICY, scope='HISTORICAL_ARTIFACT_SCOPED_UNVERIFIED_INCOMPLETE',
                  input_bytes=base64.b64encode(raw).decode(), parameters={}, basis_digest=basis(raw),
                  status='COMPUTED_OVER_RETAINED_GRAPH', node_count=n, group_count=len(groups),
                  reported_occurrence_count=len(tables['occurrences']),
                  unreported_group_count=sum(not g['occurrence_ids'] for g in groups),
                  self_loop_group_count=sum(g['caller_node_id'] == g['callee_node_id'] for g in groups),
                  distinct_nonloop_pair_count=q, nodes=rows, digest='',
                  density=dict(policy='DIRECTED_DISTINCT_NONLOOP_PAIRS', status='DEFINED' if n >= 2 else 'UNDEFINED_DENOMINATOR',
                               value=dict(numerator=q, denominator=n*(n-1)) if n >= 2 else None))
    for direction in ('in', 'out'):
        result[direction+'_degree_histogram'] = [dict(degree=d, node_count=count) for d, count in sorted(Counter(row[direction+'_group_degree'] for row in rows).items())]
    result['digest'] = digest(VERSION+':result', result)
    return result

if __name__ == '__main__':
    if sys.argv[1:] == ['--vectors']:
        vectors = []
        for raw in (b'{}', b'{}\n'):
            result = metrics(raw, dict(endpoints=[], groups=[], occurrences=[]))
            vectors.append(dict(input=raw.decode(), basis=result['basis_digest'], digest=result['digest']))
        print(json.dumps(vectors, indent=2))
    else:
        raw = sys.stdin.buffer.read()
        print(canonical(metrics(raw, json.loads(raw)['tables'])).decode())
