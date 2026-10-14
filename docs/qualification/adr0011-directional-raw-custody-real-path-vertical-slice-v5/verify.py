#!/usr/bin/env python3
"""ADR0011_V5_REFRAME: exactly four independent causal witnesses; never GO."""
import argparse,hashlib,json,os,pathlib,re,subprocess,tempfile
ROOT=pathlib.Path(__file__).resolve().parent
BASE='04a4ca85ffbfd3916010897fb6fcc193ba47252d'
CAMPAIGN_HEAD='49334ce23de37c0094f50d21d57ab251a46130ca'
WITNESSES={
 'EXACT32':'TestV5CausalExact32MiB',
 'PLUS_ONE':'TestV5CausalOver32MiBPreEffect',
 'PARTIAL_EXTERNAL':'TestV5CausalPartialExternalFailure',
 'EARLY_TERMINAL':'TestV5CausalEarlyTerminalOnce'}
def sha(raw):return hashlib.sha256(raw).hexdigest()
def require(value,message):
 if not value:raise RuntimeError(message)
def run(args,cwd=None,env=None,timeout=120):
 p=subprocess.run(args,cwd=cwd,env={**os.environ,'GOWORK':'off',**(env or {})},stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True,timeout=timeout)
 return p.returncode,p.stdout
def ordinary_roundtrip(source):
 return re.search(r'^func \(m \*Manager\) RoundTrip\(.*?^}',source,re.M|re.S).group(0)
