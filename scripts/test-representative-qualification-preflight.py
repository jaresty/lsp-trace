#!/usr/bin/env python3
import importlib.util
import json
import os
import subprocess
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts/prepare-representative-qualification.py"
MATRIX = ROOT / "qualification/representative-preflight/matrix.v1.json"
OLD_REVISION = "7a6a2698f0ebf584eca7d348d9972534dc6e08f2"
SPEC = importlib.util.spec_from_file_location("representative_qualification", SCRIPT)
PREPARER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PREPARER)

def require(condition, assertion, detail=""):
    if not condition: raise AssertionError(f"{assertion}: {detail}")
    print(f"PASS {assertion}")

def invoke(args=(), extra=None):
    env = {"PATH": os.environ.get("PATH", "")}
    if extra: env.update(extra)
    return subprocess.run(["python3", str(SCRIPT), *map(str, args)], cwd=ROOT, env=env, text=True, capture_output=True)

def run(args=(), extra=None):
    result = invoke(args, extra)
    require(result.returncode == 0, "ASSERT_COMMAND_SUCCESS", result.stderr)
    return result.stdout, json.loads(result.stdout) if result.stdout else None

def by_id(report, app_id): return next(app for app in report["applications"] if app["id"] == app_id)

def invalid_matrix(mutator, expected):
    with tempfile.TemporaryDirectory() as tmp:
        value = json.loads(MATRIX.read_text(encoding="utf-8")); mutator(value)
        path = Path(tmp) / "matrix.json"; path.write_text(json.dumps(value), encoding="utf-8")
        first = invoke(("--matrix", path)); second = invoke(("--matrix", path))
        require(first.returncode == second.returncode == 2, expected+"_EXIT")
        require(first.stderr == second.stderr and first.stderr.startswith("preflight error: invalid matrix:"), expected+"_DIAGNOSTIC")
        require("Traceback" not in first.stderr and not first.stdout, expected+"_CONTROLLED")

raw1, base = run(); raw2, _ = run()
require(raw1 == raw2, "ASSERT_PREFLIGHT_DETERMINISTIC_BYTES")
require(base["installed_state_evidence"] == {"observed": False, "custody": "UNKNOWN"}, "ASSERT_INSTALLED_ABSENT_UNKNOWN")
ops = {op["number"]: op for op in base["operations"]}
require(ops[33]["source_state"] == "INTEGRATED" and ops[33]["installed_state"] == "UNKNOWN", "ASSERT_OPERATION_33_SOURCE_INSTALLED_DISTINCT")
require(ops[34]["source_state"] == ops[35]["source_state"] == "NOT_IMPLEMENTED", "ASSERT_OPERATIONS_34_35_NOT_IMPLEMENTED")
require(by_id(base, "go-repository-fixture")["checks"]["fixture"] == "AVAILABLE", "ASSERT_GOPLS_REPOSITORY_FIXTURE_AVAILABLE")
require(by_id(base, "school-surveys")["checks"]["fixture"] == "OUT_OF_BAND", "ASSERT_PRIVATE_FIXTURE_NOT_ACCESSED")
require(base["source_graph_complete"] == "UNKNOWN", "ASSERT_SOURCE_GRAPH_COMPLETE_UNKNOWN")
require(base["qualification_performed"] is False and base["receipt_created"] is False, "ASSERT_NO_QUALIFICATION_OR_RECEIPT")
require("feature" in base["evidence_semantics"]["claim_ceiling"].lower(), "ASSERT_CLAIM_CEILING")

