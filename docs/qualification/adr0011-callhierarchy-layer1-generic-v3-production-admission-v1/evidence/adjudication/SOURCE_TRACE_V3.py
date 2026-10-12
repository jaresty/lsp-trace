import csv
import hashlib
import json
from pathlib import Path

OUT = Path('/tmp/adr0011-layer1-generic-v3-c15-adjudication-Pjut6V')
EVIDENCE = Path('/tmp/adr0011-layer1-generic-v3-c15-enum-evidence-ywmvWy')
V2 = json.loads((OUT / 'ADJUDICATION_V2.json').read_text())
rows = V2['rows']
assert len(rows) == 33 and len({r['id'] for r in rows}) == 33
by_id = {r['id']: r for r in rows}

NOACC = ('No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. '
         'finishAccepted explicitly emits the reservations ledger dimension as available known-zero '
         '(before=after=delta=0); under the accepted C15 semantics this is non-custodial generic '
         'observation storage, so no C15 byte reservation/release symbol applies.')

# V2 swapped accounting and owner in these eight independently distinct owned-method-pair proofs.
owner_by_id = {
    'alloc.owned.method.pair.copycommitments.builtin-append.append.1': 'copyCommitments return slice local/result.',
    'alloc.owned.method.pair.copyframebytes.builtin-append.append.1': 'request copyFrameBytes method return slice.',
    'alloc.owned.method.pair.copyframebytes.builtin-append.append.2': 'response copyFrameBytes method return slice.',
    'alloc.owned.method.pair.copyparamsbytes.builtin-append.append.1': 'copyParamsBytes method return slice.',
    'alloc.owned.method.pair.copyresultbytes.builtin-append.append.1': 'copyResultBytes method return slice.',
    'alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.builtin-append.append.1': 'constructor local owned, then returned request observation field frameBytes.',
    'alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.builtin-append.append.1': 'constructor local owned, then returned response observation field frameBytes.',
    'alloc.owned.method.pair.privatecallhierarchylayer1withpredecessors.builtin-append.append.1': 'new callHierarchyOpaqueCommitmentList stored in the returned attachment field predecessors.',
}
for site_id, owner in owner_by_id.items():
    row = by_id[site_id]
    assert row['classification'] == 'NECESSARY'
    assert row['initial_owner'] == NOACC
    row['accounting_reservation_rule'] = NOACC
    row['initial_owner'] = owner

# These are exact pre-existing expressions whose ordinals shifted in the accepted patch.
lifecycle = {
    'alloc.sessionruntime.runlifecycle.builtin-make.make.3': {
        'expr': 'make(map[lspwire.RequestKey]*Request)',
        'field': 'r.requests',
        'kind': 'requests map',
    },
    'alloc.sessionruntime.runlifecycle.builtin-make.make.4': {
        'expr': 'make(map[string]openDocument)',
        'field': 'r.documents',
        'kind': 'documents map',
    },
}
for site_id, exact in lifecycle.items():
    row = by_id[site_id]
    assert row['classification'] == 'REUSE'
    row['exact_expression_or_input'] = exact['expr'] + ' assigned to restarted runtimeSession field ' + exact['field']
    row['allocation_and_nonempty_behavior'] = ('Unconditional when the successful RESTART assignment executes; the empty map value is newly '
        'constructed for the restarted runtimeSession. REUSE is retained only because this exact expression '
        'and field assignment pre-exist base ce4a1838f8a10bafc13429031e55df7e4dd812c6; the patch only shifts its enumeration ordinal.')
    row['accounting_reservation_rule'] = ('Unchanged baseline restarted-runtimeSession allocation; no generic-observation '
        'privateB4ByteAccountV2 reservation or lease is introduced.')
    row['initial_owner'] = 'Restarted runtimeSession field ' + exact['field'] + ' (' + exact['kind'] + ').'
    row['transfer_custody_chain'] = ('fresh ' + exact['kind'] + ' -> assignment to restarted runtimeSession field ' + exact['field'] +
        ' -> runtime request/document lifecycle operations.')
    row['natural_unreachability_release_retirement'] = ('The map remains reachable through the restarted runtimeSession until that session is stopped, '
        'restarted again, shut down, or otherwise becomes unreachable under the pre-existing runtimeSession lifecycle.')
    row['refusal_terminal_applicability'] = ('Generic observation refusal and owner terminalization are inapplicable; this is pre-existing '
        'RESTART initialization of runtimeSession state.')
    row['concurrency_exact_once_applicability'] = ('The assignment occurs in the pre-existing lifecycle restart path; this adjudication does not '
        'infer additional synchronization or exact-once behavior from the allocation expression.')
    row['copy_allocation_necessity_or_reuse'] = ('REUSE only in delta-adjudication terms: the exact allocation expression and ownership field '
        'pre-exist the base. The expression still constructs a fresh empty map when RESTART executes.')

