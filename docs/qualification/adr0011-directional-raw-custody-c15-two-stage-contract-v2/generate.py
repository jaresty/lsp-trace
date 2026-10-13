#!/usr/bin/env python3
"""Deterministic documentation-only producer for the ADR0011 C15 two-stage contract."""
import csv
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
OUT = Path(__file__).resolve().parent
HEAD = "a1cd77950ffa45ebed52310878f772f3c7344756"
V1 = ROOT / "docs/qualification/adr0011-directional-raw-custody-c15-enumeration-v1"
TARGET = "ADR0011_DIRECTIONAL_RAW_CUSTODY_C15_TWO_STAGE_CONTRACT_GO"
TERMINALS = "normal|complete-empty|malformed|protocol|transport|unsupported|timeout|cancel|refusal|STOP|RESTART|Shutdown|stale-generation|race|duplicate-release"


def digest_bytes(data):
    return hashlib.sha256(data).hexdigest()


def file_digest(path):
    return digest_bytes(path.read_bytes())


def stable_id(namespace, *parts):
    return digest_bytes((namespace + "\0" + "\0".join(parts)).encode())


def write_tsv(name, fields, rows):
    with (OUT / name).open("w", newline="") as f:
        writer = csv.DictWriter(f, fields, delimiter="\t", lineterminator="\n")
        writer.writeheader()
        writer.writerows(rows)


# Preserve v1 as immutable negative evidence. The package is read, never rewritten.
with (V1 / "UNRESOLVED.tsv").open(newline="") as f:
    producer_rows = list(csv.DictReader(f, delimiter="\t"))
if len(producer_rows) != 47 or len({r["row_id"] for r in producer_rows}) != 47:
    raise SystemExit("expected exact immutable producer set: 47 unique rows")

missing_symbol = "reserveDirectionalComponentV2"
missing_component = "ALL_SIX_COMPONENTS"
missing_id = stable_id(
    "adr0011.c15.lease-source.v2.row",
    "sessionruntime/call_hierarchy_directional_raw_v1_private.go",
    missing_symbol,
    "RESERVE",
    missing_component,
    "planned exact pre-effect component/bytes/attempt/account reservation",
)

reconciliation = []
for index, row in enumerate(sorted(producer_rows, key=lambda r: r["row_id"]), 1):
    reconciliation.append({
        "identity_ordinal": str(index),
        "identity": row["row_id"],
        "identity_kind": "PRESERVED_V1_IDENTITY",
        "identity_provenance": "v1/UNRESOLVED.tsv exact row_id",
        "v1_producer_row_id": row["row_id"],
        "source_or_proposed_path": row["source_path"],
        "symbol": row["symbol"],
        "operation_kind": row["operation_kind"],
        "component": row["component"],
        "reconciliation": "PRESERVED",
        "cause": "exact identity present in immutable v1 producer47",
    })
reconciliation.append({
    "identity_ordinal": "48",
    "identity": missing_id,
    "identity_kind": "NEW_PLANNED_ARCHITECTURE_OBLIGATION",
    "identity_provenance": "v2 correction derived from predecessor architecture/interface reservation requirement; not a v1 or adjudicator-transcript identity",
    "v1_producer_row_id": "",
    "source_or_proposed_path": "sessionruntime/call_hierarchy_directional_raw_v1_private.go",
    "symbol": missing_symbol,
    "operation_kind": "RESERVE",
    "component": missing_component,
    "reconciliation": "INTRODUCED_BY_V2",
    "cause": "v1 generator iterated ALLOCATE and ALIAS plus one TRANSFER and RELEASE, but never emitted the architecture-required pre-effect RESERVE obligation",
})
reconciliation.sort(key=lambda r: int(r["identity_ordinal"]))
write_tsv("IDENTITY_ACCOUNTING_47_PLUS_1.tsv", list(reconciliation[0]), reconciliation)

