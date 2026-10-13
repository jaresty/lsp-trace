#!/usr/bin/env python3
import copy, csv, hashlib, io, json, pathlib, re, shutil, subprocess, sys, tempfile

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]
CONTRACT = HERE.parent / "adr0011-directional-raw-custody-c15-two-stage-contract-v2"
V3 = HERE.parent / "adr0011-directional-raw-custody-32mib-private-interface-go-v3"
HEAD = "9145212f2b826d433360a3cdb498df98abd0b6ae"
STAGE_A_SHA256 = "0c9909507b6abd5fd16034c70f7b34f1ae04719822af669ba8c3f373e4a49a1d"
V3_MANIFEST_SHA256 = "02153887105c30cc6eaa3b976ec49e7f32b97afcf35ff6b8aaf185529e029704"
ADJACENCY_RULE = "EXACT_ID_CONST_THEN_ONE_BLANK_LINE_THEN_FUNC"
BRIDGE_FIELDS = ["stage_a_row_id", "planned_status", "symbol", "v3_constant", "adjacency_rule", "canonical_call_site", "definition_count", "total_reference_count", "canonical_call_occurrence_count"]
OUT_FILES = {"README.md", "BRIDGE.tsv", "SOURCE_EVIDENCE.tsv", "COUNT_RECONCILIATION.md", "HARNESS_EVIDENCE.md", "DEPENDENCIES.sha256", "REVIEW_REQUEST.md", "sessionruntime_private.go.txt", "lspwire_private.go.txt", "verify_package.py"}
SOURCE_PINS = {
    "sessionruntime/sessionruntime.go": "ad06c64bf3e267587dc94197a80ece1a03bc7933bd9263b4bb1ac78cad69fb8d",
    "internal/lspwire/read_frame.go": "9e58a1ae4a79171799134d8640c99731842589125f229088f39196d3d7b9fa58",
    "internal/lspwire/successor_ingress.go": "df4e53790ba611164b2d1d35751bf56bf788abf447cc7bdd3601a0617ac49fa2",
    "sessionruntime/b4_owned_method_pair_lease.go": "b53e51b89effa526337b96fcf883078da332b9b1a1457fcd1fc44a3db2e585e9",
    "sessionruntime/b4_c15_ownership_registry_private.go": "314a9a89f1f483ac9ced32479bccdea1fb8d1649e083c9a7c167f2baca444274",
}
CONST_RE = re.compile(r'^const\s+(privateStageA[A-Za-z0-9]+)\s*=\s*"([0-9a-f]{64})"$')
FUNC_RE = re.compile(r'^func\s+\(m \*Manager\)\s+([A-Za-z0-9]+)\(')
ALL_FUNC_RE = re.compile(r'^func\s+\(m \*Manager\)\s+([A-Za-z0-9]+)\(', re.MULTILINE)


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def load_tsv(path):
    with path.open(newline="") as source:
        return list(csv.DictReader(source, delimiter="\t"))


def fail(errors, assertion, detail):
    errors.append((assertion, detail))


