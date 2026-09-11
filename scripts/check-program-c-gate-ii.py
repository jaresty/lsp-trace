#!/usr/bin/env python3
"""Validate the private, non-authorizing Program C Gate II outcome ledger."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import stat
from pathlib import Path, PurePosixPath
from typing import Any, Callable

ALLOWED = {"PASS", "FAIL", "BLOCKED"}
EXPECTED = [f"A-{i:02d}" for i in range(1, 11)]
REVISION = "69ca49f456a7a38cf370131834a2178d9aae17fe"
VERSION = "v0.17.1-0.20260426204603-69ca49f456a7"
SHA256_PREFIX = "sha256:"

EXPECTED_STATES = {
    "A-01": "PASS", "A-02": "PASS", "A-03": "BLOCKED", "A-04": "PASS",
    "A-05": "PASS", "A-06": "FAIL", "A-07": "PASS", "A-08": "PASS",
    "A-09": "PASS", "A-10": "PASS",
}
EXPECTED_EVIDENCE = {
    "A-01": ["qualification/program-c/gate-i-receipts.tsv"],
    "A-02": ["qualification/program-c/candidate-comparison.v1.json"],
    "A-03": ["qualification/program-c/i-04-license-inputs.v2.json"],
    "A-04": [
        "qualification/program-c/a-04-current-platform-scope.v1.json",
        "qualification/program-c/a-04-darwin-arm64-build-reproducibility.v1.json",
    ],
    "A-05": ["qualification/program-c/candidates/gonum-leiden/candidate-execution.af7f038.receipt.json"],
    "A-06": ["qualification/program-c/candidates/gonum-leiden/candidate-execution.af7f038.receipt.json"],
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


def assertion(gate: str, detail: str) -> None:
    raise ValueError(f"ASSERT_GATE_II_{gate.replace('-', '_')}: {detail}")


def require(gate: str, condition: bool, detail: str) -> None:
    if not condition:
        assertion(gate, detail)


def digest(data: bytes) -> str:
    return SHA256_PREFIX + hashlib.sha256(data).hexdigest()


def safe_evidence(root: Path, relative: str) -> Path:
    """Return a repository-contained regular file, rejecting all symlink components."""
    if not relative or "\\" in relative:
        raise ValueError(f"invalid Gate II evidence path: {relative}")
    pure = PurePosixPath(relative)
    if pure.is_absolute() or any(part in {"", ".", ".."} for part in pure.parts):
        raise ValueError(f"invalid Gate II evidence path: {relative}")
    current = root
    for part in pure.parts:
        current = current / part
        try:
            mode = os.lstat(current).st_mode
        except OSError as exc:
            raise ValueError(f"invalid Gate II evidence path: {relative}: {exc.strerror}") from exc
        if stat.S_ISLNK(mode):
            raise ValueError(f"invalid Gate II evidence path: {relative}: symlink")
    try:
        resolved = current.resolve(strict=True)
    except OSError as exc:
        raise ValueError(f"invalid Gate II evidence path: {relative}: {exc.strerror}") from exc
    if not resolved.is_relative_to(root) or not stat.S_ISREG(os.stat(resolved).st_mode):
        raise ValueError(f"invalid Gate II evidence path: {relative}")
    return resolved


def load_json(gate: str, path: Path) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as exc:
        assertion(gate, f"invalid JSON evidence {path.name}: {exc}")
    require(gate, isinstance(value, dict), f"JSON evidence {path.name} must be an object")
    return value


def check_a01(gate: str, paths: list[Path]) -> None:
    rows = []
    for line in paths[0].read_text(encoding="utf-8").splitlines():
        if line and not line.startswith("#"):
            fields = line.split("\t")
            require(gate, len(fields) == 4, "Gate I row shape")
            rows.append(fields)
    require(gate, [r[0] for r in rows] == [f"I-{i:02d}" for i in range(1, 9)], "exact I-01 through I-08")
    require(gate, all(r[1] == "PASS" and r[2] and r[3].startswith(SHA256_PREFIX) for r in rows), "all Gate I rows pass with receipt digests")


def check_a02(gate: str, paths: list[Path]) -> None:
    doc = load_json(gate, paths[0])
    candidates = doc.get("evaluated_candidates")
    require(gate, doc.get("gate_id") == gate and doc.get("result") == "PASS", "comparison identity/result")
    require(gate, isinstance(candidates, list) and len(candidates) == 2, "exactly two evaluated candidates")
    require(gate, {c.get("name") for c in candidates} == {"Gonum Louvain", "Gonum Leiden"}, "candidate identities")
    selection = doc.get("selection", {})
    require(gate, selection.get("candidate") == "Gonum Leiden" and selection.get("revision") == REVISION, "exact selected Leiden commit")


def check_a03(gate: str, paths: list[Path]) -> None:
    doc = load_json(gate, paths[0])
    require(gate, doc.get("gate_id") == "I-04", "license input identity")
    entry = doc.get("entry", {})
    require(gate, entry.get("revision") == REVISION and entry.get("license") == "BSD-3-Clause", "exact license input")
    policy = doc.get("policy", "")
    require(gate, "no legal compatibility conclusion" in policy and "production adoption approval" in policy, "approval remains absent")


def check_a04(gate: str, paths: list[Path]) -> None:
    scope, receipt = (load_json(gate, path) for path in paths)
    required = scope.get("current_required_coordinates")
    excluded = scope.get("historical_broader_coordinates")
    require(gate, scope.get("schema_version") == "lsp-trace.private.program-c.a-04-platform-scope.v1", "scope schema")
    require(gate, scope.get("gate_id") == gate and scope.get("status") == "CURRENT_INTENDED_INITIAL_SCOPE", "scope identity/status")
    require(gate, required == [{"goos": "darwin", "goarch": "arm64", "cgo_enabled": "0"}], "exact Darwin arm64 scope")
    expected_excluded = {(o, a) for o in ("linux", "darwin", "windows") for a in ("amd64", "arm64")} - {("darwin", "arm64")}
    require(gate, isinstance(excluded, list) and {(x.get("goos"), x.get("goarch")) for x in excluded} == expected_excluded, "complete historical coordinate disposition")
    require(gate, all(x.get("disposition") == "OUT_OF_CURRENT_SCOPE" for x in excluded), "historical coordinates are only out of scope")
    require(gate, set(scope.get("out_of_scope_is_not", [])) == {"PASS", "WAIVED", "QUALIFIED", "UNSUPPORTED"}, "out-of-scope non-entailments")
    predecessor = scope.get("predecessor_policy", {})
    require(gate, predecessor.get("path") == "qualification/program-c/i-05-portability-questions.v1.json" and predecessor.get("disposition") == "HISTORICAL_POLICY_RETAINED_BYTE_IDENTICAL", "historical policy binding")
    requirement = scope.get("reproducibility_requirement", {})
    require(gate, requirement.get("minimum_identical_input_builds", 0) >= 2 and requirement.get("all_executable_digests_must_match") is True, "two-build requirement")
    require(gate, receipt.get("schema_version") == "lsp-trace.private.program-c.a-04-build-reproducibility.v1", "receipt schema")
    require(gate, receipt.get("gate_id") == gate and receipt.get("result") == "PASS", "receipt result")
    require(gate, len(receipt.get("repository_revision", "")) == 40, "exact repository revision")
    qualifier = receipt.get("qualifier", {})
    qualifier_path = safe_evidence(paths[0].parents[2], qualifier.get("path", ""))
    require(gate, qualifier.get("sha256") == digest(qualifier_path.read_bytes()), "exact qualifier identity")
    require(gate, receipt.get("scope_policy", {}).get("sha256") == digest(paths[0].read_bytes()), "receipt binds exact scope policy")
    candidate = receipt.get("candidate", {})
    require(gate, candidate.get("module") == "gonum.org/v1/gonum" and candidate.get("revision") == REVISION and candidate.get("pseudo_version") == VERSION, "exact module identity")
    coordinate = receipt.get("coordinate", {})
    require(gate, coordinate == {"goos": "darwin", "goarch": "arm64", "cgo_enabled": "0"}, "exact observed coordinate")
    builds = receipt.get("builds")
    require(gate, isinstance(builds, list) and len(builds) >= 2, "at least two builds")
    commands = [b.get("command") for b in builds]
    hashes = [b.get("executable_sha256") for b in builds]
    require(gate, all(b.get("outcome") == "PASS" and b.get("failure") is None for b in builds), "typed successful build outcomes")
    require(gate, all(isinstance(command, list) and command for command in commands) and len({tuple(command) for command in commands}) == 1, "identical build commands")
    require(gate, len(set(hashes)) == 1 and hashes[0] and hashes[0].startswith(SHA256_PREFIX), "matching executable hashes")
    identities = receipt.get("identities", {})
    require(gate, set(identities) == {"tool", "source", "module", "command", "environment", "package"}, "complete identity classes")
    require(gate, receipt.get("typed_failure_policy") == requirement.get("typed_outcomes"), "typed failure policy binding")
    network = receipt.get("network", {})
    require(gate, network.get("goproxy") == "https://proxy.golang.org", "exact authorized module proxy")
    require(gate, network.get("general_fallback_allowed") is False and network.get("fallback_attempted") is False, "no general network fallback")


def check_a05(gate: str, paths: list[Path]) -> None:
    doc = load_json(gate, paths[0])
    candidate = doc.get("candidate", {})
    runtime = doc.get("runtime", {})
    runs = doc.get("runs")
    require(gate, candidate.get("revision") == REVISION and candidate.get("version") == VERSION, "exact deterministic candidate")
    require(gate, runtime.get("goos") == "darwin" and runtime.get("goarch") == "arm64", "declared deterministic runtime")
    require(gate, isinstance(runs, list) and len(runs) >= 4, "three replay runs plus permutation")
    outputs = [r.get("result", {}).get("output", {}).get("digest") for r in runs]
    require(gate, outputs and len(set(outputs)) == 1 and outputs[0] == doc.get("canonical_output", {}).get("digest"), "canonical deterministic digest")


def check_a06(gate: str, paths: list[Path]) -> None:
    doc = load_json(gate, paths[0])
    require(gate, doc.get("candidate", {}).get("revision") == REVISION, "resource evidence candidate")
    require(gate, len(doc.get("runs", [])) == 4 and doc.get("limits", {}).get("nodes") == 10000 and doc.get("limits", {}).get("edges") == 100000, "honest bounded single-fixture evidence")


def check_a07(gate: str, paths: list[Path]) -> None:
    doc = load_json(gate, paths[0])
    bindings = set(doc.get("identity_bindings", []))
    required_bindings = {"admitted_graph_sha256", "projection_sha256", "projection_policy_id", "community_artifact_sha256", "algorithm_name", "algorithm_version", "parameters_canonical_sha256", "seed", "resource_policy_sha256"}
    require(gate, doc.get("gate_id") == gate and doc.get("contract_status") == "COMPLETE" and doc.get("implementation_status") == "NOT_IMPLEMENTED", "contract completeness only")
    require(gate, bindings == required_bindings, "complete identity bindings")
    universe = doc.get("relation_occurrence_universe", {})
    require(gate, universe.get("partition") == ["INTRA_COMMUNITY", "CROSSING_COMMUNITY"] and "intra_count + crossing_count" in universe.get("invariant", ""), "occurrence partition and denominator")
    measures = doc.get("required_measures", {})
    require(gate, set(measures) == {"intra_occurrences", "crossing_occurrences", "conductance", "high_centrality_crossing_nodes", "bridges_and_articulation_points", "path_witnesses", "hub_crossing_status"}, "all required boundary measures")
    require(gate, set(doc.get("outcome_accounting", {})) >= {"EMPTY", "INCOMPLETE", "UNAVAILABLE", "required_counts"}, "empty/incomplete/unavailable accounting")
    witnesses = doc.get("witness_policy", {})
    require(gate, witnesses.get("crossing_witnesses_required_when_nonempty") is True and witnesses.get("path_witnesses_required_when_nonempty") is True, "witness requirements")
    ceiling = doc.get("claim_ceiling", {})
    require(gate, ceiling.get("structural_only") is True and ceiling.get("semantic_boundary_claims") is False, "structural non-semantic ceiling")


def check_a08(gate: str, paths: list[Path]) -> None:
    doc = load_json(gate, paths[0])
    require(gate, doc.get("gate_id") == gate and doc.get("contract_status") == "COMPLETE" and doc.get("implementation_status") == "NOT_IMPLEMENTED", "contract completeness only")
    matching = doc.get("label_independent_matching", {})
    require(gate, matching.get("numeric_or_text_labels_are_identities") is False and "bipartite" in matching.get("assignment", "") and matching.get("tie_break"), "label-independent deterministic matching")
    unmatched = doc.get("unmatched_handling", {})
    require(gate, unmatched.get("included_in_denominators") is True and unmatched.get("never_dropped") is True, "unmatched accounting")
    require(gate, set(doc.get("metrics", {})) == {"node_reassignment_rate", "unmatched_community_rate", "variation_of_information_bits", "pairwise_jaccard_summary"}, "metrics and denominators")
    thresholds = doc.get("thresholds", {})
    require(gate, set(thresholds) == {"node_reassignment_rate_max", "unmatched_community_rate_max", "variation_of_information_bits_max", "pairwise_jaccard_minimum_min", "comparison_result"}, "complete thresholds")
    runs = doc.get("run_accounting", {})
    require(gate, runs.get("minimum_distinct_seeds", 0) >= 2 and runs.get("runs_per_seed") == 3 and runs.get("maximum_runs_per_seed") == 3, "seed/run accounting")
    failures = doc.get("failed_incomplete_policy", {})
    require(gate, set(failures) == {"failed_run", "incomplete_run", "missing_run", "unstable_outcome", "incomplete_outcome"}, "failed/incomplete handling")
    publication = doc.get("publication_policy", {})
    require(gate, all(publication.get(key) is True for key in ("block_on_unstable", "block_on_incomplete", "block_on_failed_run", "block_on_identity_mismatch")), "publication blocking")
    require(gate, "not a universal stability" in doc.get("claim_ceiling", ""), "non-universal stability ceiling")


def check_a09(gate: str, paths: list[Path]) -> None:
    doc = load_json(gate, paths[0])
    allowed = {x.get("id") for x in doc.get("allowed", [])}
    prohibited = {x.get("id") for x in doc.get("prohibited", [])}
    require(gate, {"structural-community", "structural-crossing", "incomplete-result"} <= allowed, "positive neutrality examples")
    require(gate, {"feature-identity", "business-boundary", "ownership-identity", "service-identity", "stability-overclaim", "custody-upgrade"} <= prohibited, "prohibited neutrality examples")
    require(gate, "PASS" in paths[1].read_text(encoding="utf-8"), "neutrality receipt passes")


def check_a10(gate: str, paths: list[Path]) -> None:
    plan, review = load_json(gate, paths[0]), load_json(gate, paths[1])
    raw_review = paths[2].read_bytes()
    require(gate, plan.get("gate_id") == gate and plan.get("execution_status") == "NOT_EXECUTED" and plan.get("tests_pass_claimed") is False, "plan is explicitly unexecuted")
    guards = plan.get("planned_guard_classes", {})
    require(gate, set(guards) == {"seed_replay", "canonicalization", "instability", "boundary_accounting", "resources", "validation", "compatibility", "cli_mcp_parity"}, "all required guard classes")
    provenance = plan.get("provenance", {})
    require(gate, len(provenance.get("starting_repository_revision", "")) == 40 and provenance.get("requirements_sha256", "").startswith(SHA256_PREFIX) and provenance.get("predecessor_ledger_sha256", "").startswith(SHA256_PREFIX), "plan provenance")
    require(gate, review.get("schema_version") == "lsp-trace.private.program-c.a-10-test-plan-review.v1" and review.get("result") == "APPROVED", "review identity/result")
    require(gate, review.get("read_only") is True and review.get("independent_process") is True, "independent read-only review")
    require(gate, review.get("plan_sha256") == digest(paths[0].read_bytes()) and review.get("raw_review_sha256") == digest(raw_review), "review binds exact plan and raw evidence")
    require(gate, len(review.get("reviewed_commit", "")) == 40 and review.get("reviewer", {}).get("tool") and review.get("reviewer", {}).get("model"), "attributable reviewed commit/model")
    require(gate, set(review.get("coverage_confirmed", [])) == set(guards), "review confirms every guard class")
    require(gate, review.get("execution_status_confirmed") == "NOT_EXECUTED" and review.get("tests_pass_claimed_confirmed") is False, "review preserves non-execution ceiling")


CHECKS: dict[str, Callable[[str, list[Path]], None]] = {
    "A-01": check_a01, "A-02": check_a02, "A-03": check_a03, "A-04": check_a04,
    "A-05": check_a05, "A-06": check_a06, "A-07": check_a07, "A-08": check_a08,
    "A-09": check_a09, "A-10": check_a10,
}


def evaluate(root: Path, outcomes: str) -> tuple[dict[str, int], bool]:
    root = root.resolve(strict=True)
    ledger = safe_evidence(root, outcomes)
    rows: list[tuple[str, str]] = []
    for number, line in enumerate(ledger.read_text(encoding="utf-8").splitlines(), 1):
        if not line or line.startswith("#"):
            continue
        fields = line.split("\t")
        if len(fields) != 4:
            raise ValueError(f"invalid Gate II row at line {number}")
        gate, state, evidence, basis = fields
        if state not in ALLOWED or not evidence or not basis:
            raise ValueError(f"invalid Gate II fields at line {number}")
        relative_paths = evidence.split(";")
        paths = [safe_evidence(root, relative) for relative in relative_paths]
        require(gate, state == EXPECTED_STATES.get(gate), f"expected state {EXPECTED_STATES.get(gate)}")
        require(gate, relative_paths == EXPECTED_EVIDENCE.get(gate), "exact ordered evidence inventory")
        CHECKS[gate](gate, paths)
        rows.append((gate, state))
    if [gate for gate, _ in rows] != EXPECTED:
        raise ValueError("Gate II must contain exactly ordered A-01 through A-10")
    counts = {state: sum(value == state for _, value in rows) for state in sorted(ALLOWED)}
    allowed = counts["FAIL"] == 0 and counts["BLOCKED"] == 0 and counts["PASS"] == 10
    return counts, allowed


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", default=".")
    parser.add_argument("--outcomes", default="qualification/program-c/gate-ii-current-outcomes.v3.tsv")
    args = parser.parse_args()
    try:
        counts, allowed = evaluate(Path(args.root), args.outcomes)
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