with tempfile.TemporaryDirectory(prefix="private-qualification-") as tmp:
    tmp_path = Path(tmp).resolve(); secret = tmp_path / "do-not-report-provider"
    secret.write_text("#!/bin/sh\necho 999.999.999\n", encoding="utf-8"); os.chmod(secret, 0o700)
    present_raw, present = run(extra={"CSHARP_LS_BIN": str(secret)})
    school = by_id(present, "school-surveys")
    require(school["checks"]["provider_executable"] == "PRESENT", "ASSERT_PROVIDER_PRESENCE")
    require(school["checks"]["provider_version"] == "VERSION_UNVERIFIED", "ASSERT_PROVIDER_VERSION_UNVERIFIED")
    require(school["provider"]["required_version"] == "0.27.0.0" and school["provider"]["version_requirement_is_evidence"] is False, "ASSERT_REQUIREMENT_NOT_VERSION_EVIDENCE")
    require(str(secret) not in present_raw and tmp not in present_raw, "ASSERT_PRIVATE_PROVIDER_PATH_REDACTED")
    _, asserted = run(extra={"CSHARP_LS_BIN": str(secret), "SCHOOL_SURVEYS_DEPENDENCIES_READY": "1", "SCHOOL_SURVEYS_WORKSPACE_READY": "1", "SCHOOL_SURVEYS_SESSION_READY": "1"})
    school = by_id(asserted, "school-surveys")
    require(school["checks"]["session"] == "SESSION_UNVERIFIED", "ASSERT_ENV_SESSION_NOT_READY")
    require(school["checks"]["session_prerequisite_custody"] == "OPERATOR_ASSERTED", "ASSERT_SESSION_PREREQUISITE_ASSERTED")
    ember = by_id(base, "javascript-ember")["provider"]
    require(ember["package_version"] == "1.0.3" and ember["protocol_identity"] == "ember-glint@1", "ASSERT_EMBER_PACKAGE_PROTOCOL_DISTINCT")

    evidence = tmp_path / "installed.json"
    evidence.write_text(json.dumps({"schema_version":"lsp-trace.installed-state-evidence.v1","custody":"OPERATOR_ASSERTED","binary":{"revision":OLD_REVISION},"operations":[{"number":33,"state":"UNAVAILABLE_OLD_BUILD"},{"number":34,"state":"UNKNOWN"},{"number":35,"state":"UNKNOWN"}]}), encoding="utf-8")
    _, installed = run(("--installed-state", evidence))
    require(installed["installed_state_evidence"]["binary"]["revision"] == OLD_REVISION, "ASSERT_INSTALLED_REVISION_ATTRIBUTABLE")
    require(next(op for op in installed["operations"] if op["number"] == 33)["installed_state"] == "UNAVAILABLE_OLD_BUILD", "ASSERT_INSTALLED_OLD_BUILD")
    bad = json.loads(evidence.read_text()); bad["binary"]["revision"] = "7a6a"; evidence.write_text(json.dumps(bad))
    mismatch = invoke(("--installed-state", evidence))
    require(mismatch.returncode == 2 and "binary.revision" in mismatch.stderr, "ASSERT_INSTALLED_REVISION_UNPARSEABLE")

    output = tmp_path / "report.json"; result = invoke(("--output", output))
    require(result.returncode == 0 and output.is_file() and (output.stat().st_mode & 0o777) == 0o600, "ASSERT_PRIVATE_ATOMIC_OUTPUT")
    existing = invoke(("--output", output)); require(existing.returncode == 2 and "target already exists" in existing.stderr, "ASSERT_OUTPUT_EXISTING_REJECTED")
    link = tmp_path / "link.json"; link.symlink_to(output)
    symlink = invoke(("--output", link)); require(symlink.returncode == 2 and "target already exists" in symlink.stderr and link.is_symlink(), "ASSERT_OUTPUT_SYMLINK_REJECTED")
    parent_link = tmp_path / "parent-link"; parent_link.symlink_to(tmp_path, target_is_directory=True)
    parent_symlink = invoke(("--output", parent_link/"report.json")); require(parent_symlink.returncode == 2 and "parent component" in parent_symlink.stderr, "ASSERT_OUTPUT_SYMLINK_PARENT_REJECTED")
    real = tmp_path / "real"; (real / "child").mkdir(parents=True)
    early_link = tmp_path / "early-link"; early_link.symlink_to(real, target_is_directory=True)
    early_symlink = invoke(("--output", early_link/"child"/"report.json")); require(early_symlink.returncode == 2 and "parent component" in early_symlink.stderr, "ASSERT_OUTPUT_EARLY_ANCESTOR_SYMLINK_REJECTED")
    missing = invoke(("--output", tmp_path/"missing"/"report.json")); require(missing.returncode == 2 and "parent component" in missing.stderr, "ASSERT_OUTPUT_MISSING_PARENT_REJECTED")
    traversal = invoke(("--output", tmp_path/".."/"escape.json")); require(traversal.returncode == 2 and "traversal" in traversal.stderr, "ASSERT_OUTPUT_TRAVERSAL_REJECTED")
    hidden = invoke(("--output", tmp_path/".secret")); require(hidden.returncode == 2 and "basename" in hidden.stderr, "ASSERT_OUTPUT_UNSAFE_BASENAME_REJECTED")
    root = invoke(("--output", Path("/"))); require(root.returncode == 2 and "basename" in root.stderr, "ASSERT_OUTPUT_ROOT_REJECTED")

    race_parent = tmp_path / "race-parent"; (race_parent / "child").mkdir(parents=True)
    moved_parent = tmp_path / "race-parent-moved"
    def swap_ancestor(stage):
        if stage == "parents_pinned":
            race_parent.rename(moved_parent); (race_parent / "child").mkdir(parents=True)
    try:
        PREPARER.safe_publish(race_parent / "child" / "report.json", "{}\n", _test_hook=swap_ancestor)
        race_rejected = False
    except PREPARER.PreflightError as exc:
        race_rejected = "parent identity changed" in str(exc)
    require(race_rejected and not (race_parent / "child" / "report.json").exists() and not (moved_parent / "child" / "report.json").exists(), "ASSERT_OUTPUT_ANCESTOR_SWAP_REJECTED")

    competitor = tmp_path / "competitor.json"
    def create_competitor(stage):
        if stage == "before_publish": competitor.write_text("competitor\n", encoding="utf-8")
    try:
        PREPARER.safe_publish(competitor, "report\n", _test_hook=create_competitor)
        competitor_rejected = False
    except PREPARER.PreflightError as exc:
        competitor_rejected = "target already exists" in str(exc)
    require(competitor_rejected and competitor.read_text(encoding="utf-8") == "competitor\n", "ASSERT_OUTPUT_COMPETITOR_RACE_PRESERVED")
    require(not list(tmp_path.rglob(".representative-preflight-*")), "ASSERT_OUTPUT_TEMP_CLEANUP")
    require(all("Traceback" not in r.stderr for r in (existing, symlink, parent_symlink, early_symlink, missing, traversal, hidden, root)), "ASSERT_OUTPUT_NO_TRACEBACK")