existing_specs = [
    ("sessionruntime/sessionruntime.go", 1564, "Manager", "ACCOUNTING_OWNER", "MANAGER_ACCOUNTING_DOMAIN", "EXISTING_OBSERVED", "current Manager owns the accounting-domain lock and lifecycle integration point"),
    ("sessionruntime/b4_explicit_bytes_c15_private.go", 125, "privateB4ByteAccountV2", "ACCOUNT_LIFETIME", "C15_ACCOUNT", "EXISTING_OBSERVED", "current versioned transaction account type"),
    ("sessionruntime/b4_explicit_bytes_c15_private.go", 147, "newPrivateB4ByteAccountV2", "ACCOUNT_CREATE", "C15_ACCOUNT", "EXISTING_OBSERVED", "current account constructor"),
    ("sessionruntime/b4_explicit_bytes_c15_private.go", 174, "(*privateB4ByteAccountV2).reserve", "LEASE_RESERVE", "C15_LEASE", "EXISTING_OBSERVED", "current byte-lease reservation API"),
    ("sessionruntime/b4_explicit_bytes_c15_private.go", 379, "(*privateB4ByteAccountV2).aliasPrivateB4C15", "LEASE_ALIAS", "C15_LEASE", "EXISTING_OBSERVED", "current account-local alias capability"),
    ("sessionruntime/b4_explicit_bytes_c15_private.go", 495, "(*privateB4ByteLeaseV2).Release", "LEASE_RELEASE", "C15_LEASE", "EXISTING_OBSERVED", "current lease release API"),
    ("sessionruntime/b4_explicit_bytes_c15_private.go", 557, "(*privateB4ByteAccountV2).transferDiagnosticBacking", "LEASE_TRANSFER", "C15_LEASE", "EXISTING_OBSERVED", "current single diagnostic transfer capability"),
    ("sessionruntime/b4_explicit_bytes_c15_private.go", 561, "(*privateB4ByteAccountV2).transferDiagnosticBackingPair", "LEASE_TRANSFER", "C15_LEASE_SET", "EXISTING_OBSERVED", "current paired diagnostic transfer capability"),
    ("sessionruntime/b4_explicit_bytes_c15_private.go", 660, "(*privateB4ByteAccountV2).requestTerminalRelease", "TERMINAL_CLEANUP", "C15_ACCOUNT", "EXISTING_OBSERVED", "current account terminal-release request"),
    ("sessionruntime/b4_explicit_bytes_c15_private.go", 670, "(*privateB4ByteAccountV2).maybeReleaseBackingLocked", "TERMINAL_CLEANUP", "C15_ACCOUNT", "EXISTING_OBSERVED", "current terminal backing cleanup"),
    ("sessionruntime/b4_owned_method_pair_lease.go", 95, "OwnedMethodPairLease.WithPair", "NONREUSABLE_BORROW", "NEARBY_LEASE", "EXISTING_NOT_REUSABLE", "nearby callback borrow is lifetime-bound and is not a reusable directional acquisition"),
    ("sessionruntime/b4_owned_method_pair_lease.go", 300, "callHierarchyObservationLease.withObservation", "NONREUSABLE_BORROW", "NEARBY_LEASE", "EXISTING_NOT_REUSABLE", "nearby observation borrow is lifetime-bound and is not reusable"),
    ("sessionruntime/b4_owned_method_pair_lease.go", 628, "callHierarchyLayer1GenericLease.Callback", "NONREUSABLE_BORROW", "NEARBY_LEASE", "EXISTING_NOT_REUSABLE", "nearby generic callback lease is synchronous and not reusable"),
]
existing = []
for path, line, symbol, kind, component, classification, observation in existing_specs:
    p = ROOT / path
    existing.append({
        "stage_a_row_id": stable_id("adr0011.c15.two-stage.stage-a.existing.v1", path, symbol, kind, component),
        "row_kind": "EXISTING_SOURCE",
        "status": classification,
        "source_path": path,
        "source_start_line": str(line),
        "source_end_line": str(line),
        "source_sha256": file_digest(p),
        "symbol": symbol,
        "operation_kind": kind,
        "component": component,
        "attempt_identity": "CURRENT_SOURCE_ONLY",
        "account_identity": "CURRENT_SOURCE_ONLY",
        "lease_identity": "CURRENT_SOURCE_ONLY",
        "terminal_paths": TERMINALS if kind == "TERMINAL_CLEANUP" else "",
        "predecessor_lineage": "GENESIS",
        "obligation": observation,
        "authority_ceiling": "SOURCE_OBSERVATION_ONLY; no future necessity or reuse conclusion",
    })
