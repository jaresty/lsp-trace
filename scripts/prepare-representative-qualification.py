#!/usr/bin/env python3
"""Emit a deterministic, source-free representative qualification preflight report."""
import argparse
import json
import os
import shutil
import stat
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_MATRIX = ROOT / "qualification/representative-preflight/matrix.v1.json"
ENV = {
    "go-repository-fixture": ("GOPLS_BIN", ()),
    "school-surveys": ("CSHARP_LS_BIN", ("SCHOOL_SURVEYS_DEPENDENCIES_READY", "SCHOOL_SURVEYS_WORKSPACE_READY")),
    "javascript-ember": ("EMBER_PROVIDER_BIN", ("EMBER_DEPENDENCIES_READY", "EMBER_WORKSPACE_READY")),
}
CUSTODY = {"OPERATOR_ASSERTED", "CRYPTOGRAPHICALLY_VERIFIED", "PRODUCT_VERIFIED"}
VERSION_STATE = {"VERSION_UNVERIFIED", "VERSION_VERIFIED", "VERSION_MISMATCH", "VERSION_UNPARSEABLE", "NOT_OBSERVED"}
INSTALLED_STATE = {"UNKNOWN", "UNAVAILABLE_OLD_BUILD", "AVAILABLE"}
HEX64 = set("0123456789abcdef")

class PreflightError(Exception):
    pass

def fail(message):
    raise PreflightError(message)

def exact_keys(value, required, optional, where):
    if not isinstance(value, dict):
        fail(f"invalid matrix: {where}: expected object")
    missing = sorted(set(required) - set(value))
    unknown = sorted(set(value) - set(required) - set(optional))
    if missing:
        fail(f"invalid matrix: {where}: missing {','.join(missing)}")
    if unknown:
        fail(f"invalid matrix: {where}: unknown {','.join(unknown)}")

def text(value, where):
    if not isinstance(value, str) or not value:
        fail(f"invalid matrix: {where}: expected non-empty string")

def string_list(value, where, allowed=None):
    if not isinstance(value, list) or not value or any(not isinstance(v, str) or not v for v in value):
        fail(f"invalid matrix: {where}: expected non-empty string array")
    if len(value) != len(set(value)):
        fail(f"invalid matrix: {where}: duplicate value")
    if allowed is not None and any(v not in allowed for v in value):
        fail(f"invalid matrix: {where}: unsupported value")

def version(value, where):
    text(value, where)
    parts = value.split(".")
    if len(parts) not in (3, 4) or any(not p.isdigit() or (len(p) > 1 and p.startswith("0")) for p in parts):
        fail(f"invalid matrix: {where}: invalid version")

