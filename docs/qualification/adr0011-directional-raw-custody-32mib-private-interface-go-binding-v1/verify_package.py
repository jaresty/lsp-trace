#!/usr/bin/env python3
import ast, copy, csv, hashlib, io, json, pathlib, re, shutil, subprocess, sys, tempfile

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]
HEAD = "d19ea0acf0f4ed4a344f60e5f20f75ef1c1b1063"
V3_REL = "docs/qualification/adr0011-directional-raw-custody-32mib-private-interface-go-v3"
BRIDGE_REL = "docs/qualification/adr0011-directional-raw-custody-c15-two-stage-bridge-v1"
CONTRACT_REL = "docs/qualification/adr0011-directional-raw-custody-c15-two-stage-contract-v2"
V3_MANIFEST = "02153887105c30cc6eaa3b976ec49e7f32b97afcf35ff6b8aaf185529e029704"
BRIDGE_MANIFEST = "4233ff8a64ee99888d620bf17c87f478ff664069df7ff9821ddad405843021af"
STAGE_A_SHA = "0c9909507b6abd5fd16034c70f7b34f1ae04719822af669ba8c3f373e4a49a1d"
IDENTITY_SHA = "3ec9e03b697b50d0db98019f10a502b8a3fc87929afe7d13b1c87eac70941930"
COUNTS_SHA = "9e460f62bc760d6d03b65bedc47c6f2a619593728f1932dea668044ebf9ed3d1"
FIELDS = ["stage_a_row_id", "planned_status", "symbol", "v3_constant", "adjacency_rule", "canonical_call_site", "definition_count", "total_reference_count", "canonical_call_occurrence_count"]
RULE = "EXACT_ID_CONST_THEN_ONE_BLANK_LINE_THEN_FUNC"
OUT_FILES = {"README.md", "BINDING.md", "BRIDGE.tsv", "SOURCE_EVIDENCE.tsv", "HARNESS_EVIDENCE.md", "REVIEW_REQUEST.md", "DEPENDENCIES.sha256", "sessionruntime_private.go.txt", "lspwire_private.go.txt", "verify_package.py"}
SOURCE_PINS = {
    "sessionruntime/sessionruntime.go": "ad06c64bf3e267587dc94197a80ece1a03bc7933bd9263b4bb1ac78cad69fb8d",
    "sessionruntime/b4_c15_ownership_registry_private.go": "314a9a89f1f483ac9ced32479bccdea1fb8d1649e083c9a7c167f2baca444274",
    "internal/lspwire/read_frame.go": "9e58a1ae4a79171799134d8640c99731842589125f229088f39196d3d7b9fa58",
    "internal/lspwire/successor_ingress.go": "df4e53790ba611164b2d1d35751bf56bf788abf447cc7bdd3601a0617ac49fa2",
    "sessionruntime/b4_owned_method_pair_lease.go": "b53e51b89effa526337b96fcf883078da332b9b1a1457fcd1fc44a3db2e585e9",
}
CONST_RE = re.compile(r'^const\s+(privateStageA[A-Za-z0-9]+)\s*=\s*"([0-9a-f]{64})"$')
FUNC_RE = re.compile(r'^func\s+\(m \*Manager\)\s+([A-Za-z0-9]+)\(')
ALL_FUNC_RE = re.compile(r'^func\s+\(m \*Manager\)\s+([A-Za-z0-9]+)\(', re.MULTILINE)


def sha(data): return hashlib.sha256(data).hexdigest()
def data(path): return path.read_bytes()
def committed(rel): return subprocess.check_output(["git", "-C", str(ROOT), "show", f"{HEAD}:{rel}"])
def need(name, ok):
    print(("PASS " if ok else "FAIL ") + name)
    if not ok: raise SystemExit(1)

def manifest(rel, expected):
    raw = data(ROOT / rel / "SHA256SUMS")
    need(rel + "_MANIFEST_DIGEST", sha(raw) == expected)
    need(rel + "_MANIFEST_COMMITTED", raw == committed(rel + "/SHA256SUMS"))
    rows = {}
    for line in raw.decode().splitlines():
        digest, name = line.split(None, 1)
        rows[name] = digest
        actual = data(ROOT / rel / name)
        need(rel + "_ENTRY_" + name, sha(actual) == digest)
        need(rel + "_COMMITTED_" + name, actual == committed(rel + "/" + name))
    return rows

