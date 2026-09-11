#!/usr/bin/env python3
"""Fail-closed semantic validation for the private Program C Gate II v4 successor."""

from __future__ import annotations

import argparse
import copy
import hashlib
import importlib.util
import json
from collections import defaultdict
from pathlib import Path
from typing import Any, Callable

HERE = Path(__file__).resolve().parent
_SPEC = importlib.util.spec_from_file_location("program_c_gate_ii_v3", HERE / "check-program-c-gate-ii.py")
if _SPEC is None or _SPEC.loader is None:
    raise RuntimeError("cannot load immutable Gate II v3 checker")
V3 = importlib.util.module_from_spec(_SPEC)
_SPEC.loader.exec_module(V3)

SHA256_PREFIX = "sha256:"
REVISION = "69ca49f456a7a38cf370131834a2178d9aae17fe"
VERSION = "v0.17.1-0.20260426204603-69ca49f456a7"
REPAIR_REVISION = "8c99aefbd55f1cf589450602394d0968591026e3"
APPROVAL_REVISION = "4403719"
EXPECTED_GATES = [f"A-{index:02d}" for index in range(1, 11)]
EXPECTED_STATES = {gate: "PASS" for gate in EXPECTED_GATES}
EXPECTED_CLASSES = ["directed", "weighted", "disconnected", "singleton", "high-degree-hub", "adversarial-input-order"]
EXPECTED_FAILURES = {
    "CAP_BREACH", "DERIVATION_REJECTED", "IDENTITY_FAILURE", "INPUT_DIGEST_MISMATCH",
    "MEMORY_EXCEEDED", "MEMORY_UNSUPPORTED", "NONCANONICAL_OUTPUT", "NONDETERMINISM",
    "NONZERO_EXIT", "PANIC", "RECEIPT_INVALID", "RECEIPT_WRITE", "SCHEDULE_INVALID",
    "SUPERVISOR_UNSUPPORTED", "TIMEOUT",
}
EXPECTED_NO_FALLBACK = {
    "fallback": False,
    "substitution": False,
    "sampling": False,
    "truncation": False,
    "approximation": False,
    "silent_omission": False,
}
EXPECTED_EXCLUSIONS = [
    "production dependency adoption",
    "production implementation",
    "deployment",
    "public API or CLI surface",
    "candidates other than the two exact named candidates",
    "revisions or versions other than the two exact named revisions",
    "licenses other than BSD-3-Clause",
    "distribution modes other than linked and bundled",
    "general network fallback",
    "root go.mod or go.sum changes",
    "organization-wide policy or precedent",
    "authority inferred from repository policy, preference, metadata, or license text",
]
EXPECTED_EVIDENCE = {
    "A-01": ["qualification/program-c/gate-i-receipts.tsv"],
    "A-02": ["qualification/program-c/candidate-comparison.v1.json"],
    "A-03": ["qualification/program-c/a-03-caller-approval.v1.json"],
    "A-04": [
        "qualification/program-c/a-04-current-platform-scope.v1.json",
        "qualification/program-c/a-04-darwin-arm64-build-reproducibility.v1.json",
    ],
    "A-05": [
        "qualification/program-c/candidates/gonum-louvain/candidate-qualification.8c99aef.receipt.json",
        "qualification/program-c/candidates/gonum-leiden/candidate-qualification.8c99aef.receipt.json",
    ],
    "A-06": [
        "qualification/program-c/a-06-fixture-inventory.v1.json",
        "qualification/program-c/candidates/gonum-louvain/candidate-qualification.8c99aef.receipt.json",
        "qualification/program-c/candidates/gonum-leiden/candidate-qualification.8c99aef.receipt.json",
    ],
    "A-07": ["qualification/program-c/a-07-boundary-accounting-contract.v1.json"],
    "A-08": ["qualification/program-c/a-08-instability-contract.v1.json"],
    "A-09": [
        "qualification/program-c/i-08-neutrality-examples.v1.json",
        "qualification/program-c/gate-i-i-08-neutrality.receipt.txt",
    ],
    "A-10": [
        "qualification/program-c/a-10-implementation-test-plan.v1.json",
        "qualification/program-c/a-10-implementation-test-plan.v1.review.json",
        "qualification/program-c/a-10-implementation-test-plan.v1.review.txt",
    ],
}
EXPECTED_RECEIPTS = {
    "Gonum Louvain": {
        "schema": "lsp-trace.private.program-c.gonum-louvain.gate-ii-qualification.v1",
        "file_sha256": "sha256:256ca22dc84e025f04adceb96d0c5475d93de17d94669d5c5f037efc153cfcc9",
        "receipt_sha256": "sha256:935e10f4d52dafa48edab94118ed5dc3fde04ba04a3ea63e431cc8fced943851",
        "algorithm": "community.Modularize",
        "version": "v0.17.0",
        "revision": "",
        "module_sum": "h1:XKuoEMdKHiUPRL0QhxePqGI7Mb6r7+elIvbVqMpvmTI=",
        "executable_sha256": "sha256:165000a0e0494a1604d0b8bcf09535e8964ba450df00a055923a28c48aba8258",
    },
    "Gonum Leiden": {
        "schema": "lsp-trace.private.program-c.gonum-leiden.gate-ii-qualification.v1",
        "file_sha256": "sha256:3684ce8994dff593316c1e4e14d2a6c3effe62996fd7d1493e0561f0128d4084",
        "receipt_sha256": "sha256:d7a89f3fe7af393a056b0bbd8e935557ae9225fd8bfbf33a6698bc3186f93818",
        "algorithm": "community.Leiden",
        "version": VERSION,
        "revision": REVISION,
        "module_sum": "h1:V43GU8qUQ/EYbEPGzTP0PANTR6RiJgEJdU/vgsFhQiU=",
        "executable_sha256": "sha256:5358362fe62dc4a1f2efedf8526fb0b156fae866bc503c1bd1dbe798584d8b4c",
    },
}
BOUNDARY_SHA256 = {
    "scripts/check-program-c-gate-ii.py": "sha256:bc489cb099abd71d57ef27a253e93db6f3f71babc8ff539f45fd8a7c6eb09943",
    "scripts/test-program-c-gate-ii.py": "sha256:a3675ad6bb8e5346c8e8cf54a2ba803adb48f3db349e521394483d63656366ee",
    "qualification/program-c/gate-ii-current-outcomes.v3.tsv": "sha256:e74e09b069945644986224b66ef84083ebbd4e41c34e65d1f0c3f52c580c4e64",
    "qualification/program-c/gate-ii-current-outcomes.v3.2a00a7d.receipt.json": "sha256:65e7b430c168353c0d8091fdc4b0935c082f8afb0ecb4960636e78fd2521f29c",
    "qualification/program-c/candidates/gonum-louvain/candidate-qualification.04343f6.receipt.json": "sha256:653665e9028e209848b45d34541a1d8cffb4d53bfbf43fb485e4a3484f3e1fd6",
    "qualification/program-c/candidates/gonum-leiden/candidate-qualification.04343f6.receipt.json": "sha256:64c82e4440085a2ea0592f681ed01513cac4385cc5ac51334e8f266f86815ff7",
    "qualification/program-c/candidates/gonum-louvain/candidate-qualification.8c99aef.receipt.json": EXPECTED_RECEIPTS["Gonum Louvain"]["file_sha256"],
    "qualification/program-c/candidates/gonum-leiden/candidate-qualification.8c99aef.receipt.json": EXPECTED_RECEIPTS["Gonum Leiden"]["file_sha256"],
}
APPROVAL_SELF = "sha256:3fb9df62968665931c051447e1519537629edb4363d07c5108cc4b2454211cda"
INVENTORY_SHA256 = "sha256:f4d600a8e465993f1e8f0664feef8234baa0dc65057b1433291a28c614f47fbc"
ROOT_GO_BLOBS = {
    "go.mod": "780c2bed983ebf1845745ec98af9eb0fefb8f50b",
    "go.sum": "9cfb61ae460edf2f2f14989cee9dfeaaac9242be",
}


