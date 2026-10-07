#!/usr/bin/env python3
"""Fail-closed checker for the ADR0011 Package P4 C closure matrix."""
import argparse
import hashlib
import json
import re
import subprocess
from pathlib import Path, PurePosixPath

EXPECTED = [f"C{i:02d}" for i in range(1, 19)] + [
    "bytes_scanned", "messages_processed", "objects_materialized",
    "events_emitted", "documents_acquired", "logical_buffer_bytes_reserved",
]
SHA256 = re.compile(r"[0-9a-f]{64}\Z")
COMMIT = re.compile(r"[0-9a-f]{40}\Z")
REQUIRED_CROSS = {
    "transaction_lifetime", "charging_distinctions", "failure_cleanup",
    "precedence", "compatibility", "source_pins", "package_boundaries",
    "tracked_fixture_prerequisite",
}
PACKET_PATHS = {
    "docs/qualification/adr0011-c-closure-matrix.json",
    "docs/qualification/adr0011-c-closure-matrix.review.md",
    "scripts/check-adr0011-c-closure.py",
}


def safe_file(root: Path, value: str) -> Path:
    pure = PurePosixPath(value)
    if not value or pure.is_absolute() or any(part in {"", ".", ".."} for part in pure.parts):
        raise ValueError(f"unsafe path:{value}")
    path = root.joinpath(*pure.parts)
    if not path.is_file() or path.is_symlink() or not path.resolve().is_relative_to(root):
        raise ValueError(f"missing-or-unsafe-file:{value}")
    return path


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def git_text(root: Path, *args: str) -> str:
    run = subprocess.run(
        ["git", "-C", str(root), *args], text=True,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE,
    )
    if run.returncode != 0:
        raise ValueError(f"git {' '.join(args)}: {run.stderr.strip()}")
    return run.stdout.strip()


def commit_exists(root: Path, value: str) -> bool:
    if not COMMIT.fullmatch(value or ""):
        return False
    run = subprocess.run(
        ["git", "-C", str(root), "cat-file", "-e", f"{value}^{{commit}}"],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    )
    return run.returncode == 0


def is_ancestor(root: Path, older: str, newer: str) -> bool:
    run = subprocess.run(
        ["git", "-C", str(root), "merge-base", "--is-ancestor", older, newer],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    )
    return run.returncode == 0


