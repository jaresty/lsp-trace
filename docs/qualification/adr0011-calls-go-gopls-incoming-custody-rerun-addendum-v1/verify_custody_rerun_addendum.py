#!/usr/bin/env python3
from __future__ import annotations

import copy
import hashlib
import importlib.util
import json
import os
import pathlib
import shutil
import subprocess
import sys
import contextlib
import io
import tempfile
from typing import Any, Dict, Iterable, List

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]
EXPECTED_HEAD = "c3d77247f5a2e06c0d6ac5b1b55a3df7dbea588a"
PROFILE = "CALLS_GO_GOPLS_INCOMING_EXACT_V1"
GENERIC_PROFILE = "CALLS_LSP_INCOMING_EXACT_V1"
DISPOSITION = "INCOMING_CUSTODY_RERUN_ADDENDUM_AUTHORED_NOT_GO"
RUN_NAMESPACE = "lsp-trace.adr0011.calls.go.gopls.incoming.custody_rerun.v1"
ENV_RUN = "TEST_ONLY_ENV_PROBE_V5"
DRY_RUN = "TEST_ONLY_DRY_FULL_PLAN_V2"
SEAL_RUN = "TEST_ONLY_SEAL_FIXTURE"
EXPECTED = {
    "target_count": 5900,
    "batch_size": 50,
    "batch_count": 118,
    "max_raw_response_bytes": 33554432,
    "source_revision": "4b910fc0c517cb7801fb51d17d295897230d11fa",
    "source_tree": "761474bf5d90bbe3468df62b1a68a1fa26dc2c82",
    "target_tsv_sha256": "8aa361546ab5245ae0ac2f945e3d19b36a48b745bfe7187e5baef6d6d9a14bd4",
}
REQUIRED_ROLES = {
    "gopls_canonical_path_length_digest_version",
    "exact_argv_cwd_complete_env_passed_to_popen",
    "prelaunch_complete_env_private_seal",
    "allowlisted_public_summary_no_env_leakage",
    "atomic_fresh_run_namespace_reservation",
    "initialize_bytes_root_workspace_capability",
    "go_executable_from_exact_captured_path_length_digest_version",
    "closed_go_toolchain_inventory_from_goroot_gotooldir",
    "runner_python_runtime_and_source_hashes",
    "post_full_run_seal_and_verify",
    "historical_failure_refusal_frontier_preservation",
    "authority_non_merge_boundary",
}
REQUIRED_PACKAGE_FILES = {
    "README.md",
    "ADDENDUM.md",
    "EXECUTION_CONTRACT.json",
    "ROLE_MATRIX.json",
    "PRESUMPTION_LEDGER.md",
    "REVIEW_REQUEST.md",
    "RESULT.json",
    "MANIFEST.json",
    "capture_incoming_custody_rerun.py",
    "verify_custody_rerun_addendum.py",
}


def load_capture_module():
    spec = importlib.util.spec_from_file_location("custody_capture_wrapper_for_verify", HERE / "capture_incoming_custody_rerun.py")
    if spec is None or spec.loader is None:
        raise RuntimeError("cannot import capture wrapper")
    mod = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = mod
    spec.loader.exec_module(mod)
    return mod

CAP = load_capture_module()