def derive(stage_rows, go_text):
    errors = []
    if len(stage_rows) != 20:
        fail(errors, "STAGE_A_EXACT_ROW_COUNT", f"got {len(stage_rows)}, want 20")
    ids = [r.get("stage_a_row_id", "") for r in stage_rows]
    symbols = [r.get("proposed_symbol", "") for r in stage_rows]
    if len(set(ids)) != len(ids):
        fail(errors, "STAGE_A_UNIQUE_IDS", "duplicate Stage A ID")
    if len(set(symbols)) != len(symbols):
        fail(errors, "STAGE_A_UNIQUE_SYMBOLS", "duplicate Stage A symbol")
    expected_map = dict(zip(ids, symbols))
    if len(expected_map) != 20 or set(expected_map) != set(ids):
        fail(errors, "STAGE_A_EXACT_ID_SET", "extra, missing, or duplicate ID")

    lines = go_text.splitlines()
    declarations = []
    for index, line in enumerate(lines):
        match = CONST_RE.fullmatch(line)
        if not match:
            continue
        constant, row_id = match.groups()
        function = None
        if index + 2 < len(lines) and lines[index + 1] == "":
            function_match = FUNC_RE.match(lines[index + 2])
            if function_match:
                function = function_match.group(1)
        if function is None:
            fail(errors, "SNAPSHOT_ADJACENCY", f"{row_id} is not const/one-blank-line/function adjacent")
        declarations.append((row_id, constant, function, index + 1))

    declaration_ids = [d[0] for d in declarations]
    if len(declarations) != 20:
        fail(errors, "SNAPSHOT_EXACT_DECLARATION_COUNT", f"got {len(declarations)}, want 20")
    if len(set(declaration_ids)) != len(declaration_ids):
        fail(errors, "SNAPSHOT_UNIQUE_IDS", "duplicate snapshot ID declaration")
    snapshot_map = {row_id: symbol for row_id, _, symbol, _ in declarations}
    if set(snapshot_map) != set(expected_map):
        fail(errors, "SNAPSHOT_EXACT_ID_SET", "snapshot ID set differs from committed Stage A set")
    if snapshot_map != expected_map:
        fail(errors, "SNAPSHOT_EXACT_ID_SYMBOL_MAP", "snapshot ID-to-symbol map differs from committed TSV")

    constants = {row_id: constant for row_id, constant, _, _ in declarations}
    all_definitions = ALL_FUNC_RE.findall(go_text)
    try:
        canonical = go_text[go_text.index("func compilePlannedV3("):go_text.index("func compileAllPlannedV3(")]
    except ValueError:
        canonical = ""
        fail(errors, "SNAPSHOT_CANONICAL_REGION", "canonical compile region missing")
    canonical_lines = [line.strip() for line in canonical.splitlines()]

    expected_rows = []
    by_id = {r.get("stage_a_row_id", ""): r for r in stage_rows}
    for row_id in ids:
        symbol = expected_map.get(row_id, "")
        definition_count = all_definitions.count(symbol)
        if definition_count != 1:
            fail(errors, "SNAPSHOT_EXACT_DEFINITION_COUNT", f"{row_id}/{symbol}: {definition_count}")
        all_symbol_calls = re.findall(r"\bm\." + re.escape(symbol) + r"\s*\(", go_text)
        total_reference_count = len(all_symbol_calls)
        symbol_calls = [line for line in canonical_lines if re.search(r"\bm\." + re.escape(symbol) + r"\s*\(", line)]
        if not symbol_calls:
            fail(errors, "SNAPSHOT_CANONICAL_CALL_PRESENT", f"{row_id}/{symbol}: absent")
            canonical_call = ""
            canonical_occurrence_count = 0
        else:
            canonical_call = symbol_calls[0]
            canonical_occurrence_count = canonical_lines.count(canonical_call)
        if canonical_occurrence_count != 1:
            fail(errors, "SNAPSHOT_EXACT_CANONICAL_CALL_OCCURRENCE_COUNT", f"{row_id}/{symbol}: {canonical_occurrence_count}")
        if canonical_call and not re.search(r"\bm\." + re.escape(symbol) + r"\s*\(", canonical_call):
            fail(errors, "SNAPSHOT_CALL_SYMBOL_BINDING", f"{row_id}/{symbol}: foreign call")
        expected_rows.append({
            "stage_a_row_id": row_id,
            "planned_status": by_id[row_id].get("status", ""),
            "symbol": symbol,
            "v3_constant": constants.get(row_id, ""),
            "adjacency_rule": ADJACENCY_RULE,
            "canonical_call_site": canonical_call,
            "definition_count": "1",
            "total_reference_count": str(total_reference_count),
            "canonical_call_occurrence_count": "1",
        })
    return expected_rows, errors


def validate_bridge(stage_rows, go_text, bridge_rows, fieldnames):
    expected_rows, errors = derive(stage_rows, go_text)
    if fieldnames != BRIDGE_FIELDS:
        fail(errors, "BRIDGE_EXACT_COLUMNS", f"got {fieldnames!r}")
    if len(bridge_rows) != 20:
        fail(errors, "BRIDGE_EXACT_ROW_COUNT", f"got {len(bridge_rows)}, want 20")
    bridge_ids = [r.get("stage_a_row_id", "") for r in bridge_rows]
    if len(set(bridge_ids)) != len(bridge_ids):
        fail(errors, "BRIDGE_UNIQUE_IDS", "duplicate bridge ID")
    expected_ids = {r["stage_a_row_id"] for r in expected_rows}
    if set(bridge_ids) != expected_ids:
        fail(errors, "BRIDGE_EXACT_ID_SET", "bridge has extra or missing ID")
    expected_by_id = {r["stage_a_row_id"]: r for r in expected_rows}
    for row in bridge_rows:
        row_id = row.get("stage_a_row_id", "")
        expected = expected_by_id.get(row_id)
        if expected is None:
            continue
        for field in BRIDGE_FIELDS[1:]:
            if row.get(field) != expected[field]:
                fail(errors, "BRIDGE_EXACT_" + field.upper(), f"{row_id}: got {row.get(field)!r}, want {expected[field]!r}")
        call = row.get("canonical_call_site", "")
        symbol = row.get("symbol", "")
        if not re.search(r"\bm\." + re.escape(symbol) + r"\s*\(", call):
            fail(errors, "BRIDGE_CALL_SYMBOL_BINDING", f"{row_id}/{symbol}: {call!r}")
        if expected["symbol"] != symbol:
            fail(errors, "BRIDGE_ID_SYMBOL_BINDING", f"{row_id}: {symbol!r}")
    return errors


