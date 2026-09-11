#!/usr/bin/env python3
import argparse
import csv
import hashlib
import json
import re
import sys
from pathlib import Path

EXPECTED = {
    "calls-v1": ["CALLS"],
    "callback-flow-v1": ["BINDS_ARGUMENT", "PASSES_CALLBACK", "INVOKES_TASK"],
    "state-flow-v1": ["UPDATES_STATE"],
    "ui-lifecycle-v1": ["RENDERS_FROM", "TRIGGERS_RELOAD"],
}
STATUSES = {"MISSING", "PASS", "FAIL", "BLOCKED", "UNSUPPORTED", "INCOMPLETE"}
DIGEST = re.compile(r"sha256:[0-9a-f]{64}\Z")


def canonical_digest(profile):
    value = {"name": profile["name"], "relations": profile["relations"]}
    payload = json.dumps(value, sort_keys=True, separators=(",", ":")).encode()
    return "sha256:" + hashlib.sha256(payload).hexdigest()


def fail(assertion, reason):
    print(f"{assertion} result=FAIL reason={reason}")
    raise SystemExit(1)


def regular_receipt(root, value, digest):
    path = Path(value)
    if path.is_absolute() or ".." in path.parts:
        return False
    full = root / path
    if not full.is_file() or full.is_symlink():
        return False
    actual = "sha256:" + hashlib.sha256(full.read_bytes()).hexdigest()
    return actual == digest


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--profiles", required=True)
    parser.add_argument("--matrix", required=True)
    parser.add_argument("--root", required=True)
    parser.add_argument("--require-profile", action="append", default=[])
    args = parser.parse_args()
    root = Path(args.root).resolve()

    try:
        artifact = json.loads(Path(args.profiles).read_text())
    except (OSError, json.JSONDecodeError) as exc:
        fail("ASSERT_PROGRAM_C_PROFILE_MANIFEST", f"unreadable:{exc}")

    if artifact.get("schema_version") != "lsp-trace.program-c-projection-profiles.v1" or artifact.get("status") != "DEFERRED":
        fail("ASSERT_PROGRAM_C_PROFILE_MANIFEST", "identity-or-status")
    profiles = artifact.get("profiles")
    if not isinstance(profiles, list) or [p.get("name") for p in profiles] != list(EXPECTED):
        fail("ASSERT_PROGRAM_C_PROFILE_INVENTORY", "profile-set-or-order")

    by_name = {}
    for profile in profiles:
        name = profile["name"]
        relations = profile.get("relations")
        if not isinstance(relations, list) or [r.get("kind") for r in relations] != EXPECTED[name]:
            fail("ASSERT_PROGRAM_C_PROFILE_RELATIONS", name)
        for relation in relations:
            if relation != {"kind": relation["kind"], "direction": "directed", "multiplicity": "preserve", "weight": 1}:
                fail("ASSERT_PROGRAM_C_PROFILE_SEMANTICS", name)
        if profile.get("logical_digest") != canonical_digest(profile):
            fail("ASSERT_PROGRAM_C_PROFILE_DIGESTS", name)
        by_name[name] = profile

    compositions = artifact.get("compositions")
    if not isinstance(compositions, list) or len(compositions) != 1:
        fail("ASSERT_PROGRAM_C_ALL_QUALIFIED", "composition-count")
    composition = compositions[0]
    if composition.get("name") != "all-qualified-v1" or composition.get("requires_all_members") is not True or composition.get("on_missing_member") != "INCOMPLETE":
        fail("ASSERT_PROGRAM_C_ALL_QUALIFIED", "composition-policy")
    members = composition.get("members")
    if not isinstance(members, list) or [m.get("profile") for m in members] != list(EXPECTED):
        fail("ASSERT_PROGRAM_C_ALL_QUALIFIED", "composition-members")
    for member in members:
        if member.get("digest") != by_name[member["profile"]]["logical_digest"]:
            fail("ASSERT_PROGRAM_C_ALL_QUALIFIED", "member-digest")

    mixed = artifact.get("mixed_language_policy", {})
    if not all(mixed.get(key) is True for key in (
        "require_passing_tuple_per_edge",
        "require_qualified_cross_language_edges",
        "preserve_relation_kind",
        "preserve_provider_identity",
        "preserve_language_framework_scope",
    )):
        fail("ASSERT_PROGRAM_C_MIXED_LANGUAGE_POLICY", "missing-fail-closed-rule")

    try:
        with Path(args.matrix).open(newline="") as stream:
            rows = [row for row in csv.reader((line for line in stream if not line.startswith("#")), delimiter="\t") if row]
    except OSError as exc:
        fail("ASSERT_PROGRAM_C_PROFILE_MATRIX", f"unreadable:{exc}")

    seen = set()
    status_by_profile = {name: [] for name in EXPECTED}
    for row in rows:
        if len(row) != 9:
            fail("ASSERT_PROGRAM_C_PROFILE_MATRIX", "column-count")
        profile, language, framework, provider, version, required, status, receipt, digest = row
        key = (profile, language, framework, provider, version)
        if key in seen:
            fail("ASSERT_PROGRAM_C_PROFILE_MATRIX", "duplicate-tuple")
        seen.add(key)
        if profile not in EXPECTED or not all((language, framework, provider, version)):
            fail("ASSERT_PROGRAM_C_PROFILE_MATRIX", "unknown-or-empty-coordinate")
        if required not in {"yes", "no"} or status not in STATUSES:
            fail("ASSERT_PROGRAM_C_PROFILE_MATRIX", "invalid-required-or-status")
        if status == "MISSING":
            if receipt != "-" or digest != "-":
                fail("ASSERT_PROGRAM_C_PROFILE_MATRIX", "missing-row-has-receipt")
        else:
            if not DIGEST.fullmatch(digest) or not regular_receipt(root, receipt, digest):
                fail("ASSERT_PROGRAM_C_PROFILE_MATRIX", "unverified-receipt")
        status_by_profile[profile].append((required, status))

    for name, statuses in status_by_profile.items():
        if not statuses:
            fail("ASSERT_PROGRAM_C_PROFILE_MATRIX", f"missing-profile:{name}")

    print("ASSERT_PROGRAM_C_PROFILE_MANIFEST result=PASS")
    print("ASSERT_PROGRAM_C_PROFILE_INVENTORY result=PASS profiles=" + ",".join(EXPECTED))
    print("ASSERT_PROGRAM_C_PROFILE_RELATIONS result=PASS")
    print("ASSERT_PROGRAM_C_PROFILE_DIGESTS result=PASS")
    print("ASSERT_PROGRAM_C_ALL_QUALIFIED result=PASS")
    print("ASSERT_PROGRAM_C_MIXED_LANGUAGE_POLICY result=PASS")
    print(f"ASSERT_PROGRAM_C_PROFILE_MATRIX result=PASS tuples={len(rows)}")

    def require(name):
        if name in EXPECTED:
            required_rows = [status for required, status in status_by_profile[name] if required == "yes"]
            return bool(required_rows) and all(status == "PASS" for status in required_rows)
        if name == "all-qualified-v1":
            return all(require(member) for member in EXPECTED)
        fail("ASSERT_PROGRAM_C_REQUESTED_PROFILE", f"unknown-profile:{name}")

    for name in args.require_profile:
        if not require(name):
            fail("ASSERT_PROGRAM_C_REQUESTED_PROFILE", f"not-qualified:{name}")
        print(f"ASSERT_PROGRAM_C_REQUESTED_PROFILE result=PASS profile={name}")


if __name__ == "__main__":
    main()