def assertion(gate: str, detail: str) -> None:
    raise ValueError(f"ASSERT_GATE_II_{gate.replace('-', '_')}: {detail}")


def require(gate: str, condition: bool, detail: str) -> None:
    if not condition:
        assertion(gate, detail)


def digest(data: bytes) -> str:
    return SHA256_PREFIX + hashlib.sha256(data).hexdigest()


def git_blob(data: bytes) -> str:
    return hashlib.sha1(f"blob {len(data)}\0".encode() + data).hexdigest()


def canonical_self(value: dict[str, Any], field: str) -> str:
    copied = copy.deepcopy(value)
    copied[field] = ""
    return digest(json.dumps(copied, separators=(",", ":"), ensure_ascii=False).encode())


def load_json(gate: str, path: Path) -> dict[str, Any]:
    return V3.load_json(gate, path)


def safe(root: Path, relative: str) -> Path:
    return V3.safe_evidence(root, relative)


def check_boundaries(root: Path) -> None:
    for relative, expected in BOUNDARY_SHA256.items():
        path = safe(root, relative)
        require("BOUNDARY", digest(path.read_bytes()) == expected, f"immutable predecessor changed: {relative}")
    for relative, expected in ROOT_GO_BLOBS.items():
        path = safe(root, relative)
        require("BOUNDARY", git_blob(path.read_bytes()) == expected, f"root module changed: {relative}")
    old_receipt = load_json("BOUNDARY", safe(root, "qualification/program-c/gate-ii-current-outcomes.v3.2a00a7d.receipt.json"))
    require("BOUNDARY", old_receipt.get("receipt_sha256") == "sha256:76f6473900e41aae7ad6202316e63b659afbe0418954e07041888acb56ce8c3f", "v3 retained receipt identity")


