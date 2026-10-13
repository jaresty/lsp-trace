#!/usr/bin/env python3
import csv
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
V1 = ROOT / "sessionruntime/testdata/c15-census-v1"
V2 = ROOT / "sessionruntime/testdata/c15-census-v2"
CURRENT = ROOT / "docs/qualification/adr0011-main-convergence-tier3-8c6ae21e-v1/c15/run1"
EXPECTED = {
    V1 / "candidates.tsv": (3805, 2, 541478, "13d45a61cb9ad2e5dad8d944458a9b7f459c9a2cd0336cb76700fa801599ca6e"),
    V1 / "classifications.tsv": (3805, 5, 687057, "cde6aa72034acf743c582cec56ff778470f8f258bde0ecdc4398653f3de10e04"),
    V2 / "candidates.tsv": (3847, 2, 547527, "536d11dcfa927d1ce5db24fbd8a956a5170a8f762ee53d8daa3664288172afb2"),
    V2 / "classifications.tsv": (3847, 5, 694776, "494d2f0045db3e34b7c03e2a959aaf21e2fee203bfd3768097df7930ddf6b730"),
}

def parse(path, fields):
    data = path.read_bytes()
    assert data.endswith(b"\n"), f"missing terminal LF: {path}"
    rows = list(csv.reader(data.decode("utf-8").splitlines(), delimiter="\t"))
    assert all(len(row) == fields for row in rows), f"field count: {path}"
    ids = [row[0] for row in rows]
    assert all(ids), f"empty ID: {path}"
    assert len(ids) == len(set(ids)), f"duplicate ID: {path}"
    return data, rows, {row[0]: row for row in rows}

records = {}
for path, (count, fields, size, digest) in EXPECTED.items():
    data, rows, by_id = parse(path, fields)
    actual = hashlib.sha256(data).hexdigest()
    assert len(rows) == count and len(data) == size and actual == digest, path
    records[path] = by_id
    print(f"catalog\t{path.relative_to(ROOT)}\theaderless\tfields={fields}\trows={count}\tbytes={size}\tsha256={actual}")

v1c = records[V1 / "candidates.tsv"]
v1x = records[V1 / "classifications.tsv"]
v2c = records[V2 / "candidates.tsv"]
v2x = records[V2 / "classifications.tsv"]
assert set(v1c) == set(v1x) and set(v2c) == set(v2x)
added = sorted(set(v2c) - set(v1c))
removed = sorted(set(v1c) - set(v2c))
changed_candidates = [key for key in v1c if v1c[key] != v2c[key]]
changed_classifications = [key for key in v1x if v1x[key] != v2x[key]]
assert len(added) == 42 and not removed and not changed_candidates and not changed_classifications
categories = {}
for key in added:
    row = v2x[key]
    assert row[1] == "excluded" and row[2] == "" and row[3] and row[4]
    categories[row[3]] = categories.get(row[3], 0) + 1
assert categories == {"ephemeral-runtime": 18, "external-library": 24}, categories
print(f"delta\tadded={len(added)}\tremoved=0\tshared_candidate_changes=0\tshared_classification_changes=0")
print("excluded_categories\tephemeral-runtime=18\texternal-library=24")

manifest_path = V2 / "manifest.json"
manifest_data = manifest_path.read_bytes()
manifest = json.loads(manifest_data)
assert hashlib.sha256(manifest_data).hexdigest() == "548e6af3625b248ab7e8a99b29ac24bbb3f674b0ffdb14365d4dfaaf49c89daf"
assert manifest["schema"] == "lsp-trace/c15-census-successor-v2"
assert manifest["base_v1_count"] == 3805 and manifest["final_v2_count"] == 3847
assert manifest["added_excluded"] == 42 and manifest["removed"] == 0 and manifest["reclassified"] == 0
print("manifest\tsessionruntime/testdata/c15-census-v2/manifest.json\tsha256=548e6af3625b248ab7e8a99b29ac24bbb3f674b0ffdb14365d4dfaaf49c89daf")

for name, fields, count, digest in [
    ("candidates.tsv", 2, 4155, "aa54b17734ff8136bbe1df1b559cd5103a3b5a86c04547fc6b78e093928b4341"),
    ("classifications.tsv", 5, 4155, "bfbecdd3df47a415adfcfd36b3dd6db1cd45fcd64f1ead89596f8ac92f77989f"),
]:
    data, rows, _ = parse(CURRENT / name, fields)
    assert len(rows) == count and hashlib.sha256(data).hexdigest() == digest
    print(f"current\t{(CURRENT / name).relative_to(ROOT)}\theaderless\tfields={fields}\trows={count}\tsha256={digest}")
location_data = (CURRENT / "all-source-locations.tsv").read_bytes()
location_rows = list(csv.reader(location_data.decode("utf-8").splitlines(), delimiter="\t"))
assert len(location_rows) == 4156 and len(location_rows[1:]) == 4155
assert hashlib.sha256(location_data).hexdigest() == "53bab2230a4ccbc12992b5f9163a882deb3f31bac792c252749e0cab47020043"
print("current\tdocs/qualification/adr0011-main-convergence-tier3-8c6ae21e-v1/c15/run1/all-source-locations.tsv\theader=1\tdata_rows=4155\tsha256=53bab2230a4ccbc12992b5f9163a882deb3f31bac792c252749e0cab47020043")
print("ACCOUNTING_RESOLUTION=PASS")
