#!/usr/bin/env python3
import csv, hashlib, json, pathlib, shutil, subprocess, tempfile

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]
HEAD = "ff74ed966d9580e60ea10a3cad887f92449aa755"
BINDING = HERE.parent / "adr0011-directional-raw-custody-32mib-private-interface-go-binding-v1"
ARCHITECTURE = HERE.parent / "adr0011-directional-raw-custody-c15-ledger-partition-go-v1"
BRIDGE = BINDING / "BRIDGE.tsv"
FROZEN = [
    "sessionruntime/call_hierarchy_directional_raw_v1_private.go",
    "sessionruntime/sessionruntime.go",
    "sessionruntime/b4_explicit_bytes_c15_private.go",
    "sessionruntime/b4_owned_method_pair_lease.go",
    "sessionruntime/b4_c15_ownership_registry_private.go",
    "internal/lspwire/read_frame.go",
    "internal/lspwire/successor_ingress.go",
]
CHANGED = set(FROZEN[:4])

def run(args, cwd, log=None):
    p = subprocess.run(args, cwd=cwd, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    if log is not None:
        log.write("$ " + " ".join(args) + "\n" + p.stdout + "\n")
    if p.returncode:
        raise SystemExit(f"command failed {args}:\n{p.stdout}")
    return p.stdout

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def replace_once(path, old, new, label):
    text = path.read_text()
    if text.count(old) != 1:
        raise SystemExit(label + " seam mismatch")
    path.write_text(text.replace(old, new, 1))

if run(["git", "rev-parse", "HEAD"], ROOT).strip() != HEAD:
    raise SystemExit("HEAD mismatch")
dirty = run(["git", "status", "--porcelain"], ROOT).splitlines()
if any(not line[3:].startswith(str(HERE.relative_to(ROOT)) + "/") for line in dirty):
    raise SystemExit("dirty outside successor package")
binding_rows = list(csv.DictReader(BRIDGE.open(), delimiter="\t"))
if len(binding_rows) != 20 or len({r["stage_a_row_id"] for r in binding_rows}) != 20 or not all(r["planned_status"].startswith("PLANNED_") for r in binding_rows):
    raise SystemExit("accepted binding bridge is not exact20 PLANNED-only")
rows = [{"stage_a_row_id": r["stage_a_row_id"], "planned_status": r["planned_status"], "production_symbol": r["symbol"].removesuffix("V3") + "V1"} for r in binding_rows]
if len({r["production_symbol"] for r in rows}) != 20:
    raise SystemExit("derived production symbols are not exact20 unique")

work = pathlib.Path(tempfile.mkdtemp(prefix="adr0011-production-v2-build-"))
try:
    run(["git", "clone", "--quiet", "--no-hardlinks", str(ROOT), str(work)], ROOT)
    run(["git", "checkout", "--quiet", HEAD], work)

    account = work / "sessionruntime/b4_explicit_bytes_c15_private.go"
    replace_once(account,
        "\tc15ZeroLeases      []privateB4C15LeaseState\n}",
        "\tc15ZeroLeases      []privateB4C15LeaseState\n\tdirectional         privateB4DirectionalPartitionV1\n}",
        "account directional partition")

    manager = work / "sessionruntime/sessionruntime.go"
    replace_once(manager,
        "\tcallHierarchyLayer1GenericOwners             []*callHierarchyLayer1GenericOwner\n",
        "\tcallHierarchyLayer1GenericOwners             []*callHierarchyLayer1GenericOwner\n\tdirectionalRawV1                             privateCallHierarchyDirectionalLedgerV1\n",
        "Manager directional field")
    replace_once(manager,
        "\tdiagnosticOwner, failure := newManagerDiagnosticOwner()\n\tif failure != \"\" {\n\t\treturn nil, errors.New(\"sessionruntime: manager diagnostic owner unavailable\")\n\t}\n\treturn &Manager{",
        "\tdiagnosticOwner, failure := newManagerDiagnosticOwner()\n\tif failure != \"\" {\n\t\treturn nil, errors.New(\"sessionruntime: manager diagnostic owner unavailable\")\n\t}\n\tdirectionalAccount, failure := newPrivateB4ByteAccountV2()\n\tif failure != \"\" {\n\t\treturn nil, errors.New(\"sessionruntime: directional raw account unavailable\")\n\t}\n\treturn &Manager{directionalRawV1: privateCallHierarchyDirectionalLedgerV1{account: directionalAccount, attempts: make(map[privateCallHierarchyDirectionalAttemptIDV1]*privateCallHierarchyDirectionalAttemptV1)}, ",
        "Manager construction")
    replace_once(manager,
        "result, genericTerminal = m.publishCallHierarchyLayer1GenericOwnerLocked(result, newCallHierarchyLayer1GenericOwner(observation))",
        "result, genericTerminal = m.publishCallHierarchyLayer1GenericOwnerLocked(result, newCallHierarchyLayer1GenericOwner(observation, m.newDirectionalTerminalV1Locked()))",
        "accepted owner binding")
    replace_once(manager,
        "\tfor _, genericOwner := range genericOwners {\n\t\tgenericOwner.requestTerminal()\n\t\tgenericOwner.waitReleased()\n\t}\n\treturn nil\n",
        "\tfor _, genericOwner := range genericOwners {\n\t\tgenericOwner.requestTerminal()\n\t\tgenericOwner.waitReleased()\n\t}\n\tif m.directionalRawV1.account != nil {\n\t\tm.directionalRawV1.account.requestTerminalRelease()\n\t}\n\treturn nil\n",
        "Shutdown account terminal")

    owner = work / "sessionruntime/b4_owned_method_pair_lease.go"
    replace_once(owner,
        "\tobservation                               callHierarchyLayer1GenericObservation\n",
        "\tobservation                               callHierarchyLayer1GenericObservation\n\tdirectionalRawV1                          *privateCallHierarchyDirectionalTerminalV1\n",
        "owner terminal field")
    replace_once(owner,
        "func newCallHierarchyLayer1GenericOwner(observation callHierarchyLayer1GenericObservation) *callHierarchyLayer1GenericOwner {\n\to := &callHierarchyLayer1GenericOwner{observation: copyGenericObservation(observation)}",
        "func newCallHierarchyLayer1GenericOwner(observation callHierarchyLayer1GenericObservation, directional *privateCallHierarchyDirectionalTerminalV1) *callHierarchyLayer1GenericOwner {\n\to := &callHierarchyLayer1GenericOwner{observation: copyGenericObservation(observation), directionalRawV1: directional}",
        "owner constructor")
    replace_once(owner,
        "\to.releaseStarted = true\n\to.observation = callHierarchyLayer1GenericObservation{}\n",
        "\to.releaseStarted = true\n\tif o.directionalRawV1 != nil {\n\t\to.directionalRawV1.releaseLocked()\n\t\to.directionalRawV1 = nil\n\t}\n\to.observation = callHierarchyLayer1GenericObservation{}\n",
        "owner release")
    replace_once(owner,
        "\to := newCallHierarchyLayer1GenericOwner(a.finishAccepted(result, result.Failure, \"\", callHierarchyLayer1GenericAfterSnapshot{ledger: a.before.ledger}))\n\tm.mu.Lock()\n\tresult, terminal := m.publishCallHierarchyLayer1GenericOwnerLocked(result, o)",
        "\tm.mu.Lock()\n\to := newCallHierarchyLayer1GenericOwner(a.finishAccepted(result, result.Failure, \"\", callHierarchyLayer1GenericAfterSnapshot{ledger: a.before.ledger}), m.newDirectionalTerminalV1Locked())\n\tresult, terminal := m.publishCallHierarchyLayer1GenericOwnerLocked(result, o)",
        "refusal owner binding")

    lines = '''package sessionruntime

import (
    "math"
    "lsp-trace/internal/session"
)

const (
    privateCallHierarchyDirectionalMaxBytesV1 uint64 = 32 << 20
    privateCallHierarchyCombinedMaxBytesV1 uint64 = 64 << 20
)

type privateB4PartitionIDV1 uint8
const (
    privateB4PartitionGeneralC15V2 privateB4PartitionIDV1 = iota + 1
    privateB4PartitionDirectionalRawV1
)

type privateB4DirectionalPartitionV1 struct {
    live uint64
    cumulative uint64
    generation uint64
    holder uint64
    occupied bool
}

type privateB4DirectionalLeaseV1 struct {
    account *privateB4ByteAccountV2
    partition privateB4PartitionIDV1
    generation uint64
    holder uint64
    capacity uint64
    active bool
}

type privateCallHierarchyDirectionalLedgerV1 struct {
    account *privateB4ByteAccountV2
    attempts map[privateCallHierarchyDirectionalAttemptIDV1]*privateCallHierarchyDirectionalAttemptV1
    sequence uint64
}
type privateCallHierarchyDirectionalAttemptIDV1 struct { Sequence uint64 }
type privateCallHierarchyDirectionalAttemptV1 struct {
    account *privateB4ByteAccountV2
    leases []privateB4DirectionalLeaseV1
    terminal bool
}
type privateCallHierarchyDirectionalTerminalV1 struct { attempt *privateCallHierarchyDirectionalAttemptV1 }

func (a *privateB4ByteAccountV2) reserveDirectionalV1(capacity uint64) (privateB4DirectionalLeaseV1, session.Failure) {
    if a == nil || capacity == 0 { return privateB4DirectionalLeaseV1{}, session.ResourceExhausted }
    a.mu.Lock()
    defer a.mu.Unlock()
    d := &a.directional
    general := a.table.general.live
    if a.terminalRequested || a.backingReleased || d.occupied || d.generation == math.MaxUint64 { return privateB4DirectionalLeaseV1{}, session.ResourceExhausted }
    if capacity > privateCallHierarchyDirectionalMaxBytesV1 { return privateB4DirectionalLeaseV1{}, session.ResourceExhausted }
    if general > privateB4MaxOwnedBytes || general > math.MaxUint64-capacity || general+capacity > privateCallHierarchyCombinedMaxBytesV1 { return privateB4DirectionalLeaseV1{}, session.ResourceExhausted }
    if d.cumulative > math.MaxUint64-capacity { return privateB4DirectionalLeaseV1{}, session.ResourceExhausted }
    d.generation++
    d.holder = d.generation
    d.live = capacity
    d.cumulative += capacity
    d.occupied = true
    return privateB4DirectionalLeaseV1{account:a, partition:privateB4PartitionDirectionalRawV1, generation:d.generation, holder:d.holder, capacity:capacity, active:true}, ""
}
func (l privateB4DirectionalLeaseV1) alias() privateB4DirectionalLeaseV1 {
    if l.account == nil || l.partition != privateB4PartitionDirectionalRawV1 || !l.active { return privateB4DirectionalLeaseV1{} }
    l.account.mu.Lock(); defer l.account.mu.Unlock()
    d := &l.account.directional
    if !d.occupied || d.generation != l.generation || d.holder != l.holder || d.live != l.capacity { return privateB4DirectionalLeaseV1{} }
    return l
}
func transferDirectionalV1(source *privateB4DirectionalLeaseV1, destination privateB4PartitionIDV1) (privateB4DirectionalLeaseV1, session.Failure) {
    if source == nil || source.account == nil || source.partition != privateB4PartitionDirectionalRawV1 || destination != source.partition || !source.active { return privateB4DirectionalLeaseV1{}, session.ResourceExhausted }
    a := source.account
    a.mu.Lock()
    defer a.mu.Unlock()
    d := &a.directional
    if !d.occupied || d.generation != source.generation || d.holder != source.holder || d.live != source.capacity || d.holder == math.MaxUint64 { return privateB4DirectionalLeaseV1{}, session.ResourceExhausted }
    d.holder++
    out := *source
    out.holder = d.holder
    source.active = false
    return out, ""
}
func (l *privateB4DirectionalLeaseV1) release() bool {
    if l == nil || l.account == nil || l.partition != privateB4PartitionDirectionalRawV1 || !l.active { return false }
    a := l.account; a.mu.Lock(); defer a.mu.Unlock(); d := &a.directional
    if !d.occupied || d.generation != l.generation || d.holder != l.holder || d.live != l.capacity { l.active=false; return false }
    d.live=0; d.occupied=false; l.active=false; return true
}
func releaseDirectionalGenerationV1(l *privateB4DirectionalLeaseV1) bool {
    if l == nil || l.account == nil || l.partition != privateB4PartitionDirectionalRawV1 { return false }
    a := l.account
    a.mu.Lock()
    defer a.mu.Unlock()
    d := &a.directional
    if !d.occupied || d.generation != l.generation || d.live != l.capacity { l.active=false; return false }
    d.live=0
    d.occupied=false
    l.active=false
    return true
}
func (t *privateCallHierarchyDirectionalTerminalV1) releaseLocked() {
    if t == nil || t.attempt == nil || t.attempt.terminal { return }
    t.attempt.terminal=true
    for i := range t.attempt.leases { releaseDirectionalGenerationV1(&t.attempt.leases[i]); t.attempt.leases[i]=privateB4DirectionalLeaseV1{} }
    t.attempt.leases=nil
}
func (m *Manager) newDirectionalTerminalV1Locked() *privateCallHierarchyDirectionalTerminalV1 {
    ledger := &m.directionalRawV1
    for id, attempt := range ledger.attempts { if attempt.terminal { delete(ledger.attempts,id) } }
    ledger.sequence++
    attempt := &privateCallHierarchyDirectionalAttemptV1{account:ledger.account}
    ledger.attempts[privateCallHierarchyDirectionalAttemptIDV1{Sequence:ledger.sequence}] = attempt
    return &privateCallHierarchyDirectionalTerminalV1{attempt:attempt}
}
func reserveDirectionalComponentV1(a *privateCallHierarchyDirectionalAttemptV1, n uint64) (privateB4DirectionalLeaseV1, session.Failure) {
    if a == nil || a.account == nil || a.terminal || len(a.leases) != 0 { return privateB4DirectionalLeaseV1{}, session.ResourceExhausted }
    lease, failure := a.account.reserveDirectionalV1(n); if failure == "" { a.leases=append(a.leases,lease) }; return lease,failure
}
func copyDirectionalComponentV1(l privateB4DirectionalLeaseV1, source []byte) ([]byte, privateB4DirectionalLeaseV1, session.Failure) {
    if uint64(len(source)) > l.capacity || !l.alias().active { return nil,privateB4DirectionalLeaseV1{},session.ResourceExhausted }
    return append([]byte(nil),source...),l,""
}
func aliasDirectionalComponentV1(l privateB4DirectionalLeaseV1) (privateB4DirectionalLeaseV1, session.Failure) { a:=l.alias(); if !a.active{return privateB4DirectionalLeaseV1{},session.ResourceExhausted}; return a,"" }
func releaseDirectionalTerminalCoreV1(l *privateB4DirectionalLeaseV1) bool { return l.release() }
'''.splitlines()
    for row in rows:
        symbol = row["production_symbol"]
        rid, status = row["stage_a_row_id"], row["planned_status"]
        lines += ["", f"// {rid} {status}", f'const privateStageA{symbol[0].upper()+symbol[1:]} = "{rid}"']
        if status == "PLANNED_RESERVATION":
            lines.append(f"func {symbol}(a *privateCallHierarchyDirectionalAttemptV1, n uint64) (privateB4DirectionalLeaseV1, session.Failure) {{ return reserveDirectionalComponentV1(a,n) }}")
        elif status == "PLANNED_COPY":
            lines.append(f"func {symbol}(l privateB4DirectionalLeaseV1, b []byte) ([]byte, privateB4DirectionalLeaseV1, session.Failure) {{ return copyDirectionalComponentV1(l,b) }}")
        elif status == "PLANNED_ALIAS":
            lines.append(f"func {symbol}(l privateB4DirectionalLeaseV1) (privateB4DirectionalLeaseV1, session.Failure) {{ return aliasDirectionalComponentV1(l) }}")
        elif status == "PLANNED_TRANSFER":
            lines.append(f"func {symbol}(l *privateB4DirectionalLeaseV1) (privateB4DirectionalLeaseV1, session.Failure) {{ return transferDirectionalV1(l,privateB4PartitionDirectionalRawV1) }}")
        else:
            lines.append(f"func {symbol}(l *privateB4DirectionalLeaseV1) bool {{ return releaseDirectionalTerminalCoreV1(l) }}")
    (work / FROZEN[0]).write_text("\n".join(lines) + "\n")
    run(["gofmt", "-w", *sorted(CHANGED)], work)
    run(["git", "add", "-N", FROZEN[0]], work)
    patch = run(["git", "diff", "--binary", "--", *FROZEN], work)
    (HERE / "PROPOSED.patch").write_text(patch)
    with (HERE / "SOURCE_HASHES.tsv").open("w") as out:
        out.write("path\tpre_sha256\tpost_sha256\n")
        for path in FROZEN:
            src, dst = ROOT/path, work/path
            out.write(f"{path}\t{sha(src) if src.exists() else 'ABSENT'}\t{sha(dst) if dst.exists() else 'ABSENT'}\n")
    with (HERE / "STAGE_A_BRIDGE.tsv").open("w", newline="") as out:
        writer=csv.writer(out,delimiter="\t",lineterminator="\n")
        writer.writerow(["stage_a_row_id","planned_status","production_symbol"])
        for row in rows: writer.writerow([row["stage_a_row_id"],row["planned_status"],row["production_symbol"]])
    dependencies = [
        (sha(BINDING / "SHA256SUMS"), "IMMUTABLE_BINDING_MANIFEST", str((BINDING / "SHA256SUMS").relative_to(ROOT))),
        (sha(BRIDGE), "IMMUTABLE_BINDING_BRIDGE", str(BRIDGE.relative_to(ROOT))),
        (sha(ARCHITECTURE / "SHA256SUMS"), "IMMUTABLE_ARCHITECTURE_MANIFEST", str((ARCHITECTURE / "SHA256SUMS").relative_to(ROOT))),
    ]
    dependencies.extend((sha(ROOT / path) if (ROOT / path).exists() else "ABSENT", "EXACT_HEAD_SOURCE", path) for path in sorted(FROZEN))
    (HERE / "DEPENDENCIES.sha256").write_text("".join(f"{digest}  {kind}  {path}\n" for digest, kind, path in dependencies))
    (HERE / "COMPILE_ENV.txt").write_text(run(["go","version"],work)+run(["go","env","GOOS","GOARCH","GOVERSION","CGO_ENABLED"],work))
    raw=run(["go","list","-json","./..."],work); decoder=json.JSONDecoder(); offset=0; packages=[]
    while offset < len(raw):
        while offset < len(raw) and raw[offset].isspace(): offset+=1
        if offset>=len(raw): break
        obj,offset=decoder.raw_decode(raw,offset); packages.append(obj)
    selected={"lsp-trace/sessionruntime","lsp-trace/internal/lspwire"}; changed=True
    while changed:
        changed=False
        for pkg in packages:
            path=pkg.get("ImportPath","")
            if path not in selected and selected.intersection(pkg.get("Imports",[])): selected.add(path); changed=True
    with (HERE/"COMPILE.log").open("w") as log:
        log.write("PRODUCTION_INTERFACE_LIFECYCLE_PLUMBING_ONLY\nMECHANICALLY_DERIVED_REVERSE_DEPENDENT_CLOSURE\n")
        for package in sorted(selected): log.write("CLOSURE "+package+"\n")
        run(["go","test","-run","^$",*sorted(selected)],work,log)
        run(["go","vet",*sorted(selected)],work,log)
finally:
    shutil.rmtree(work,ignore_errors=True)
print("PRODUCTION_INTERFACE_V2_PROPOSAL_BUILT")