def check_approval(gate: str, path: Path, enforce_identity: bool = True) -> dict[str, Any]:
    doc = load_json(gate, path)
    require(gate, doc.get("schema_version") == "lsp-trace.private.program-c.a-03-caller-approval.v1" and doc.get("gate_id") == gate and doc.get("decision") == "APPROVED", "approval identity/result")
    require(gate, canonical_self(doc, "approval_sha256") == doc.get("approval_sha256"), "approval self-digest")
    if enforce_identity:
        require(gate, doc.get("approval_sha256") == APPROVAL_SELF, "exact immutable approval identity")
    authority = doc.get("authority", {})
    require(gate, authority == {
        "kind": "CALLER_ASSERTED",
        "inferred": False,
        "asserted_in": "current Program C Gate II task request",
        "exact_instruction": "Record caller-asserted narrow BSD-3-Clause linked/bundled approval for A-03",
        "repository_policy_is_authority": False,
        "license_text_is_authority": False,
        "legal_advice": False,
    }, "exact caller-asserted non-inferred authority")
    grant = doc.get("grant", {})
    require(gate, grant.get("license") == "BSD-3-Clause" and grant.get("distribution_modes") == ["linked", "bundled"], "exact BSD-3-Clause linked/bundled grant")
    candidates = grant.get("candidates")
    require(gate, isinstance(candidates, list) and len(candidates) == 2, "exactly two approved candidates")
    require(gate, candidates[0] == {
        "name": "Gonum Louvain", "algorithm": "community.Modularize", "module": "gonum.org/v1/gonum",
        "version": "v0.17.0", "module_sum": EXPECTED_RECEIPTS["Gonum Louvain"]["module_sum"],
    }, "exact Louvain approval scope")
    require(gate, candidates[1] == {
        "name": "Gonum Leiden", "algorithm": "community.Leiden", "module": "gonum.org/v1/gonum",
        "revision": REVISION, "pseudo_version": VERSION, "module_sum": EXPECTED_RECEIPTS["Gonum Leiden"]["module_sum"],
    }, "exact Leiden approval scope")
    require(gate, doc.get("exclusions") == EXPECTED_EXCLUSIONS, "exact narrow exclusions")
    bindings = doc.get("bindings", {})
    expected_commits = {
        "qualifier_implementation": "04343f62cb7e2c5cc9d1222eee49438e254d8812",
        "failed_attempt_receipts": "13a5d7179753a20c0fe63d82fa901fb9be3add20",
        "canonicalization_repair": REPAIR_REVISION,
        "successful_receipts": "faa52ec2f39e4430f33d25e4ca6b75e5ff172649",
    }
    require(gate, bindings.get("predecessor_commits") == expected_commits, "exact predecessor commit chain")
    for group in ("candidate_inventories", "license_inputs"):
        records = bindings.get(group)
        require(gate, isinstance(records, list) and records, f"nonempty {group} bindings")
        for record in records:
            bound = safe(path.parents[2], record.get("path", ""))
            require(gate, record.get("sha256") == digest(bound.read_bytes()), f"exact {group} binding {record.get('path')}")
    inventory_binding = bindings.get("fixture_inventory", {})
    inventory_path = safe(path.parents[2], inventory_binding.get("path", ""))
    require(gate, inventory_binding.get("sha256") == INVENTORY_SHA256 == digest(inventory_path.read_bytes()), "exact fixture inventory binding")
    receipt_bindings = bindings.get("successful_qualification_receipts")
    require(gate, isinstance(receipt_bindings, list) and len(receipt_bindings) == 2, "exact successful receipt bindings")
    for record, (name, expected) in zip(receipt_bindings, EXPECTED_RECEIPTS.items()):
        receipt_path = safe(path.parents[2], record.get("path", ""))
        receipt_doc = load_json(gate, receipt_path)
        require(gate, record.get("file_sha256") == expected["file_sha256"] == digest(receipt_path.read_bytes()), f"exact {name} receipt file binding")
        require(gate, record.get("receipt_sha256") == expected["receipt_sha256"] == receipt_doc.get("receipt_sha256"), f"exact {name} receipt self binding")
        require(gate, receipt_doc.get("result") == "PASS", f"bound {name} receipt passes")
    ceiling = doc.get("claim_ceiling", "")
    require(gate, all(term in ceiling for term in ("caller-asserted", "exact qualified Gonum Louvain v0.17.0", "Gonum Leiden 69ca49f456a7", "does not authorize production")), "narrow approval claim ceiling")
    return doc


