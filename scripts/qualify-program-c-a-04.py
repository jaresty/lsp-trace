#!/usr/bin/env python3
"""Produce private Darwin/arm64 deterministic-build evidence for Program C A-04."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import platform
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any

MODULE = "gonum.org/v1/gonum"
REVISION = "69ca49f456a7a38cf370131834a2178d9aae17fe"
VERSION = "v0.17.1-0.20260426204603-69ca49f456a7"
MODULE_QUERY = f"{MODULE}@{REVISION}"
SCOPE = Path("qualification/program-c/a-04-current-platform-scope.v1.json")
OUTPUT = Path("qualification/program-c/a-04-darwin-arm64-build-reproducibility.v1.json")
CANDIDATE_MODULE = Path("qualification/program-c/candidates/gonum-leiden")
PACKAGE = "./cmd/candidate"
TYPED_OUTCOMES = [
    "PASS", "PLATFORM_MISMATCH", "TOOL_IDENTITY_FAILURE", "SOURCE_IDENTITY_FAILURE",
    "MODULE_ACQUISITION_FAILURE", "BUILD_FAILURE", "EXECUTABLE_IDENTITY_FAILURE", "HASH_MISMATCH",
]


class QualificationFailure(Exception):
    def __init__(self, kind: str, detail: str):
        super().__init__(detail)
        self.kind = kind
        self.detail = detail


def sha256(data: bytes) -> str:
    return "sha256:" + hashlib.sha256(data).hexdigest()


def canonical(value: Any) -> bytes:
    return json.dumps(value, sort_keys=True, separators=(",", ":")).encode("utf-8")


def run(command: list[str], *, cwd: Path, env: dict[str, str], kind: str) -> subprocess.CompletedProcess[bytes]:
    try:
        completed = subprocess.run(command, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False)
    except OSError as exc:
        raise QualificationFailure(kind, str(exc)) from exc
    if completed.returncode != 0:
        detail = completed.stderr.decode("utf-8", "replace").strip() or f"exit status {completed.returncode}"
        raise QualificationFailure(kind, detail)
    return completed


def source_identity(module_root: Path) -> list[dict[str, Any]]:
    paths = sorted([module_root / "go.mod", module_root / "go.sum", *module_root.rglob("*.go")])
    identities = []
    for path in paths:
        if path.is_symlink() or not path.is_file():
            raise QualificationFailure("SOURCE_IDENTITY_FAILURE", f"invalid source path: {path}")
        data = path.read_bytes()
        identities.append({"path": path.relative_to(module_root).as_posix(), "bytes": len(data), "sha256": sha256(data)})
    if len(identities) < 3:
        raise QualificationFailure("SOURCE_IDENTITY_FAILURE", "incomplete candidate source inventory")
    return identities


def write_immutable(path: Path, receipt: dict[str, Any]) -> None:
    receipt["receipt_sha256"] = sha256(canonical(receipt))
    data = json.dumps(receipt, indent=2, sort_keys=True).encode("utf-8") + b"\n"
    path.parent.mkdir(parents=True, exist_ok=True)
    try:
        with path.open("xb") as handle:
            handle.write(data)
    except FileExistsError as exc:
        raise QualificationFailure("EXECUTABLE_IDENTITY_FAILURE", f"immutable output already exists: {path}") from exc


def qualify(root: Path, output: Path, go_tool_arg: str) -> dict[str, Any]:
    root = root.resolve(strict=True)
    scope_path = root / SCOPE
    expected_output = root / OUTPUT
    output = output if output.is_absolute() else root / output
    if output.resolve() != expected_output.resolve():
        raise QualificationFailure("SOURCE_IDENTITY_FAILURE", f"output must be {OUTPUT.as_posix()}")
    if platform.system() != "Darwin" or platform.machine().lower() != "arm64":
        raise QualificationFailure("PLATFORM_MISMATCH", f"observed {platform.system()}/{platform.machine()}")
    if not scope_path.is_file() or scope_path.is_symlink():
        raise QualificationFailure("SOURCE_IDENTITY_FAILURE", "scope policy missing or symlinked")
    scope = json.loads(scope_path.read_text(encoding="utf-8"))
    if scope.get("current_required_coordinates") != [{"goos": "darwin", "goarch": "arm64", "cgo_enabled": "0"}]:
        raise QualificationFailure("SOURCE_IDENTITY_FAILURE", "scope policy is not exact Darwin arm64 CGO=0")

    module_root = root / CANDIDATE_MODULE
    sources = source_identity(module_root)
    go_mod = (module_root / "go.mod").read_text(encoding="utf-8")
    if f"require {MODULE} {VERSION}" not in go_mod:
        raise QualificationFailure("SOURCE_IDENTITY_FAILURE", "candidate go.mod does not pin exact pseudo-version")

    go_tool = Path(go_tool_arg).expanduser().resolve(strict=True)
    if not go_tool.is_file():
        raise QualificationFailure("TOOL_IDENTITY_FAILURE", f"not a regular Go tool: {go_tool}")
    go_bytes = go_tool.read_bytes()
    qualifier_path = Path(__file__).resolve(strict=True)
    repository_revision = run(
        ["/usr/bin/git", "rev-parse", "HEAD"], cwd=root, env=dict(os.environ), kind="SOURCE_IDENTITY_FAILURE"
    ).stdout.decode("utf-8").strip()
    repository_status = run(
        ["/usr/bin/git", "status", "--porcelain"], cwd=root, env=dict(os.environ), kind="SOURCE_IDENTITY_FAILURE"
    ).stdout
    if len(repository_revision) != 40 or repository_status:
        raise QualificationFailure("SOURCE_IDENTITY_FAILURE", "qualification requires an exact clean repository revision")

    env = dict(os.environ)
    env.update({
        "GOOS": "darwin",
        "GOARCH": "arm64",
        "CGO_ENABLED": "0",
        "GOPROXY": "https://proxy.golang.org",
    })
    version_result = run([str(go_tool), "version"], cwd=module_root, env=env, kind="TOOL_IDENTITY_FAILURE")
    env_result = run([str(go_tool), "env", "GOOS", "GOARCH", "CGO_ENABLED", "GOROOT", "GOTOOLDIR"], cwd=module_root, env=env, kind="TOOL_IDENTITY_FAILURE")
    env_lines = env_result.stdout.decode("utf-8").splitlines()
    if env_lines[:3] != ["darwin", "arm64", "0"] or len(env_lines) != 5:
        raise QualificationFailure("TOOL_IDENTITY_FAILURE", f"unexpected go env: {env_lines}")

    acquisition_command = [str(go_tool), "mod", "download", "-json", MODULE_QUERY]
    acquisition = run(acquisition_command, cwd=module_root, env=env, kind="MODULE_ACQUISITION_FAILURE")
    try:
        module_download = json.loads(acquisition.stdout)
    except json.JSONDecodeError as exc:
        raise QualificationFailure("MODULE_ACQUISITION_FAILURE", f"invalid go mod download JSON: {exc}") from exc
    if module_download.get("Path") != MODULE or module_download.get("Version") != VERSION or module_download.get("Sum") != "h1:V43GU8qUQ/EYbEPGzTP0PANTR6RiJgEJdU/vgsFhQiU=":
        raise QualificationFailure("MODULE_ACQUISITION_FAILURE", "downloaded module identity mismatch")

    with tempfile.TemporaryDirectory(prefix="lsp-trace-program-c-a04-") as directory:
        executable = Path(directory) / "candidate"
        build_command = [
            str(go_tool), "-C", str(module_root), "build", "-trimpath", "-buildvcs=false",
            "-ldflags=-buildid=", "-o", str(executable), PACKAGE,
        ]
        builds = []
        for index in range(2):
            executable.unlink(missing_ok=True)
            try:
                completed = run(build_command, cwd=root, env=env, kind="BUILD_FAILURE")
                if not executable.is_file() or executable.is_symlink():
                    raise QualificationFailure("EXECUTABLE_IDENTITY_FAILURE", "build did not produce a regular executable")
                executable_digest = sha256(executable.read_bytes())
                builds.append({
                    "index": index,
                    "command": build_command,
                    "outcome": "PASS",
                    "failure": None,
                    "stdout_sha256": sha256(completed.stdout),
                    "stderr_sha256": sha256(completed.stderr),
                    "executable_sha256": executable_digest,
                    "executable_bytes": executable.stat().st_size,
                })
            except QualificationFailure as exc:
                builds.append({"index": index, "command": build_command, "outcome": exc.kind, "failure": exc.detail})
                raise
        if len({build["executable_sha256"] for build in builds}) != 1:
            raise QualificationFailure("HASH_MISMATCH", "identical build commands produced different executable hashes")

    identities = {
        "tool": {"path": str(go_tool), "sha256": sha256(go_bytes), "version": version_result.stdout.decode("utf-8").strip(), "goroot": env_lines[3], "gotooldir": env_lines[4]},
        "source": {"module_root": CANDIDATE_MODULE.as_posix(), "files": sources, "manifest_sha256": sha256(canonical(sources))},
        "module": {"path": MODULE, "revision": REVISION, "pseudo_version": VERSION, "sum": module_download["Sum"], "go_mod_sum": module_download.get("GoModSum")},
        "command": {"acquisition": acquisition_command, "build": builds[0]["command"], "identical_build_command_count": 2},
        "environment": {"GOOS": "darwin", "GOARCH": "arm64", "CGO_ENABLED": "0"},
        "package": {"argument": PACKAGE, "module": "lsp-trace/qualification/program-c/gonum-leiden", "main": "cmd/candidate/main.go"},
    }
    return {
        "schema_version": "lsp-trace.private.program-c.a-04-build-reproducibility.v1",
        "gate_id": "A-04",
        "repository_revision": repository_revision,
        "qualifier": {
            "path": qualifier_path.relative_to(root).as_posix(),
            "sha256": sha256(qualifier_path.read_bytes()),
        },
        "result": "PASS",
        "scope_policy": {"path": SCOPE.as_posix(), "sha256": sha256(scope_path.read_bytes()), "policy_id": scope["policy_id"]},
        "candidate": {"module": MODULE, "revision": REVISION, "pseudo_version": VERSION},
        "coordinate": {"goos": "darwin", "goarch": "arm64", "cgo_enabled": "0"},
        "qualification_command": [sys.executable, *sys.argv],
        "identities": identities,
        "builds": builds,
        "deterministic_executable_sha256": builds[0]["executable_sha256"],
        "typed_failure_policy": TYPED_OUTCOMES,
        "network": {
            "authorized_acquisition": f"go mod download -json {MODULE_QUERY}",
            "goproxy": env["GOPROXY"],
            "general_fallback_allowed": False,
            "fallback_attempted": False,
        },
        "claim_ceiling": "PASS establishes two identical-command deterministic builds only for the exact Darwin arm64 CGO-disabled qualification coordinate and pinned module. It does not qualify historical broader coordinates or authorize production adoption.",
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", default=".")
    parser.add_argument("--output", default=OUTPUT.as_posix())
    parser.add_argument("--go-tool", required=True)
    args = parser.parse_args()
    root = Path(args.root)
    output = Path(args.output)
    try:
        receipt = qualify(root, output, args.go_tool)
        target = output if output.is_absolute() else root.resolve() / output
        write_immutable(target, receipt)
    except QualificationFailure as exc:
        print(f"PROGRAM_C_A_04 outcome={exc.kind} detail={exc.detail}", file=sys.stderr)
        return 1
    print(f"PROGRAM_C_A_04 outcome=PASS builds=2 executable_sha256={receipt['deterministic_executable_sha256']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