def read_bridge(path):
    with path.open(newline="") as source:
        reader = csv.DictReader(source, delimiter="\t")
        return list(reader), reader.fieldnames


def run_self_tests(stage_rows, go_text, bridge_rows, fieldnames):
    cases = []
    def add(name, assertion, mutate):
        changed = copy.deepcopy(bridge_rows)
        mutate(changed)
        cases.append((name, assertion, changed))
    add("false-id", "BRIDGE_EXACT_ID_SET", lambda r: r[0].__setitem__("stage_a_row_id", "f" * 64))
    add("false-constant", "BRIDGE_EXACT_V3_CONSTANT", lambda r: r[0].__setitem__("v3_constant", "privateStageAForeignV3"))
    add("false-rule", "BRIDGE_EXACT_ADJACENCY_RULE", lambda r: r[0].__setitem__("adjacency_rule", "ARBITRARY_RULE"))
    add("false-count", "BRIDGE_EXACT_DEFINITION_COUNT", lambda r: r[0].__setitem__("definition_count", "7"))
    add("false-total-reference-count", "BRIDGE_EXACT_TOTAL_REFERENCE_COUNT", lambda r: r[0].__setitem__("total_reference_count", "1"))
    add("false-canonical-occurrence-count", "BRIDGE_EXACT_CANONICAL_CALL_OCCURRENCE_COUNT", lambda r: r[0].__setitem__("canonical_call_occurrence_count", "9"))
    add("foreign-call", "BRIDGE_EXACT_CANONICAL_CALL_SITE", lambda r: r[0].__setitem__("canonical_call_site", r[1]["canonical_call_site"]))
    add("extra-row", "BRIDGE_EXACT_ROW_COUNT", lambda r: r.append(copy.deepcopy(r[0])))
    add("missing-row", "BRIDGE_EXACT_ROW_COUNT", lambda r: r.pop())
    add("duplicate-row", "BRIDGE_UNIQUE_IDS", lambda r: r[1].__setitem__("stage_a_row_id", r[0]["stage_a_row_id"]))
    for name, assertion, changed in cases:
        observed = validate_bridge(stage_rows, go_text, changed, fieldnames)
        observed_assertions = {item[0] for item in observed}
        if assertion not in observed_assertions:
            print(f"FAIL SELFTEST_{name} expected {assertion}, observed {sorted(observed_assertions)}")
            raise SystemExit(1)
        print(f"SELFTEST_FAIL_OBSERVED {name} {assertion}")
    restored = validate_bridge(stage_rows, go_text, bridge_rows, fieldnames)
    if restored:
        print("FAIL SELFTEST_RESTORE", restored)
        raise SystemExit(1)
    print("SELFTEST_RESTORE_PASS")


def need(name, condition):
    print(("PASS " if condition else "FAIL ") + name)
    if not condition:
        raise SystemExit(1)


