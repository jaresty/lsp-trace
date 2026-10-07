#!/usr/bin/env python3
import base64, hashlib, json, shutil
from pathlib import Path
ROOT=Path(__file__).parent
OUT=ROOT/'inputs'
DEFERRED=ROOT/'DEFERRED_BOUNDARIES.json'
PUBLISHED_LIMITS=json.loads((ROOT/'LIMITS.json').read_bytes())
def exact_limit(name): return PUBLISHED_LIMITS[name]
def one_below_limit(name): return exact_limit(name)-1
REQ_SCHEMA='lsp-trace.adr0007.location-intersection.request.private.v5'
ENV_SCHEMA='lsp-trace.adr0007.source-admission-envelope.private.v5'
ADM_SCHEMA='lsp-trace.adr0007.source-admission.private.v2'
POL='sha256:ba2dd40c552f9492be66b67b760f366d5c80af292278a33510f97dd2a1b7972e'
LIM='sha256:55553e121b51ad8f3d4c491ffb5f2635cbc608131d721afcdb0423046625a8d3'
Z='sha256:'+'0'*64

def compact(v): return json.dumps(v,separators=(',',':'),ensure_ascii=False).encode()+b'\n'
def source(path,data=b'a\xf0\x9f\x98\x80b\r\nxy\n'):
 d='sha256:'+hashlib.sha256(data).hexdigest()
 return {'path':path,'revision':'rev-v5','fileDigest':d,'objectDigest':d,'bytes':base64.b64encode(data).decode()}
def admission_digest(srcs):
 h=hashlib.sha256(); h.update(ADM_SCHEMA.encode())
 for s in sorted(srcs,key=lambda x:x['path'].encode()):
  for k in ('path','revision','fileDigest','objectDigest'): h.update(b'\0'); h.update(s[k].encode())
 return 'sha256:'+h.hexdigest()
def binding(srcs=None):
 srcs=srcs or [source('src/a')]; srcs=sorted(srcs,key=lambda x:x['path'].encode())
 return {'schema':ENV_SCHEMA,'outcome':'COMPLETE','binding':{'schema':ADM_SCHEMA,'admissionDigest':admission_digest(srcs),'sources':srcs}}
def pos(a,b): return {'line':a,'character':b}
def rng(a,b,c,d): return {'start':pos(a,b),'end':pos(c,d)}
def member(i='m',r=None,**kw):
 s=source(kw.pop('path','src/a')); x={'id':i,'path':s['path'],'revision':s['revision'],'fileDigest':s['fileDigest'],'objectDigest':s['objectDigest'],'ranges':r or [rng(0,1,0,3)],'available':True,'policyAllowed':True,'score':5}; x.update(kw); return x
def request(i,selector=None,relation='INTERSECTS',members=None,topK=10,adm=None):
 b=binding(); return {'schema':REQ_SCHEMA,'id':i,'relation':relation,'selector':selector or {'kind':'EXACT_FILE','path':'src/a'},'admissionDigest':adm or b['binding']['admissionDigest'],'policyDigest':POL,'limitsDigest':LIM,'topK':topK,'members':members if members is not None else [member()]}
CASES=[
('01-exact-intersects','exact-file intersects',request('c01'),'DESIGN.md §4 lines 63-65; §4 line 91'),
('02-contained-by','contained-by relation',request('c02',selector={'kind':'RANGE_UNION','union':[{'path':'src/a','ranges':[rng(0,0,0,4)]}]},relation='CONTAINED_BY'),'DESIGN.md §4 lines 73-91'),
('03-contains','contains relation',request('c03',selector={'kind':'RANGE_UNION','union':[{'path':'src/a','ranges':[rng(0,1,0,3)]}]},relation='CONTAINS',members=[member(r=[rng(0,0,0,4)])]),'DESIGN.md §4 line 91'),
('04-adjacent-half-open','adjacent half-open ranges',request('c04',selector={'kind':'RANGE_UNION','union':[{'path':'src/a','ranges':[rng(0,0,0,1)]}]},members=[member(r=[rng(0,1,0,3)])]),'DESIGN.md §4 lines 89-91'),
('05-union-repeat','UTF-16-valid repeated-path range union merge/dedup',request('c05',selector={'kind':'RANGE_UNION','union':[{'path':'src/a','ranges':[rng(0,0,0,3)]},{'path':'src/a','ranges':[rng(0,0,0,3),rng(0,1,0,3)]}]}),'DESIGN.md §4 lines 73-87'),
('06-prefix-segment','prefix frozen expansion',request('c06',selector={'kind':'PATH_PREFIX','path':'src','frozenPaths':['src/a']}),'DESIGN.md §4 lines 69-87'),
('07-schema-unknown-field','request unknown field',dict(request('c07'),unexpected=True),'DESIGN.md §1 lines 7-17'),
('08-malformed-json','malformed request bytes',None,'DESIGN.md §1 lines 9-13'),
('09-binding-absent','binding absent',request('c09'),'DESIGN.md §3 lines 25-26,43-45'),
('10-binding-malformed','binding projection malformed',request('c10'),'DESIGN.md §3 lines 33-41'),
('11-typed-invalid-request','typed INVALID_REQUEST binding',request('c11'),'DESIGN.md §3 lines 48,57'),
('12-typed-invalid-source','typed INVALID_SOURCE binding',request('c12'),'DESIGN.md §3 lines 49,57'),
('13-typed-duplicate-source','typed DUPLICATE_SOURCE binding',request('c13'),'DESIGN.md §3 lines 50,57'),
('14-typed-resource-limit','typed RESOURCE_LIMIT binding',request('c14'),'DESIGN.md §3 lines 51-57'),
('15-binding-digest-mismatch','binding admission digest differs',request('c15',adm=Z),'DESIGN.md §3 lines 29-31,54'),
('16-member-eligible','RANGE_UNION member contained by selector is ELIGIBLE',request('c16',selector={'kind':'RANGE_UNION','union':[{'path':'src/a','ranges':[rng(0,0,1,0)]}]}),'DESIGN.md §4 lines 73-91; §5 lines 97-104'),
('17-member-ineligible','RANGE_UNION member adjacent to selector is INELIGIBLE',request('c17',selector={'kind':'RANGE_UNION','union':[{'path':'src/a','ranges':[rng(0,0,1,0)]}]},members=[member(r=[rng(1,0,1,1)])]),'DESIGN.md §4 lines 73-91; §5 lines 97-104'),
('18-member-unavailable','UNAVAILABLE_LOCATION member inputs',request('c18',members=[member(available=False)]),'DESIGN.md §5 lines 97-106'),
('19-member-invalid-location','INVALID_LOCATION member inputs',request('c19',members=[member(revision='other')]),'DESIGN.md §5 lines 97-106'),
('20-member-duplicate','DUPLICATE_MEMBER inputs',request('c20',members=[member('same'),member('same',available=False)]),'DESIGN.md §5 lines 95-106'),
('21-member-filtered','FILTERED_BY_POLICY member inputs',request('c21',members=[member(policyAllowed=False)]),'DESIGN.md §5 lines 97-106'),
('22-ranking-topk','ranking and topK setup',request('c22',members=[member('low',score=7),member('tie-a',score=9),member('tie-b',score=9)],topK=2),'DESIGN.md §6 lines 112-118'),
('23-frozen-paths-plus-one','raw frozen path count 1001 expands to one source',request('c23',selector={'kind':'PATH_PREFIX','path':'src','frozenPaths':[f'src/{i:04d}' for i in range(1001)]}),'DESIGN.md §§7-8; ALGORITHM.md lines 134-147'),
('24-cancel-deadline','simultaneous cancellation and deadline condition',request('c24'),'DESIGN.md §7 lines 120-126'),
('25-witness-plus-one','unique witness count 10001 exceeds maxWitnesses 10000',request('c25',members=[member(f'm{i:05d}') for i in range(10001)]),'DESIGN.md §§6,8; ALGORITHM.md lines 134-147'),
('26-deadline-only','deadline expired without cancellation',request('c26'),'DESIGN.md §7 lines 120-126'),
]

