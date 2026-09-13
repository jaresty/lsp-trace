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

class FaultOps:
    def __init__(self, method, *, fail_at=1, fail_count=1, zero_write=False):
        self.method = method
        self.fail_at = fail_at
        self.fail_count = fail_count
        self.zero_write = zero_write
        self.calls = 0

    def __getattr__(self, name):
        original = getattr(os, name)
        if name != self.method:
            return original
        def injected(*args, **kwargs):
            self.calls += 1
            if self.zero_write and self.calls == self.fail_at:
                return 0
            if self.fail_at <= self.calls < self.fail_at + self.fail_count:
                if name == "close":
                    original(*args, **kwargs)
                raise OSError(5, "private injected detail")
            return original(*args, **kwargs)
        return injected

def injected_failure(parent, method, *, fail_at=1, fail_count=1, zero_write=False, basename=None):
    target = parent / (basename or f"{method}.json")
    try:
        PREPARER.safe_publish(target, "report\n", _ops=FaultOps(method, fail_at=fail_at, fail_count=fail_count, zero_write=zero_write))
        error = None
    except PREPARER.PreflightError as exc:
        error = str(exc)
    return target, error

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

    empty_output = invoke(("--output", ""))
    require(empty_output.returncode == 2 and not empty_output.stdout, "ASSERT_OUTPUT_EMPTY_REJECTED")
    require(empty_output.stderr == "preflight error: unsafe output: basename is required\n", "ASSERT_OUTPUT_EMPTY_DETERMINISTIC")

    output = tmp_path / "report.json"; result = invoke(("--output", output))
    require(result.returncode == 0 and output.is_file() and (output.stat().st_mode & 0o777) == 0o600, "ASSERT_PRIVATE_ATOMIC_OUTPUT")
    with tempfile.TemporaryDirectory(prefix="relative-output-", dir=ROOT) as relative_tmp:
        relative_path = Path(relative_tmp) / "résumé-安全.json"
        relative = invoke(("--output", relative_path.relative_to(ROOT)))
        require(relative.returncode == 0 and relative_path.is_file(), "ASSERT_OUTPUT_RELATIVE_UNICODE_BASENAME")
    umask_dir = tmp_path / "umask"; umask_dir.mkdir()
    old_umask = os.umask(0o0777)
    try:
        restrictive = invoke(("--output", umask_dir / "report.json"))
    finally:
        os.umask(old_umask)
    require(restrictive.returncode == 0 and (umask_dir / "report.json").stat().st_mode & 0o777 == 0, "ASSERT_OUTPUT_RESTRICTIVE_UMASK")
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

    for method in ("open", "fstat", "write", "fsync", "link", "close"):
        target, message = injected_failure(tmp_path, method, basename=f"injected-{method}.json")
        require(message is not None and "private injected detail" not in message and str(tmp_path) not in message, "ASSERT_OUTPUT_"+method.upper()+"_CONTROLLED")
        require(not target.exists(), "ASSERT_OUTPUT_"+method.upper()+"_NO_FINAL")
    zero_target, zero_message = injected_failure(tmp_path, "write", zero_write=True, basename="zero-write.json")
    require(zero_message == "unsafe output: write made no progress" and not zero_target.exists(), "ASSERT_OUTPUT_ZERO_WRITE_CONTROLLED")

    committed_target, committed_message = injected_failure(tmp_path, "unlink", fail_at=1, fail_count=2, basename="committed.json")
    require(committed_target.read_text(encoding="utf-8") == "report\n", "ASSERT_OUTPUT_UNLINK_FAILURE_FINAL_PRESERVED")
    require(committed_message == "unsafe output: publication committed; temporary cleanup incomplete", "ASSERT_OUTPUT_UNLINK_FAILURE_SAFE_ORPHAN")
    require(str(tmp_path) not in committed_message and ".representative-preflight-" not in committed_message and "rollback" not in committed_message, "ASSERT_OUTPUT_UNLINK_FAILURE_PRIVATE")

    before_fds = len(os.listdir("/dev/fd"))
    for method in ("fsync", "close"):
        for index in range(16):
            _, repeated_message = injected_failure(tmp_path, method, basename=f"repeat-{method}-{index}.json")
            require(repeated_message is not None, "ASSERT_OUTPUT_REPEATED_FAILURE_CONTROLLED")
    require(len(os.listdir("/dev/fd")) <= before_fds + 1, "ASSERT_OUTPUT_REPEATED_FAILURE_FD_STABLE")

    orphans = list(tmp_path.rglob(".representative-preflight-*"))
    require(len(orphans) == 1 and orphans[0].is_file(), "ASSERT_OUTPUT_TEMP_CLEANUP_OR_SAFE_ORPHAN")
    require(output.is_file() and competitor.read_text(encoding="utf-8") == "competitor\n", "ASSERT_OUTPUT_NO_COMPETITOR_OR_FINAL_DELETION")
    require(all("Traceback" not in r.stderr for r in (empty_output, existing, symlink, parent_symlink, early_symlink, missing, traversal, hidden, root)), "ASSERT_OUTPUT_NO_TRACEBACK")

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