def sha(path: pathlib.Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def digest_jsonable(obj: Any) -> str:
    return hashlib.sha256(json.dumps(obj, sort_keys=True, separators=(",", ":")).encode("utf-8")).hexdigest()


def j(name: str) -> Any:
    return json.loads((HERE / name).read_text(encoding="utf-8"))


def req(label: str, ok: bool) -> None:
    print(("PASS " if ok else "FAIL ") + label)
    if not ok:
        raise AssertionError(label)


def rel(path: pathlib.Path) -> str:
    return str(path.relative_to(HERE))


def iter_package_files() -> Iterable[pathlib.Path]:
    for path in sorted(HERE.rglob("*")):
        if path.is_file() and "__pycache__" not in path.parts:
            yield path



def expect_raises(label: str, fn) -> None:
    try:
        fn()
    except Exception:
        print("PASS " + label)
        return
    raise AssertionError(label)

def public_scan_string(value: str, *, context: str) -> None:
    package_abs = str(HERE)
    private_prefixes = ["/private/", "/Users/", "/tmp/", package_abs, str(ROOT)]
    if value.startswith("/") or any(prefix in value for prefix in private_prefixes):
        raise AssertionError(f"public artifact leaks absolute/private path at {context}: {value[:120]!r}")
    if value.startswith("file://"):
        raise AssertionError(f"public artifact leaks file URI at {context}: {value[:120]!r}")
    # Reject argv-style path elements embedded in public fields. URI-ish schema names are allowed above.
    for token in value.replace('\n', ' ').split():
        if (token.startswith("/") and token != "/") or token.startswith("file://"):
            raise AssertionError(f"public artifact leaks path token at {context}: {token[:120]!r}")


def scan_public_obj(obj: Any, *, context: str) -> None:
    forbidden_key_names = {
        "workspace", "runner", "original_runner", "gopls_path", "gopls_launch_argv",
        "complete_env_private_record", "record", "binding_private_record",
        "complete_env", "go_env_json", "env_keys_sorted", "canonical_path", "argv", "cwd",
        "rootUri", "workspaceFolders", "PATH", "GOPATH",
    }
    if isinstance(obj, dict):
        for key, value in obj.items():
            if key in forbidden_key_names:
                raise AssertionError(f"public artifact uses forbidden key {key!r} at {context}")
            public_scan_string(str(key), context=f"{context}.{key}<key>")
            scan_public_obj(value, context=f"{context}.{key}")
    elif isinstance(obj, list):
        for i, value in enumerate(obj):
            scan_public_obj(value, context=f"{context}[{i}]")
    elif isinstance(obj, str):
        public_scan_string(obj, context=context)


def scan_public_text(text: str, *, context: str) -> None:
    for line_no, line in enumerate(text.splitlines(), 1):
        public_scan_string(line, context=f"{context}:{line_no}")


def is_nominal_public_artifact(path: pathlib.Path) -> bool:
    if "__pycache__" in path.parts or not path.is_file():
        return False
    name = path.name
    if name.endswith(".private.json"):
        return False
    if path.suffix in {".json", ".jsonl", ".md", ".log", ".sha256"}:
        return True
    if name.endswith(".seal.json") or name.endswith("NOTICE.json") or "provenance" in path.parts:
        return True
    return False


def scan_all_public_artifacts() -> None:
    for path in sorted(HERE.rglob("*")):
        if not is_nominal_public_artifact(path):
            continue
        rel_path = rel(path)
        if path.suffix == ".json":
            scan_public_obj(json.loads(path.read_text(encoding="utf-8")), context=rel_path)
        else:
            scan_public_text(path.read_text(encoding="utf-8", errors="replace"), context=rel_path)


def check_public_scanner_mutations() -> None:
    listed_mutations = [
        ("seal.record", {"schema": "x", "record": "/private/tmp/leak"}),
        ("notice.binding_private_record", {"schema": "x", "binding_private_record": "/private/tmp/leak"}),
        ("provenance.workspace", {"schema": "x", "workspace": "/private/tmp/workspace"}),
        ("provenance.runner", {"schema": "x", "runner": "/private/tmp/runner.py"}),
        ("provenance.original_runner", {"schema": "x", "original_runner": "/private/tmp/original.py"}),
        ("provenance.gopls_path", {"schema": "x", "gopls_path": "/Users/user/bin/gopls"}),
        ("provenance.gopls_launch_argv", {"schema": "x", "gopls_launch_argv": ["/Users/user/bin/gopls", "serve"]}),
        ("provenance.complete_env_private_record", {"schema": "x", "complete_env_private_record": "/private/tmp/private.json"}),
        ("generic.absolute_value", {"schema": "x", "safe": "/private/tmp/leak"}),
    ]
    for label, obj in listed_mutations:
        expect_raises("PUBLIC_SCANNER_MUTATION_" + label, lambda obj=obj: scan_public_obj(obj, context=label))


def check_manifest(m: Dict[str, Any] | None = None) -> None:
    m = m or j("MANIFEST.json")
    req("SCHEMA", m.get("schema") == "lsp-trace.adr0011.calls.go.gopls.incoming.custody-rerun-addendum.manifest.v5")
    req("PROFILE_ID", m.get("profile_id") == PROFILE and m.get("binds_to_generic_profile_id") == GENERIC_PROFILE)
    req("DISPOSITION_NOT_GO", m.get("producer_disposition") == DISPOSITION and m.get("producer_may_issue_go") is False)
    req("RUN_NAMESPACE_FRESH", m.get("fresh_run_namespace") == RUN_NAMESPACE and m.get("supplements_historical_evidence") is True)
    req("NO_HISTORICAL_OVERWRITE", m.get("historical_evidence_mutation") == "FORBIDDEN")
    req("HEAD_PIN", m.get("worktree_head") == EXPECTED_HEAD)
    inv = m.get("frozen_incoming_workload", {})
    for key, expected in EXPECTED.items():
        req("INVARIANT_" + key, inv.get(key) == expected)
    req("INCOMING_ONLY", m.get("direction") == "incoming" and m.get("outgoing_run_authority") is False and m.get("frontier_inference") == "FORBIDDEN")
    req("FRESH_RETRY_FREE", inv.get("retry_policy") == "RETRY_FREE_FRESH_RUN_ONLY" and m.get("fresh_run_dir_policy") == "MUST_NOT_EXIST_AT_ALL_AT_RESERVATION")
    req("RAW_CEILING_UNCHANGED", m.get("forbidden_effects", {}).get("runtime_memory_ceiling_change") is False)
    for k in ["runtime_code_change", "schema_publication", "public_surface_change", "composition_change", "custody_policy_change", "production_admission", "authority_merge", "outgoing_execution"]:
        req("FORBIDDEN_EFFECT_" + k, m.get("forbidden_effects", {}).get(k) is False)
    roles = {r.get("role"): r for r in m.get("custody_roles", [])}
    req("CUSTODY_ROLE_SET", REQUIRED_ROLES.issubset(set(roles)))
    for role in REQUIRED_ROLES:
        req("ROLE_FAILS_CLOSED_" + role, roles[role].get("fail_closed_if_missing") is True and roles[role].get("missing_disposition") == "NO_GO_FAIL_CLOSED")
    env_role = roles["exact_argv_cwd_complete_env_passed_to_popen"]
    req("PRIVATE_ENV_VALUES_RETAINED", env_role.get("sensitive_handling") == "PRIVATE_VALUES_RETAINED_NOT_PUBLIC")
    req("ENV_SOURCE_CAPTURED_ENV", env_role.get("source") == "env=captured_env")
    summary_role = roles["allowlisted_public_summary_no_env_leakage"]
    req("SUMMARY_ALLOWLIST_ROLE", summary_role.get("construction") == "ALLOWLIST_ONLY" and summary_role.get("raw_environment_values_public") is False)
    seal_role = roles["post_full_run_seal_and_verify"]
    req("POST_FULL_SEAL_ROLE", seal_role.get("full_run_executed_now") is False and seal_role.get("synthetic_tamper_tests") is True)
    req("FULL_COMMAND_INCOMING_ONLY", "--run-full" in m.get("execution", {}).get("full_rerun_command_after_review", "") and "--direction incoming" in m.get("execution", {}).get("full_rerun_command_after_review", ""))
    req("FULL_COMMAND_NOT_RUN", m.get("full_campaign_executed_by_this_package") is False)
    req("TEST_ONLY_CANNOT_MASQUERADE", m.get("test_only_modes", {}).get("may_masquerade_as_denominator_evidence") is False)
    req("RESIDUAL_UNKNOWN_FRONTIER", "UNKNOWN" in json.dumps(m.get("preserved_unknowns", [])))


def check_execution_contract(c: Dict[str, Any] | None = None) -> None:
    c = c or j("EXECUTION_CONTRACT.json")
    req("EXEC_CONTRACT_SCHEMA", c.get("schema") == "lsp-trace.adr0011.calls.go.gopls.incoming.custody-rerun.execution-contract.v2")
    req("EXEC_INCOMING_ONLY", c.get("direction") == "incoming" and c.get("forbid_outgoing") is True)
    req("EXEC_EXACT5900", c.get("target_count") == 5900 and c.get("batch_size") == 50 and c.get("batch_count") == 118)
    req("EXEC_RETRY_FREE", c.get("retry_policy") == "RETRY_FREE_FRESH_RUN_ONLY" and c.get("fresh_run_dir_policy") == "MUST_NOT_EXIST_AT_ALL_AT_RESERVATION")
    req("EXEC_RAW32", c.get("max_raw_response_bytes") == 33554432)
    req("EXEC_ENV_BINDING", c.get("popen_env_source") == "captured_env" and c.get("require_explicit_popen_env") is True)
    req("EXEC_PUBLIC_SUMMARY_ALLOWLIST", c.get("public_summary_policy", {}).get("construction") == "ALLOWLIST_ONLY" and c.get("public_summary_policy", {}).get("raw_environment_values_public") is False)
    req("EXEC_FINALIZE_VERIFY", c.get("post_full_run_seal", {}).get("finalize_command_required") is True and c.get("post_full_run_seal", {}).get("verify_full_run_mode") is True)
    req("EXEC_NO_GO", c.get("producer_may_issue_go") is False and c.get("production_admission") == "REFUSED_NOT_REQUESTED_NOT_GRANTED")


def check_sources_and_checksums() -> None:
    m = j("MANIFEST.json")
    for name, rec in m.get("source_pins", {}).items():
        path = ROOT / rec["path"]
        req("SOURCE_PIN_EXISTS_" + name, path.exists())
        req("SOURCE_PIN_SHA_" + name, sha(path) == rec["sha256"])
    checksums = {}
    for line in (HERE / "CHECKSUMS.sha256").read_text(encoding="utf-8").splitlines():
        if not line.strip():
            continue
        digest, file_rel = line.split("  ", 1)
        checksums[file_rel] = digest
    for required in REQUIRED_PACKAGE_FILES:
        req("CHECKSUM_LISTS_" + required, required in checksums)
    for path in iter_package_files():
        r = rel(path)
        if r in {"CHECKSUMS.sha256", "VERIFICATION.log", "TAMPER_TESTS.log"}:
            continue
        req("CHECKSUM_HAS_" + r, r in checksums)
        req("CHECKSUM_SHA_" + r, sha(path) == checksums[r])


def check_wrapper_source() -> None:
    text = (HERE / "capture_incoming_custody_rerun.py").read_text(encoding="utf-8")
    req("WRAPPER_POPEN_ENV_EXPLICIT", "env=captured_env" in text)
    req("WRAPPER_PUBLIC_SUMMARY_ALLOWLIST", "build_public_summary" in text and "PUBLIC_SUMMARY_FORBIDDEN_KEYS" in text and "assert_public_summary_allowlisted" in text)
    req("WRAPPER_COMPLETE_ENV_PRIVATE", "complete_env" in text and "private_mode=True" in text and "complete_env_sha256" in text)
    req("WRAPPER_CAPTURE_BEFORE_RUN", text.find("capture_prelaunch") < text.find("base.run_full_workload"))
    req("WRAPPER_POPEN_BINDING_BEFORE_POPEN", text.find("write_popen_env_binding") < text.find("subprocess.Popen(argv"))
    req("WRAPPER_INCOMING_ONLY", "choices=[\"incoming\"]" in text and "custody rerun addendum is incoming-only" in text)
    req("WRAPPER_NO_UNCONTROLLED_GOROOT_TREE", "does not claim an uncontrolled recursive GOROOT tree hash" in text)
    req("WRAPPER_FRESH_RESERVATION", "mkdir(mode=0o700, exist_ok=False)" in text and "refusing same-run-id resume" in text)
    req("WRAPPER_FINALIZE_VERIFY", "def finalize_run" in text and "def verify_sealed_run" in text and "changed, missing, or extra artifact" in text)
    req("WRAPPER_NOFOLLOW_RESOLUTION", "_open_relative_no_follow_fd" in text and "O_NOFOLLOW" in text and "invalid path component" in text)
    req("WRAPPER_NONBLOCK_FINAL_OPEN", "O_NONBLOCK" in text and "final=True" in text and "Prevent FIFO/special-file open from blocking" in text)


def check_public_summary() -> None:
    summary_path = HERE / f"runs/{ENV_RUN}/full-incoming/custody/prelaunch-custody.summary.json"
    private_path = HERE / f"runs/{ENV_RUN}/full-incoming/custody/prelaunch-custody.private.json"
    req("PUBLIC_SUMMARY_EXISTS", summary_path.exists())
    req("PRIVATE_CUSTODY_EXISTS", private_path.exists())
    summary = json.loads(summary_path.read_text(encoding="utf-8"))
    private = json.loads(private_path.read_text(encoding="utf-8"))
    CAP.assert_public_summary_allowlisted(summary)
    req("PUBLIC_SUMMARY_SCHEMA", summary.get("schema") == "lsp-trace.adr0011.calls.go.gopls.incoming.custody-rerun-public-summary.v2")
    text = json.dumps(summary, sort_keys=True)
    for forbidden in ["go_env_json", "complete_env", "canonical_path", "env_keys_sorted", "workspaceFolders", "rootUri", "GOPATH", "PATH"]:
        req("PUBLIC_SUMMARY_OMITS_" + forbidden, forbidden not in text)
    req("PRIVATE_RETAINS_COMPLETE_ENV", isinstance(private.get("launch", {}).get("complete_env"), dict) and len(private["launch"]["complete_env"]) > 0)
    req("PUBLIC_SUMMARY_DIGEST_BINDS_PRIVATE", summary["artifact_digests"]["private_prelaunch_record_sha256"] == sha(private_path))
    tampered_key = copy.deepcopy(summary)
    tampered_key["toolchain_summary"]["go_env_json"] = {"SECRET": "VALUE"}
    expect_raises("PUBLIC_SUMMARY_TAMPER_FORBIDDEN_KEY", lambda: CAP.assert_public_summary_allowlisted(tampered_key))
    tampered_value = copy.deepcopy(summary)
    tampered_value["toolchain_summary"]["innocent"] = "/private/tmp/secret"
    expect_raises("PUBLIC_SUMMARY_TAMPER_FORBIDDEN_VALUE", lambda: CAP.assert_public_summary_allowlisted(tampered_value))


def check_private_custody_artifacts() -> None:
    record = HERE / f"runs/{ENV_RUN}/full-incoming/custody/prelaunch-custody.private.json"
    seal_path = HERE / f"runs/{ENV_RUN}/full-incoming/custody/prelaunch-custody.seal.json"
    binding_path = HERE / f"runs/{ENV_RUN}/full-incoming/custody/popen-env-binding.private.json"
    notice_path = HERE / f"runs/{ENV_RUN}/full-incoming/TEST_ONLY_NOTICE.json"
    data = json.loads(record.read_text(encoding="utf-8"))
    seal = json.loads(seal_path.read_text(encoding="utf-8"))
    binding = json.loads(binding_path.read_text(encoding="utf-8"))
    notice = json.loads(notice_path.read_text(encoding="utf-8"))
    req("SEAL_RECORD_RELATIVE", seal.get("record_relative") == "custody/prelaunch-custody.private.json" and "record" not in seal)
    resolved_record = CAP.resolve_run_relative(record.parents[1], seal["record_relative"])
    req("SEAL_RECORD_RESOLVES", resolved_record == record)
    req("NOTICE_BINDING_PRIVATE_RELATIVE", notice.get("binding_private_record_relative") == "custody/popen-env-binding.private.json" and "binding_private_record" not in notice)
    resolved_binding = CAP.resolve_run_relative(record.parents[1], notice["binding_private_record_relative"])
    req("NOTICE_BINDING_PRIVATE_RESOLVES", resolved_binding == binding_path)
    req("SEAL_RECORD_SHA", seal["record_sha256"] == sha(record))
    req("SEAL_RECORD_LENGTH", seal["record_byte_length"] == record.stat().st_size)
    complete_env = data["launch"]["complete_env"]
    req("PRIVATE_ENV_SORTED", list(complete_env.keys()) == sorted(complete_env.keys()))
    req("PRIVATE_ENV_DIGEST", digest_jsonable(complete_env) == data["launch"]["complete_env_sha256"] == seal["complete_env_sha256"])
    req("POPEN_BINDING_SOURCE", binding.get("source") == "env=captured_env")
    req("POPEN_ENV_EQUALS_PRELAUNCH", binding["env_passed_to_popen"] == complete_env)
    req("POPEN_ENV_DIGEST_EQUALS_PRELAUNCH", binding["env_sha256_passed_to_popen"] == data["launch"]["complete_env_sha256"])
    req("CHILD_ENV_EQUALS_PASSED", binding["child_observed_env"] == binding["env_passed_to_popen"])
    inv = data["go_toolchain_inventory"]
    req("TOOLCHAIN_COMPLETE", inv.get("complete") is True and inv.get("required_record_count") == len(inv.get("required_records", [])) and len(inv.get("required_records", [])) > 0)


def check_fresh_reservation() -> None:
    runs = HERE / "runs"
    with tempfile.TemporaryDirectory(dir=runs) as td:
        existing = pathlib.Path(td) / "full-incoming"
        existing.mkdir()
        expect_raises("FRESH_RESERVATION_EXISTING_RUN_DIR_REJECTED", lambda: CAP.reserve_fresh_run_dir(existing))
    with tempfile.TemporaryDirectory(dir=runs) as td:
        parent = pathlib.Path(td) / "RUNID"
        parent.mkdir()
        candidate = parent / "full-incoming"
        expect_raises("FRESH_RESERVATION_EXISTING_RUN_ID_REJECTED", lambda: CAP.reserve_fresh_run_dir(candidate))
    with tempfile.TemporaryDirectory(dir=runs) as td:
        target = pathlib.Path(td) / "target"
        target.mkdir()
        link = pathlib.Path(td) / "link"
        try:
            link.symlink_to(target, target_is_directory=True)
            expect_raises("FRESH_RESERVATION_SYMLINK_REJECTED", lambda: CAP.reserve_fresh_run_dir(link / "full-incoming"))
        except (OSError, NotImplementedError):
            print("PASS FRESH_RESERVATION_SYMLINK_REJECTED_SKIPPED_UNSUPPORTED")


def check_test_only_dry_plan() -> None:
    plan_path = HERE / f"runs/{DRY_RUN}/full-incoming/full-plan.json"
    plan = json.loads(plan_path.read_text(encoding="utf-8"))
    run_dir = plan_path.parent
    req("DRY_PLAN_MODE", plan.get("mode") == "DRY_FULL_PLAN")
    req("DRY_PLAN_COUNTS", plan.get("target_count") == 5900 and plan.get("batch_size") == 50 and plan.get("batch_count") == 118)
    req("DRY_PLAN_INCOMING", plan.get("direction") == "incoming")
    req("DRY_PLAN_NO_LEDGER_RAW", not (run_dir / "ledgers").exists() and not (run_dir / "raw").exists())
    notice = run_dir / "TEST_ONLY_NOTICE.json"
    n = json.loads(notice.read_text(encoding="utf-8"))
    req("DRY_PLAN_NOTICE_NON_DENOMINATOR", n.get("may_masquerade_as_full_campaign_evidence") is False and n.get("full_campaign_executed") is False)
    req("DRY_PLAN_BATCH_COUNT", len(sorted((run_dir / "batches").glob("batch-*.manifest.json"))) == 118)


def copy_synthetic_fixture(tmp_root: pathlib.Path) -> pathlib.Path:
    src_parent = HERE / f"runs/{SEAL_RUN}"
    dst_parent = tmp_root / SEAL_RUN
    shutil.copytree(src_parent, dst_parent, symlinks=False)
    return dst_parent / "full-incoming"



def check_no_follow_run_relative_adversaries() -> None:
    runs = HERE / "runs"
    with tempfile.TemporaryDirectory(dir=runs) as td:
        run = pathlib.Path(td) / "NOFOLLOW_ADVERSARY" / "full-incoming"
        run.mkdir(parents=True)
        (run / "target.txt").write_text("target must not be read through symlink\n", encoding="utf-8")
        (run / "dir").mkdir()
        (run / "dir" / "file.txt").write_text("dir file\n", encoding="utf-8")
        (run / "real-intermediate").mkdir()
        (run / "real-intermediate" / "file.txt").write_text("intermediate target\n", encoding="utf-8")
        expect_raises("NOFOLLOW_ABSOLUTE_REF_REJECTED", lambda: CAP.resolve_run_relative(run, "/target.txt"))
        expect_raises("NOFOLLOW_DOTDOT_REF_REJECTED", lambda: CAP.resolve_run_relative(run, "../target.txt"))
        expect_raises("NOFOLLOW_DOT_REF_REJECTED", lambda: CAP.resolve_run_relative(run, "./target.txt"))
        expect_raises("NOFOLLOW_EMPTY_REF_REJECTED", lambda: CAP.resolve_run_relative(run, ""))
        expect_raises("NOFOLLOW_AMBIGUOUS_COMPONENT_REJECTED", lambda: CAP.resolve_run_relative(run, "dir//file.txt"))
        try:
            (run / "link-in-root").symlink_to(run / "target.txt")
            expect_raises("NOFOLLOW_IN_ROOT_SYMLINK_TO_IN_ROOT_TARGET_REJECTED", lambda: CAP.resolve_run_relative(run, "link-in-root"))
        except (OSError, NotImplementedError):
            print("PASS NOFOLLOW_IN_ROOT_SYMLINK_TO_IN_ROOT_TARGET_REJECTED_SKIPPED_UNSUPPORTED")
        try:
            outside = pathlib.Path(td) / "outside.txt"
            outside.write_text("outside must not be read\n", encoding="utf-8")
            (run / "link-outside").symlink_to(outside)
            expect_raises("NOFOLLOW_SYMLINK_TO_OUTSIDE_REJECTED", lambda: CAP.resolve_run_relative(run, "link-outside"))
        except (OSError, NotImplementedError):
            print("PASS NOFOLLOW_SYMLINK_TO_OUTSIDE_REJECTED_SKIPPED_UNSUPPORTED")
        try:
            (run / "intermediate-link").symlink_to(run / "real-intermediate", target_is_directory=True)
            expect_raises("NOFOLLOW_INTERMEDIATE_DIRECTORY_SYMLINK_REJECTED", lambda: CAP.resolve_run_relative(run, "intermediate-link/file.txt"))
        except (OSError, NotImplementedError):
            print("PASS NOFOLLOW_INTERMEDIATE_DIRECTORY_SYMLINK_REJECTED_SKIPPED_UNSUPPORTED")
        try:
            (run / "dir" / "final-link").symlink_to(run / "target.txt")
            expect_raises("NOFOLLOW_FINAL_SYMLINK_REJECTED", lambda: CAP.resolve_run_relative(run, "dir/final-link"))
        except (OSError, NotImplementedError):
            print("PASS NOFOLLOW_FINAL_SYMLINK_REJECTED_SKIPPED_UNSUPPORTED")


def check_bounded_nonregular_rejection() -> None:
    runs = HERE / "runs"
    common = f"""
import importlib.util, json, os, pathlib, shutil, socket, stat, sys, tempfile
HERE = pathlib.Path({str(HERE)!r})
spec = importlib.util.spec_from_file_location('capture_under_nonregular_test', HERE / 'capture_incoming_custody_rerun.py')
mod = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = mod
spec.loader.exec_module(mod)
root = pathlib.Path(tempfile.mkdtemp(dir=HERE / 'runs'))
run = root / 'NONREGULAR' / 'full-incoming'
run.mkdir(parents=True)

def assert_rejects(rel):
    attempted = []
    for name, fn in [
        ('resolve', lambda: mod.resolve_run_relative(run, rel)),
        ('read', lambda: mod.read_run_relative_text(run, rel)),
        ('sha', lambda: mod.sha256_run_relative(run, rel)),
        ('byte_len', lambda: mod.byte_len_run_relative(run, rel)),
    ]:
        try:
            fn()
        except Exception:
            attempted.append(name)
        else:
            raise SystemExit('accepted nonregular via ' + name + ': ' + rel)
    if attempted != ['resolve', 'read', 'sha', 'byte_len']:
        raise SystemExit('unexpected attempts ' + repr(attempted))
try:
{{SETUP}}
finally:
    shutil.rmtree(root, ignore_errors=True)
"""
    cases = {
        "FIFO": """
if not hasattr(os, 'mkfifo'):
    print('SKIP FIFO mkfifo unavailable')
else:
    os.mkfifo(run / 'fifo')
    assert_rejects('fifo')
    print('PASS BOUNDED_NONREGULAR_FIFO_REJECTED')
""",
        "UNIX_SOCKET": """
sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
old_cwd = os.getcwd()
os.chdir(run)
try:
    sock.bind('sock')
finally:
    os.chdir(old_cwd)
    sock.close()
assert_rejects('sock')
print('PASS BOUNDED_NONREGULAR_UNIX_SOCKET_REJECTED')
""",
        "DIRECTORY": """
(run / 'directory').mkdir()
assert_rejects('directory')
print('PASS BOUNDED_NONREGULAR_DIRECTORY_REJECTED')
""",
        "DEVICE_OR_SUBSTITUTE": """
created = False
if hasattr(os, 'mknod') and hasattr(os, 'makedev'):
    for mode, dev in [(stat.S_IFCHR | 0o600, os.makedev(0, 0)), (stat.S_IFCHR | 0o600, os.makedev(3, 2))]:
        try:
            os.mknod(run / 'device_node', mode, dev)
            created = True
            break
        except Exception:
            pass
if created:
    assert_rejects('device_node')
    print('PASS BOUNDED_NONREGULAR_DEVICE_NODE_REJECTED')
else:
    if not hasattr(os, 'mkfifo'):
        (run / 'nonregular_substitute_dir').mkdir()
        assert_rejects('nonregular_substitute_dir')
    else:
        os.mkfifo(run / 'nonregular_substitute_fifo')
        assert_rejects('nonregular_substitute_fifo')
    print('PASS BOUNDED_NONREGULAR_DEVICE_SUBSTITUTE_REJECTED')
""",
    }
    for label, setup in cases.items():
        indented_setup = "\n".join(("    " + line) if line else line for line in setup.splitlines())
        code = common.replace("{SETUP}", indented_setup)
        cp = subprocess.run([sys.executable, "-c", code], text=True, capture_output=True, timeout=3)
        if cp.returncode != 0:
            raise AssertionError(f"bounded nonregular case {label} failed: stdout={cp.stdout!r} stderr={cp.stderr!r}")
        for line in cp.stdout.splitlines():
            if line.startswith("PASS ") or line.startswith("SKIP "):
                print(line)

def check_synthetic_seal_and_tampers() -> None:
    run_dir = HERE / f"runs/{SEAL_RUN}/full-incoming"
    result = CAP.verify_sealed_run(run_dir, test_only_synthetic=True)
    req("SYNTHETIC_SEAL_VERIFIES", result["status"] == "FINAL_RUN_SEAL_VERIFIED_TEST_ONLY")
    runs = HERE / "runs"
    with tempfile.TemporaryDirectory(dir=runs) as td:
        tmp_root = pathlib.Path(td)
        changed = copy_synthetic_fixture(tmp_root / "changed")
        (changed / "FINAL_SUMMARY.json").write_text("changed\n", encoding="utf-8")
        expect_raises("SYNTHETIC_SEAL_TAMPER_CHANGED_FAILS", lambda: CAP.verify_sealed_run(changed, test_only_synthetic=True))
        missing = copy_synthetic_fixture(tmp_root / "missing")
        (missing / "verification.json").unlink()
        expect_raises("SYNTHETIC_SEAL_TAMPER_MISSING_FAILS", lambda: CAP.verify_sealed_run(missing, test_only_synthetic=True))
        extra = copy_synthetic_fixture(tmp_root / "extra")
        (extra / "EXTRA.txt").write_text("extra\n", encoding="utf-8")
        expect_raises("SYNTHETIC_SEAL_TAMPER_EXTRA_FAILS", lambda: CAP.verify_sealed_run(extra, test_only_synthetic=True))
        symlinked = copy_synthetic_fixture(tmp_root / "symlink")
        try:
            (symlinked / "SYMLINK").symlink_to(symlinked / "FINAL_SUMMARY.json")
            expect_raises("SYNTHETIC_SEAL_TAMPER_SYMLINK_FAILS", lambda: CAP.verify_sealed_run(symlinked, test_only_synthetic=True))
        except (OSError, NotImplementedError):
            print("PASS SYNTHETIC_SEAL_TAMPER_SYMLINK_FAILS_SKIPPED_UNSUPPORTED")
        inroot = copy_synthetic_fixture(tmp_root / "inroot_symlink")
        try:
            (inroot / "IN_ROOT_LINK").symlink_to(inroot / "FINAL_SUMMARY.json")
            expect_raises("SYNTHETIC_SEAL_TAMPER_IN_ROOT_SYMLINK_FAILS", lambda: CAP.verify_sealed_run(inroot, test_only_synthetic=True))
        except (OSError, NotImplementedError):
            print("PASS SYNTHETIC_SEAL_TAMPER_IN_ROOT_SYMLINK_FAILS_SKIPPED_UNSUPPORTED")
        outside_link = copy_synthetic_fixture(tmp_root / "outside_symlink")
        try:
            outside = tmp_root / "outside_payload.txt"
            outside.write_text("outside\n", encoding="utf-8")
            (outside_link / "OUTSIDE_LINK").symlink_to(outside)
            expect_raises("SYNTHETIC_SEAL_TAMPER_OUTSIDE_SYMLINK_FAILS", lambda: CAP.verify_sealed_run(outside_link, test_only_synthetic=True))
        except (OSError, NotImplementedError):
            print("PASS SYNTHETIC_SEAL_TAMPER_OUTSIDE_SYMLINK_FAILS_SKIPPED_UNSUPPORTED")
        intermediate = copy_synthetic_fixture(tmp_root / "intermediate_symlink")
        try:
            shutil.move(str(intermediate / "custody"), str(intermediate / "custody-real"))
            (intermediate / "custody").symlink_to(intermediate / "custody-real", target_is_directory=True)
            expect_raises("SYNTHETIC_SEAL_TAMPER_INTERMEDIATE_SYMLINK_FAILS", lambda: CAP.verify_sealed_run(intermediate, test_only_synthetic=True))
        except (OSError, NotImplementedError):
            print("PASS SYNTHETIC_SEAL_TAMPER_INTERMEDIATE_SYMLINK_FAILS_SKIPPED_UNSUPPORTED")
        final_link = copy_synthetic_fixture(tmp_root / "final_symlink")
        try:
            target = final_link / "custody/prelaunch-custody.summary.json"
            victim = final_link / "custody/prelaunch-custody.private.json"
            victim.unlink()
            victim.symlink_to(target)
            expect_raises("SYNTHETIC_SEAL_TAMPER_FINAL_SYMLINK_FAILS", lambda: CAP.verify_sealed_run(final_link, test_only_synthetic=True))
        except (OSError, NotImplementedError):
            print("PASS SYNTHETIC_SEAL_TAMPER_FINAL_SYMLINK_FAILS_SKIPPED_UNSUPPORTED")


def check_head() -> None:
    head = subprocess.check_output(["git", "-C", str(ROOT), "rev-parse", "HEAD"], text=True).strip()
    req("HEAD_PIN", head == EXPECTED_HEAD)


def checks() -> None:
    check_head()
    check_manifest()
    check_execution_contract()
    check_sources_and_checksums()
    check_wrapper_source()
    scan_all_public_artifacts()
    check_public_scanner_mutations()
    check_public_summary()
    check_private_custody_artifacts()
    check_fresh_reservation()
    check_no_follow_run_relative_adversaries()
    check_bounded_nonregular_rejection()
    check_test_only_dry_plan()
    check_synthetic_seal_and_tampers()
    print("INCOMING_CUSTODY_RERUN_ADDENDUM_VERIFY_OK_NOT_GO")


def tamper(label: str, mut) -> None:
    m = copy.deepcopy(j("MANIFEST.json"))
    try:
        mut(m)
        with contextlib.redirect_stdout(io.StringIO()):
            check_manifest(m)
    except AssertionError:
        print("PASS TAMPER " + label)
        return
    raise AssertionError("tamper not detected: " + label)


def tampers() -> None:
    tamper("target_count", lambda m: m["frozen_incoming_workload"].__setitem__("target_count", 5899))
    tamper("batch_size", lambda m: m["frozen_incoming_workload"].__setitem__("batch_size", 51))
    tamper("batch_count", lambda m: m["frozen_incoming_workload"].__setitem__("batch_count", 117))
    tamper("raw_ceiling", lambda m: m["frozen_incoming_workload"].__setitem__("max_raw_response_bytes", 33554433))
    tamper("producer_go", lambda m: m.__setitem__("producer_may_issue_go", True))
    tamper("outgoing_authority", lambda m: m.__setitem__("outgoing_run_authority", True))
    tamper("history_overwrite", lambda m: m.__setitem__("historical_evidence_mutation", "allowed"))
    tamper("fresh_policy_drift", lambda m: m.__setitem__("fresh_run_dir_policy", "ALLOW_RESUME"))
    tamper("missing_complete_env_role", lambda m: m.__setitem__("custody_roles", [r for r in m["custody_roles"] if r["role"] != "exact_argv_cwd_complete_env_passed_to_popen"]))
    tamper("missing_public_summary_role", lambda m: m.__setitem__("custody_roles", [r for r in m["custody_roles"] if r["role"] != "allowlisted_public_summary_no_env_leakage"]))
    tamper("missing_fresh_reservation_role", lambda m: m.__setitem__("custody_roles", [r for r in m["custody_roles"] if r["role"] != "atomic_fresh_run_namespace_reservation"]))
    tamper("missing_seal_role", lambda m: m.__setitem__("custody_roles", [r for r in m["custody_roles"] if r["role"] != "post_full_run_seal_and_verify"]))
    tamper("env_public", lambda m: next(r for r in m["custody_roles"] if r["role"] == "exact_argv_cwd_complete_env_passed_to_popen").__setitem__("sensitive_handling", "PUBLIC"))
    tamper("summary_allows_env", lambda m: next(r for r in m["custody_roles"] if r["role"] == "allowlisted_public_summary_no_env_leakage").__setitem__("raw_environment_values_public", True))
    tamper("seal_claims_full_run", lambda m: next(r for r in m["custody_roles"] if r["role"] == "post_full_run_seal_and_verify").__setitem__("full_run_executed_now", True))
    print("INCOMING_CUSTODY_RERUN_ADDENDUM_TAMPER_TESTS_OK_FAIL_CLOSED")


if __name__ == "__main__":
    try:
        if len(sys.argv) > 1 and sys.argv[1] == "--tamper-tests":
            tampers()
        else:
            checks()
    except AssertionError as exc:
        print("VERIFY_FAILED", exc, file=sys.stderr)
        sys.exit(1)