def main():
 ap=argparse.ArgumentParser();ap.add_argument('--source',default='/private/tmp/adr0011-incoming-calls-v1-6yiY4m');args=ap.parse_args()
 outdir=ROOT/'verification';outdir.mkdir(exist_ok=True)
 result={'status':'NOT_VERIFIED','binding':'ADR0011_V5_REFRAME','production_go':False,'base':BASE,'witnesses':[],'mutations':[]}
 previous_result_path=outdir/'RESULT.json'
 previous_result_raw=previous_result_path.read_bytes() if previous_result_path.exists() else None
 previous_result=json.loads(previous_result_raw) if previous_result_raw else None
 previous_result_sha256=sha(previous_result_raw) if previous_result_raw else None
 def checked(name,argv,cwd=None):
  code,out=run(argv,cwd);(outdir/(name+'.log')).write_text(out);require(code==0,name+' failed');return out
 def witness(key,fresh,suffix=''):
  filename=key+suffix;receipt=outdir/(filename+'.json');receipt.unlink(missing_ok=True)
  code,out=run(['go','test','./sessionruntime','-run','^'+WITNESSES[key]+'$','-count=1','-timeout','30s','-v'],fresh,{'ADR0011_V5_CAUSAL_RECEIPT':str(receipt)})
  (outdir/(filename+'.log')).write_text(out)
  require(code==0 and 'ASSERT_C4_'+key+' PASS' in out,'witness failed: '+filename)
  data=json.loads(receipt.read_text());require(data['witness']==key and data['production_go'] is False,'wrong witness identity')
  require(data['entry']=='incomingops.SessionClient.call -> private Runtime -> actual Manager -> shared transport','not production executor-client entry')
  require(data['release']['terminal_wins']==1 and data['release']['live_after']==0 and data['release']['generic_released'],'release not observed')
  require(data['facts']['source_ordinal']==1,'provider ordinal absent')
  require(any(o['selected'] and o['source_ordinal']==1 for o in data['observations']),'source selection observation absent')
  for o in data['observations']:require(o['length']>0 and len(o['sha256'])==64 and o['generation']>0 and o['request_id']>0,'incomplete exact-byte observation')
  if key=='EXACT32':
   facts=data['facts'];require(facts['accepted'] and data['release']['live_before']==32<<20 and data['release']['retained_allocations']==2,'wrong equality boundary')
   frames=facts['actual_frames'];boundaries=facts['allocation_boundaries'];observed=[o for o in data['observations'] if o['component'] in {'request-frame','response-frame'}]
   fields=('component','request_id','length','sha256')
   require(len(frames)==len(observed)==len(boundaries)==2 and [f['component'] for f in frames]==['request-frame','response-frame'] and [tuple(o[f] for f in fields) for o in observed]==[tuple(o[f] for f in fields) for o in frames],'EXACT32 incomplete request/response correspondence')
   total=0
   for i,(f,b) in enumerate(zip(frames,boundaries),1):
    total+=f['length'];require(b['component']==f['component'] and b['length']==f['length'] and b['live']==b['cumulative']==total and b['allocations']==b['ownership_objects']==i and b['occupied'],'allocation occurred without prior reservation/ownership')
   require(total==32<<20,'wrong acquired byte total')
  if key=='PLUS_ONE':
   facts=data['facts'];zero=facts['zero_wire'];reuse=facts['reuse'];reservation=facts['reservation'];refusal=facts['refusal']
   require(facts['proposed_not_acquired'] and facts['attempted_bytes']==(32<<20)+1 and facts['wire_effect_delta']==0 and data['release']['retained_allocations']==0,'not pre-effect plus-one refusal')
   require(zero['measurement']=='Manager stdin Write calls and raw bytes before provider frame decoding' and zero['write_calls']==0 and zero['raw_bytes']==0 and zero['written_bytes']==0 and zero['raw_sha256']==sha(b'') and zero['decoded_packet_delta']==0,'refusal lacks independent zero-call/zero-raw-byte transport receipt')
   require(reservation['live']==0 and reservation['cumulative']==0 and reservation['retained_allocations']==0 and reservation['occupied'] is False,'refusal charged directional reservation')
   refused=[o for o in data['observations'] if o['component']=='request-frame-reservation-refusal']
   require(len(refused)==1 and refusal['failure']==refused[0]['failure']=='RESOURCE_EXHAUSTED' and refusal['proposed_length']==refused[0]['length']==(32<<20)+1 and refusal['proposed_sha256']==refused[0]['sha256'],'refusal commitment receipt mismatch')
   require(reuse['same_client'] and reuse['same_manager'] and reuse['session_id_before']==reuse['session_id_after'] and bool(reuse['session_id_before']) and reuse['generation_before']==reuse['generation_after']==refused[0]['generation'] and reuse['ready_before'] and reuse['ready_after'] and reuse['success'] and reuse['null'] and reuse['request_id']>refused[0]['request_id'] and reuse['write_calls']>0 and reuse['raw_bytes']>0 and reuse['written_bytes']==reuse['raw_bytes'] and len(reuse['raw_sha256'])==64 and reuse['raw_sha256']==reuse['decoded_request_sha256'],'same READY session did not execute a valid subsequent request')
  if key=='PARTIAL_EXTERNAL':
   facts=data['facts'];require(facts['failed'] and facts['external_plus_partial_frames']==3 and facts['failure']=='SESSION_POISONED' and facts['refusal_failure']=='RESOURCE_EXHAUSTED' and facts['failure']!=facts['refusal_failure'],'wrong typed acquired partial/external failure')
   acquired=[o for o in data['observations'] if o['request_id']==2];require(len(acquired)==4 and all(o['failure']==facts['failure'] for o in acquired),'partial acquired observations relabeled as refusal')
  if key=='EARLY_TERMINAL':require(data['facts']['null_early_return'],'early return not observed')
  return {'name':key,'test':WITNESSES[key],'pid':data['pid'],'exit_code':code,'log_sha256':sha(out.encode()),'receipt_sha256':sha(receipt.read_bytes())}
 try:
  seal=(ROOT/'SELF_HASHES.json').read_bytes();require(sha(seal)==(ROOT/'SELF_HASHES.sha256').read_text().strip(),'seal mismatch')
  members=json.loads(seal);require(set(members)=={'union.patch','PATH_MANIFEST.json','WITNESS_CONTRACT.json','CRITICAL_EDGE_MUTATIONS.json','README.md','verify.py'},'unexpected sealed package member')
  for name,digest in members.items():require(sha((ROOT/name).read_bytes())==digest,'self hash mismatch: '+name)
  manifest=json.loads((ROOT/'PATH_MANIFEST.json').read_text());contract=json.loads((ROOT/'WITNESS_CONTRACT.json').read_text());mutations=json.loads((ROOT/'CRITICAL_EDGE_MUTATIONS.json').read_text())
  require(manifest['base']==BASE and manifest['binding']=='ADR0011_V5_REFRAME' and not manifest['production_go'],'authority mismatch')
  require(contract['witnesses']==WITNESSES and contract['fresh_process_per_witness'] and contract['broader_scenarios_deferred'],'not the four-witness contract')
  require(contract['phase2_causal_corrections']=={'allocation_boundary':'physical make; prior live/cumulative charge and ownership object per request/response','exact32_correspondence':'bijection with fixture request and response frames','partial_external_failure':'SESSION_POISONED, not RESOURCE_EXHAUSTED','admission_mutation':'skip actual Manager admission call; deterministic pre-pending missing-ownership rejection'},'Phase2 correction contract missing')
  required={'request-frame-before-reservation':'ASSERT_C4_REQUEST_ALLOCATION_BOUNDARY','response-frame-before-reservation':'ASSERT_C4_RESPONSE_ALLOCATION_BOUNDARY','request-frame-observation-omitted':'ASSERT_C4_EXACT32_COMPLETE_CORRESPONDENCE','partial-external-resource-relabel':'ASSERT_C4_PARTIAL_EXTERNAL_TYPED_FAILURE','partial-external-observation-relabel':'ASSERT_C4_PARTIAL_EXTERNAL_TYPED_FAILURE'}
  by_id={g['id']:g for g in mutations};require(len(by_id)==len(mutations) and all(by_id.get(k,{}).get('assertion')==v for k,v in required.items()),'independent correction counterfactual gates missing')
  require(by_id['manager-admission']['old']=='if failure := m.admitDirectionalV5Locked(req); failure != "" {' and by_id['manager-admission']['new']=='if failure := session.Failure(""); failure != "" {','admission gate must skip production admission, not inject denial')
  require(contract['plus_one_transport_probe']=={'before_frame_decoding':True,'write_calls':0,'raw_bytes':0,'written_bytes':0,'same_ready_session_reuse_required':True},'PLUS_ONE transport/reuse contract missing')
  prefix=[g for g in mutations if g['id']=='refusal-incomplete-prefix-write']
  require(len(prefix)==1 and prefix[0]['witness']=='PLUS_ONE' and prefix[0]['path']=='internal/lspwire/read_frame.go' and prefix[0]['assertion']=='ASSERT_C4_PLUS_ONE_ZERO_WIRE' and 'w.Write([]byte{\'C\'})' in prefix[0]['new'],'incomplete-prefix behavioral mutation missing')
  patch=(ROOT/'union.patch').read_text();patch_paths=re.findall(r'^diff --git a/(\S+) b/(\S+)$',patch,re.M)
  require(all(a==b for a,b in patch_paths) and sorted(a for a,b in patch_paths)==sorted(x['path'] for x in manifest['paths']),'patch path mismatch')
  require(len(patch_paths)==14 and sum(x['family']=='test' for x in manifest['paths'])==1,'aggregate test package was frozen')
  require(all(x['family'] in {'invocation','acquisition','transport','session','test'} for x in manifest['paths']),'unauthorized family')
  def source_state(suffix=''):
   head=checked('source-head'+suffix,['git','rev-parse','HEAD'],args.source).strip()
   status=checked('source-status'+suffix,['git','status','--porcelain'],args.source)
   diff=checked('source-diff'+suffix,['git','diff','--binary','HEAD'],args.source)
   untracked=checked('source-untracked'+suffix,['git','ls-files','--others','--exclude-standard','-z'],args.source).split('\0')
   state={'head':head,'status':status,'tracked_diff_sha256':sha(diff.encode()),'untracked_sha256':{p:sha((pathlib.Path(args.source)/p).read_bytes()) for p in untracked if p}}
   (outdir/('source-state'+suffix+'.json')).write_text(json.dumps(state,indent=2)+'\n');return state
  source_before=source_state();require(source_before['head'] in {BASE,CAMPAIGN_HEAD},'source HEAD mismatch');result['source_before']=source_before
  with tempfile.TemporaryDirectory(prefix='adr0011-v5-four-') as tmp:
   fresh=pathlib.Path(tmp)/'fresh';checked('clone',['git','clone','--local','--no-hardlinks','--no-checkout',str(pathlib.Path(args.source).resolve()),str(fresh)])
   def fresh_union_state():
    diff=checked('fresh-union-diff',['git','diff','--binary','HEAD'],fresh)
    return {'tracked_diff_sha256':sha(diff.encode()),'path_sha256':{item['path']:sha((fresh/item['path']).read_bytes()) for item in manifest['paths']}}
   checked('checkout',['git','checkout','--detach',BASE],fresh)
   base_runtime=(fresh/'sessionruntime/sessionruntime.go').read_text()
   for item in manifest['paths']:
    p=fresh/item['path'];require((sha(p.read_bytes()) if p.exists() else None)==item['base_sha256'],'base path hash mismatch: '+item['path'])
   checked('patch-check',['git','apply','--check',str(ROOT/'union.patch')],fresh);checked('fresh-apply',['git','apply',str(ROOT/'union.patch')],fresh)
   for item in manifest['paths']:require(sha((fresh/item['path']).read_bytes())==item['sha256'],'fresh apply hash mismatch: '+item['path'])
   result['fresh_union']=fresh_union_state()
   require(result['fresh_union']['path_sha256']=={item['path']:item['sha256'] for item in manifest['paths']},'fresh union path hashes are not sealed to current PATH_MANIFEST')
   stale=[]
   if previous_result:
    result['previous_result_sha256']=previous_result_sha256
    candidates=[]
    value=previous_result.get('fresh_union')
    if isinstance(value,dict):candidates.append(('fresh_union',value))
    for key in ('source_before','source_after'):
     value=previous_result.get(key)
     if isinstance(value,dict) and value.get('head')==CAMPAIGN_HEAD and value.get('status'):
      candidates.append((key,value))
    for key,value in candidates:
     if value.get('tracked_diff_sha256') and value['tracked_diff_sha256']!=result['fresh_union']['tracked_diff_sha256']:
      stale.append({'field':key+'.tracked_diff_sha256','old':value['tracked_diff_sha256'],'new':result['fresh_union']['tracked_diff_sha256']})
     path_hashes=value.get('path_sha256') or value.get('untracked_sha256') or {}
     for item in manifest['paths']:
      old=path_hashes.get(item['path'])
      if old and old!=item['sha256']:stale.append({'field':key+'.'+item['path'],'old':old,'new':item['sha256']})
   result['previous_result_stale_hashes']=stale
   require(ordinary_roundtrip(base_runtime)==ordinary_roundtrip((fresh/'sessionruntime/sessionruntime.go').read_text()),'ordinary Runtime.RoundTrip changed')
   carrier=(fresh/'internal/directionalraw/carrier.go').read_text();require(re.findall(r'^type ([A-Z]\w*)\b',carrier,re.M)==['PreparedSelection','Request','Reservation','Lease','View','Observation'],'not exactly six carriers')
   test_source=(fresh/'sessionruntime/directional_v5_four_causal_private_test.go').read_text();require(set(re.findall(r'^func (Test\w+)\(',test_source,re.M))==set(WITNESSES.values()),'extra qualification witnesses')
   for key in WITNESSES:result['witnesses'].append(witness(key,fresh))
   require(len({w['pid'] for w in result['witnesses']})==4,'witness process identities not distinct')
   for gate in mutations:
    require(gate['witness'] in WITNESSES,'mutation outside four witnesses')
    path=fresh/gate['path'];original=path.read_text();require(original.count(gate['old'])==1,'nonunique critical edge: '+gate['id'])
    path.write_text(original.replace(gate['old'],gate['new'],1))
    try:
     code,out=run(['go','test','./sessionruntime','-run','^'+WITNESSES[gate['witness']]+'$','-count=1','-timeout','30s','-v'],fresh,{'ADR0011_V5_CAUSAL_RECEIPT':''})
     (outdir/('mutation-'+gate['id']+'.log')).write_text(out)
     failure_lines=[line.strip() for line in out.splitlines() if re.search(r': '+re.escape(gate['assertion'])+r'(?::|$)',line)]
     require(code==1 and '[build failed]' not in out and 'setup failed' not in out and 'panic:' not in out and 'test timed out' not in out and 'context deadline exceeded' not in out and '--- FAIL: '+WITNESSES[gate['witness']] in out and len(failure_lines)==1,'critical edge not assertion-specifically detected: '+gate['id'])
     result['mutations'].append({'id':gate['id'],'witness':gate['witness'],'critical_edge':gate['edge'],'exit_code':code,'assertion':gate['assertion'],'assertion_failure_line':failure_lines[0],'log_sha256':sha(out.encode()),'compiled':True,'assertion_failed':True,'timed_out':False,'behaviorally_detected':True})
    finally:path.write_text(original)
   for item in manifest['paths']:require(sha((fresh/item['path']).read_bytes())==item['sha256'],'restore hash mismatch')
   result['restored_witnesses']=[witness(key,fresh,'-restored') for key in WITNESSES]
  result['source_after']=source_state('-after');require(result['source_after']==source_before,'campaign HEAD or working-tree bytes changed')
  result['status']='AUTHORED_NOT_GO';result['verifier_exit_code']=0
 except Exception as exc:result['error']=str(exc);result['verifier_exit_code']=1
 raw=(json.dumps(result,indent=2)+'\n').encode();(outdir/'RESULT.json').write_bytes(raw);(outdir/'RESULT.sha256').write_text(sha(raw)+'\n')
 reread=json.loads((outdir/'RESULT.json').read_text())
 if reread.get('verifier_exit_code')==0:
  require(reread.get('fresh_union',{}).get('path_sha256')=={item['path']:item['sha256'] for item in json.loads((ROOT/'PATH_MANIFEST.json').read_text())['paths']},'sealed RESULT path hashes are stale')
  require((outdir/'RESULT.sha256').read_text().strip()==sha((outdir/'RESULT.json').read_bytes()),'sealed RESULT.sha256 mismatch')
 print(json.dumps({'status':result['status'],'exit_code':result['verifier_exit_code'],'independent_witnesses':len(result['witnesses']),'critical_edge_gates':len(result['mutations']),'stale_previous_result_hashes':len(result.get('previous_result_stale_hashes',[])),'error':result.get('error'),'result':str(outdir/'RESULT.json')},indent=2))
 return result['verifier_exit_code']
if __name__=='__main__':raise SystemExit(main())
