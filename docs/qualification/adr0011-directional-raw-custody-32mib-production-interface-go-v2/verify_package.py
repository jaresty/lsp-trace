#!/usr/bin/env python3
import csv, hashlib, json, pathlib, re, shutil, subprocess, tempfile

HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[2]
HEAD="ff74ed966d9580e60ea10a3cad887f92449aa755"
TOKEN="ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRODUCTION_INTERFACE_V2_GO"
BINDING=HERE.parent/"adr0011-directional-raw-custody-32mib-private-interface-go-binding-v1"
ARCHITECTURE=HERE.parent/"adr0011-directional-raw-custody-c15-ledger-partition-go-v1"
FROZEN={
 "sessionruntime/call_hierarchy_directional_raw_v1_private.go",
 "sessionruntime/sessionruntime.go",
 "sessionruntime/b4_explicit_bytes_c15_private.go",
 "sessionruntime/b4_owned_method_pair_lease.go",
 "sessionruntime/b4_c15_ownership_registry_private.go",
 "internal/lspwire/read_frame.go",
 "internal/lspwire/successor_ingress.go",
}
CHANGED={
 "sessionruntime/call_hierarchy_directional_raw_v1_private.go",
 "sessionruntime/sessionruntime.go",
 "sessionruntime/b4_explicit_bytes_c15_private.go",
 "sessionruntime/b4_owned_method_pair_lease.go",
}
REQUIRED={"README.md","INTERFACE.md","OWNERSHIP_LIFECYCLE.md","WRITESET.md","REVIEW_REQUEST.md","PROPOSED.patch","STAGE_A_BRIDGE.tsv","SOURCE_HASHES.tsv","DEPENDENCIES.sha256","COMPILE.log","COMPILE_ENV.txt","build_proposal.py","verify_package.py","SHA256SUMS"}

def req(name, condition):
 print(("PASS " if condition else "FAIL ")+name)
 if not condition: raise AssertionError(name)
def run(args,cwd,expect=True):
 print("$ "+" ".join(args)); p=subprocess.run(args,cwd=cwd,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT); print(p.stdout,end="")
 if expect and p.returncode: raise AssertionError("COMMAND_"+"_".join(args))
 return p
def sha(p): return hashlib.sha256(p.read_bytes()).hexdigest()
def plus(patch): return "\n".join(x[1:] for x in patch.splitlines() if x.startswith("+") and not x.startswith("+++"))
def paths(patch): return set(re.findall(r"^diff --git a/(\S+) b/\S+$",patch,re.M))
def reject(name,fn):
 try: fn()
 except AssertionError: print("PASS ADVERSARIAL_"+name+"_REJECTED"); return
 raise AssertionError("ADVERSARIAL_"+name+"_ACCEPTED")