def tsv_bytes(raw):
    source = io.StringIO(raw.decode(), newline="")
    reader = csv.DictReader(source, delimiter="\t")
    return list(reader), reader.fieldnames

def derive(stage_rows, go):
    errors = []
    def err(code, detail): errors.append((code, detail))
    ids = [r.get("stage_a_row_id", "") for r in stage_rows]
    symbols = [r.get("proposed_symbol", "") for r in stage_rows]
    if len(stage_rows) != 20: err("STAGE_EXACT_ROW_COUNT", str(len(stage_rows)))
    if len(set(ids)) != 20: err("STAGE_UNIQUE_IDS", "not 20")
    if len(set(symbols)) != 20: err("STAGE_UNIQUE_SYMBOLS", "not 20")
    if any(not re.fullmatch(r"[0-9a-f]{64}", x) for x in ids): err("STAGE_ID_SHAPE", "invalid")
    if any(not r.get("status", "").startswith("PLANNED_") for r in stage_rows): err("STAGE_PLANNED_ONLY", "non-planned")
    expected = dict(zip(ids, symbols))
    lines = go.splitlines()
    declarations = []
    for i, line in enumerate(lines):
        m = CONST_RE.fullmatch(line)
        if not m: continue
        constant, row_id = m.groups()
        function = None
        if i + 2 < len(lines) and lines[i + 1] == "":
            f = FUNC_RE.match(lines[i + 2])
            if f: function = f.group(1)
        if function is None: err("SNAPSHOT_ADJACENCY", row_id)
        declarations.append((row_id, constant, function))
    if len(declarations) != 20: err("SNAPSHOT_EXACT_DECLARATION_COUNT", str(len(declarations)))
    if len({x[0] for x in declarations}) != 20: err("SNAPSHOT_UNIQUE_IDS", "not 20")
    actual = {row_id: symbol for row_id, _, symbol in declarations}
    if actual != expected: err("SNAPSHOT_EXACT_ID_SYMBOL_MAP", "map mismatch")
    constants = {row_id: constant for row_id, constant, _ in declarations}
    definitions = ALL_FUNC_RE.findall(go)
    try:
        canonical = go[go.index("func compilePlannedV3("):go.index("func compileAllPlannedV3(")]
    except ValueError:
        canonical = ""; err("SNAPSHOT_CANONICAL_REGION", "missing")
    canonical_lines = [x.strip() for x in canonical.splitlines()]
    by_id = {r.get("stage_a_row_id", ""): r for r in stage_rows}
    result = []
    for row_id in ids:
        symbol = expected.get(row_id, "")
        definition_count = definitions.count(symbol)
        if definition_count != 1: err("SNAPSHOT_DEFINITION_COUNT", row_id)
        total = len(re.findall(r"\bm\." + re.escape(symbol) + r"\s*\(", go))
        calls = [x for x in canonical_lines if re.search(r"\bm\." + re.escape(symbol) + r"\s*\(", x)]
        call = calls[0] if calls else ""
        if not call: err("SNAPSHOT_CANONICAL_CALL_PRESENT", row_id)
        occurrence = canonical_lines.count(call) if call else 0
        if occurrence != 1: err("SNAPSHOT_CANONICAL_OCCURRENCE", row_id)
        result.append({
            "stage_a_row_id": row_id, "planned_status": by_id[row_id].get("status", ""),
            "symbol": symbol, "v3_constant": constants.get(row_id, ""), "adjacency_rule": RULE,
            "canonical_call_site": call, "definition_count": str(definition_count),
            "total_reference_count": str(total), "canonical_call_occurrence_count": str(occurrence),
        })
    return result, errors

def compare(stage_rows, go, bridge_rows, fields):
    expected, errors = derive(stage_rows, go)
    def err(code, detail): errors.append((code, detail))
    if fields != FIELDS: err("BRIDGE_EXACT_COLUMNS", repr(fields))
    if len(bridge_rows) != 20: err("BRIDGE_EXACT_ROW_COUNT", str(len(bridge_rows)))
    ids = [r.get("stage_a_row_id", "") for r in bridge_rows]
    if len(set(ids)) != len(ids): err("BRIDGE_UNIQUE_IDS", "duplicate")
    wanted = {r["stage_a_row_id"]: r for r in expected}
    if set(ids) != set(wanted): err("BRIDGE_EXACT_ID_SET", "set mismatch")
    for row in bridge_rows:
        row_id = row.get("stage_a_row_id", "")
        if row_id not in wanted: continue
        for field in FIELDS[1:]:
            if row.get(field) != wanted[row_id][field]: err("BRIDGE_EXACT_" + field.upper(), row_id)
        if not re.search(r"\bm\." + re.escape(row.get("symbol", "")) + r"\s*\(", row.get("canonical_call_site", "")):
            err("BRIDGE_FOREIGN_CALL", row_id)
    return errors

