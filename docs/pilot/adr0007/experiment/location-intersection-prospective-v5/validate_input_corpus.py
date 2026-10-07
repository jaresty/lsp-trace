#!/usr/bin/env python3
"""Private input-corpus validator: structure, canonical transport, and census only."""
import argparse, base64, hashlib, json, sys
from pathlib import Path
ROOT=Path(__file__).parent; INPUTS=ROOT/'inputs'; CENSUS=ROOT/'INPUT_CORPUS.json'; SCHEMAS=ROOT/'INPUT_SCHEMAS.json'
FORBIDDEN={'EXPECTED_RESULT.json','RESULT.json','OUTCOME.json','ORACLE.json'}
CASE_KEYS={'schema','caseId','causalPerturbation','specCitations'}
COND_KEYS={'schema','cancel','deadlineExpired','limitsProfile','boundarySetup'}
def load(p): return json.loads(p.read_bytes())
def digest(p):
 b=p.read_bytes(); return {'path':p.relative_to(ROOT).as_posix(),'bytes':len(b),'sha256':'sha256:'+hashlib.sha256(b).hexdigest()}
def validate(write=False):
 errors=[]
 try:
  schemas=load(SCHEMAS)
  if set(schemas)!={'schema','existing','closed'} or schemas.get('existing')!={'request':'request.schema.json','binding':'source-binding.schema.json'} or set(schemas.get('closed',{}))!={'condition','case','corpus'}: errors.append('INPUT_SCHEMAS.json closed/reference shape')
 except Exception as e: errors.append(f'INPUT_SCHEMAS.json: {e}')
 dirs=sorted(p for p in INPUTS.iterdir() if p.is_dir()) if INPUTS.exists() else []
 if len(dirs)!=24: errors.append(f'case-count: {len(dirs)} != 24')
 records=[]
 for d in dirs:
  names={p.name for p in d.iterdir() if p.is_file()}
  required={'CONDITION.json','REQUEST.raw.json','CASE.json'}
  if not required<=names: errors.append(f'{d.name}: missing {sorted(required-names)}')
  if len({'BINDING.json','BINDING.ABSENT'}&names)!=1: errors.append(f'{d.name}: binding marker xor failed')
  if names&FORBIDDEN: errors.append(f'{d.name}: forbidden artifacts {sorted(names&FORBIDDEN)}')
  allowed=required|{'BINDING.json','BINDING.ABSENT'}
  if names-allowed: errors.append(f'{d.name}: extra files {sorted(names-allowed)}')
  try:
   case=load(d/'CASE.json'); cond=load(d/'CONDITION.json')
   if set(case)!=CASE_KEYS or case.get('caseId')!=d.name or not isinstance(case.get('causalPerturbation'),str) or not case.get('causalPerturbation') or not case.get('specCitations'): errors.append(f'{d.name}: CASE.json closed shape')
   if not set(cond)<=COND_KEYS or set(cond)<{'schema','cancel','deadlineExpired','limitsProfile'}: errors.append(f'{d.name}: CONDITION.json closed shape')
  except Exception as e: errors.append(f'{d.name}: metadata parse: {e}')
  bp=d/'BINDING.json'
  if bp.exists():
   try:
    env=load(bp)
    arrays=[]
    if env.get('outcome')=='COMPLETE' and isinstance(env.get('binding'),dict): arrays=[env['binding'].get('sources',[])]
    elif isinstance(env.get('input'),list): arrays=[env['input']]
    for arr in arrays:
     for item in arr:
      if isinstance(item,dict) and isinstance(item.get('bytes'),str):
       raw=base64.b64decode(item['bytes'],validate=True)
       if base64.b64encode(raw).decode()!=item['bytes']: errors.append(f'{d.name}: noncanonical base64')
   except Exception as e: errors.append(f'{d.name}: binding transport: {e}')
  files=sorted((digest(p) for p in d.iterdir() if p.is_file()),key=lambda x:x['path'])
  records.append({'caseId':d.name,'files':files})
 corpus={'schema':'lsp-trace.adr0007.location-input-corpus.private.v5','caseCount':len(dirs),'cases':records}
 encoded=json.dumps(corpus,separators=(',',':'),ensure_ascii=False).encode()+b'\n'
 if write: CENSUS.write_bytes(encoded)
 elif not CENSUS.exists() or CENSUS.read_bytes()!=encoded: errors.append('INPUT_CORPUS.json census/digests differ')
 if errors:
  for e in errors: print('FAIL input-structure:',e,file=sys.stderr)
  return 1
 print(f'PASS input-structure canonical-transport census cases={len(dirs)}')
 return 0
if __name__=='__main__':
 ap=argparse.ArgumentParser(); ap.add_argument('--write-census',action='store_true'); a=ap.parse_args(); raise SystemExit(validate(a.write_census))