def validate_matrix(m):
    exact_keys(m, {"schema_version", "coordinate_convention", "source_graph_complete", "evidence_semantics", "platform", "applications", "operations"}, set(), "root")
    if m["schema_version"] != "lsp-trace.representative-qualification-preparation.v1": fail("invalid matrix: schema_version: unsupported value")
    if m["coordinate_convention"] != "zero-based-session": fail("invalid matrix: coordinate_convention: unsupported value")
    if m["source_graph_complete"] != "UNKNOWN": fail("invalid matrix: source_graph_complete: unsupported value")
    exact_keys(m["evidence_semantics"], {"calls", "claim_ceiling"}, set(), "evidence_semantics")
    if m["evidence_semantics"]["calls"] != "SERVER_REPORTED_CALL_HIERARCHY_ONLY": fail("invalid matrix: evidence_semantics.calls: unsupported value")
    text(m["evidence_semantics"]["claim_ceiling"], "evidence_semantics.claim_ceiling")
    exact_keys(m["platform"], {"goos", "required_tools", "live_execution"}, set(), "platform")
    string_list(m["platform"]["goos"], "platform.goos", {"darwin", "linux", "windows"})
    string_list(m["platform"]["required_tools"], "platform.required_tools", {"git", "go", "node", "python3"})
    if m["platform"]["live_execution"] != "NOT_RUN": fail("invalid matrix: platform.live_execution: unsupported value")
    if not isinstance(m["applications"], list) or not m["applications"]: fail("invalid matrix: applications: expected non-empty array")
    app_ids, provider_ids = [], []
    for i, app in enumerate(m["applications"]):
        where = f"applications[{i}]"
        exact_keys(app, {"id", "language", "provider", "fixture", "prerequisites", "default_outcome"}, {"framework", "claim_condition"}, where)
        text(app["id"], where+".id"); text(app["language"], where+".language")
        app_ids.append(app["id"])
        if app["id"] not in ENV: fail(f"invalid matrix: {where}.id: unsupported value")
        exact_keys(app["provider"], {"id", "protocol_identity", "required_version"}, {"package_version"}, where+".provider")
        provider_ids.append(app["provider"]["id"])
        text(app["provider"]["id"], where+".provider.id"); text(app["provider"]["protocol_identity"], where+".provider.protocol_identity")
        version(app["provider"]["required_version"], where+".provider.required_version")
        if "package_version" in app["provider"]: version(app["provider"]["package_version"], where+".provider.package_version")
        exact_keys(app["fixture"], {"kind", "id"}, set(), where+".fixture")
        if app["fixture"]["kind"] not in {"REPOSITORY_LOCAL", "OUT_OF_BAND_PRIVATE"}: fail(f"invalid matrix: {where}.fixture.kind: unsupported value")
        text(app["fixture"]["id"], where+".fixture.id"); string_list(app["prerequisites"], where+".prerequisites")
        if app["default_outcome"] != "NOT_RUN": fail(f"invalid matrix: {where}.default_outcome: unsupported value")
    if len(app_ids) != len(set(app_ids)): fail("invalid matrix: applications: duplicate id")
    if len(provider_ids) != len(set(provider_ids)): fail("invalid matrix: applications: duplicate provider id")
    if not isinstance(m["operations"], list): fail("invalid matrix: operations: expected array")
    expected = [(33, "trace", "INTEGRATED"), (34, "census", "NOT_IMPLEMENTED"), (35, "context", "NOT_IMPLEMENTED")]
    if len(m["operations"]) != 3: fail("invalid matrix: operations: expected exactly 33,34,35")
    seen = []
    for i, (op, identity) in enumerate(zip(m["operations"], expected)):
        where = f"operations[{i}]"
        exact_keys(op, {"number", "name", "source_state", "installed_default_state", "assertions"}, set(), where)
        if isinstance(op["number"], bool) or not isinstance(op["number"], int): fail(f"invalid matrix: {where}.number: expected integer")
        seen.append(op["number"])
        if (op["number"], op["name"], op["source_state"]) != identity: fail(f"invalid matrix: {where}: fixed operation identity mismatch")
        if op["installed_default_state"] != "UNKNOWN": fail(f"invalid matrix: {where}.installed_default_state: must be UNKNOWN")
        string_list(op["assertions"], where+".assertions")
    if len(seen) != len(set(seen)): fail("invalid matrix: operations: duplicate number")
    return m

def load_json(path, label):
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        fail(f"invalid {label}: unreadable JSON ({exc.__class__.__name__})")

def load_installed(path):
    if path is None: return None
    e = load_json(path, "installed-state evidence")
    exact_keys(e, {"schema_version", "custody", "binary", "operations"}, set(), "installed evidence")
    if e["schema_version"] != "lsp-trace.installed-state-evidence.v1": fail("invalid installed-state evidence: schema_version")
    if e["custody"] not in CUSTODY: fail("invalid installed-state evidence: custody")
    exact_keys(e["binary"], {"revision"}, {"sha256"}, "installed evidence.binary")
    revision = e["binary"]["revision"]
    if not isinstance(revision, str) or len(revision) != 40 or any(c not in HEX64 for c in revision): fail("invalid installed-state evidence: binary.revision")
    if "sha256" in e["binary"] and (not isinstance(e["binary"]["sha256"], str) or len(e["binary"]["sha256"]) != 64 or any(c not in HEX64 for c in e["binary"]["sha256"])): fail("invalid installed-state evidence: binary.sha256")
    if not isinstance(e["operations"], list): fail("invalid installed-state evidence: operations")
    numbers = []
    for i, op in enumerate(e["operations"]):
        exact_keys(op, {"number", "state"}, set(), f"installed evidence.operations[{i}]")
        if op["number"] not in {33,34,35} or op["state"] not in INSTALLED_STATE: fail(f"invalid installed-state evidence: operations[{i}]")
        numbers.append(op["number"])
    if len(numbers) != len(set(numbers)): fail("invalid installed-state evidence: duplicate operation")
    return e

def present_executable(name):
    value = os.environ.get(name, "")
    return bool(value and Path(value).is_file() and os.access(value, os.X_OK))