def static_verify(patch, bridge):
 p=plus(patch)
 req("PATCH_WITHIN_EXPANDED_SET",paths(patch)<=FROZEN)
 req("PATCH_EXACT_CHANGED_SET",paths(patch)==CHANGED)
 req("NO_TEST_PATH",not any(x.endswith("_test.go") for x in paths(patch)))
 req("GENERAL_CONSTANT_UNCHANGED",not re.search(r"^\+[^\n]*\bprivateB4MaxOwnedBytes\b[^\n]*=",patch,re.M) and "privateB4MaxOwnedBytes" in p)
 req("LEGACY_24600_PRESERVED","unsafe.Sizeof(privateB4ByteLedger{}) == 24600" not in p)
 req("DISTINCT_PARTITIONS",all(x in p for x in ["privateB4PartitionGeneralC15V2","privateB4PartitionDirectionalRawV1","privateB4DirectionalPartitionV1"]))
 req("LIMITS",p.count("32 << 20")>=1 and "64 << 20" in p)
 req("ONE_SLOT","d.occupied" in p and "len(a.leases) != 0" in p)
 req("CHECKED_COMBINED","general > math.MaxUint64-capacity" in p and "general+capacity > privateCallHierarchyCombinedMaxBytesV1" in p)
 req("PRE_EFFECT_CUMULATIVE_REFUSAL",p.index("d.cumulative > math.MaxUint64-capacity") < p.index("d.generation++"))
 req("REAL_ACCOUNT","newPrivateB4ByteAccountV2()" in p and "*privateB4ByteAccountV2" in p)
 req("OPAQUE_PARTITION_LEASE","type privateB4DirectionalLeaseV1 struct" in p and re.search(r"partition\s+privateB4PartitionIDV1",p) is not None)
 req("SAME_PARTITION_ALIAS","l.partition != privateB4PartitionDirectionalRawV1" in p)
 req("CROSS_TRANSFER_REFUSAL","destination != source.partition" in p)
 req("NO_SHARED_GENERAL_RESERVE","a.table.general.reserve(capacity)" not in p and "a.account.reserveDirectionalV1(n)" in p)
 req("LIFECYCLE_WIRED",p.count("m.newDirectionalTerminalV1Locked()") == 2 and "o.directionalRawV1.releaseLocked()" in p and "m.directionalRawV1.account.requestTerminalRelease()" in p)
 req("EXACT20",len(bridge)==20 and len({r["stage_a_row_id"] for r in bridge})==20 and len({r["production_symbol"] for r in bridge})==20)
 req("PLANNED_ONLY",all(r["planned_status"].startswith("PLANNED_") for r in bridge))
 for row in bridge:
  req("BRIDGE_"+row["production_symbol"],row["stage_a_row_id"] in patch and row["planned_status"] in patch and re.search(r"func "+re.escape(row["production_symbol"])+r"\(",patch) is not None)

req("HEAD",run(["git","rev-parse","HEAD"],ROOT).stdout.strip()==HEAD)
status=run(["git","status","--porcelain","--untracked-files=all"],ROOT).stdout.splitlines()
prefix=str(HERE.relative_to(ROOT))+"/"
req("STATUS_ONLY_PACKAGE",bool(status) and all(line[3:].startswith(prefix) for line in status))
req("PACKAGE_FILES",{p.name for p in HERE.iterdir() if p.is_file()}==REQUIRED)
patch=(HERE/"PROPOSED.patch").read_text()
bridge=list(csv.DictReader((HERE/"STAGE_A_BRIDGE.tsv").open(),delimiter="\t"))
binding=list(csv.DictReader((BINDING/"BRIDGE.tsv").open(),delimiter="\t"))
derived=[{"stage_a_row_id":r["stage_a_row_id"],"planned_status":r["planned_status"],"production_symbol":r["symbol"].removesuffix("V3")+"V1"} for r in binding]
req("BRIDGE_DERIVED_FROM_IMMUTABLE_BINDING",bridge==derived)
expected_dependencies=[
 (sha(BINDING/"SHA256SUMS"),"IMMUTABLE_BINDING_MANIFEST",str((BINDING/"SHA256SUMS").relative_to(ROOT))),
 (sha(BINDING/"BRIDGE.tsv"),"IMMUTABLE_BINDING_BRIDGE",str((BINDING/"BRIDGE.tsv").relative_to(ROOT))),
 (sha(ARCHITECTURE/"SHA256SUMS"),"IMMUTABLE_ARCHITECTURE_MANIFEST",str((ARCHITECTURE/"SHA256SUMS").relative_to(ROOT))),
]
expected_dependencies.extend((sha(ROOT/path) if (ROOT/path).exists() else "ABSENT","EXACT_HEAD_SOURCE",path) for path in sorted(FROZEN))
actual_dependencies=[tuple(line.split(None,2)) for line in (HERE/"DEPENDENCIES.sha256").read_text().splitlines()]
req("IMMUTABLE_DEPENDENCIES_DERIVED",actual_dependencies==expected_dependencies)
static_verify(patch,bridge)
reject("SIMPLE_CONSTANT_INCREASE",lambda: static_verify(patch+"\n+const privateB4MaxOwnedBytes = uint64(64 << 20)\n",bridge))
reject("SHARED_BUCKET",lambda: static_verify(patch.replace("a.account.reserveDirectionalV1(n)","a.account.reserve(n)"),bridge))
reject("CROSS_TRANSFER",lambda: static_verify(patch.replace("destination != source.partition","false"),bridge))
reject("UNWIRED_ACCEPTED",lambda: static_verify(patch.replace("m.newDirectionalTerminalV1Locked()","nil",1),bridge))
reject("EXTRA_FILE",lambda: static_verify(patch+"\ndiff --git a/sessionruntime/extra.go b/sessionruntime/extra.go\n",bridge))
fake=pathlib.Path(tempfile.mkdtemp(prefix="adr0011-v2-fake-log-"))/"COMPILE.log"; fake.write_text("ALL PASSED\n")
req("FORGED_LOG_NOT_AUTHORITY",fake.read_text() != (HERE/"COMPILE.log").read_text() and "ALL PASSED" not in patch); shutil.rmtree(fake.parent)