def selftests(stage_rows, go, rows, fields):
    cases = []
    def case(name, assertion, mutate):
        changed = copy.deepcopy(rows); mutate(changed); cases.append((name, assertion, changed))
    case("forged-count", "BRIDGE_EXACT_TOTAL_REFERENCE_COUNT", lambda r: r[0].__setitem__("total_reference_count", "999"))
    case("count-preserving-symbol-substitution", "BRIDGE_EXACT_SYMBOL", lambda r: (r[0].__setitem__("symbol", r[1]["symbol"]), r[0].__setitem__("canonical_call_site", r[1]["canonical_call_site"])))
    case("foreign-call", "BRIDGE_EXACT_CANONICAL_CALL_SITE", lambda r: r[0].__setitem__("canonical_call_site", r[1]["canonical_call_site"]))
    case("duplicate-id", "BRIDGE_UNIQUE_IDS", lambda r: r[1].__setitem__("stage_a_row_id", r[0]["stage_a_row_id"]))
    case("extra-row", "BRIDGE_EXACT_ROW_COUNT", lambda r: r.append(copy.deepcopy(r[0])))
    case("missing-row", "BRIDGE_EXACT_ROW_COUNT", lambda r: r.pop())
    for name, assertion, changed in cases:
        observed = {x[0] for x in compare(stage_rows, go, changed, fields)}
        need("SELFTEST_" + name + "_" + assertion, assertion in observed)
    need("SELFTEST_RESTORE", not compare(stage_rows, go, rows, fields))