def classify(app):
    provider_var, dependencies = ENV[app["id"]]
    fixture_available = app["fixture"]["kind"] == "REPOSITORY_LOCAL" and (ROOT / app["fixture"]["id"] / "calls.go").is_file()
    executable = present_executable(provider_var)
    checks = {
        "fixture": "AVAILABLE" if fixture_available else "OUT_OF_BAND",
        "provider_executable": "PRESENT" if executable else "ABSENT",
        "provider_version": "VERSION_UNVERIFIED" if executable else "NOT_OBSERVED",
        "session": "SESSION_UNVERIFIED" if os.environ.get(app["id"].upper().replace("-", "_")+"_SESSION_READY") == "1" else "NOT_OBSERVED",
        "session_prerequisite_custody": "OPERATOR_ASSERTED" if os.environ.get(app["id"].upper().replace("-", "_")+"_SESSION_READY") == "1" else "NOT_OBSERVED",
        "dependencies": "OPERATOR_ASSERTED" if dependencies and all(os.environ.get(v) == "1" for v in dependencies) else ("NOT_REQUIRED" if not dependencies else "NOT_OBSERVED"),
    }
    if not executable: outcome, reason = "UNAVAILABLE", "PROVIDER_EXECUTABLE_ABSENT"
    elif checks["dependencies"] == "NOT_OBSERVED": outcome, reason = "UNAVAILABLE", "APPLICATION_DEPENDENCY_NOT_OBSERVED"
    else: outcome, reason = "NOT_RUN", "SESSION_UNVERIFIED"
    return {"id": app["id"], "provider": {"id": app["provider"]["id"], "protocol_identity": app["provider"]["protocol_identity"], "required_version": app["provider"]["required_version"], "package_version": app["provider"].get("package_version"), "version_requirement_is_evidence": False}, "checks": checks, "outcome": outcome, "reason": reason}

def build_report(matrix_path=DEFAULT_MATRIX, installed_path=None):
    matrix = validate_matrix(load_json(matrix_path, "matrix"))
    installed = load_installed(installed_path)
    installed_by_number = {op["number"]: op["state"] for op in installed["operations"]} if installed else {}
    tools = {name: ("AVAILABLE" if shutil.which(name) else "UNAVAILABLE") for name in matrix["platform"]["required_tools"]}
    operations = []
    for op in matrix["operations"]:
        operations.append({"number": op["number"], "name": op["name"], "source_state": op["source_state"], "installed_state": installed_by_number.get(op["number"], "UNKNOWN"), "assertions": op["assertions"]})
    return {
        "schema_version": "lsp-trace.representative-qualification-preflight-report.v1",
        "matrix_schema_version": matrix["schema_version"],
        "platform": {"required_tools": tools, "outcome": "PREPARED" if all(v == "AVAILABLE" for v in tools.values()) else "UNAVAILABLE"},
        "applications": [classify(app) for app in matrix["applications"]],
        "operations": operations,
        "installed_state_evidence": ({"observed": True, "custody": installed["custody"], "binary": installed["binary"]} if installed else {"observed": False, "custody": "UNKNOWN"}),
        "evidence_semantics": matrix["evidence_semantics"], "coordinate_convention": matrix["coordinate_convention"],
        "source_graph_complete": matrix["source_graph_complete"], "qualification_performed": False, "receipt_created": False,
    }

def safe_publish(path, encoded):
    target = Path(path)
    if target.name in {"", ".", ".."} or target.name != str(target) and any(part == ".." for part in target.parts): fail("unsafe output: traversal is not allowed")
    if target.name.startswith("."): fail("unsafe output: basename must not start with dot")
    parent = target.parent
    if not parent.exists() or not parent.is_dir(): fail("unsafe output: parent must be an existing directory")
    if parent.is_symlink(): fail("unsafe output: symlink parent rejected")
    if target.is_symlink(): fail("unsafe output: symlink target rejected")
    if target.exists(): fail("unsafe output: target already exists")
    fd, temporary = tempfile.mkstemp(prefix=".representative-preflight-", dir=parent)
    try:
        os.fchmod(fd, stat.S_IRUSR | stat.S_IWUSR)
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            handle.write(encoded); handle.flush(); os.fsync(handle.fileno())
        try:
            os.link(temporary, target, follow_symlinks=False)
        except FileExistsError:
            fail("unsafe output: target already exists")
        os.unlink(temporary)
    finally:
        try: os.unlink(temporary)
        except FileNotFoundError: pass

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--matrix", type=Path, default=DEFAULT_MATRIX, help=argparse.SUPPRESS)
    parser.add_argument("--installed-state", type=Path, help="optional attributable installed binary/operation evidence JSON")
    parser.add_argument("--output", help="atomically create a private report file instead of stdout")
    args = parser.parse_args()
    try:
        encoded = json.dumps(build_report(args.matrix, args.installed_state), sort_keys=True, separators=(",", ":")) + "\n"
        if args.output: safe_publish(args.output, encoded)
        else: print(encoded, end="")
    except PreflightError as exc:
        print(f"preflight error: {exc}", file=sys.stderr)
        return 2
    return 0

if __name__ == "__main__":
    sys.exit(main())
