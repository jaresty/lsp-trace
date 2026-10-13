#!/usr/bin/env python3
import csv
import hashlib
import pathlib
import re
import subprocess

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]
STAGE_A = HERE.parent / "adr0011-directional-raw-custody-c15-two-stage-contract-v2" / "STAGE_A_PLANNED_OBLIGATIONS.tsv"
EXPECTED_HEAD = "acf563e7708095d8a32c809b49d97c7432d49b03"
PINS = {
    "adr0011-directional-raw-custody-32mib-private-interface-go-v1": "d36211da45adb55e83f0b112a62140d7bec5ef2076edee12c3facebbd692af7c",
    "adr0011-directional-raw-custody-32mib-private-interface-go-v2": "8f329e1012278a7e92220d0499047a7b486402fcc2db8d10f7b4b2dcb06feeda",
}
ID_RE = re.compile(r'^const\s+(privateStageA[A-Za-z0-9]+)\s*=\s*"([0-9a-f]{64})"$', re.MULTILINE)
FUNC_RE = re.compile(r'^func\s+\(m \*Manager\)\s+([a-zA-Z0-9]+)\(', re.MULTILINE)


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def require(name, condition):
    print(("PASS " if condition else "FAIL ") + name)
    if not condition:
        raise SystemExit(1)


for package, expected in PINS.items():
    package_path = HERE.parent / package
    require("IMMUTABLE_" + package, digest(package_path / "SHA256SUMS") == expected)
    for line in (package_path / "SHA256SUMS").read_text().splitlines():
        expected_file, name = line.split(None, 1)
        require(f"PIN_{package}_{name}", digest(package_path / name) == expected_file)

with STAGE_A.open(newline="") as source:
    rows = list(csv.DictReader(source, delimiter="\t"))
require("STAGE_A_EXACT_20", len(rows) == 20)
expected_by_id = {row["stage_a_row_id"]: row["proposed_symbol"] for row in rows}
require("STAGE_A_UNIQUE_IDS", len(expected_by_id) == len(rows))
require("STAGE_A_UNIQUE_SYMBOLS", len({row["proposed_symbol"] for row in rows}) == len(rows))
require("STAGE_A_ROW_KIND", all(row["row_kind"] == "PLANNED_OBLIGATION" for row in rows))

go = (HERE / "sessionruntime_private.go.txt").read_text()
lines = go.splitlines()
actual_by_id = {}
constant_names = set()
for index, line in enumerate(lines):
    match = ID_RE.match(line)
    if not match:
        continue
    constant_name, row_id = match.groups()
    require("STAGE_A_CONSTANT_UNIQUE_" + row_id, row_id not in actual_by_id and constant_name not in constant_names)
    constant_names.add(constant_name)
    next_index = index + 1
    while next_index < len(lines) and not lines[next_index].strip():
        next_index += 1
    function_match = FUNC_RE.match(lines[next_index]) if next_index < len(lines) else None
    require("STAGE_A_ADJACENT_FUNCTION_" + row_id, function_match is not None)
    actual_by_id[row_id] = function_match.group(1)

require("STAGE_A_DECLARATION_EXACT_MAP", actual_by_id == expected_by_id)
require("STAGE_A_NO_EXTRA_IDS", set(actual_by_id) == set(expected_by_id))
require("STAGE_A_NO_MISSING_IDS", set(expected_by_id) == set(actual_by_id))
require("STAGE_A_EXACT_20_DECLARATIONS", len(actual_by_id) == 20 and len(constant_names) == 20)

function_definitions = FUNC_RE.findall(go)
for row_id, symbol in expected_by_id.items():
    require("STAGE_A_SYMBOL_DEFINED_" + symbol, function_definitions.count(symbol) == 1)

canonical_start = go.index("func compilePlannedV3(")
canonical_end = go.index("func compileAllPlannedV3(")
canonical = go[canonical_start:canonical_end]
for row_id, symbol in expected_by_id.items():
    require("STAGE_A_CANONICAL_REFERENCE_" + symbol, re.search(r"\b" + re.escape(symbol) + r"\s*\(", canonical) is not None)

require("TOKEN", "ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRIVATE_INTERFACE_V3_GO" in (HERE / "README.md").read_text())
for marker in [
    "reserveDirectionalComponentV3", "acquireDirectionalOwnedV3", "declareDirectionalAliasV3",
    "transferDirectionalAcquisitionCoreV3", "releaseDirectionalTerminalCoreV3",
    "compileLockOrderSurfaceV3", "rejectAccountHeldInversionV3", "sync.WaitGroup",
    "^uint64(0)", "privateCallHierarchyDirectionalMaxBytesV3+1", "wg.Wait()",
    "reserveDirectionalComponentV3(account, attempt, privateCallHierarchyDirectionalComponentInvalidV3, 1)",
    "transferDirectionalAcquisitionV3(owner, nil, account)",
    "borrowDirectionalV3(owner, nil, func([]byte) error { return nil })",
    'errors.New("planned callback refusal")',
    "callbackRefusal := m.borrowDirectionalV3",
]:
    require("HARNESS_SHAPE_" + marker, marker in go)
require("SEQUENTIAL_DUPLICATE_RELEASE", canonical.count("m.releaseDirectionalTerminalV3(owner, account, privateCallHierarchyTerminalCompleteV3)") == 2)
require("THREE_RACE_GOROUTINES", canonical.count("go func()") == 3)
require("NO_GOPLS", "gopls" not in go.lower())

for line in (HERE / "SHA256SUMS").read_text().splitlines():
    expected_file, name = line.split(None, 1)
    require(f"MANIFEST_{name}", digest(HERE / name) == expected_file)
require("HEAD", subprocess.check_output(["git", "-C", str(ROOT), "rev-parse", "HEAD"], text=True).strip() == EXPECTED_HEAD)
status = subprocess.check_output(["git", "-C", str(ROOT), "status", "--porcelain"], text=True).splitlines()
prefix = "docs/qualification/adr0011-directional-raw-custody-32mib-private-interface-go-v3/"
require("WRITE_SCOPE", bool(status) and all(line[3:].startswith(prefix) for line in status))
require("AUTHORED_NOT_GO", all("AUTHORED_NOT_GO" in (HERE / name).read_text() for name in ["README.md", "INTERFACE.md", "STATE_AND_OWNERSHIP.md", "WRITESET.md", "HARNESS_EVIDENCE.md", "REVIEW_REQUEST.md"]))
print("V3_PACKAGE_VERIFY_OK")