with (HERE/"SOURCE_HASHES.tsv").open() as f: hashes=list(csv.DictReader(f,delimiter="\t"))
req("HASH_PATHS",{r["path"] for r in hashes}==FROZEN)
for row in hashes:
 p=ROOT/row["path"]; req("PRE_"+row["path"],row["pre_sha256"]==("ABSENT" if not p.exists() else sha(p)))

probe=r'''package sessionruntime
import (
 "context"
 "testing"
)
func TestDirectionalV2ExactLimitAliasTransferReleaseReuse(t *testing.T) {
 a,f:=newPrivateB4ByteAccountV2(); if f!=""{t.Fatal(f)}
 baseline:=a.snapshot().Live; if baseline!=privateB4ByteLedgerV2TableBytes{t.Fatalf("baseline=%d",baseline)}
 l,f:=a.reserveDirectionalV1(privateCallHierarchyDirectionalMaxBytesV1); if f!=""||!l.active{t.Fatalf("exact: %v",f)}
 if _,f:=a.reserveDirectionalV1(1); f==""{t.Fatal("second directional slot admitted")}
 alias:=l.alias(); if !alias.active||alias.partition!=l.partition{t.Fatal("same-partition alias failed")}
 if _,f:=transferDirectionalV1(&alias,privateB4PartitionGeneralC15V2); f==""||!alias.active{t.Fatal("cross transfer changed source")}
 moved,f:=transferDirectionalV1(&alias,privateB4PartitionDirectionalRawV1); if f!=""||alias.active||!moved.active{t.Fatal("same-partition transfer")}
 if l.release(){t.Fatal("stale alias released transferred lease")}
 if !moved.release(){t.Fatal("transferred release")}
 l2,f:=a.reserveDirectionalV1(privateCallHierarchyDirectionalMaxBytesV1); if f!=""||l2.generation==l.generation{t.Fatal("fresh reuse")}
 l2.release(); a.requestTerminalRelease()
}
func TestDirectionalV2GeneralCombinedAndPlusOne(t *testing.T) {
 a,f:=newPrivateB4ByteAccountV2(); if f!=""{t.Fatal(f)}
 base:=a.snapshot().Live
 g,f:=a.reserve(privateB4MaxOwnedBytes-base); if f!=""{t.Fatal(f)}
 d,f:=a.reserveDirectionalV1(privateCallHierarchyDirectionalMaxBytesV1); if f!=""{t.Fatal(f)}
 if a.snapshot().Live!=privateB4MaxOwnedBytes || a.directional.live!=privateCallHierarchyDirectionalMaxBytesV1{t.Fatal("partition counters")}
 d.release(); g.Release(); a.requestTerminalRelease()
 b,_:=newPrivateB4ByteAccountV2(); if _,f:=b.reserveDirectionalV1(privateCallHierarchyDirectionalMaxBytesV1+1); f==""||b.directional.live!=0||b.directional.occupied{t.Fatal("plus one was not pre-effect")}
}
func TestDirectionalV2ManagerConstructionShutdown(t *testing.T) {
 m,err:=New(Config{Limits:Limits{MaxSessions:1,MaxRequests:1,MaxChildren:1,MaxCancels:1,MaxTombstones:1,MaxObservations:8,MaxOperations:1},Starter:ManagedStarter{}}); if err!=nil{t.Fatal(err)}
 if m.directionalRawV1.account==nil||m.directionalRawV1.attempts==nil{t.Fatal("manager directional lifecycle unwired")}
 if err:=m.Shutdown(context.Background()); err!=nil{t.Fatal(err)}
 if !m.directionalRawV1.account.terminalRequested{t.Fatal("manager shutdown did not request release")}
}
func TestDirectionalV2TerminalBinding(t *testing.T) {
 a,_:=newPrivateB4ByteAccountV2(); m:=&Manager{directionalRawV1:privateCallHierarchyDirectionalLedgerV1{account:a,attempts:map[privateCallHierarchyDirectionalAttemptIDV1]*privateCallHierarchyDirectionalAttemptV1{}}}
 term:=m.newDirectionalTerminalV1Locked(); l,f:=reserveDirectionalComponentV1(term.attempt,32); if f!=""{t.Fatal(f)}
 moved,f:=transferDirectionalV1(&l,privateB4PartitionDirectionalRawV1); if f!=""||!moved.active{t.Fatal("transfer before terminal")}
 o:=newCallHierarchyLayer1GenericOwner(callHierarchyLayer1GenericObservation{},term); o.requestTerminal(); o.waitReleased()
 if moved.account.directional.occupied||moved.account.directional.live!=0||!term.attempt.terminal{t.Fatal("terminal did not settle transferred custody")}
 if moved.release(){t.Fatal("terminal left transferred holder releasable")}
 next:=m.newDirectionalTerminalV1Locked(); if next.attempt==term.attempt||m.directionalRawV1.sequence!=2{t.Fatal("attempt reuse identity")}
}
'''
work=pathlib.Path(tempfile.mkdtemp(prefix="adr0011-production-v2-verify-"))
try:
 run(["git","clone","--quiet","--no-hardlinks",str(ROOT),str(work)],ROOT)
 run(["git","checkout","--quiet",HEAD],work)
 run(["git","apply","--check",str(HERE/"PROPOSED.patch")],work)
 run(["git","apply",str(HERE/"PROPOSED.patch")],work)
 raw=run(["go","list","-json","./..."],work).stdout; dec=json.JSONDecoder(); off=0; packages=[]
 while off<len(raw):
  while off<len(raw) and raw[off].isspace(): off+=1
  if off>=len(raw): break
  obj,off=dec.raw_decode(raw,off); packages.append(obj)
 selected={"lsp-trace/sessionruntime","lsp-trace/internal/lspwire"}; changed=True
 while changed:
  changed=False
  for pkg in packages:
   ip=pkg.get("ImportPath","")
   if ip not in selected and selected.intersection(pkg.get("Imports",[])): selected.add(ip); changed=True
 req("CLOSURE_SEEDS",{"lsp-trace/sessionruntime","lsp-trace/internal/lspwire"}<=selected)
 run(["go","test","-run","^$",*sorted(selected)],work)
 run(["go","vet",*sorted(selected)],work)
 probe_path=work/"sessionruntime/directional_v2_disposable_private_test.go"; probe_path.write_text(probe)
 run(["gofmt","-w",str(probe_path)],work)
 run(["go","test","-count=1","./sessionruntime","-run","^TestDirectionalV2"],work)
 run(["go","test","-race","-count=1","./sessionruntime","-run","^TestDirectionalV2"],work)
 req("FOCUSED_PRIVATE_LIFECYCLE_PROBES",True)
finally: shutil.rmtree(work,ignore_errors=True)

req("LOG_LABEL",(HERE/"COMPILE.log").read_text().startswith("PRODUCTION_INTERFACE_LIFECYCLE_PLUMBING_ONLY\n"))
docs="\n".join((HERE/n).read_text() for n in ["README.md","INTERFACE.md","OWNERSHIP_LIFECYCLE.md","WRITESET.md","REVIEW_REQUEST.md"])
req("TOKEN_AND_BOUNDARY",TOKEN in docs and docs.count("AUTHORED_NOT_GO")>=5 and "semantic C15 closure" in docs)
for line in (HERE/"SHA256SUMS").read_text().splitlines():
 h,name=line.split(None,1); req("SUM_"+name,sha(HERE/name)==h)
print("PRODUCTION_INTERFACE_V2_PACKAGE_VERIFY_OK")
