#!/usr/bin/env python3
"""Emit a deterministic, source-free representative qualification preflight report."""
import argparse
import json
import os
import shutil
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MATRIX = ROOT / "qualification/representative-preflight/matrix.v1.json"
ENV = {
    "go-repository-fixture": ("GOPLS_BIN", "GOPLS_SESSION_READY", ()),
    "school-surveys": ("CSHARP_LS_BIN", "SCHOOL_SURVEYS_SESSION_READY", ("SCHOOL_SURVEYS_DEPENDENCIES_READY", "SCHOOL_SURVEYS_WORKSPACE_READY")),
    "javascript-ember": ("EMBER_PROVIDER_BIN", "EMBER_SESSION_READY", ("EMBER_DEPENDENCIES_READY", "EMBER_WORKSPACE_READY")),
}

def present_executable(name):
    value = os.environ.get(name, "")
    return bool(value and Path(value).is_file() and os.access(value, os.X_OK))

def classify(app):
    provider_var, session_var, dependencies = ENV[app["id"]]
    fixture_available = app["fixture"]["kind"] == "REPOSITORY_LOCAL" and (ROOT / app["fixture"]["id"] / "calls.go").is_file()
    checks = {
        "fixture": "AVAILABLE" if fixture_available else "OUT_OF_BAND",
        "provider": "AVAILABLE" if present_executable(provider_var) else "UNAVAILABLE",
        "session": "READY" if os.environ.get(session_var) == "1" else "ABSENT",
        "dependencies": "AVAILABLE" if all(os.environ.get(v) == "1" for v in dependencies) else ("NOT_REQUIRED" if not dependencies else "UNAVAILABLE"),
    }
    if checks["provider"] == "UNAVAILABLE":
        outcome, reason = "UNAVAILABLE", "PROVIDER_DEPENDENCY_ABSENT"
    elif checks["dependencies"] == "UNAVAILABLE":
        outcome, reason = "UNAVAILABLE", "APPLICATION_DEPENDENCY_ABSENT"
    elif checks["session"] == "ABSENT":
        outcome, reason = "NOT_RUN", "MANAGED_SESSION_ABSENT"
    else:
        outcome, reason = "NOT_RUN", "LIVE_QUALIFICATION_PROHIBITED_BY_PREFLIGHT"
    return {"id": app["id"], "provider": app["provider"], "checks": checks, "outcome": outcome, "reason": reason}

def build_report():
    matrix = json.loads(MATRIX.read_text(encoding="utf-8"))
    tools = {name: ("AVAILABLE" if shutil.which(name) else "UNAVAILABLE") for name in matrix["platform"]["required_tools"]}
    return {
        "schema_version": "lsp-trace.representative-qualification-preflight-report.v1",
        "matrix_schema_version": matrix["schema_version"],
        "platform": {"required_tools": tools, "outcome": "PREPARED" if all(v == "AVAILABLE" for v in tools.values()) else "UNAVAILABLE"},
        "applications": [classify(app) for app in matrix["applications"]],
        "operations": [{"number": op["number"], "name": op["name"], "exposure": op["exposure"], "outcome": op["unexposed_outcome"], "assertions": op["assertions"]} for op in matrix["operations"]],
        "evidence_semantics": matrix["evidence_semantics"],
        "coordinate_convention": matrix["coordinate_convention"],
        "source_graph_complete": matrix["source_graph_complete"],
        "qualification_performed": False,
        "receipt_created": False,
    }

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", help="write report to this path instead of stdout")
    args = parser.parse_args()
    encoded = json.dumps(build_report(), sort_keys=True, separators=(",", ":")) + "\n"
    if args.output:
        Path(args.output).write_text(encoded, encoding="utf-8")
    else:
        print(encoded, end="")

if __name__ == "__main__":
    main()
