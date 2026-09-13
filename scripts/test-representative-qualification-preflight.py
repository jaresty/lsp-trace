#!/usr/bin/env python3
import json
import os
import subprocess
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts/prepare-representative-qualification.py"

def require(condition, assertion, detail=""):
    if not condition:
        raise AssertionError(f"{assertion}: {detail}")
    print(f"PASS {assertion}")

def run(extra=None):
    env = {"PATH": os.environ.get("PATH", "")}
    if extra:
        env.update(extra)
    raw = subprocess.check_output(["python3", str(SCRIPT)], cwd=ROOT, env=env, text=True)
    return raw, json.loads(raw)

def by_id(report, app_id):
    return next(app for app in report["applications"] if app["id"] == app_id)

raw1, base = run()
raw2, _ = run()
require(raw1 == raw2, "ASSERT_PREFLIGHT_DETERMINISTIC_BYTES")
require(by_id(base, "go-repository-fixture")["checks"]["fixture"] == "AVAILABLE", "ASSERT_GOPLS_REPOSITORY_FIXTURE_AVAILABLE")
require(by_id(base, "go-repository-fixture")["reason"] == "PROVIDER_DEPENDENCY_ABSENT", "ASSERT_MISSING_PROVIDER_DISTINCT")
require(by_id(base, "school-surveys")["checks"]["fixture"] == "OUT_OF_BAND", "ASSERT_SCHOOL_SURVEYS_PRIVATE_FIXTURE_NOT_ACCESSED")
require(by_id(base, "javascript-ember")["checks"]["fixture"] == "OUT_OF_BAND", "ASSERT_EMBER_PRIVATE_FIXTURE_NOT_ACCESSED")

with tempfile.TemporaryDirectory(prefix="private-qualification-") as tmp:
    secret = str(Path(tmp) / "do-not-report-provider")
    Path(secret).write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
    os.chmod(secret, 0o700)
    provider_only_raw, provider_only = run({"CSHARP_LS_BIN": secret})
    school = by_id(provider_only, "school-surveys")
    require(school["reason"] == "APPLICATION_DEPENDENCY_ABSENT", "ASSERT_MISSING_APPLICATION_DEPENDENCY_DISTINCT")
    require(secret not in provider_only_raw and tmp not in provider_only_raw, "ASSERT_PRIVATE_PROVIDER_PATH_REDACTED")
    deps_raw, deps = run({"CSHARP_LS_BIN": secret, "SCHOOL_SURVEYS_DEPENDENCIES_READY": "1", "SCHOOL_SURVEYS_WORKSPACE_READY": "1"})
    require(by_id(deps, "school-surveys")["reason"] == "MANAGED_SESSION_ABSENT", "ASSERT_MISSING_SESSION_DISTINCT")
    ready_raw, ready = run({"CSHARP_LS_BIN": secret, "SCHOOL_SURVEYS_DEPENDENCIES_READY": "1", "SCHOOL_SURVEYS_WORKSPACE_READY": "1", "SCHOOL_SURVEYS_SESSION_READY": "1"})
    require(by_id(ready, "school-surveys")["reason"] == "LIVE_QUALIFICATION_PROHIBITED_BY_PREFLIGHT", "ASSERT_READY_PREREQUISITES_STILL_NOT_RUN")
    require(secret not in deps_raw + ready_raw, "ASSERT_PRIVATE_PATH_NEVER_SERIALIZED")

ops = {op["number"]: op for op in base["operations"]}
require(ops[33]["name"] == "trace" and ops[33]["outcome"] == "SKIPPED", "ASSERT_OPERATION_33_TRACE_GATED_UNTIL_EXPOSED")
require("CALLS-server-reported-only" in ops[33]["assertions"], "ASSERT_OPERATION_33_CALLS_SERVER_ONLY")
require("zero-based-session-coordinates" in ops[33]["assertions"], "ASSERT_OPERATION_33_COORDINATES")
require(ops[34]["outcome"] == ops[35]["outcome"] == "SKIPPED", "ASSERT_OPERATIONS_34_35_SKIPPED_UNTIL_EXPOSED")
require(base["source_graph_complete"] == "UNKNOWN", "ASSERT_SOURCE_GRAPH_COMPLETE_UNKNOWN")
require(base["qualification_performed"] is False and base["receipt_created"] is False, "ASSERT_NO_QUALIFICATION_OR_RECEIPT")
require("feature" in base["evidence_semantics"]["claim_ceiling"].lower() and "architecture" in base["evidence_semantics"]["claim_ceiling"].lower(), "ASSERT_NO_FEATURE_ARCHITECTURE_CLAIMS")
require(all("source" not in app for app in base["applications"]), "ASSERT_REPORT_CONTAINS_NO_SOURCE")