# A capacity-growing occurrence allocates backing required by the registry's retained pointer set.
pub_id = 'alloc.b4.owned.method.pair.lease.publishcallhierarchylayer1genericownerlocked.builtin-append.append.1'
pub = by_id[pub_id]
pub['classification'] = 'NECESSARY'
pub['allocation_and_nonempty_behavior'] = ('Only after owner != nil and the manager is neither closed nor generation-retired. Allocation is '
    'capacity-dependent: append reuses backing when len < cap and allocates replacement backing when growth is required. '
    'No capacity invariant or preallocation exists, so this does not claim every append allocates.')
pub['initial_owner'] = ('On a capacity-growing occurrence, manager registry field m.callHierarchyLayer1GenericOwners owns the '
    'replacement backing after assignment; without growth it continues to own the existing backing.')
pub['transfer_custody_chain'] = ('owner pointer -> append result assigned to manager registry field '
    'm.callHierarchyLayer1GenericOwners under m.mu -> detached worklist on STOP/RESTART or Shutdown local -> '
    'requestTerminal/waitReleased for the pointed owner.')
pub['natural_unreachability_release_retirement'] = ('On growth, the superseded backing becomes naturally unreachable after assignment. The active '
    'registry backing becomes naturally unreachable when STOP/RESTART detach or Shutdown clears/transfers the registry; '
    'explicit owner Release does not remove the registry entry.')
pub['refusal_terminal_applicability'] = ('Closed or generation-retired publication skips append and returns the owner for synchronous '
    'terminalization. Admitted publication retains the pointer in the Manager registry; explicit Release alone does not remove it.')
pub['concurrency_exact_once_applicability'] = ('m.mu serializes registry append, detach, and Shutdown transfer. Owner terminal exact-once '
    'behavior governs the pointed owner, not whether this append grows its backing.')
pub['copy_allocation_necessity_or_reuse'] = ('NECESSARY on capacity-growing occurrences: replacement backing is required to retain the complete '
    'Manager registry pointer set after append. Because no capacity invariant or preallocation exists, growth occurrence '
    'frequency is unproved and no claim is made that every append allocates.')
pub['evidence_ceiling'] = (pub['evidence_ceiling'] + ' For this append, the evidence proves the conditional growth rule and absence of a '
    'capacity guarantee; it does not prove which runtime invocations grow or how often growth occurs.')

counts = {k: sum(r['classification'] == k for r in rows) for k in ('NECESSARY', 'REUSE', 'UNRESOLVED')}
assert counts == {'NECESSARY': 29, 'REUSE': 4, 'UNRESOLVED': 0}, counts

# The accounting universe is exactly the union of the three delta TSV candidate_id columns.
universe = []
delta_artifacts = ['ADDED.tsv', 'CHANGED.tsv', 'REMOVED.tsv']
for name in delta_artifacts:
    with (EVIDENCE / name).open(newline='') as f:
        universe.extend(record['candidate_id'] for record in csv.DictReader(f, delimiter='\t'))
assert len(universe) == 352 and len(set(universe)) == 352
selected = {r['id'] for r in rows}
complement = sorted(set(universe) - selected)
assert len(selected) == 33 and len(complement) == 319
assert not selected.intersection(complement)
assert selected.union(complement) == set(universe)
canonical = ('\n'.join(complement) + '\n').encode()
complement_hash = hashlib.sha256(canonical).hexdigest()
assert complement_hash == V2['fp_complement_proof']['complement_sha256']
assert complement == V2['fp_complement_proof']['complement_ids']