def verify(harness=False):
    need("HEAD", subprocess.check_output(["git", "-C", str(ROOT), "rev-parse", "HEAD"], text=True).strip() == HEAD)
    stage_path = CONTRACT / "STAGE_A_PLANNED_OBLIGATIONS.tsv"
    need("COMMITTED_STAGE_A_SHA256", digest(stage_path) == STAGE_A_SHA256)
    stage_rows = load_tsv(stage_path)
    go_text = (HERE / "sessionruntime_private.go.txt").read_text()
    v3_manifest_path = V3 / "SHA256SUMS"
    need("V3_MANIFEST_UNCHANGED", digest(v3_manifest_path) == V3_MANIFEST_SHA256)
    v3_manifest = {name: expected for expected, name in (line.split(None, 1) for line in v3_manifest_path.read_text().splitlines())}
    need("V3_GO_MANIFEST_ENTRY", digest(V3 / "sessionruntime_private.go.txt") == v3_manifest.get("sessionruntime_private.go.txt"))
    need("V3_LSPWIRE_MANIFEST_ENTRY", digest(V3 / "lspwire_private.go.txt") == v3_manifest.get("lspwire_private.go.txt"))
    need("V3_SNAPSHOT_EXACT", (HERE / "sessionruntime_private.go.txt").read_bytes() == (V3 / "sessionruntime_private.go.txt").read_bytes())
    need("V3_LSPWIRE_SNAPSHOT_EXACT", (HERE / "lspwire_private.go.txt").read_bytes() == (V3 / "lspwire_private.go.txt").read_bytes())
    bridge_rows, fieldnames = read_bridge(HERE / "BRIDGE.tsv")
    errors = validate_bridge(stage_rows, go_text, bridge_rows, fieldnames)
    for assertion, detail in errors:
        print("FAIL", assertion, detail)
    need("INDEPENDENT_BRIDGE_DERIVATION", not errors)
    run_self_tests(stage_rows, go_text, bridge_rows, fieldnames)

    for path, expected in SOURCE_PINS.items():
        need("SOURCE_PIN_" + path, digest(ROOT / path) == expected)
    manifest = (HERE / "SHA256SUMS").read_text().splitlines()
    need("PACKAGE_CHECKSUM_SET", {line.split(None, 1)[1] for line in manifest} == OUT_FILES)
    for line in manifest:
        expected, name = line.split(None, 1)
        need("PACKAGE_CHECKSUM_" + name, digest(HERE / name) == expected)
    counts = json.loads((CONTRACT / "COUNTS.json").read_text())
    need("COUNT_47_PLUS_1", counts["preserved_v1_identity_count"] == 47 and counts["new_planned_architecture_obligation_count"] == 1 and counts["identity_accounting_denominator"] == 48)
    need("UNPINNED_REPORT_LABEL", counts["model_reported_48_provenance"] == "UNPINNED_INCONSISTENT_REPORT_NO_EXACT_TRANSCRIPT_ARTIFACT")
    docs = "\n".join((HERE / name).read_text() for name in ["README.md", "COUNT_RECONCILIATION.md", "HARNESS_EVIDENCE.md", "REVIEW_REQUEST.md"])
    need("AUTHORITY_LABELS", "AUTHORED_NOT_GO" in docs and "INTERFACE_COHERENCE_ONLY" in docs and "C15_CLOSED" not in docs and "ACCEPTED_GO" not in docs)
    status = subprocess.check_output(["git", "-C", str(ROOT), "status", "--porcelain"], text=True).splitlines()
    prefix = "docs/qualification/" + HERE.name + "/"
    need("PACKAGE_ONLY_SCOPE", bool(status) and all(line[3:].startswith(prefix) for line in status))
    need("NO_PRODUCTION_TEST_MUTATION", not subprocess.check_output(["git", "-C", str(ROOT), "diff", "--name-only"], text=True).strip())

    if harness:
        with tempfile.TemporaryDirectory(prefix="adr0011-bridge-harness-") as directory:
            target = pathlib.Path(directory)
            (target / "sessionruntime").mkdir()
            (target / "internal/lspwire").mkdir(parents=True)
            shutil.copyfile(HERE / "sessionruntime_private.go.txt", target / "sessionruntime/call_hierarchy_directional_raw_v1_private.go")
            shutil.copyfile(HERE / "lspwire_private.go.txt", target / "internal/lspwire/read_frame.go")
            (target / "go.mod").write_text("module example.invalid/adr0011-bridge-harness\n\ngo 1.26\n")
            commands = [
                ["gofmt", "-w", "sessionruntime/call_hierarchy_directional_raw_v1_private.go", "internal/lspwire/read_frame.go"],
                ["go", "test", "./..."],
                ["go", "test", "-run", "^$", "./..."],
                ["go", "vet", "./..."],
            ]
            for command in commands:
                subprocess.run(command, cwd=target, check=True)
            for name in ["go.mod", "internal/lspwire/read_frame.go", "sessionruntime/call_hierarchy_directional_raw_v1_private.go"]:
                print(digest(target / name), name)
            print(subprocess.check_output(["go", "version"], text=True).strip())
        print("INTERFACE_COHERENCE_ONLY")
    print("BRIDGE_PACKAGE_VERIFY_OK")


if __name__ == "__main__":
    verify("--harness" in sys.argv)