def graph_value(gate: str, graph_bytes: str) -> dict[str, Any]:
    try:
        value = json.loads(graph_bytes)
    except (TypeError, json.JSONDecodeError) as exc:
        assertion(gate, f"invalid exact graph bytes: {exc}")
    require(gate, isinstance(value, dict), "graph bytes must decode to object")
    return value


def check_fixture_inventory(gate: str, path: Path, enforce_identity: bool = True) -> dict[str, Any]:
    if enforce_identity:
        require(gate, digest(path.read_bytes()) == INVENTORY_SHA256, "exact immutable fixture inventory identity")
    doc = load_json(gate, path)
    require(gate, doc.get("schema_version") == "lsp-trace.private.program-c.a-06-fixture-inventory.v1", "fixture inventory schema")
    require(gate, doc.get("inventory_id") == "program-c-gate-ii-final-qualification-v1", "fixture inventory id")
    require(gate, doc.get("seed_inventory") == [1, 2], "exact seed inventory")
    require(gate, doc.get("schedule") == {
        "base_repeats_per_candidate_fixture_seed": 3,
        "deterministic_input_permutations_per_candidate_fixture_seed": 1,
        "permutation": "reverse exact node insertion order and exact directed-edge insertion order",
    }, "exact repeat/permutation schedule")
    require(gate, doc.get("limits") == {
        "wall_seconds_per_run": 60,
        "aggregate_process_tree_rss_bytes_per_run": 536870912,
        "maximum_nodes_per_fixture": 10000,
        "maximum_directed_weighted_edges_per_fixture": 100000,
    }, "exact resource/cardinality limits")
    require(gate, doc.get("required_classes") == EXPECTED_CLASSES, "exact six required classes")
    expected_scope = [
        {"name": "Gonum Louvain", "module": "gonum.org/v1/gonum", "version": "v0.17.0"},
        {"name": "Gonum Leiden", "module": "gonum.org/v1/gonum", "revision": REVISION, "version": VERSION},
    ]
    require(gate, doc.get("candidate_scope") == expected_scope, "exact two-candidate scope")
    fixtures = doc.get("fixtures")
    require(gate, isinstance(fixtures, list) and len(fixtures) == 4, "exact four-fixture inventory")
    expected_ids = ["retained-calls-v1", "disconnected-singleton", "high-degree-hub", "adversarial-order"]
    require(gate, [fixture.get("id") for fixture in fixtures] == expected_ids, "exact ordered fixture ids")
    covered: set[str] = set()
    totals = [0, 0]
    root = path.parents[2]
    for fixture in fixtures:
        graph_record = fixture.get("graph", {})
        graph_path = safe(root, graph_record.get("path", ""))
        graph_bytes = graph_path.read_bytes()
        require(gate, graph_record.get("sha256") == digest(graph_bytes), f"exact graph digest {fixture.get('id')}")
        try:
            graph_text = graph_bytes.decode("utf-8")
        except UnicodeDecodeError as exc:
            assertion(gate, f"non-UTF-8 graph {fixture.get('id')}: {exc}")
        graph = graph_value(gate, graph_text)
        require(gate, graph.get("schema_version") == "lsp-trace.private.program-c.graph-fixture.v1" and graph.get("directed") is True and graph.get("weighted") is True, f"graph schema {fixture.get('id')}")
        nodes, edges = graph.get("nodes"), graph.get("edges")
        require(gate, isinstance(nodes, list) and nodes and all(isinstance(node, str) and node for node in nodes) and len(nodes) == len(set(nodes)), f"valid nodes {fixture.get('id')}")
        require(gate, isinstance(edges, list), f"valid edges {fixture.get('id')}")
        edge_keys: set[tuple[str, str]] = set()
        for edge in edges:
            require(gate, isinstance(edge, dict) and set(edge) == {"from", "to", "weight"}, f"edge shape {fixture.get('id')}")
            require(gate, edge.get("from") in nodes and edge.get("to") in nodes and isinstance(edge.get("weight"), int) and edge.get("weight") > 0, f"edge semantics {fixture.get('id')}")
            key = (edge["from"], edge["to"])
            require(gate, key not in edge_keys, f"duplicate directed edge {fixture.get('id')}")
            edge_keys.add(key)
        require(gate, fixture.get("node_count") == len(nodes) and fixture.get("directed_weighted_edge_count") == len(edges), f"fixture counts {fixture.get('id')}")
        require(gate, fixture.get("directed") is True and fixture.get("weighted") is True and fixture.get("weight_semantics") == "positive integer admitted CALL occurrence count", f"directed weighted semantics {fixture.get('id')}")
        classes = fixture.get("covered_classes")
        require(gate, isinstance(classes, list) and classes and set(classes) <= set(EXPECTED_CLASSES), f"fixture classes {fixture.get('id')}")
        covered.update(classes)
        totals[0] += len(nodes)
        totals[1] += len(edges)
        provenance = fixture.get("provenance", {})
        source_path = safe(root, provenance.get("source_path", ""))
        require(gate, provenance.get("source_sha256") == digest(source_path.read_bytes()) and provenance.get("derivation"), f"fixture provenance {fixture.get('id')}")
        projection = fixture.get("projection", {})
        require(gate, projection.get("name") == "calls-v1" and projection.get("logical_digest") == "sha256:f4fb309c6e849b5a8e6057f355b1db6ee3c53414c3c430b67be17f68fe9e97f9", f"fixture projection {fixture.get('id')}")
    require(gate, covered == set(EXPECTED_CLASSES) and totals == [21, 25], "complete six-class 21-node/25-edge denominator")
    require(gate, doc.get("claim_ceiling", "").startswith("This private fixture inventory authorizes only exact bounded Program C qualification"), "fixture claim ceiling")
    return doc


