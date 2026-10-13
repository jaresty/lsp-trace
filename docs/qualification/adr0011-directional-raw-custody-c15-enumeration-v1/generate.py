#!/usr/bin/env python3
import csv, hashlib, json, os, pathlib, struct
ROOT=pathlib.Path(__file__).resolve().parents[3]
OUT=pathlib.Path(__file__).resolve().parent
HEAD='a1cd77950ffa45ebed52310878f772f3c7344756'
FIELDS='row_id revision predecessor_row_id predecessor_revision status source_path source_start_line source_end_line source_sha256 symbol operation_kind component attempt_scope account_scope backing_identity_rule source_lease_identity_rule destination_lease_identity_rule capacity_rule reservation_site allocation_site release_site borrow_alias_site transfer_site terminal_paths baseline_row_id delta provisional_class contrary_evidence rejection_condition authority_ceiling adjudicator_id adjudication_revision notes'.split()
SIX=['sessionruntime/call_hierarchy_directional_raw_v1_private.go','sessionruntime/sessionruntime.go','internal/lspwire/read_frame.go','internal/lspwire/successor_ingress.go','sessionruntime/b4_owned_method_pair_lease.go','sessionruntime/b4_c15_ownership_registry_private.go']
COMP=['PREPARE_FRAME','PREPARE_RESULT','SELECTED_HANDOFF','DIRECTIONAL_FRAME','DIRECTIONAL_RESULT','RETAINED_EXTERNAL_EVIDENCE']
# Exact current-source retained copies relevant to frame/result/handoff/external custody.
BASE=[
('internal/lspwire/read_frame.go',76,'ReadFrame','COPY','PREPARE_FRAME','append([]byte(nil), capture.raw...)'),
('internal/lspwire/read_frame.go',102,'ReadFrameBounded','COPY','PREPARE_FRAME','append([]byte(nil), capture.raw...)'),
('internal/lspwire/successor_ingress.go',543,'(*SuccessorIngressReader).consumeCopy','COPY','PREPARE_FRAME','append([]byte(nil), r.prefetch[:n]...)'),
('internal/lspwire/successor_ingress.go',581,'(*SuccessorIngressReader).ReadFrame','ALLOCATE','PREPARE_FRAME','make([]byte, 0, headerCapacity)'),
('internal/lspwire/successor_ingress.go',650,'(*SuccessorIngressReader).ReadFrame','ALLOCATE','PREPARE_FRAME','make([]byte, 0, length)'),
('internal/lspwire/successor_ingress.go',700,'(*SuccessorIngressReader).ReadFrame','ALLOCATE','DIRECTIONAL_FRAME','make([]byte, 0, frameCapacity)'),
('internal/lspwire/successor_ingress.go',753,'(*SuccessorIngressReader).ReadFrame','COPY','PREPARE_RESULT','owned-capture append([]byte(nil), body[start:end]...)'),
('internal/lspwire/successor_ingress.go',755,'(*SuccessorIngressReader).ReadFrame','COPY','PREPARE_RESULT','legacy-capture append([]byte(nil), body[start:end]...)'),
('internal/lspwire/successor_ingress.go',795,'CloneSuccessorMessageOwned','COPY','DIRECTIONAL_RESULT','copy(dst, src)'),
('sessionruntime/b4_owned_method_pair_lease.go',83,'newPrivateB4OwnedMethodPairOwner','COPY','RETAINED_EXTERNAL_EVIDENCE','pair.Params = append([]byte(nil), source.Params...)'),
('sessionruntime/b4_owned_method_pair_lease.go',85,'newPrivateB4OwnedMethodPairOwner','COPY','RETAINED_EXTERNAL_EVIDENCE','pair.Result = append([]byte(nil), source.Result...)'),
('sessionruntime/b4_owned_method_pair_lease.go',269,'copyCallHierarchyObservation','COPY','PREPARE_FRAME','request.frameBytes = append([]byte(nil), source.request.frameBytes...)'),
('sessionruntime/b4_owned_method_pair_lease.go',274,'copyCallHierarchyObservation','COPY','DIRECTIONAL_FRAME','response.frameBytes = append([]byte(nil), source.response.frameBytes...)'),
('sessionruntime/b4_owned_method_pair_lease.go',279,'copyCallHierarchyObservation','COPY','PREPARE_RESULT','selected.exactBytes = append([]byte(nil), source.selected.exactBytes...)'),
('sessionruntime/b4_owned_method_pair_lease.go',284,'copyCallHierarchyObservation','COPY','SELECTED_HANDOFF','handoff.selected.exactBytes = append([]byte(nil), source.handoff.selected.exactBytes...)'),
('sessionruntime/b4_owned_method_pair_lease.go',285,'copyCallHierarchyObservation','COPY','SELECTED_HANDOFF','handoff.successorBytes = append([]byte(nil), source.handoff.successorBytes...)'),
('sessionruntime/b4_owned_method_pair_lease.go',559,'callHierarchyGenericFrame','COPY','PREPARE_FRAME','append([]byte(nil), raw...)'),
('sessionruntime/b4_owned_method_pair_lease.go',585,'copyGenericObservation','COPY','RETAINED_EXTERNAL_EVIDENCE','o.error.data = append([]byte(nil), o.error.data...)'),
('sessionruntime/b4_owned_method_pair_lease.go',586,'copyGenericObservation','COPY','RETAINED_EXTERNAL_EVIDENCE','o.error.serializedBytes = append([]byte(nil), o.error.serializedBytes...)'),
('sessionruntime/b4_owned_method_pair_lease.go',587,'copyGenericObservation','COPY','RETAINED_EXTERNAL_EVIDENCE','o.external.notificationsBytes = append([]byte(nil), o.external.notificationsBytes...)'),
('sessionruntime/b4_owned_method_pair_lease.go',588,'copyGenericObservation','COPY','RETAINED_EXTERNAL_EVIDENCE','o.external.responsesBytes = append([]byte(nil), o.external.responsesBytes...)'),
('sessionruntime/b4_owned_method_pair_lease.go',589,'copyGenericObservation','COPY','PREPARE_FRAME','o.frames.request.frameBytes = append([]byte(nil), o.frames.request.frameBytes...)'),
('sessionruntime/b4_owned_method_pair_lease.go',590,'copyGenericObservation','COPY','DIRECTIONAL_FRAME','o.frames.response.frameBytes = append([]byte(nil), o.frames.response.frameBytes...)'),
('sessionruntime/b4_owned_method_pair_lease.go',591,'copyGenericObservation','COPY','DIRECTIONAL_RESULT','o.result.bytes = append(json.RawMessage(nil), o.result.bytes...)'),
('sessionruntime/b4_owned_method_pair_lease.go',782,'(*callHierarchyLayer1GenericAttemptLineage).finishAccepted','COPY','RETAINED_EXTERNAL_EVIDENCE','responsesBytes: append([]byte(nil), body...)'),
('sessionruntime/b4_owned_method_pair_lease.go',788,'(*callHierarchyLayer1GenericAttemptLineage).finishAccepted','COPY','RETAINED_EXTERNAL_EVIDENCE','data: append([]byte(nil), e.Data...)'),
]
ALIASES=[
('sessionruntime/b4_owned_method_pair_lease.go',112,'OwnedMethodPairLease.WithPair','BORROW','RETAINED_EXTERNAL_EVIDENCE','callback view aliases owner pair'),
('sessionruntime/b4_owned_method_pair_lease.go',311,'callHierarchyObservationLease.withObservation','BORROW','RETAINED_EXTERNAL_EVIDENCE','callback observation view'),
('sessionruntime/b4_owned_method_pair_lease.go',639,'callHierarchyLayer1GenericLease.Callback','BORROW','RETAINED_EXTERNAL_EVIDENCE','callback generic view'),
]
TERMINALS=[
('internal/lspwire/successor_ingress.go',358,'(*SuccessorIngressReader).Close','RELEASE','PREPARE_FRAME','reader close releases reader-owned allocations'),
('sessionruntime/b4_owned_method_pair_lease.go',146,'OwnedMethodPairLease.Release','RELEASE','RETAINED_EXTERNAL_EVIDENCE','lease release clears pair and charges'),
('sessionruntime/b4_owned_method_pair_lease.go',654,'(*callHierarchyLayer1GenericOwner).releaseLocked','RELEASE','RETAINED_EXTERNAL_EVIDENCE','generic owner exact winner'),
('sessionruntime/b4_owned_method_pair_lease.go',679,'(*callHierarchyLayer1GenericOwner).requestTerminal','RELEASE','RETAINED_EXTERNAL_EVIDENCE','terminal convergence entry'),
]

