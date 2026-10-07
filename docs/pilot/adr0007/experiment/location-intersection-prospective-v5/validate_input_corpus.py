#!/usr/bin/env python3
"""Validate only prospective ADR0007 v5 inputs; never evaluate location outcomes."""
import argparse, base64, hashlib, json, re as regex, subprocess, sys, tempfile
from pathlib import Path
ARTIFACT_ROOT=Path(__file__).resolve().parent
REPO_ROOT=Path(__file__).resolve().parents[5]
HELPER=ARTIFACT_ROOT/'cmd/inputcheck/main.go'
ROOT=ARTIFACT_ROOT; INPUTS=ROOT/'inputs'; CENSUS=ROOT/'INPUT_CORPUS.json'; SCHEMAS=ROOT/'INPUT_SCHEMAS.json'; DEFERRED=ROOT/'DEFERRED_BOUNDARIES.json'
EXPECTED_COUNT=26
FORBIDDEN_NAMES={'EXPECTED_RESULT.json','RESULT.json','OUTCOME.json','ORACLE.json','FREEZE.json','EVALUATOR.json'}
FORBIDDEN_CONTENT=(b'EXPECTED_RESULT',b'OUTCOME_ORACLE',b'V5_EVALUATOR',b'FREEZE.json')
CASE_KEYS=('schema','caseId','causalPerturbation','specCitations')
COND_KEYS=('schema','cancel','deadlineExpired','limitsProfile','boundarySetup')
ROOT_KEYS=('schema','id','relation','selector','admissionDigest','policyDigest','limitsDigest','topK','members')

def load(p): return json.loads(p.read_bytes())
def compact(v): return json.dumps(v,separators=(',',':'),ensure_ascii=False).encode()+b'\n'
def digest(p):
 b=p.read_bytes(); return {'path':p.relative_to(ROOT).as_posix(),'bytes':len(b),'sha256':'sha256:'+hashlib.sha256(b).hexdigest()}
def canonical(p, *, order=None):
 try: v=load(p)
 except Exception as e: return None,f'{p.name} parse: {e}'
 if order:
  expected=tuple(k for k in order if k in v)
  if tuple(v)[:len(expected)]!=expected: return v,f'{p.name} field order'
 if p.read_bytes()!=compact(v): return v,f'{p.name} not canonical compact LF JSON'
 return v,None
def run(cmd):
 return subprocess.run(cmd,cwd=REPO_ROOT,text=True,capture_output=True)

