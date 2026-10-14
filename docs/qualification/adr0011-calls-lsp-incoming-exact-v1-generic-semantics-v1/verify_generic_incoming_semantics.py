#!/usr/bin/env python3
"""Deterministic verifier for CALLS_LSP_INCOMING_EXACT_V1 generic incoming semantics.

Docs/evidence-only. This verifier checks self-authored fixtures/oracles and performs
in-memory negative tamper tests. It does not call an LSP server, qualify a provider,
publish a schema, change production code, or issue GO.
"""
from __future__ import annotations

import argparse
import copy
import hashlib
import json
import sys
from pathlib import Path
from typing import Any

PACKAGE = Path("docs/qualification/adr0011-calls-lsp-incoming-exact-v1-generic-semantics-v1")
DISPOSITION = "GENERIC_INCOMING_SEMANTICS_AUTHORED_NOT_GO"
PROFILE = "CALLS_LSP_INCOMING_EXACT_V1"


def load_json(path: Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def resolve_refs(value: Any, fixtures: dict[str, Any]) -> Any:
    if isinstance(value, dict):
        if set(value) == {"$ref"}:
            ref = value["$ref"]
            if not ref.startswith("#/items/"):
                raise ValueError(f"unsupported ref {ref}")
            key = ref.rsplit("/", 1)[1]
            return copy.deepcopy(fixtures["items"][key])
        return {k: resolve_refs(v, fixtures) for k, v in value.items()}
    if isinstance(value, list):
        return [resolve_refs(v, fixtures) for v in value]
    return value


def is_nonneg_int(v: Any) -> bool:
    return isinstance(v, int) and not isinstance(v, bool) and v >= 0


def validate_pos(pos: Any) -> tuple[bool, str | None]:
    if not isinstance(pos, dict):
        return False, "POSITION_NOT_OBJECT"
    if set(pos) != {"line", "character"}:
        return False, "POSITION_SHAPE"
    if not is_nonneg_int(pos["line"]) or not is_nonneg_int(pos["character"]):
        return False, "POSITION"
    return True, None


def validate_range(rng: Any) -> tuple[bool, str | None]:
    if not isinstance(rng, dict):
        return False, "RANGE_NOT_OBJECT"
    if set(rng) != {"start", "end"}:
        return False, "RANGE_SHAPE"
    ok, diag = validate_pos(rng["start"])
    if not ok:
        return False, diag
    ok, diag = validate_pos(rng["end"])
    if not ok:
        return False, diag
    start = (rng["start"]["line"], rng["start"]["character"])
    end = (rng["end"]["line"], rng["end"]["character"])
    if start > end:
        return False, "RANGE_ORDER"
    return True, None


def contains_range(outer: dict[str, Any], inner: dict[str, Any]) -> bool:
    outer_start = (outer["start"]["line"], outer["start"]["character"])
    outer_end = (outer["end"]["line"], outer["end"]["character"])
    inner_start = (inner["start"]["line"], inner["start"]["character"])
    inner_end = (inner["end"]["line"], inner["end"]["character"])
    return outer_start <= inner_start and inner_end <= outer_end


def retained_endpoint(item: dict[str, Any]) -> dict[str, Any]:
    endpoint = {field: copy.deepcopy(item[field]) for field in ("name", "kind", "uri", "range", "selectionRange")}
    for optional in ("tags", "detail", "data"):
        if optional in item:
            endpoint[optional] = copy.deepcopy(item[optional])
    # Keep the LSP source order for readability when optional fields exist.
    ordered: dict[str, Any] = {"name": endpoint["name"], "kind": endpoint["kind"]}
    if "tags" in endpoint:
        ordered["tags"] = endpoint["tags"]
    if "detail" in endpoint:
        ordered["detail"] = endpoint["detail"]
    ordered["uri"] = endpoint["uri"]
    ordered["range"] = endpoint["range"]
    ordered["selectionRange"] = endpoint["selectionRange"]
    if "data" in endpoint:
        ordered["data"] = endpoint["data"]
    return ordered


def validate_item(item: Any) -> tuple[bool, str | None, dict[str, Any] | None]:
    if not isinstance(item, dict):
        return False, "MALFORMED_MEMBER_FROM_NOT_OBJECT", None
    for field in ("name", "kind", "uri", "range", "selectionRange"):
        if field not in item:
            return False, f"MALFORMED_FROM_MISSING_{field.upper()}", None
    if not isinstance(item["name"], str) or item["name"] == "":
        return False, "MALFORMED_FROM_NAME", None
    if not isinstance(item["kind"], int) or isinstance(item["kind"], bool):
        return False, "MALFORMED_FROM_KIND", None
    if not isinstance(item["uri"], str) or item["uri"] == "":
        return False, "MALFORMED_FROM_URI", None
    ok, diag = validate_range(item["range"])
    if not ok:
        return False, f"MALFORMED_FROM_RANGE_{diag.removeprefix('RANGE_')}", None
    ok, diag = validate_range(item["selectionRange"])
    if not ok:
        return False, f"MALFORMED_FROM_SELECTIONRANGE_{diag.removeprefix('RANGE_')}", None
    if not contains_range(item["range"], item["selectionRange"]):
        return False, "MALFORMED_FROM_SELECTIONRANGE_CONTAINMENT", None
    if "tags" in item and (not isinstance(item["tags"], list) or not all(isinstance(v, int) and not isinstance(v, bool) for v in item["tags"])):
        return False, "MALFORMED_FROM_TAGS", None
    if "detail" in item and not isinstance(item["detail"], str):
        return False, "MALFORMED_FROM_DETAIL", None
    return True, None, retained_endpoint(item)


def diagnostic_from_range_error(diag: str | None) -> str:
    if diag == "RANGE_ORDER":
        return "MALFORMED_RANGE_ORDER"
    return "MALFORMED_RANGE_POSITION"


def malformed(diag: str) -> dict[str, Any]:
    return {"classification": "MALFORMED", "relationships": [], "occurrences": [], "diagnostics": [diag], "atomic_refusal": True}


def analyze_response(response: Any, fixtures: dict[str, Any]) -> dict[str, Any]:
    callee = fixtures["queried_callee"]["name"]
    if response is None:
        return {"classification": "SUCCESS_EMPTY_TOP_LEVEL_NULL", "relationships": [], "occurrences": [], "diagnostics": []}
    if not isinstance(response, list):
        return {"classification": "MALFORMED", "relationships": [], "occurrences": [], "diagnostics": ["MALFORMED_TOP_LEVEL_NOT_ARRAY_OR_NULL"], "atomic_refusal": True}
    relationships: list[dict[str, Any]] = []
    occurrences: list[dict[str, Any]] = []
    for member_ordinal, member in enumerate(response):
        if not isinstance(member, dict):
            return malformed("MALFORMED_MEMBER_NOT_OBJECT")
        if "from" not in member:
            return malformed("MALFORMED_MEMBER_FROM_ABSENT")
        ok, diag, endpoint = validate_item(member["from"])
        if not ok:
            return malformed(diag or "MALFORMED_MEMBER_FROM")
        if "fromRanges" not in member:
            return malformed("MALFORMED_MEMBER_REQUIRED_PROPERTY_ABSENT")
        ranges = member["fromRanges"]
        if not isinstance(ranges, list):
            return malformed("MALFORMED_MEMBER_FROMRANGES_NOT_ARRAY")
        caller = endpoint["name"] if endpoint else member["from"].get("name")
        rel: dict[str, Any] = {"from": caller, "to": callee, "member_ordinal": member_ordinal, "range_count": len(ranges), "from_endpoint": endpoint}
        if len(ranges) == 0:
            rel["occurrence_evidence"] = "ZERO_CALL_SITE_OCCURRENCES"
        relationships.append(rel)
        for range_ordinal, rng in enumerate(ranges):
            ok, diag = validate_range(rng)
            if not ok:
                return malformed(diagnostic_from_range_error(diag))
            occurrences.append({"from": caller, "to": callee, "member_ordinal": member_ordinal, "range_ordinal": range_ordinal, "range": rng})
    classification = "SUCCESS_EMPTY_TOP_LEVEL_ARRAY" if not response else ("SUCCESS_NONEMPTY_MEMBERS_ZERO_OCCURRENCES" if not occurrences else "SUCCESS_OCCURRENCES")
    return {"classification": classification, "relationships": relationships, "occurrences": occurrences, "diagnostics": []}


def comparable(actual: dict[str, Any], expected: dict[str, Any]) -> tuple[dict[str, Any], dict[str, Any]]:
    keys = set(expected) | {"classification", "relationships", "occurrences", "diagnostics", "atomic_refusal", "valid_prefix_salvage"}
    a = {k: actual.get(k) for k in keys if k in actual or k in expected}
    e = {k: expected.get(k) for k in keys if k in actual or k in expected}
    if e.get("valid_prefix_salvage") is False and a.get("valid_prefix_salvage") is None and actual.get("atomic_refusal") is True:
        a["valid_prefix_salvage"] = False
    return a, e


def verify_cases(fixtures: dict[str, Any], oracles: dict[str, Any]) -> list[str]:
    failures: list[str] = []
    if fixtures.get("profile") != PROFILE or oracles.get("profile") != PROFILE:
        failures.append("profile mismatch")
    if fixtures.get("disposition") != DISPOSITION or oracles.get("disposition") != DISPOSITION:
        failures.append("disposition mismatch")
    oracle_cases = oracles["cases"]
    fixture_ids = [case["id"] for case in fixtures["cases"]]
    if fixture_ids != list(oracle_cases.keys()):
        failures.append("fixture/oracle case order or membership mismatch")
    for case in fixtures["cases"]:
        cid = case["id"]
        response = resolve_refs(case["response"], fixtures)
        actual = analyze_response(response, fixtures)
        expected = copy.deepcopy(oracle_cases[cid])
        a, e = comparable(actual, expected)
        if a != e:
            failures.append(f"{cid}: actual {json.dumps(a, sort_keys=True)} != expected {json.dumps(e, sort_keys=True)}")
    return failures


def endpoint_case(oracles: dict[str, Any], case_id: str) -> dict[str, Any]:
    return oracles["cases"][case_id]["relationships"][0]["from_endpoint"]


def tamper_tests(fixtures: dict[str, Any], oracles: dict[str, Any]) -> list[str]:
    tests: list[tuple[str, dict[str, Any], dict[str, Any]]] = []
    o1 = copy.deepcopy(oracles)
    o1["cases"]["IN-MEMBER-ABSENT-FROMRANGES"] = {"classification": "SUCCESS_NONEMPTY_MEMBERS_ZERO_OCCURRENCES", "relationships": [{"from": "A", "to": "B", "member_ordinal": 0, "range_count": 0}], "occurrences": [], "diagnostics": []}
    tests.append(("tamper_absent_fromRanges_admitted", fixtures, o1))

    o2 = copy.deepcopy(oracles)
    o2["cases"]["IN-MEMBER-EMPTY-FROMRANGES"] = {"classification": "MALFORMED", "relationships": [], "occurrences": [], "diagnostics": ["MALFORMED_MEMBER_FROMRANGES_EMPTY"], "atomic_refusal": True}
    tests.append(("tamper_empty_fromRanges_refused", fixtures, o2))

    o3 = copy.deepcopy(oracles)
    o3["cases"]["IN-MEMBER-DUPLICATE-RANGES"]["relationships"][0]["range_count"] = 1
    o3["cases"]["IN-MEMBER-DUPLICATE-RANGES"]["occurrences"] = o3["cases"]["IN-MEMBER-DUPLICATE-RANGES"]["occurrences"][:1]
    tests.append(("tamper_duplicate_range_collapsed", fixtures, o3))

    o4 = copy.deepcopy(oracles)
    rel = o4["cases"]["IN-MEMBER-NONEMPTY-TWO-RANGES"]["relationships"][0]
    rel["from"], rel["to"] = rel["to"], rel["from"]
    for occ in o4["cases"]["IN-MEMBER-NONEMPTY-TWO-RANGES"]["occurrences"]:
        occ["from"], occ["to"] = occ["to"], occ["from"]
    tests.append(("tamper_direction_reversed", fixtures, o4))

    o5 = copy.deepcopy(oracles)
    o5["cases"]["IN-MIXED-PREFIX-THEN-MALFORMED"] = {"classification": "SUCCESS_NONEMPTY_MEMBERS_ZERO_OCCURRENCES", "relationships": [copy.deepcopy(oracles["cases"]["IN-MEMBER-EMPTY-FROMRANGES"]["relationships"][0])], "occurrences": [], "diagnostics": []}
    tests.append(("tamper_prefix_salvaged", fixtures, o5))

    o6 = copy.deepcopy(oracles)
    endpoint_case(o6, "IN-FROM-URI-MUTATION")["uri"] = "file:///fixture/a.go"
    tests.append(("tamper_endpoint_uri_mutation_erased", fixtures, o6))

    o7 = copy.deepcopy(oracles)
    endpoint_case(o7, "IN-FROM-KIND-MUTATION")["kind"] = 12
    tests.append(("tamper_endpoint_kind_mutation_erased", fixtures, o7))

    o8 = copy.deepcopy(oracles)
    endpoint_case(o8, "IN-FROM-RANGE-MUTATION")["range"] = copy.deepcopy(endpoint_case(oracles, "IN-MEMBER-EMPTY-FROMRANGES")["range"])
    tests.append(("tamper_endpoint_range_mutation_erased", fixtures, o8))

    o9 = copy.deepcopy(oracles)
    endpoint_case(o9, "IN-FROM-SELECTIONRANGE-MUTATION")["selectionRange"] = copy.deepcopy(endpoint_case(oracles, "IN-MEMBER-EMPTY-FROMRANGES")["selectionRange"])
    tests.append(("tamper_endpoint_selectionRange_mutation_erased", fixtures, o9))

    o10 = copy.deepcopy(oracles)
    o10["cases"]["IN-MEMBER-EMPTY-FROMRANGES"]["occurrences"] = [{"from": "A", "to": "B", "member_ordinal": 0, "range_ordinal": 0, "range": copy.deepcopy(endpoint_case(oracles, "IN-MEMBER-EMPTY-FROMRANGES")["range"])}]
    o10["cases"]["IN-MEMBER-EMPTY-FROMRANGES"]["classification"] = "SUCCESS_OCCURRENCES"
    tests.append(("tamper_endpoint_range_synthesized_as_occurrence", fixtures, o10))

    failures: list[str] = []
    for name, f, o in tests:
        if not verify_cases(f, o):
            failures.append(f"{name}: tamper was not detected")
    return failures


def verify_checksums(root: Path, package: Path) -> list[str]:
    checksum_path = package / "CHECKSUMS.sha256"
    if not checksum_path.exists():
        return ["CHECKSUMS.sha256 missing"]
    failures: list[str] = []
    seen: set[str] = set()
    for line in checksum_path.read_text(encoding="utf-8").splitlines():
        if not line.strip():
            continue
        try:
            got_sha, rel = line.split("  ", 1)
        except ValueError:
            failures.append(f"malformed checksum line: {line}")
            continue
        if rel == "CHECKSUMS.sha256":
            failures.append("CHECKSUMS.sha256 must not include itself")
            continue
        if rel in seen:
            failures.append(f"duplicate checksum target: {rel}")
            continue
        seen.add(rel)
        p = package / rel
        if not p.exists():
            failures.append(f"checksum target missing: {rel}")
            continue
        actual = sha256(p)
        if actual != got_sha:
            failures.append(f"checksum mismatch {rel}: {actual} != {got_sha}")
    manifest_path = package / "MANIFEST.json"
    if manifest_path.exists():
        manifest = load_json(manifest_path)
        manifest_files = {entry["path"] for entry in manifest.get("files", [])}
        expected = manifest_files - {"CHECKSUMS.sha256"}
        missing = sorted(expected - seen)
        extra = sorted(seen - expected)
        for rel in missing:
            failures.append(f"checksum missing manifest file: {rel}")
        for rel in extra:
            failures.append(f"checksum target not in manifest: {rel}")
    else:
        failures.append("MANIFEST.json missing")
    return failures


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--source", default=".", help="repository root")
    args = ap.parse_args()
    root = Path(args.source).resolve()
    package = root / PACKAGE
    fixtures = load_json(package / "fixtures.json")
    oracles = load_json(package / "ORACLES.json")
    failures = []
    failures.extend(verify_cases(fixtures, oracles))
    tamper_failures = tamper_tests(fixtures, oracles)
    failures.extend(tamper_failures)
    failures.extend(verify_checksums(root, package))
    if failures:
        print(json.dumps({"status": "FAIL", "disposition": DISPOSITION, "failures": failures}, indent=2, sort_keys=True))
        return 1
    print(json.dumps({"status": "PASS", "disposition": DISPOSITION, "authority": 0, "accepted": False, "profile": PROFILE, "cases": len(fixtures["cases"]), "tamper_tests": 10}, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