def sha(path): return hashlib.sha256((ROOT/path).read_bytes()).hexdigest()
def rid(path,sym,op,comp,back):
 s=b'adr0011.c15.lease-source.v2.row\0'+path.encode()+b'\0'+sym.encode()+b'\0'+op.encode()+b'\0'+comp.encode()+b'\0'+back.encode()
 return hashlib.sha256(s).hexdigest()
def row(path,line,sym,op,comp,back,delta='UNCHANGED',base=''):
 present=(ROOT/path).exists(); h=sha(path) if present else 'ABSENT_REQUIRED_NEW'
 r={k:'' for k in FIELDS}; r.update(row_id=rid(path,sym,op,comp,back),revision='1',predecessor_row_id='NULL',predecessor_revision='0',status='UNRESOLVED',source_path=path,source_start_line=str(line),source_end_line=str(line),source_sha256=h,symbol=sym,operation_kind=op,component=comp,attempt_scope='incoming|outgoing exact attempt UNRESOLVED',account_scope='Manager accounting domain UNRESOLVED',backing_identity_rule=back,source_lease_identity_rule='UNRESOLVED',destination_lease_identity_rule='UNRESOLVED',capacity_rule='exact retained capacity; expression requires independent adjudication',reservation_site='UNRESOLVED',allocation_site=f'{path}:{line}',release_site='UNRESOLVED',borrow_alias_site=f'{path}:{line}' if op in ('BORROW','ALIAS') else '',transfer_site=f'{path}:{line}' if op=='TRANSFER' else '',terminal_paths='normal|empty|malformed|protocol|transport|timeout|cancel|refusal|STOP|RESTART|Shutdown|stale|race; mapping UNRESOLVED',baseline_row_id=base,delta=delta,provisional_class='UNRESOLVED',contrary_evidence='Independent reviewer must challenge hidden copy, alias escape, lease mismatch, release gap, and identity pairing.',rejection_condition='Reject if source bytes/site/operation/component/identity or lifecycle mapping is incorrect or incomplete.',authority_ceiling='STRUCTURAL_SOURCE_LOCATOR_ONLY; authority=0; no semantic acceptance or implementation authority',adjudicator_id='',adjudication_revision='',notes='mechanically enumerated; independent row adjudication required')
 return r