def validate(write=False):
 errors=[]
 schemas,se=canonical(SCHEMAS)
 if se: errors.append(se)
 elif set(schemas)!={'schema','existing','closed'} or schemas.get('existing')!={'request':'request.schema.json','binding':'source-binding.schema.json'} or set(schemas.get('closed',{}))!={'condition','case','corpus'}: errors.append('INPUT_SCHEMAS.json closed/reference shape')
 dirs=sorted(p for p in INPUTS.iterdir() if p.is_dir()) if INPUTS.exists() else []
 if len(dirs)!=EXPECTED_COUNT: errors.append(f'case-count: {len(dirs)} != {EXPECTED_COUNT}')
 deferred,de=canonical(DEFERRED)
 expected_deferred={'schema':'lsp-trace.adr0007.location-deferred-boundaries.private.v5','variants':[{'symbol':s,'status':'DEFERRED'} for s in ('W','W-1','B','B-1')]}
 if de: errors.append(de)
 elif deferred!=expected_deferred: errors.append('DEFERRED_BOUNDARIES.json must contain exactly value-free W/W-1/B/B-1 variants')
 binary=Path(tempfile.gettempdir())/'lsp-trace-location-inputcheck'
 built=run(['go','build','-o',str(binary),str(HELPER)])
 if built.returncode: errors.append('Go input checker build: '+built.stderr.strip())
 records=[]
 for d in dirs:
  names={p.name for p in d.iterdir() if p.is_file()}; required={'CONDITION.json','REQUEST.raw.json','CASE.json'}
  if not required<=names: errors.append(f'{d.name}: missing {sorted(required-names)}')
  if len({'BINDING.json','BINDING.ABSENT'}&names)!=1: errors.append(f'{d.name}: binding marker xor failed')
  if names&FORBIDDEN_NAMES: errors.append(f'{d.name}: forbidden artifacts {sorted(names&FORBIDDEN_NAMES)}')
  allowed=required|{'BINDING.json','BINDING.ABSENT'}
  if names-allowed: errors.append(f'{d.name}: extra files {sorted(names-allowed)}')
  for p in d.iterdir():
   if p.is_file():
    upper=p.read_bytes().upper()
    for token in FORBIDDEN_CONTENT:
     if token in upper: errors.append(f'{d.name}: prohibited content token {token.decode()}')
  case,ce=canonical(d/'CASE.json',order=CASE_KEYS); cond,co=canonical(d/'CONDITION.json',order=COND_KEYS)
  if ce: errors.append(f'{d.name}: {ce}')
  if co: errors.append(f'{d.name}: {co}')
  if case is not None and (tuple(case)!=CASE_KEYS or case.get('caseId')!=d.name or not isinstance(case.get('causalPerturbation'),str) or not case.get('causalPerturbation') or not isinstance(case.get('specCitations'),list) or not case['specCitations']): errors.append(f'{d.name}: CASE.json schema')
  if case is not None and regex.search(r'(?<![A-Za-z0-9-])(W-1|B-1|W|B)(?![A-Za-z0-9-])',case.get('causalPerturbation','')): errors.append(f'{d.name}: unproven W/B boundary label')
  if cond is not None and (not set(cond)<=set(COND_KEYS) or tuple(cond)[:4]!=COND_KEYS[:4] or cond.get('schema')!='lsp-trace.adr0007.location-input-condition.private.v5' or not isinstance(cond.get('cancel'),bool) or not isinstance(cond.get('deadlineExpired'),bool) or cond.get('limitsProfile')!='PUBLISHED_V5' or ('boundarySetup' in cond and not isinstance(cond['boundarySetup'],dict))): errors.append(f'{d.name}: CONDITION.json schema')
  if built.returncode==0:
   for key,path in (('case',d/'CASE.json'),('condition',d/'CONDITION.json')):
    r=subprocess.run([str(binary),'metadata',str(SCHEMAS),key,str(path)],text=True,capture_output=True)
    if r.returncode: errors.append(f'{d.name}: {key} metadata schema: {r.stderr.strip()}')
  req=d/'REQUEST.raw.json'
  if d.name=='08-malformed-json':
   try: json.loads(req.read_bytes()); errors.append(f'{d.name}: declared malformed JSON parsed')
   except json.JSONDecodeError as e:
    if e.msg!='Expecting value' or e.pos!=10: errors.append(f'{d.name}: malformed class differs: {e.msg}@{e.pos}')
  else:
   rv,re=canonical(req,order=ROOT_KEYS)
   if re: errors.append(f'{d.name}: {re}')
   if d.name=='23-frozen-paths-plus-one':
    frozen=rv.get('selector',{}).get('frozenPaths',[]) if isinstance(rv,dict) else []
    setup=cond.get('boundarySetup',{}) if isinstance(cond,dict) else {}
    if len(frozen)!=1001 or len(set(frozen))!=1001 or setup!={'maxFrozenPaths':1000,'rawFrozenPaths':1001,'expandedSources':1,'expectedPrecedence':'FROZEN_PATHS'}: errors.append(f'{d.name}: concrete raw=1001 expanded=1 frozen-path boundary')
   if d.name=='25-witness-plus-one':
    members=rv.get('members',[]) if isinstance(rv,dict) else []
    ids=[m.get('id') for m in members if isinstance(m,dict)]
    setup=cond.get('boundarySetup',{}) if isinstance(cond,dict) else {}
    if len(ids)!=10001 or len(set(ids))!=10001 or setup!={'maxWitnesses':10000,'uniqueWitnesses':10001}: errors.append(f'{d.name}: unique witnesses must equal 10001')
   if built.returncode==0:
    r=subprocess.run([str(binary),'schema',str(ARTIFACT_ROOT/'request.schema.json'),str(req)],text=True,capture_output=True)
    if d.name=='07-schema-unknown-field':
     if r.returncode==0: errors.append(f'{d.name}: declared request schema violation accepted')
    elif r.returncode: errors.append(f'{d.name}: request schema: {r.stderr.strip()}')
  bp=d/'BINDING.json'
  if bp.exists():
   env,be=canonical(bp)
   if be: errors.append(f'{d.name}: {be}')
   if env is not None:
    arrays=[env.get('input',[])] if isinstance(env.get('input'),list) else []
    if env.get('outcome')=='COMPLETE' and isinstance(env.get('binding'),dict): arrays=[env['binding'].get('sources',[])]
    for arr in arrays:
     for item in arr:
      if isinstance(item,dict) and isinstance(item.get('bytes'),str):
       try:
        raw=base64.b64decode(item['bytes'],validate=True)
        if base64.b64encode(raw).decode()!=item['bytes']: raise ValueError('noncanonical')
       except Exception as e: errors.append(f'{d.name}: strict base64: {e}')
    if built.returncode==0:
     r=subprocess.run([str(binary),'schema',str(ARTIFACT_ROOT/'source-binding.schema.json'),str(bp)],text=True,capture_output=True)
     if d.name=='10-binding-malformed':
      if r.returncode==0: errors.append(f'{d.name}: declared binding schema violation accepted')
     elif r.returncode: errors.append(f'{d.name}: binding schema: {r.stderr.strip()}')
     if d.name in {'11-typed-invalid-request','12-typed-invalid-source','13-typed-duplicate-source','14-typed-resource-limit'}:
      r=subprocess.run([str(binary),'admission',str(bp)],text=True,capture_output=True)
      if r.returncode: errors.append(f'{d.name}: admission reproduction: {r.stderr.strip()}')
  else:
   if (d/'BINDING.ABSENT').read_bytes()!=b'ABSENT\n': errors.append(f'{d.name}: BINDING.ABSENT must equal ABSENT\\n')
  files=sorted((digest(p) for p in d.iterdir() if p.is_file()),key=lambda x:x['path'])
  records.append({'caseId':d.name,'files':files})
 corpus={'schema':'lsp-trace.adr0007.location-input-corpus.private.v5','caseCount':len(dirs),'scopeChange':{'from':30,'to':26,'reason':'remove unproven oracle-derived W/W-1/B/B-1 cases and defer their values'},'cases':records}
 encoded=compact(corpus)
 if write:
  tmp=CENSUS.with_suffix('.json.tmp'); tmp.write_bytes(encoded); tmp.replace(CENSUS)
 elif not CENSUS.exists() or CENSUS.read_bytes()!=encoded: errors.append('INPUT_CORPUS.json census/digests differ')
 if not write and CENSUS.exists():
  cv,e=canonical(CENSUS,order=('schema','caseCount','scopeChange','cases'))
  if e: errors.append(e)
  elif cv.get('caseCount')!=EXPECTED_COUNT or len(cv.get('cases',[]))!=EXPECTED_COUNT: errors.append('INPUT_CORPUS.json exact census')
  if built.returncode==0:
   r=subprocess.run([str(binary),'metadata',str(SCHEMAS),'corpus',str(CENSUS)],text=True,capture_output=True)
   if r.returncode: errors.append('INPUT_CORPUS.json metadata schema: '+r.stderr.strip())
 try: binary.unlink()
 except FileNotFoundError: pass
 if errors:
  for e in errors: print('FAIL input-structure:',e,file=sys.stderr)
  return 1
 print(f'PASS input-structure canonical-schema admission-only census cases={len(dirs)}')
 return 0
if __name__=='__main__':
 ap=argparse.ArgumentParser(); ap.add_argument('--write-census',action='store_true'); a=ap.parse_args(); raise SystemExit(validate(a.write_census))