def special_binding(n):
 if n=='09-binding-absent': return None
 if n=='10-binding-malformed': return {'schema':ENV_SCHEMA,'outcome':'COMPLETE'}
 inp=[source('src/a')]
 if n=='11-typed-invalid-request': return {'schema':ENV_SCHEMA,'outcome':'INVALID_REQUEST','detail':'LIMITS_OR_SOURCES','input':[]}
 if n=='12-typed-invalid-source':
  inp[0]['fileDigest']=Z
  return {'schema':ENV_SCHEMA,'outcome':'INVALID_SOURCE','detail':'FILE_DIGEST','input':inp}
 if n=='13-typed-duplicate-source':
  x=source('src/a'); return {'schema':ENV_SCHEMA,'outcome':'DUPLICATE_SOURCE','detail':'DUPLICATE_PATH','duplicatePath':'src/a','input':[x,x]}
 if n=='14-typed-resource-limit': return {'schema':ENV_SCHEMA,'outcome':'RESOURCE_LIMIT','detail':'SOURCES','input':[source(f'src/{i:04d}') for i in range(1001)]}
 if n=='15-binding-digest-mismatch':
  b=binding(); b['binding']['admissionDigest']=Z; return b
 return binding()

def main():
 if OUT.exists(): shutil.rmtree(OUT)
 OUT.mkdir()
 for idx,(name,pert,req,cite) in enumerate(CASES,1):
  d=OUT/name; d.mkdir()
  cond={'schema':'lsp-trace.adr0007.location-input-condition.private.v5','cancel':name=='24-cancel-deadline','deadlineExpired':name in {'24-cancel-deadline','26-deadline-only'},'limitsProfile':'PUBLISHED_V5'}
  boundaries={
   '23-frozen-paths-plus-one':{'maxFrozenPaths':exact_limit('maxFrozenPaths'),'rawFrozenPaths':exact_limit('maxFrozenPaths')+1,'expandedSources':1,'expectedPrecedence':'FROZEN_PATHS'},
   '25-witness-plus-one':{'maxWitnesses':exact_limit('maxWitnesses'),'uniqueWitnesses':exact_limit('maxWitnesses')+1},
  }
  if name in boundaries: cond['boundarySetup']=boundaries[name]
  (d/'CONDITION.json').write_bytes(compact(cond))
  raw=b'{"schema":' if name=='08-malformed-json' else compact(req)
  (d/'REQUEST.raw.json').write_bytes(raw)
  b=special_binding(name)
  if b is None: (d/'BINDING.ABSENT').write_bytes(b'ABSENT\n')
  else: (d/'BINDING.json').write_bytes(compact(b))
  meta={'schema':'lsp-trace.adr0007.location-input-case.private.v5','caseId':name,'causalPerturbation':pert,'specCitations':[cite]}
  (d/'CASE.json').write_bytes(compact(meta))
 DEFERRED.write_bytes(compact({'schema':'lsp-trace.adr0007.location-deferred-boundaries.private.v5','variants':[{'symbol':s,'status':'DEFERRED'} for s in ('W','W-1','B','B-1')]}))
 print(f'generated {len(CASES)} cases and 4 value-free deferred boundary variants')
if __name__=='__main__': main()