out = {
    'schema': 'lsp-trace/adr0011-c15-semantic-adjudication-v3',
    'scope': 'Exact 33 blocker IDs, corrected source-site adjudication with delta-universe accounting.',
    'inputs': V2['inputs'],
    'review_context': {
        'predecessor': 'ADJUDICATION_V2.json',
        'corrections': [
            'Restored NOACC to accounting_reservation_rule and exact site owner to initial_owner for eight NECESSARY rows.',
            'Corrected RESTART make.3/make.4 to the exact requests/documents map expressions, fields, owners, and transfers; retained REUSE only because the expressions pre-exist base.',
            'Corrected complement-universe provenance to ADDED.tsv + CHANGED.tsv + REMOVED.tsv.',
            'Adjudicated publication append NECESSARY for capacity-growing occurrences without claiming every append allocates.',
        ],
    },
    'counts': {'total': 33, **counts},
    'go_rule': 'Propose C15_DELTA_GO only when all 33 exact blocker IDs are adjudicated, UNRESOLVED == 0, and the exact delta-universe complement has 319 IDs.',
    'c15_delta_go': True,
    'disposition': 'C15_DELTA_GO',
    'exact33_proof': V2['exact33_proof'],
    'fp_complement_proof': {
        'universe_artifacts': delta_artifacts,
        'universe_definition': 'candidate_id union from ADDED.tsv, CHANGED.tsv, and REMOVED.tsv only',
        'universe_count': len(universe),
        'universe_unique_count': len(set(universe)),
        'selected_count': len(selected),
        'selected_unique_count': len(selected),
        'complement_count': len(complement),
        'complement_unique_count': len(set(complement)),
        'intersection_count': len(selected.intersection(complement)),
        'union_count': len(selected.union(complement)),
        'union_equals_universe': selected.union(complement) == set(universe),
        'complement_canonicalization': 'UTF-8, lexicographically sorted IDs, one LF after every ID including final',
        'complement_sha256': complement_hash,
        'complement_ids': complement,
    },
    'rows': rows,
}
(OUT / 'ADJUDICATION_V3.json').write_text(json.dumps(out, indent=2) + '\n')

md = [
    '# ADR-0011 C15 exact33 source-site adjudication V3', '',
    f"- Schema: `{out['schema']}`",
    f"- Counts: total=33 NECESSARY={counts['NECESSARY']} REUSE={counts['REUSE']} UNRESOLVED={counts['UNRESOLVED']}",
    f"- Disposition: `{out['disposition']}`",
    '- `C15_DELTA_GO`: proposed because all 33 are complete, unresolved is zero, and the exact complement is 319.', '',
    '## Independent 352 delta accounting', '',
    '- Universe provenance: `ADDED.tsv` + `CHANGED.tsv` + `REMOVED.tsv` candidate IDs (not `all-source-locations.tsv`).',
    '- Universe: 352 unique IDs.',
    '- Exact adjudicated set: 33 unique IDs.',
    '- FP complement: 319 unique IDs.',
    '- Intersection(exact33, complement): 0.',
    '- Union count: 352; union equals universe: `true`.',
    f'- Canonical FP complement SHA-256: `{complement_hash}` (sorted IDs, LF-terminated).', '',
    '## V3 corrections', '',
    *['- ' + item for item in out['review_context']['corrections']], '',
    '## Evidence ceiling', '', rows[0]['evidence_ceiling'], '',
    '## Rows', ''
]
for number, r in enumerate(rows, 1):
    md.extend([
        f"### {number}. `{r['id']}` — {r['classification']}",
        f"- Source: `{r['source']['path']}:{r['source']['span']}`; AST `{r['source']['ast_hash']}` ({r['source']['ast_kind']})",
        f"- Exact expression/input: {r['exact_expression_or_input']}",
        f"- Allocation/nonempty behavior: {r['allocation_and_nonempty_behavior']}",
        f"- Accounting/reservation: {r['accounting_reservation_rule']}",
        f"- Initial owner: {r['initial_owner']}",
        f"- Transfer/custody: {r['transfer_custody_chain']}",
        f"- Natural release/retirement: {r['natural_unreachability_release_retirement']}",
        f"- Refusal/terminal: {r['refusal_terminal_applicability']}",
        f"- Concurrency/exact-once: {r['concurrency_exact_once_applicability']}",
        f"- Necessity/reuse reason: {r['copy_allocation_necessity_or_reuse']}",
        f"- Evidence ceiling: {r['evidence_ceiling']}", ''
    ])
md.extend(['## FP complement IDs', '', f'Canonical hash: `{complement_hash}`', '', '```text', *complement, '```', ''])
(OUT / 'ADJUDICATION_V3.md').write_text('\n'.join(md))
print(json.dumps({'counts': out['counts'], 'disposition': out['disposition'], 'complement_count': len(complement), 'complement_sha256': complement_hash}))
