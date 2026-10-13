#!/usr/bin/env python3
import csv, hashlib, json, pathlib, re, shutil, subprocess, tempfile
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[2]
HEAD='950a3f1ae1bd2cffa8e246b51819b7a7930a68a4'
BINDING=HERE.parent/'adr0011-directional-raw-custody-32mib-private-interface-go-binding-v1'
BRIDGE=BINDING/'BRIDGE.tsv'
FROZEN={'sessionruntime/call_hierarchy_directional_raw_v1_private.go','sessionruntime/sessionruntime.go','internal/lspwire/read_frame.go','internal/lspwire/successor_ingress.go','sessionruntime/b4_owned_method_pair_lease.go','sessionruntime/b4_c15_ownership_registry_private.go'}
def sha(p): return hashlib.sha256(p.read_bytes()).hexdigest()
def req(name,condition):
 print(('PASS ' if condition else 'FAIL ')+name)
 if not condition: raise AssertionError(name)
def run(args,cwd,expect=True):
 print('$ '+' '.join(args))
 p=subprocess.run(args,cwd=cwd,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
 print(p.stdout,end='')
 if expect and p.returncode: raise AssertionError('COMMAND_'+'_'.join(args))
 return p
def added(patch): return ''.join(line[1:]+'\n' for line in patch.splitlines() if line.startswith('+') and not line.startswith('+++'))
def authoritative_rows():
 rows=list(csv.DictReader(BRIDGE.open(),delimiter='\t'))
 req('AUTHORITY_COMMITTED',run(['git','cat-file','-e',HEAD+':'+str(BRIDGE.relative_to(ROOT))],ROOT).returncode==0)
 req('AUTHORITY_EXACT20',len(rows)==20 and len({r['stage_a_row_id'] for r in rows})==20 and len({r['symbol'] for r in rows})==20)
 req('AUTHORITY_PLANNED',all(r['planned_status'].startswith('PLANNED_') for r in rows))
 return rows
def static_verify(patch,stage_rows,source_rows):
 paths=set(re.findall(r'^diff --git a/(\S+) b/\S+$',patch,re.M))
 req('PATCH_PATHS',paths<=FROZEN and paths=={'sessionruntime/call_hierarchy_directional_raw_v1_private.go','sessionruntime/sessionruntime.go','sessionruntime/b4_owned_method_pair_lease.go'})
 plus=added(patch)
 for forbidden in ['type Manager struct','type privateB4ByteAccountV2 struct','type privateB4ByteLeaseV2 struct','type callHierarchyLayer1GenericOwner struct']:
  req('NO_STANDIN_'+forbidden.split()[1],forbidden not in plus)
 req('NO_FAKE_FIELDS',not re.search(r'\.(ID|Capacity|Active)\b',patch))
 required=['newPrivateB4ByteAccountV2()','directionalRawV1: privateCallHierarchyDirectionalLedgerV1{account: directionalAccount','m.newDirectionalTerminalV1Locked()','o.directionalRawV1.releaseLocked()','m.directionalRawV1.account.requestTerminalRelease()']
 for value in required: req('WIRED_'+re.sub(r'\W+','_',value).strip('_'),value in plus)
 req('WIRED_BOTH_OWNER_CREATION_PATHS',plus.count('m.newDirectionalTerminalV1Locked()')==2)
 expected=[(r['stage_a_row_id'],r['planned_status'],r['symbol'].replace('V3','V1')) for r in source_rows]
 actual=[(r['stage_a_row_id'],r['planned_status'],r['production_symbol']) for r in stage_rows]
 req('MAP_EXTERNAL_EQUAL',actual==expected)
 for rid,status,symbol in expected:
  req('MAP_'+symbol,rid in patch and status in patch and re.search(r'func '+re.escape(symbol)+r'\(',patch) is not None)
def closure(work):
 raw=run(['go','list','-json','./...'],work).stdout
 decoder=json.JSONDecoder(); offset=0; packages=[]
 while offset < len(raw):
  while offset < len(raw) and raw[offset].isspace(): offset+=1
  if offset>=len(raw): break
  obj,offset=decoder.raw_decode(raw,offset); packages.append(obj)
 selected={'lsp-trace/sessionruntime','lsp-trace/internal/lspwire'}
 changed=True
 while changed:
  changed=False
  for pkg in packages:
   path=pkg.get('ImportPath','')
   if path not in selected and selected.intersection(pkg.get('Imports',[])):
    selected.add(path); changed=True
 return sorted(selected)
def reject(name,fn):
 try: fn()
 except AssertionError:
  print('PASS ADVERSARIAL_'+name+'_REJECTED'); return
 raise AssertionError('ADVERSARIAL_'+name+'_ACCEPTED')
req('HEAD',run(['git','rev-parse','HEAD'],ROOT).stdout.strip()==HEAD)
patch=(HERE/'PROPOSED.patch').read_text()
source_rows=authoritative_rows()
stage_rows=list(csv.DictReader((HERE/'STAGE_A_BRIDGE.tsv').open(),delimiter='\t'))
static_verify(patch,stage_rows,source_rows)
reject('FORGED_ID_MAP',lambda: static_verify(patch,[dict(stage_rows[0],stage_a_row_id='forged')]+stage_rows[1:],source_rows))
reject('REMOVED_UNWIRED_CALL',lambda: static_verify(patch.replace('m.newDirectionalTerminalV1Locked()','nil',1),stage_rows,source_rows))
fake_log=pathlib.Path(tempfile.mkdtemp(prefix='adr0011-fake-log-'))/'COMPILE.log'
fake_log.write_text('PRODUCTION_SPLICE_COMPILE_ONLY\nALL COMMANDS PASSED\n')
req('FABRICATED_LOG_NOT_AUTHORITY','ALL COMMANDS PASSED' not in patch and fake_log != HERE/'COMPILE.log')
shutil.rmtree(fake_log.parent)
work=pathlib.Path(tempfile.mkdtemp(prefix='adr0011-independent-verify-'))
try:
 run(['git','clone','--quiet','--no-hardlinks',str(ROOT),str(work)],ROOT)
 run(['git','checkout','--quiet',HEAD],work)
 run(['git','apply','--check',str(HERE/'PROPOSED.patch')],work)
 run(['git','apply',str(HERE/'PROPOSED.patch')],work)
 packages=closure(work)
 req('CLOSURE_INCLUDES_SEEDS','lsp-trace/sessionruntime' in packages and 'lsp-trace/internal/lspwire' in packages)
 run(['go','test','-run','^$',*packages],work)
 run(['go','vet','./sessionruntime','./internal/lspwire'],work)
 req('FRESH_EXECUTION_COMPLETE',True)
finally:
 shutil.rmtree(work,ignore_errors=True)
with (HERE/'SOURCE_HASHES.tsv').open() as f: hashes=list(csv.DictReader(f,delimiter='\t'))
req('HASH_PATHS',{r['path'] for r in hashes}==FROZEN)
for row in hashes:
 p=ROOT/row['path']; req('PRE_'+row['path'],row['pre_sha256']==('ABSENT' if not p.exists() else sha(p)))
req('LOG_LABEL_ONLY',(HERE/'COMPILE.log').read_text().startswith('PRODUCTION_SPLICE_COMPILE_ONLY\n'))
for line in (HERE/'SHA256SUMS').read_text().splitlines():
 h,name=line.split(None,1); req('SUM_'+name,sha(HERE/name)==h)
for name in ['README.md','INTERFACE.md','OWNERSHIP_LIFECYCLE.md','WRITESET.md','REVIEW_REQUEST.md']:
 req('AUTHORITY_'+name,'AUTHORED_NOT_GO' in (HERE/name).read_text())
print('PRODUCTION_INTERFACE_PACKAGE_VERIFY_OK')