write_tsv("STAGE_A_EXISTING_SOURCE.tsv", list(existing[0]), existing)

planned_specs = []
components = ["PREPARE_FRAME", "PREPARE_RESULT", "SELECTED_HANDOFF", "DIRECTIONAL_FRAME", "DIRECTIONAL_RESULT", "RETAINED_EXTERNAL_EVIDENCE"]
for component in components:
    title = component.title().replace("_", "")
    planned_specs.append({"symbol": f"reserveDirectional{title}V3", "status": "PLANNED_RESERVATION", "component": component, "identity": f"attempt_id+account_id+{component}+exact_bytes", "predecessor_roles": [], "obligation": "reserve before owned effect"})
    planned_specs.append({"symbol": f"copyDirectional{title}V3", "status": "PLANNED_COPY", "component": component, "identity": f"attempt_id+account_id+{component}+backing_id", "predecessor_roles": [(component, "PLANNED_RESERVATION")], "obligation": "owned bytes and all leases remain inseparable"})
    planned_specs.append({"symbol": f"aliasDirectional{title}V3", "status": "PLANNED_ALIAS", "component": component, "identity": f"attempt_id+account_id+{component}+owner_lease_id+offset+length", "predecessor_roles": [(component, "PLANNED_COPY")], "obligation": "zero-charge bounded alias; no escape or independent publication"})
planned_specs.append({"symbol": "transferDirectionalAcquisitionV3", "status": "PLANNED_TRANSFER", "component": "ALL_SIX_COMPONENTS", "identity": "source_attempt+destination_attempt+source_account+destination_account+complete_lease_set", "predecessor_roles": [(c, "PLANNED_COPY") for c in components] + [(c, "PLANNED_ALIAS") for c in components], "obligation": "atomically move complete bytes plus leases; preserve source on failure"})
planned_specs.append({"symbol": "releaseDirectionalTerminalV3", "status": "PLANNED_RELEASE", "component": "ALL_SIX_COMPONENTS", "identity": "attempt_id+account_id+all_active_lease_ids+release_id", "predecessor_roles": [(c, s) for c in components for s in ("PLANNED_RESERVATION", "PLANNED_COPY", "PLANNED_ALIAS")] + [("ALL_SIX_COMPONENTS", "PLANNED_TRANSFER")], "obligation": "one terminal winner clears aliases/bytes and releases every lease exactly once"})
for spec in planned_specs:
    spec["stage_a_row_id"] = stable_id("adr0011.c15.two-stage.stage-a.planned.v1", "sessionruntime/call_hierarchy_directional_raw_v1_private.go", spec["symbol"], spec["status"], spec["component"], spec["identity"])
planned_ids = {(s["component"], s["status"]): s["stage_a_row_id"] for s in planned_specs}
planned = []
for spec in planned_specs:
    symbol, status, component, identity, obligation = spec["symbol"], spec["status"], spec["component"], spec["identity"], spec["obligation"]
    predecessor_ids = [planned_ids[role] for role in spec["predecessor_roles"]]
    planned.append({
        "stage_a_row_id": spec["stage_a_row_id"],
        "row_kind": "PLANNED_OBLIGATION",
        "status": status,
        "proposed_source_path": "sessionruntime/call_hierarchy_directional_raw_v1_private.go",
        "proposed_symbol": symbol,
        "component": component,
        "attempt_account_lease_identity": identity,
        "terminal_paths": TERMINALS if status == "PLANNED_RELEASE" else "",
        "predecessor_lineage": "GENESIS" if not predecessor_ids else "|".join(sorted(predecessor_ids)),
        "obligation": obligation,
        "source_semantics": "NOT_APPLICABLE_PREIMPLEMENTATION",
        "authority_ceiling": "PLANNED_ARCHITECTURE_OBLIGATION_ONLY; no NECESSARY/REUSE classification",
    })
write_tsv("STAGE_A_PLANNED_OBLIGATIONS.tsv", list(planned[0]), planned)