def verify(harness=False):
    need("HEAD", subprocess.check_output(["git", "-C", str(ROOT), "rev-parse", "HEAD"], text=True).strip() == HEAD)
    v3 = manifest(V3_REL, V3_MANIFEST)
    bridge = manifest(BRIDGE_REL, BRIDGE_MANIFEST)
    for name in ["sessionruntime_private.go.txt", "lspwire_private.go.txt"]:
        local = data(HERE / name)
        need("SNAPSHOT_LOCAL_V3_" + name, local == data(ROOT / V3_REL / name))
        need("SNAPSHOT_LOCAL_BRIDGE_" + name, local == data(ROOT / BRIDGE_REL / name))
        need("SNAPSHOT_V3_MANIFEST_" + name, sha(local) == v3[name])
        need("SNAPSHOT_BRIDGE_MANIFEST_" + name, sha(local) == bridge[name])
        need("SNAPSHOT_COMMITTED_V3_" + name, local == committed(V3_REL + "/" + name))
        need("SNAPSHOT_COMMITTED_BRIDGE_" + name, local == committed(BRIDGE_REL + "/" + name))
    bridge_raw = data(HERE / "BRIDGE.tsv")
    source_raw = data(HERE / "SOURCE_EVIDENCE.tsv")
    need("BRIDGE_SNAPSHOT_EXACT", bridge_raw == committed(BRIDGE_REL + "/BRIDGE.tsv"))
    need("SOURCE_EVIDENCE_SNAPSHOT_EXACT", source_raw == committed(BRIDGE_REL + "/SOURCE_EVIDENCE.tsv"))
    stage_rel = CONTRACT_REL + "/STAGE_A_PLANNED_OBLIGATIONS.tsv"
    stage_raw = data(ROOT / stage_rel)
    need("STAGE_A_DIGEST", sha(stage_raw) == STAGE_A_SHA)
    need("STAGE_A_COMMITTED", stage_raw == committed(stage_rel))
    stage_rows, _ = tsv_bytes(stage_raw)
    bridge_rows, fields = tsv_bytes(bridge_raw)
    go = data(HERE / "sessionruntime_private.go.txt").decode()
    errors = compare(stage_rows, go, bridge_rows, fields)
    for code, detail in errors: print("FAIL", code, detail)
    need("INDEPENDENT_EXACT_BRIDGE", not errors)
    selftests(stage_rows, go, bridge_rows, fields)
    for path, expected in SOURCE_PINS.items():
        raw = data(ROOT / path)
        need("SOURCE_PIN_" + path, sha(raw) == expected and raw == committed(path))
    identity_rel = CONTRACT_REL + "/IDENTITY_ACCOUNTING_47_PLUS_1.tsv"
    counts_rel = CONTRACT_REL + "/COUNTS.json"
    need("IDENTITY_DIGEST", sha(data(ROOT / identity_rel)) == IDENTITY_SHA and data(ROOT / identity_rel) == committed(identity_rel))
    need("COUNTS_DIGEST", sha(data(ROOT / counts_rel)) == COUNTS_SHA and data(ROOT / counts_rel) == committed(counts_rel))
    counts = json.loads(data(ROOT / counts_rel))
    need("COUNT_47_PLUS_1", counts["preserved_v1_identity_count"] == 47 and counts["new_planned_architecture_obligation_count"] == 1 and counts["identity_accounting_denominator"] == 48)
    need("NO_FINAL_C15_CLASSIFICATION", counts["model_reported_48_provenance"] == "UNPINNED_INCONSISTENT_REPORT_NO_EXACT_TRANSCRIPT_ARTIFACT")
    docs = "\n".join((HERE / x).read_text() for x in ["README.md", "BINDING.md", "HARNESS_EVIDENCE.md", "REVIEW_REQUEST.md"])
    need("AUTHORITY_TOKEN", "ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRIVATE_INTERFACE_GO" in docs)
    need("BOUNDARY", "AUTHORED_NOT_GO" in docs and "INTERFACE_COHERENCE_ONLY" in docs)
    tree = ast.parse(pathlib.Path(__file__).read_text())
    imported = {alias.name for node in ast.walk(tree) if isinstance(node, (ast.Import, ast.ImportFrom)) for alias in node.names}
    need("NO_PROXY_VERIFIER", "verify_package.py" not in "\n".join(bridge_rows[0].values()) and "importlib" not in imported)
    manifest_lines = (HERE / "SHA256SUMS").read_text().splitlines()
    need("PACKAGE_CHECKSUM_SET", {x.split(None, 1)[1] for x in manifest_lines} == OUT_FILES)
    for line in manifest_lines:
        expected, name = line.split(None, 1); need("PACKAGE_CHECKSUM_" + name, sha(data(HERE / name)) == expected)
    status = subprocess.check_output(["git", "-C", str(ROOT), "status", "--porcelain"], text=True).splitlines()
    prefix = "docs/qualification/" + HERE.name + "/"
    need("PACKAGE_ONLY_SCOPE", bool(status) and all(x[3:].startswith(prefix) for x in status))
    need("NO_TRACKED_MUTATION", not subprocess.check_output(["git", "-C", str(ROOT), "diff", "--name-only"], text=True).strip())
    if harness:
        with tempfile.TemporaryDirectory(prefix="adr0011-private-binding-") as tmp:
            target = pathlib.Path(tmp); (target / "sessionruntime").mkdir(); (target / "internal/lspwire").mkdir(parents=True)
            shutil.copyfile(HERE / "sessionruntime_private.go.txt", target / "sessionruntime/call_hierarchy_directional_raw_v1_private.go")
            shutil.copyfile(HERE / "lspwire_private.go.txt", target / "internal/lspwire/read_frame.go")
            (target / "go.mod").write_text("module example.invalid/adr0011-private-binding\n\ngo 1.26\n")
            diff = subprocess.run(["gofmt", "-d", "sessionruntime/call_hierarchy_directional_raw_v1_private.go", "internal/lspwire/read_frame.go"], cwd=target, text=True, capture_output=True, check=True)
            need("HARNESS_GOFMT_EXACT", diff.stdout == "")
            for command in [["go", "test", "./..."], ["go", "test", "-run", "^$", "./..."], ["go", "vet", "./..."]]: subprocess.run(command, cwd=target, check=True)
            for name in ["sessionruntime/call_hierarchy_directional_raw_v1_private.go", "internal/lspwire/read_frame.go"]: print(sha(data(target / name)), name)
            print(subprocess.check_output(["go", "version"], text=True).strip())
        print("INTERFACE_COHERENCE_ONLY")
    print("BINDING_PACKAGE_VERIFY_OK")

if __name__ == "__main__": verify("--harness" in sys.argv)