def execute_selection(root: Path, command: list[str]) -> tuple[int, list[str]]:
    actual = list(command)
    if actual[:2] != ["go", "test"]:
        return -1, []
    actual.insert(2, "-json")
    run = subprocess.run(actual, cwd=root, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    selected: list[str] = []
    for line in run.stdout.splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        name = event.get("Test")
        if event.get("Action") == "run" and isinstance(name, str) and "/" not in name:
            selected.append(name)
    return run.returncode, selected


def wildcard_only(command: list[str]) -> bool:
    if not isinstance(command, list) or not command or not all(isinstance(x, str) and x for x in command):
        return True
    if "-run" not in command:
        return True
    try:
        pattern = command[command.index("-run") + 1]
    except (IndexError, ValueError):
        return True
    stripped = pattern.replace("^", "").replace("$", "").strip()
    return stripped in {"", ".", ".*", "(.+)", "(.*)"}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", default=".")
    parser.add_argument("--matrix", default="docs/qualification/adr0011-c-closure-matrix.json")
    args = parser.parse_args()
    root = Path(args.root).resolve(strict=True)
    matrix_path = safe_file(root, args.matrix)
    document = json.loads(matrix_path.read_text(encoding="utf-8"))
    failures: list[str] = []

    if document.get("schema_version") != "lsp-trace.private.adr0011-c-closure-matrix.v1":
        failures.append("MATRIX_SCHEMA")
    for field in ("audited_commit", "audited_tree", "prerequisite_commit"):
        value = document.get(field)
        if not COMMIT.fullmatch(value or ""):
            failures.append(f"MATRIX_{field.upper()}")
    audited = document.get("audited_commit", "")
    tree = document.get("audited_tree", "")
    prerequisite = document.get("prerequisite_commit", "")
    try:
        head = git_text(root, "rev-parse", "HEAD")
        if not is_ancestor(root, audited, head):
            failures.append("MATRIX_AUDITED_COMMIT_NOT_IN_PACKET_HISTORY")
        if git_text(root, "rev-parse", f"{audited}^{{tree}}") != tree:
            failures.append("MATRIX_AUDITED_TREE_MISMATCH")
        if not is_ancestor(root, prerequisite, audited):
            failures.append("MATRIX_PREREQUISITE_NOT_ANCESTOR")
        for packet_path in sorted(PACKET_PATHS):
            tracked = subprocess.run(
                ["git", "-C", str(root), "ls-files", "--error-unmatch", packet_path],
                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
            )
            if tracked.returncode != 0:
                failures.append(f"PACKET_PATH_UNTRACKED:{packet_path}")
    except ValueError as exc:
        failures.append(f"MATRIX_GIT_IDENTITY:{exc}")
    contract = document.get("contract", {})
    try:
        contract_path = safe_file(root, contract.get("path", ""))
        if contract.get("sha256") != digest(contract_path):
            failures.append("MATRIX_CONTRACT_HASH")
    except ValueError as exc:
        failures.append(f"MATRIX_CONTRACT:{exc}")

    rows = document.get("rows")
    if not isinstance(rows, list):
        rows = []
        failures.append("MATRIX_ROWS")
    ids = [row.get("id") for row in rows if isinstance(row, dict)]
    unknown_ids = sorted(set(ids) - set(EXPECTED))
    missing_ids = [row_id for row_id in EXPECTED if row_id not in ids]
    duplicates = sorted({row_id for row_id in ids if ids.count(row_id) > 1})
    if ids != EXPECTED:
        failures.append(f"ROW_INVENTORY expected={EXPECTED} got={ids}")
    if unknown_ids:
        failures.append(f"UNKNOWN_ROW_IDS:{','.join(unknown_ids)}")
    if missing_ids:
        failures.append(f"MISSING_ROW_IDS:{','.join(missing_ids)}")
    if duplicates:
        failures.append(f"DUPLICATE_ROW_IDS:{','.join(duplicates)}")

    execution_cache: dict[tuple[str, ...], tuple[int, list[str]]] = {}
    for row in rows:
        if not isinstance(row, dict):
            failures.append("ROW_NOT_OBJECT")
            continue
        row_id = row.get("id", "<missing>")
        status = row.get("status")
        if status != "CLOSED":
            failures.append(f"{row_id}:STATUS_{status or 'MISSING'}")
        for field in ("owner", "rule", "reached_stage_witness", "finite_boundary"):
            if not isinstance(row.get(field), str) or not row[field].strip():
                failures.append(f"{row_id}:MISSING_{field.upper()}")
        blockers = row.get("blockers")
        if status == "CLOSED" and blockers:
            failures.append(f"{row_id}:CLOSED_WITH_BLOCKERS")
        if status != "CLOSED" and (not isinstance(blockers, list) or not blockers):
            failures.append(f"{row_id}:UNKNOWN_WITHOUT_BLOCKER")

        commits = row.get("commits")
        if not isinstance(commits, list) or not commits:
            failures.append(f"{row_id}:MISSING_COMMITS")
        else:
            for value in commits:
                if not commit_exists(root, value):
                    failures.append(f"{row_id}:INVALID_COMMIT:{value}")
                elif COMMIT.fullmatch(audited or "") and not is_ancestor(root, value, audited):
                    failures.append(f"{row_id}:COMMIT_OUTSIDE_AUDITED_HISTORY:{value}")

        tests = row.get("tests")
        if not isinstance(tests, list) or not tests:
            failures.append(f"{row_id}:ZERO_SELECTED_TESTS")
        else:
            total = 0
            for test in tests:
                count = test.get("selected_count")
                if not isinstance(count, int) or count <= 0:
                    failures.append(f"{row_id}:ZERO_SELECTED_TESTS")
                else:
                    total += count
                command = test.get("command")
                if wildcard_only(command):
                    failures.append(f"{row_id}:WILDCARD_ONLY_QUALIFICATION")
                else:
                    key = tuple(command)
                    if key not in execution_cache:
                        execution_cache[key] = execute_selection(root, command)
                    returncode, observed_names = execution_cache[key]
                    if returncode != 0:
                        failures.append(f"{row_id}:SELECTED_TEST_COMMAND_FAILED")
                    if not observed_names:
                        failures.append(f"{row_id}:ZERO_EXECUTED_TESTS")
                    elif isinstance(count, int) and len(observed_names) != count:
                        failures.append(f"{row_id}:SELECTED_COUNT_MISMATCH:declared={count}:observed={len(observed_names)}")
                names = test.get("names")
                if not isinstance(names, list) or not names or not all(isinstance(x, str) and x for x in names):
                    failures.append(f"{row_id}:MISSING_TEST_NAMES")
                elif not wildcard_only(command) and observed_names != names:
                    failures.append(f"{row_id}:TEST_NAME_MISMATCH:declared={names}:observed={observed_names}")
                try:
                    path = safe_file(root, test.get("path", ""))
                    if not SHA256.fullmatch(test.get("sha256", "")) or digest(path) != test.get("sha256"):
                        failures.append(f"{row_id}:TEST_HASH_MISMATCH:{test.get('path')}")
                except ValueError as exc:
                    failures.append(f"{row_id}:TEST_PATH:{exc}")
            if total <= 0:
                failures.append(f"{row_id}:ZERO_SELECTED_TESTS")

        artifacts = row.get("artifacts")
        if not isinstance(artifacts, list):
            failures.append(f"{row_id}:INVALID_ARTIFACTS")
        else:
            for artifact in artifacts:
                try:
                    path = safe_file(root, artifact.get("path", ""))
                    if not SHA256.fullmatch(artifact.get("sha256", "")) or digest(path) != artifact.get("sha256"):
                        failures.append(f"{row_id}:ARTIFACT_HASH_MISMATCH:{artifact.get('path')}")
                except ValueError as exc:
                    failures.append(f"{row_id}:ARTIFACT_PATH:{exc}")

    cross = document.get("cross_cutting")
    if not isinstance(cross, dict) or set(cross) != REQUIRED_CROSS or not all(isinstance(v, str) and v for v in cross.values()):
        failures.append("CROSS_CUTTING_INCOMPLETE")

    closed = sum(isinstance(row, dict) and row.get("status") == "CLOSED" for row in rows)
    unknown = sum(isinstance(row, dict) and row.get("status") == "UNKNOWN" for row in rows)
    print(f"ADR0011_C_CLOSURE rows={len(rows)} closed={closed} unknown={unknown} failures={len(failures)}")
    for failure in failures:
        print(f"FAIL {failure}")
    if failures:
        print("C_EXIT_BLOCKED")
        return 1
    print("C_EXIT_GO")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