rows=[]
for x in BASE+ALIASES+TERMINALS: rows.append(row(*x))
# Proposed successor relationships are exact architecture requirements, but source remains absent.
for c in COMP:
 for op,back in [('ALLOCATE',f'proposed owned {c} exact backing'),('ALIAS',f'proposed zero-charge bounded alias of {c} owning lease')]:
  rows.append(row(SIX[0],0,f'proposed.{c.lower()}.{op.lower()}',op,c,back,'ADDED'))
rows.append(row(SIX[0],0,'proposed.transferDirectionalAcquisitionV2','TRANSFER','SELECTED_HANDOFF','atomic bytes plus complete lease-set transfer','ADDED'))
rows.append(row(SIX[0],0,'proposed.releaseDirectionalTerminalV2','RELEASE','RETAINED_EXTERNAL_EVIDENCE','single terminal convergence release','ADDED'))
rows.sort(key=lambda r:(r['delta'],r['source_path'],int(r['source_start_line']),r['operation_kind'],r['component']))

def write_tsv(name, selected):
 with (OUT/name).open('w',newline='') as f:
  w=csv.DictWriter(f,FIELDS,delimiter='\t',lineterminator='\n'); w.writeheader(); w.writerows(selected)
write_tsv('BASELINE_PRIOR_UNCHARGED.tsv',[r for r in rows if r['delta']=='UNCHANGED'])
for d in ['ADDED','CHANGED','REMOVED','UNCHANGED']: write_tsv(d+'.tsv',[r for r in rows if r['delta']==d])
write_tsv('ALLOCATION_SITES.tsv',[r for r in rows if r['operation_kind'] in ('ALLOCATE','COPY','RETAIN','PUBLISH')])
write_tsv('RELEASE_SITES.tsv',[r for r in rows if r['operation_kind']=='RELEASE'])
write_tsv('BORROW_ALIAS_SITES.tsv',[r for r in rows if r['operation_kind'] in ('BORROW','ALIAS')])
write_tsv('TRANSFER_SITES.tsv',[r for r in rows if r['operation_kind']=='TRANSFER'])
write_tsv('UNRESOLVED.tsv',rows)
with (OUT/'SOURCE_LOCATIONS.tsv').open('w',newline='') as f:
 w=csv.writer(f,delimiter='\t',lineterminator='\n'); w.writerow(['source_path','line','symbol','operation_kind','component','row_id']); [w.writerow([r['source_path'],r['source_start_line'],r['symbol'],r['operation_kind'],r['component'],r['row_id']]) for r in rows]