# Actual IDs include backing_identity_rule exactly as required by the documented formula.
stage_b_fields = ["actual_row_id", "source_path", "source_start_line", "source_end_line", "source_sha256", "symbol", "operation_kind", "component", "backing_identity_rule", "attempt_identity", "account_identity", "lease_identity", "classification", "contrary_evidence", "adjudicator_id", "adjudication_revision"]
write_tsv("STAGE_B_ACTUAL_CLASSIFICATION.tsv", stage_b_fields, [])
lineage_fields = ["lineage_record_id", "lineage_event", "planned_predecessor_ids", "actual_predecessor_ids", "planned_successor_ids", "actual_successor_ids", "justification", "adjudicator_id", "adjudication_revision"]
write_tsv("STAGE_B_PLANNED_ACTUAL_LINEAGE.tsv", lineage_fields, [])

# Validate the Stage A planned graph. GENESIS is the only non-row predecessor.
planned_id_set = {r["stage_a_row_id"] for r in planned}
edges = {r["stage_a_row_id"]: [] if r["predecessor_lineage"] == "GENESIS" else r["predecessor_lineage"].split("|") for r in planned}
missing_planned_predecessors = sum(p not in planned_id_set for ps in edges.values() for p in ps)
planned_branches = 0  # multiple successors are permitted; a row has exactly one emitted definition/tip.
planned_cycles = 0
visiting, visited = set(), set()
def visit(node):
    global planned_cycles
    if node in visiting:
        planned_cycles += 1
        return
    if node in visited:
        return
    visiting.add(node)
    for predecessor in edges[node]:
        visit(predecessor)
    visiting.remove(node)
    visited.add(node)
for node in sorted(planned_id_set):
    visit(node)
if missing_planned_predecessors or planned_cycles:
    raise SystemExit("invalid planned predecessor closure")

counts = {
    "target": TARGET,
    "status": "AUTHORED_NOT_GO",
    "preserved_v1_identity_count": 47,
    "preserved_v1_identity_duplicates": 0,
    "model_reported_48_provenance": "UNPINNED_INCONSISTENT_REPORT_NO_EXACT_TRANSCRIPT_ARTIFACT",
    "new_planned_architecture_obligation_count": 1,
    "identity_accounting_denominator": 48,
    "identity_accounting_formula": "47 preserved v1 identities + 1 newly introduced planned reservation architecture obligation",
    "identity_accounting_duplicates": 0,
    "identity_accounting_omissions": 0,
    "all_47_plus_1_accounted": True,
    "stage_a_existing_source_rows": len(existing),
    "stage_a_planned_obligation_rows": len(planned),
    "stage_a_unresolved_identity_accounting": 0,
    "stage_a_preimplementation_complete": True,
    "stage_a_planned_missing_predecessors": missing_planned_predecessors,
    "stage_a_planned_cycles": planned_cycles,
    "stage_a_planned_branches": planned_branches,
    "stage_b_actual_rows": 0,
    "stage_b_lineage_rows": 0,
    "stage_b_unresolved": None,
    "stage_b_implementation_go": False,
    "interface_go": False,
    "red_go": False,
    "implementation_go": False,
    "authority": 0,
}
(OUT / "COUNTS.json").write_text(json.dumps(counts, indent=2, sort_keys=True) + "\n")

source_paths = sorted({r["source_path"] for r in existing})
with (OUT / "SOURCE_MANIFEST.sha256").open("w") as f:
    f.write(f"# HEAD {HEAD}\n")
    for path in source_paths:
        p = ROOT / path
        f.write(f"{file_digest(p)}  {p.stat().st_size}  {path}\n")
    for name in ["UNRESOLVED.tsv", "COUNTS.json", "RUN_MANIFEST.sha256", "generate.py"]:
        p = V1 / name
        f.write(f"{file_digest(p)}  {p.stat().st_size}  docs/qualification/adr0011-directional-raw-custody-c15-enumeration-v1/{name}\n")

files = sorted(p for p in OUT.iterdir() if p.is_file() and p.name != "RUN_MANIFEST.sha256")
with (OUT / "RUN_MANIFEST.sha256").open("w") as f:
    for p in files:
        f.write(f"{file_digest(p)}  {p.name}\n")
print(json.dumps(counts, sort_keys=True))