def check_candidate_receipt(
    gate: str,
    path: Path,
    expected: dict[str, str],
    inventory: dict[str, Any],
    enforce_identity: bool = True,
    verify_inputs: bool = True,
) -> dict[str, Any]:
    if enforce_identity:
        require(gate, digest(path.read_bytes()) == expected["file_sha256"], f"exact immutable {expected['algorithm']} receipt file")
    doc = load_json(gate, path)
    require(gate, canonical_self(doc, "receipt_sha256") == doc.get("receipt_sha256"), "receipt self-digest")
    if enforce_identity:
        require(gate, doc.get("receipt_sha256") == expected["receipt_sha256"], "exact immutable receipt self identity")
    require(gate, doc.get("schema") == expected["schema"] and doc.get("result") == "PASS" and doc.get("recommendation") == "IMPLEMENTATION_CANDIDATE", "receipt schema/result/recommendation")
    repository = doc.get("repository", {})
    require(gate, repository.get("revision") == REPAIR_REVISION and repository.get("dirty") is False, "clean exact repair revision")
    candidate = doc.get("candidate", {})
    require(gate, candidate.get("name") in EXPECTED_RECEIPTS and candidate.get("module") == "gonum.org/v1/gonum", "exact candidate name/module")
    require(gate, candidate.get("algorithm") == expected["algorithm"] and candidate.get("version") == expected["version"] and candidate.get("module_sum") == expected["module_sum"], "exact candidate algorithm/version/module sum")
    require(gate, candidate.get("executable_sha256") == expected["executable_sha256"], "exact candidate executable")
    if expected["revision"]:
        require(gate, candidate.get("revision") == expected["revision"], "exact candidate revision")
        require(gate, candidate.get("go_mod_sum") == "h1:El3tOrEuMpv2UdMrbNlKEh9vd86bmQ6vqIcDwxEOc1E=" and candidate.get("license_sha256") == "sha256:b44d9e394ba3efc15de4e5a8ebd843bd8d6325f6ffbb0a4ff6db7181aa15dda3", "exact Leiden module/license identity")
    require(gate, doc.get("fixture_inventory") == {
        "path": "qualification/program-c/a-06-fixture-inventory.v1.json",
        "sha256": INVENTORY_SHA256,
        "inventory_id": "program-c-gate-ii-final-qualification-v1",
    }, "receipt binds exact fixture inventory")
    require(gate, doc.get("fixtures") == inventory.get("fixtures"), "receipt fixture records exactly equal inventory")
    require(gate, doc.get("schedule") == {
        "seed_inventory": [1, 2], "base_repeats_per_fixture_seed": 3,
        "deterministic_input_permutations_per_fixture_seed": 1,
    }, "exact receipt schedule")
    require(gate, doc.get("limits") == {
        "wall_seconds": 60, "aggregate_process_tree_rss_bytes": 536870912,
        "nodes": 10000, "directed_weighted_edges": 100000,
    }, "exact receipt limits")
    require(gate, set(doc.get("typed_failure_policy", [])) == EXPECTED_FAILURES, "complete typed failure policy")
    require(gate, doc.get("no_fallback_policy") == EXPECTED_NO_FALLBACK, "no fallback/substitution/sampling/truncation/omission")
    build = doc.get("build", {})
    require(gate, build.get("required_flags") == ["-trimpath", "-buildvcs=false", "-ldflags=-buildid="] and build.get("candidate_command") and build.get("qualifier_command"), "exact retained build commands")
    supervisor = doc.get("supervisor", {})
    require(gate, supervisor.get("process_group") is True and supervisor.get("canonicalization_boundary") == "external parent supervisor process" and supervisor.get("rss_metric") == "aggregate descendant RSS from /bin/ps -axo pid=,ppid=,rss=" and supervisor.get("fail_closed") is True, "external Darwin process-tree supervisor")
    runtime = doc.get("runtime", {})
    require(gate, runtime.get("goos") == "darwin" and runtime.get("goarch") == "arm64", "exact Darwin arm64 runtime")
    counts = doc.get("counts")
    require(gate, counts == {"fixtures": 4, "seeds": 2, "expected_runs": 32, "completed_runs": 32, "failed_runs": 0}, "exact complete run counts")
    inputs = doc.get("inputs")
    require(gate, isinstance(inputs, dict) and inputs, "nonempty exact input digest map")
    if verify_inputs:
        root = path.parents[4]
        for relative, expected_digest in inputs.items():
            input_path = safe(root, relative)
            require(gate, digest(input_path.read_bytes()) == expected_digest, f"exact receipt input {relative}")
    fixtures = inventory["fixtures"]
    expected_schedule: list[tuple[int, str, int, int, str, bool]] = []
    index = 0
    for fixture in fixtures:
        for seed in (1, 2):
            for repeat in (1, 2, 3):
                expected_schedule.append((index, fixture["id"], seed, repeat, "base_repeat", False))
                index += 1
            expected_schedule.append((index, fixture["id"], seed, 0, "deterministic_input_permutation", True))
            index += 1
    runs = doc.get("runs")
    require(gate, isinstance(runs, list) and len(runs) == 32, "exact 32-run receipt denominator")
    cells: dict[tuple[str, int], list[str]] = defaultdict(list)
    expected_outputs: list[dict[str, Any]] = []
    for run, expected_run in zip(runs, expected_schedule):
        run_index, fixture_id, seed, repeat, kind, reverse = expected_run
        require(gate, (run.get("index"), run.get("fixture_id"), run.get("seed"), run.get("repeat"), run.get("kind")) == (run_index, fixture_id, seed, repeat, kind), f"exact run schedule index {run_index}")
        require(gate, run.get("outcome") == "PASS" and run.get("failure") is None, f"typed passing outcome index {run_index}")
        request = run.get("request")
        require(gate, isinstance(request, dict) and request.get("seed") == seed and request.get("reverse_insertion") is reverse, f"exact run request index {run_index}")
        require(gate, run.get("request_sha256") == digest(json.dumps(request, separators=(",", ":"), ensure_ascii=False).encode()), f"exact request digest index {run_index}")
        fixture_request = request.get("fixture", {})
        graph_text = fixture_request.get("graph_bytes")
        graph = graph_value(gate, graph_text)
        fixture_record = next(item for item in fixtures if item["id"] == fixture_id)
        graph_path = safe(path.parents[4], fixture_record["graph"]["path"])
        require(gate, graph_text.encode() == graph_path.read_bytes() and fixture_request.get("graph_sha256") == fixture_record["graph"]["sha256"], f"exact graph bytes index {run_index}")
        nodes = graph.get("nodes")
        require(gate, isinstance(nodes, list), f"graph nodes index {run_index}")
        result = run.get("result", {})
        raw = result.get("raw_candidate_output", {}).get("communities")
        require(gate, isinstance(raw, list), f"raw candidate communities index {run_index}")
        raw_members: list[str] = []
        normalized: list[list[str]] = []
        empty_count = 0
        for community in raw:
            if community is None or community == []:
                empty_count += 1
                continue
            require(gate, isinstance(community, list) and all(isinstance(member, str) and member for member in community), f"raw member shape index {run_index}")
            raw_members.extend(community)
            normalized.append(sorted(community))
        require(gate, sorted(raw_members) == sorted(nodes) and len(raw_members) == len(nodes), f"raw complete exact node partition index {run_index}")
        normalized.sort()
        output = result.get("externally_canonicalized_output", {})
        require(gate, output.get("communities") == normalized and all(normalized), f"external canonical partition index {run_index}")
        require(gate, output.get("digest") == digest(json.dumps(normalized, separators=(",", ":"), ensure_ascii=False).encode()), f"canonical output digest index {run_index}")
        require(gate, result.get("canonicalization") == {"boundary": "external parent supervisor process", "empty_communities_removed": empty_count}, f"explicit normalization accounting index {run_index}")
        observation = result.get("observation", {})
        samples = observation.get("resource_samples")
        require(gate, isinstance(samples, list) and samples, f"retained resource samples index {run_index}")
        sample_times: list[int] = []
        sample_rss: list[int] = []
        for sample in samples:
            elapsed, rss = sample.get("elapsed_nanos"), sample.get("tree_rss_bytes")
            require(gate, isinstance(elapsed, int) and elapsed >= 0 and isinstance(rss, int) and 0 <= rss <= 536870912, f"bounded resource sample index {run_index}")
            sample_times.append(elapsed)
            sample_rss.append(rss)
        require(gate, sample_times == sorted(sample_times), f"ordered resource samples index {run_index}")
        require(gate, observation.get("peak_tree_rss_bytes") == max(sample_rss) and observation.get("elapsed_nanos", 60000000001) <= 60000000000 and observation.get("elapsed_nanos", -1) >= sample_times[-1] and observation.get("exit_kind") == "OK", f"exact resource observation index {run_index}")
        cells[(fixture_id, seed)].append(output["digest"])
        if kind == "deterministic_input_permutation":
            expected_outputs.append({"fixture_id": fixture_id, "seed": seed, "output": output})
    require(gate, len(cells) == 8 and all(len(values) == 4 and len(set(values)) == 1 for values in cells.values()), "three repeats plus permutation are deterministic in every cell")
    require(gate, doc.get("canonical_outputs") == expected_outputs, "exact canonical output index")
    ceiling = doc.get("claim_ceiling", "")
    require(gate, expected["version"] in ceiling and "exact bounded execution" in ceiling and "does not authorize production implementation" in ceiling and "other candidates, revisions, licenses, or distribution modes" in ceiling, "narrow candidate claim ceiling")
    return doc


