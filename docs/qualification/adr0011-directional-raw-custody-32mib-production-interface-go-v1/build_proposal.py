#!/usr/bin/env python3
import csv, hashlib, json, pathlib, shutil, subprocess, tempfile
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[2]
HEAD='950a3f1ae1bd2cffa8e246b51819b7a7930a68a4'
BINDING=HERE.parent/'adr0011-directional-raw-custody-32mib-private-interface-go-binding-v1'
BRIDGE=BINDING/'BRIDGE.tsv'
FROZEN=['sessionruntime/call_hierarchy_directional_raw_v1_private.go','sessionruntime/sessionruntime.go','internal/lspwire/read_frame.go','internal/lspwire/successor_ingress.go','sessionruntime/b4_owned_method_pair_lease.go','sessionruntime/b4_c15_ownership_registry_private.go']
def run(args,cwd,log=None):
 p=subprocess.run(args,cwd=cwd,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
 if log is not None: log.write('$ '+' '.join(args)+'\n'+p.stdout+'\n')
 if p.returncode: raise SystemExit(f'command failed {args}:\n{p.stdout}')
 return p.stdout
def sha(p): return hashlib.sha256(p.read_bytes()).hexdigest()
def replace_once(path,old,new,label):
 s=path.read_text()
 if s.count(old)!=1: raise SystemExit(label+' seam mismatch')
 path.write_text(s.replace(old,new,1))
if run(['git','rev-parse','HEAD'],ROOT).strip()!=HEAD: raise SystemExit('HEAD mismatch')
dirty=run(['git','status','--porcelain'],ROOT).splitlines()
if any(not x[3:].startswith(str(HERE.relative_to(ROOT))+'/') for x in dirty): raise SystemExit('dirty outside package')
rows=list(csv.DictReader(BRIDGE.open(),delimiter='\t'))
if len(rows)!=20 or len({r['stage_a_row_id'] for r in rows})!=20: raise SystemExit('committed bridge not exact20')
work=pathlib.Path(tempfile.mkdtemp(prefix='adr0011-production-splice-'))
try:
 run(['git','clone','--quiet','--no-hardlinks',str(ROOT),str(work)],ROOT)
 run(['git','checkout','--quiet',HEAD],work)
 manager=work/'sessionruntime/sessionruntime.go'
 replace_once(manager,'type Manager struct {\n\tmu                                           sync.Mutex\n','type Manager struct {\n\tmu                                           sync.Mutex\n\tdirectionalRawV1                             privateCallHierarchyDirectionalLedgerV1\n','Manager declaration')
 replace_once(manager,'\tdiagnosticOwner, failure := newManagerDiagnosticOwner()\n\tif failure != "" {\n\t\treturn nil, errors.New("sessionruntime: manager diagnostic owner unavailable")\n\t}\n\treturn &Manager{','\tdiagnosticOwner, failure := newManagerDiagnosticOwner()\n\tif failure != "" {\n\t\treturn nil, errors.New("sessionruntime: manager diagnostic owner unavailable")\n\t}\n\tdirectionalAccount, failure := newPrivateB4ByteAccountV2()\n\tif failure != "" {\n\t\treturn nil, errors.New("sessionruntime: directional raw account unavailable")\n\t}\n\treturn &Manager{directionalRawV1: privateCallHierarchyDirectionalLedgerV1{account: directionalAccount, attempts: make(map[privateCallHierarchyDirectionalAttemptIDV1]*privateCallHierarchyDirectionalAccountV1)}, ','Manager construction')
 replace_once(manager,'result, genericTerminal = m.publishCallHierarchyLayer1GenericOwnerLocked(result, newCallHierarchyLayer1GenericOwner(observation))','result, genericTerminal = m.publishCallHierarchyLayer1GenericOwnerLocked(result, newCallHierarchyLayer1GenericOwner(observation, m.newDirectionalTerminalV1Locked()))','accepted owner binding')
 replace_once(manager,'\tfor _, genericOwner := range genericOwners {\n\t\tgenericOwner.requestTerminal()\n\t\tgenericOwner.waitReleased()\n\t}\n\treturn nil\n','\tfor _, genericOwner := range genericOwners {\n\t\tgenericOwner.requestTerminal()\n\t\tgenericOwner.waitReleased()\n\t}\n\tif m.directionalRawV1.account != nil {\n\t\tm.directionalRawV1.account.requestTerminalRelease()\n\t}\n\treturn nil\n','shutdown account terminal')
 owner=work/'sessionruntime/b4_owned_method_pair_lease.go'
 replace_once(owner,'type callHierarchyLayer1GenericOwner struct {\n\tmu                                        sync.Mutex\n','type callHierarchyLayer1GenericOwner struct {\n\tmu                                        sync.Mutex\n\tdirectionalRawV1                          *privateCallHierarchyDirectionalTerminalV1\n','owner declaration')
 replace_once(owner,'func newCallHierarchyLayer1GenericOwner(observation callHierarchyLayer1GenericObservation) *callHierarchyLayer1GenericOwner {\n\to := &callHierarchyLayer1GenericOwner{observation: copyGenericObservation(observation)}','func newCallHierarchyLayer1GenericOwner(observation callHierarchyLayer1GenericObservation, directional *privateCallHierarchyDirectionalTerminalV1) *callHierarchyLayer1GenericOwner {\n\to := &callHierarchyLayer1GenericOwner{observation: copyGenericObservation(observation), directionalRawV1: directional}','owner constructor')
 replace_once(owner,'\to.releaseStarted = true\n\to.observation = callHierarchyLayer1GenericObservation{}\n','\to.releaseStarted = true\n\tif o.directionalRawV1 != nil {\n\t\to.directionalRawV1.releaseLocked()\n\t\to.directionalRawV1 = nil\n\t}\n\to.observation = callHierarchyLayer1GenericObservation{}\n','owner release')
 replace_once(owner,'\to := newCallHierarchyLayer1GenericOwner(a.finishAccepted(result, result.Failure, "", callHierarchyLayer1GenericAfterSnapshot{ledger: a.before.ledger}))\n\tm.mu.Lock()\n\tresult, terminal := m.publishCallHierarchyLayer1GenericOwnerLocked(result, o)','\tm.mu.Lock()\n\to := newCallHierarchyLayer1GenericOwner(a.finishAccepted(result, result.Failure, "", callHierarchyLayer1GenericAfterSnapshot{ledger: a.before.ledger}), m.newDirectionalTerminalV1Locked())\n\tresult, terminal := m.publishCallHierarchyLayer1GenericOwnerLocked(result, o)','refusal owner binding')
 lines=['package sessionruntime','','import "lsp-trace/internal/session"','','const privateCallHierarchyDirectionalMaxBytesV1 uint64 = 32 << 20','','type privateCallHierarchyDirectionalLedgerV1 struct {','\taccount *privateB4ByteAccountV2','\tattempts map[privateCallHierarchyDirectionalAttemptIDV1]*privateCallHierarchyDirectionalAccountV1','\tsequence uint64','}','','type privateCallHierarchyDirectionalAttemptIDV1 struct { Sequence uint64 }','type privateCallHierarchyDirectionalAccountV1 struct {','\tc15 *privateB4ByteAccountV2','\tleases []privateB4ByteLeaseV2','\tterminal bool','}','type privateCallHierarchyDirectionalTerminalV1 struct { account *privateCallHierarchyDirectionalAccountV1 }','','func (t *privateCallHierarchyDirectionalTerminalV1) releaseLocked() {','\tif t == nil || t.account == nil || t.account.terminal { return }','\tt.account.terminal = true','\tfor i := range t.account.leases { t.account.leases[i].release(); t.account.leases[i] = privateB4ByteLeaseV2{} }','\tt.account.leases = nil','}','','func (m *Manager) newDirectionalTerminalV1Locked() *privateCallHierarchyDirectionalTerminalV1 {','\tl := &m.directionalRawV1','\tif l.attempts == nil { l.attempts = make(map[privateCallHierarchyDirectionalAttemptIDV1]*privateCallHierarchyDirectionalAccountV1) }','\tfor id, attempt := range l.attempts { if attempt.terminal { delete(l.attempts, id) } }','\tl.sequence++','\tattempt := &privateCallHierarchyDirectionalAccountV1{c15: l.account}','\tl.attempts[privateCallHierarchyDirectionalAttemptIDV1{Sequence: l.sequence}] = attempt','\treturn &privateCallHierarchyDirectionalTerminalV1{account: attempt}','}','','func reserveDirectionalComponentV1(account *privateCallHierarchyDirectionalAccountV1, bytes uint64) (privateB4ByteLeaseV2, session.Failure) {','\tif account == nil || account.c15 == nil || account.terminal || bytes == 0 || bytes > privateCallHierarchyDirectionalMaxBytesV1 { return privateB4ByteLeaseV2{}, session.ResourceExhausted }','\tlease, failure := account.c15.reserve(bytes)','\tif failure == "" { account.leases = append(account.leases, lease) }','\treturn lease, failure','}','','func copyDirectionalComponentV1(lease privateB4ByteLeaseV2, source []byte) ([]byte, privateB4ByteLeaseV2, session.Failure) {','\tif lease.account == nil || uint64(len(source)) > privateCallHierarchyDirectionalMaxBytesV1 { return nil, privateB4ByteLeaseV2{}, session.ResourceExhausted }','\treturn append([]byte(nil), source...), lease, ""','}','','func aliasDirectionalComponentV1(lease privateB4ByteLeaseV2) (privateB4ByteLeaseV2, session.Failure) {','\talias := lease.alias(); if alias.account == nil { return privateB4ByteLeaseV2{}, session.ResourceExhausted }; return alias, ""','}','','func transferDirectionalAcquisitionCoreV1(lease *privateB4ByteLeaseV2) (privateB4ByteLeaseV2, session.Failure) {','\tif lease == nil || lease.account == nil { return privateB4ByteLeaseV2{}, session.ResourceExhausted }; out := *lease; *lease = privateB4ByteLeaseV2{}; return out, ""','}','','func releaseDirectionalTerminalCoreV1(lease *privateB4ByteLeaseV2) bool {','\tif lease == nil || lease.account == nil { return false }; lease.release(); *lease = privateB4ByteLeaseV2{}; return true','}','']
 for r in rows:
  sym=r['symbol'].replace('V3','V1'); rid=r['stage_a_row_id']; status=r['planned_status']
  lines += [f'// {rid} {status}',f'const privateStageA{sym[0].upper()+sym[1:]} = "{rid}"']
  if status=='PLANNED_RESERVATION': lines += [f'func {sym}(a *privateCallHierarchyDirectionalAccountV1, n uint64) (privateB4ByteLeaseV2, session.Failure) {{ return reserveDirectionalComponentV1(a, n) }}']
  elif status=='PLANNED_COPY': lines += [f'func {sym}(l privateB4ByteLeaseV2, b []byte) ([]byte, privateB4ByteLeaseV2, session.Failure) {{ return copyDirectionalComponentV1(l, b) }}']
  elif status=='PLANNED_ALIAS': lines += [f'func {sym}(l privateB4ByteLeaseV2) (privateB4ByteLeaseV2, session.Failure) {{ return aliasDirectionalComponentV1(l) }}']
  elif status=='PLANNED_TRANSFER': lines += [f'func {sym}(l *privateB4ByteLeaseV2) (privateB4ByteLeaseV2, session.Failure) {{ return transferDirectionalAcquisitionCoreV1(l) }}']
  else: lines += [f'func {sym}(l *privateB4ByteLeaseV2) bool {{ return releaseDirectionalTerminalCoreV1(l) }}']
  lines.append('')
 (work/FROZEN[0]).write_text('\n'.join(lines))
 run(['gofmt','-w',FROZEN[0],FROZEN[1],FROZEN[4]],work)
 run(['git','add','-N',FROZEN[0]],work)
 patch=run(['git','diff','--binary','--',*FROZEN],work)
 (HERE/'PROPOSED.patch').write_text(patch)
 with (HERE/'SOURCE_HASHES.tsv').open('w') as f:
  f.write('path\tpre_sha256\tpost_sha256\n')
  for p in FROZEN:
   src=ROOT/p; dst=work/p
   f.write(f'{p}\t{sha(src) if src.exists() else "ABSENT"}\t{sha(dst) if dst.exists() else "ABSENT"}\n')
 with (HERE/'STAGE_A_BRIDGE.tsv').open('w',newline='') as f:
  w=csv.writer(f,delimiter='\t',lineterminator='\n'); w.writerow(['stage_a_row_id','planned_status','production_symbol'])
  for r in rows: w.writerow([r['stage_a_row_id'],r['planned_status'],r['symbol'].replace('V3','V1')])
 with (HERE/'COMPILE_ENV.txt').open('w') as f:
  f.write(run(['go','version'],work)); f.write(run(['go','env','GOOS','GOARCH','GOVERSION','CGO_ENABLED'],work))
 raw=run(['go','list','-json','./...'],work)
 decoder=json.JSONDecoder(); offset=0; packages=[]
 while offset < len(raw):
  while offset < len(raw) and raw[offset].isspace(): offset += 1
  if offset >= len(raw): break
  obj, offset=decoder.raw_decode(raw,offset); packages.append(obj)
 selected={'lsp-trace/sessionruntime','lsp-trace/internal/lspwire'}
 changed=True
 while changed:
  changed=False
  for pkg in packages:
   path=pkg.get('ImportPath','')
   if path not in selected and selected.intersection(pkg.get('Imports',[])):
    selected.add(path); changed=True
 closure=sorted(selected)
 with (HERE/'COMPILE.log').open('w') as log:
  log.write('PRODUCTION_SPLICE_COMPILE_ONLY\n')
  log.write('MECHANICALLY_DERIVED_REVERSE_DEPENDENT_CLOSURE\n')
  log.write('ALL_PACKAGE_ATTEMPT_BASELINE_DOCUMENTATION_FAILURE_OBSERVED\n')
  for package in closure: log.write('CLOSURE '+package+'\n')
  run(['go','test','-run','^$',*closure],work,log)
  run(['go','vet','./sessionruntime','./internal/lspwire'],work,log)
finally:
 shutil.rmtree(work,ignore_errors=True)
print('CORRECTED_PROPOSAL_BUILT')
