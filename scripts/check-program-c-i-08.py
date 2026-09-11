#!/usr/bin/env python3
import json
import pathlib
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
PATH = ROOT / "qualification/program-c/i-08-neutrality-examples.v1.json"
EXPECTED_ALLOWED = {"structural-community", "structural-crossing", "incomplete-result"}
EXPECTED_PROHIBITED = {
    "feature-identity",
    "business-boundary",
    "ownership-identity",
    "service-identity",
    "stability-overclaim",
    "custody-upgrade",
}
REQUIRED_POLICY = ("run-local", "structural", "exact projection")


def assertion(name, passed, detail=""):
    suffix = "" if not detail else " " + detail
    print(f"{name} result={'PASS' if passed else 'FAIL'}{suffix}")
    return int(passed)


def main():
    passed = 0
    failed = 0
    try:
        if PATH.is_symlink() or not PATH.is_file():
            raise ValueError("example packet must be a repository regular file")
        data = json.loads(PATH.read_text())
    except Exception as exc:
        assertion("ASSERT_I08_PACKET", False, f"reason={type(exc).__name__}")
        print("I_08 PASS=0 FAIL=1 BLOCKED=0")
        return 1

    checks = []
    checks.append(("ASSERT_I08_SCHEMA", data.get("schema_version") == "lsp-trace.program-c-neutrality-examples.v1"))
    policy = data.get("policy", "")
    checks.append(("ASSERT_I08_RUN_LOCAL_STRUCTURAL_POLICY", all(term in policy for term in REQUIRED_POLICY)))

    allowed = data.get("allowed")
    prohibited = data.get("prohibited")
    checks.append(("ASSERT_I08_EXAMPLE_COLLECTIONS", isinstance(allowed, list) and isinstance(prohibited, list)))
    if not isinstance(allowed, list):
        allowed = []
    if not isinstance(prohibited, list):
        prohibited = []

    allowed_ids = {item.get("id") for item in allowed if isinstance(item, dict)}
    prohibited_ids = {item.get("id") for item in prohibited if isinstance(item, dict)}
    checks.append(("ASSERT_I08_ALLOWED_INVENTORY", allowed_ids == EXPECTED_ALLOWED and len(allowed) == len(EXPECTED_ALLOWED)))
    checks.append(("ASSERT_I08_PROHIBITED_INVENTORY", prohibited_ids == EXPECTED_PROHIBITED and len(prohibited) == len(EXPECTED_PROHIBITED)))

    all_items = allowed + prohibited
    checks.append(("ASSERT_I08_EXAMPLES_EXPLAINED", all(isinstance(item, dict) and item.get("statement") and item.get("reason") for item in all_items)))
    forbidden_claims = ("feature", "business-domain", "team", "service", "universally stable", "complete and authenticated")
    prohibited_text = "\n".join(item.get("statement", "") for item in prohibited if isinstance(item, dict))
    checks.append(("ASSERT_I08_NEGATIVE_SEMANTIC_COVERAGE", all(term in prohibited_text for term in forbidden_claims)))
    allowed_text = "\n".join(item.get("statement", "") for item in allowed if isinstance(item, dict))
    checks.append(("ASSERT_I08_ALLOWED_BINDING_AND_FAILURE", all(term in allowed_text for term in ("graph digest", "profile digest", "run-local", "incomplete", "memory ceiling"))))

    for name, ok in checks:
        if assertion(name, ok):
            passed += 1
        else:
            failed += 1
    print(f"I_08 PASS={passed} FAIL={failed} BLOCKED=0")
    return int(failed != 0)


if __name__ == "__main__":
    sys.exit(main())