def check_a03(gate: str, paths: list[Path]) -> None:
    check_approval(gate, paths[0])


def check_a05(gate: str, paths: list[Path]) -> None:
    inventory_path = safe(paths[0].parents[4], "qualification/program-c/a-06-fixture-inventory.v1.json")
    inventory = check_fixture_inventory(gate, inventory_path)
    for path, expected in zip(paths, EXPECTED_RECEIPTS.values()):
        check_candidate_receipt(gate, path, expected, inventory)


def check_a06(gate: str, paths: list[Path]) -> None:
    inventory = check_fixture_inventory(gate, paths[0])
    for path, expected in zip(paths[1:], EXPECTED_RECEIPTS.values()):
        check_candidate_receipt(gate, path, expected, inventory)


CHECKS: dict[str, Callable[[str, list[Path]], None]] = {
    "A-01": V3.check_a01,
    "A-02": V3.check_a02,
    "A-03": check_a03,
    "A-04": V3.check_a04,
    "A-05": check_a05,
    "A-06": check_a06,
    "A-07": V3.check_a07,
    "A-08": V3.check_a08,
    "A-09": V3.check_a09,
    "A-10": V3.check_a10,
}


def evaluate(root: Path, outcomes: str) -> tuple[dict[str, int], bool, dict[str, str]]:
    root = root.resolve(strict=True)
    check_boundaries(root)
    ledger = safe(root, outcomes)
    rows: list[tuple[str, str]] = []
    bases: dict[str, str] = {}
    for number, line in enumerate(ledger.read_text(encoding="utf-8").splitlines(), 1):
        if not line or line.startswith("#"):
            continue
        fields = line.split("\t")
        if len(fields) != 4:
            raise ValueError(f"invalid Gate II v4 row at line {number}")
        gate, state, evidence, basis = fields
        require(gate, state == EXPECTED_STATES.get(gate), "v4 requires exact PASS state")
        require(gate, bool(evidence) and bool(basis), "nonempty evidence and basis")
        relatives = evidence.split(";")
        require(gate, relatives == EXPECTED_EVIDENCE.get(gate), "exact ordered v4 evidence inventory")
        CHECKS[gate](gate, [safe(root, relative) for relative in relatives])
        rows.append((gate, state))
        bases[gate] = basis
    require("LEDGER", [gate for gate, _ in rows] == EXPECTED_GATES, "exact ordered A-01 through A-10")
    counts = {state: sum(value == state for _, value in rows) for state in ("PASS", "FAIL", "BLOCKED")}
    allowed = counts == {"PASS": 10, "FAIL": 0, "BLOCKED": 0}
    require("LEDGER", allowed, "exact all-PASS v4 counts")
    return counts, allowed, bases