# Exact source manifest for six paths and accepted inputs.
inputs=SIX+['docs/qualification/adr0011-c15-allocation-census.provisional.md','docs/qualification/adr0011-directional-raw-custody-c15-lease-source-v2-go-v1/C15_ENUMERATION_PLAN.md','docs/qualification/adr0011-directional-raw-custody-c15-lease-source-v2-go-v1/ARCHITECTURE.md','docs/qualification/adr0011-directional-raw-custody-c15-lease-source-v2-go-v1/WRITESET.md']
with (OUT/'SOURCE_MANIFEST.sha256').open('w') as f:
 f.write(f'# HEAD {HEAD}\n')
 for p in inputs:
  q=ROOT/p
  f.write((f'{sha(p)}  {q.stat().st_size}  {p}' if q.exists() else f'ABSENT_REQUIRED_NEW  0  {p}')+'\n')
counts={d:sum(r['delta']==d for r in rows) for d in ['ADDED','CHANGED','REMOVED','UNCHANGED']}; counts.update(total_rows=len(rows),baseline_rows=sum(r['delta']=='UNCHANGED' for r in rows),current_candidate_rows=len(rows),unresolved=len(rows),duplicates=len(rows)-len({r['row_id'] for r in rows}),lineage_cycles=0,missing_predecessors=0,branches=0,genesis_errors=sum(not(r['revision']=='1' and r['predecessor_row_id']=='NULL' and r['predecessor_revision']=='0') for r in rows),status='ENUMERATION_COMPLETE_NOT_ADJUDICATED',authority=0)
(OUT/'COUNTS.json').write_text(json.dumps(counts,indent=2,sort_keys=True)+'\n')
# Independent assignment intentionally contains every unresolved row.
write_tsv('INDEPENDENT_ROW_ADJUDICATION_ASSIGNMENT.tsv',rows)
# Hash generated outputs except run manifest itself.
files=sorted(p for p in OUT.iterdir() if p.is_file() and p.name!='RUN_MANIFEST.sha256')
with (OUT/'RUN_MANIFEST.sha256').open('w') as f:
 for p in files: f.write(f'{hashlib.sha256(p.read_bytes()).hexdigest()}  {p.name}\n')
print(json.dumps(counts,sort_keys=True))