invalid_matrix(lambda m: m.update({"unknown": 1}), "ASSERT_MATRIX_UNKNOWN")
invalid_matrix(lambda m: m["operations"].append({"number":999}), "ASSERT_MATRIX_OP999")
invalid_matrix(lambda m: m["applications"].append(dict(m["applications"][0])), "ASSERT_MATRIX_DUPLICATE_APP")
invalid_matrix(lambda m: m["applications"][1].update({"provider": dict(m["applications"][0]["provider"])}), "ASSERT_MATRIX_DUPLICATE_PROVIDER")
invalid_matrix(lambda m: m["operations"][0].update({"source_state":"FUTURE"}), "ASSERT_MATRIX_OUTCOME_ENUM")
invalid_matrix(lambda m: m["applications"][0]["provider"].update({"required_version":"v0.23"}), "ASSERT_MATRIX_VERSION_FORMAT")
invalid_matrix(lambda m: m["operations"][0].update({"extra":True}), "ASSERT_MATRIX_NESTED_UNKNOWN")

corpus = " ".join((raw1, json.dumps(base)))
for forbidden in (str(Path.home()), "do-not-report-provider", "PRIVATE_KEY", "password", "token="):
    require(forbidden not in corpus, "ASSERT_PRIVACY_CORPUS_"+str(abs(hash(forbidden))))