def check_successor_receipt(root: Path, relative: str, outcomes: str, counts: dict[str, int]) -> None:
    path = safe(root, relative)
    doc = load_json("RECEIPT", path)
    require("RECEIPT", doc.get("schema_version") == "lsp-trace.private.program-c.gate-ii-evaluation.v4", "v4 receipt schema")
    require("RECEIPT", canonical_self(doc, "receipt_sha256") == doc.get("receipt_sha256"), "v4 receipt self-digest")
    require("RECEIPT", doc.get("predecessor_revision") == "440371930eac5ad0a9c7d132d5097dcfa8f131b4", "exact approval predecessor revision")
    require("RECEIPT", doc.get("supersedes") == {
        "path": "qualification/program-c/gate-ii-current-outcomes.v3.2a00a7d.receipt.json",
        "receipt_sha256": "sha256:76f6473900e41aae7ad6202316e63b659afbe0418954e07041888acb56ce8c3f",
    }, "exact v3 receipt predecessor")
    require("RECEIPT", doc.get("command") == [
        "python3", "scripts/check-program-c-gate-ii-v4.py", "--root", ".", "--outcomes", outcomes,
        "--receipt", relative,
    ], "exact stable replay command")
    inputs = doc.get("inputs")
    require("RECEIPT", isinstance(inputs, dict) and inputs, "nonempty v4 receipt input map")
    for input_relative, expected_digest in inputs.items():
        require("RECEIPT", input_relative != relative, "receipt must not claim a circular self input")
        require("RECEIPT", digest(safe(root, input_relative).read_bytes()) == expected_digest, f"exact v4 receipt input {input_relative}")
    stdout = "PROGRAM_C_GATE_II PASS=10 FAIL=0 BLOCKED=0 IMPLEMENTATION_DECISION_ALLOWED=true\n"
    result = doc.get("result", {})
    require("RECEIPT", result == {
        "stdout": stdout,
        "stdout_sha256": digest(stdout.encode()),
        "counts": {"PASS": 10, "FAIL": 0, "BLOCKED": 0},
        "implementation_decision_allowed": True,
    }, "exact v4 result and counts")
    require("RECEIPT", counts == result["counts"] and doc.get("outcomes") == EXPECTED_STATES, "receipt matches evaluated ledger")
    ceiling = doc.get("claim_ceiling", "")
    require("RECEIPT", all(term in ceiling for term in ("exact bounded", "implementation candidate", "does not authorize production adoption", "public surfaces", "deployment", "semantic boundary claims")), "narrow v4 claim ceiling")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", default=".")
    parser.add_argument("--outcomes", default="qualification/program-c/gate-ii-current-outcomes.v4.tsv")
    parser.add_argument("--receipt", default="qualification/program-c/gate-ii-current-outcomes.v4.4403719.receipt.json")
    args = parser.parse_args()
    try:
        counts, allowed, _ = evaluate(Path(args.root), args.outcomes)
        check_successor_receipt(Path(args.root).resolve(strict=True), args.receipt, args.outcomes, counts)
    except (KeyError, OSError, UnicodeDecodeError, ValueError) as exc:
        raise SystemExit(str(exc)) from exc
    print(
        "PROGRAM_C_GATE_II "
        f"PASS={counts['PASS']} FAIL={counts['FAIL']} BLOCKED={counts['BLOCKED']} "
        f"IMPLEMENTATION_DECISION_ALLOWED={str(allowed).lower()}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
