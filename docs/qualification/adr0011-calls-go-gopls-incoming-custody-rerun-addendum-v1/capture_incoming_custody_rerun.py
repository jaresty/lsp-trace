#!/usr/bin/env python3
"""Fresh incoming-only custody-complete rerun wrapper for ADR0011 Go/gopls CALLS.

This private evidence tool is an addendum authoring aid. It can prepare a fresh
incoming-only exact-5900 rerun with custody closure, but it does not issue GO,
change ceilings, modify historical evidence, affect outgoing/composition/public
runtime behavior, or grant production/custody authority.
"""
from __future__ import annotations

import argparse
import datetime as _dt
import hashlib
import importlib.util
import json
import os
import pathlib
import platform
import shutil
import stat
import subprocess
import sys
import tempfile
import time
from typing import Any, Dict, Iterable, List, Optional, Tuple

PROFILE_ID = "CALLS_GO_GOPLS_INCOMING_EXACT_V1"
GENERIC_PROFILE_ID = "CALLS_LSP_INCOMING_EXACT_V1"
RUN_NAMESPACE = "lsp-trace.adr0011.calls.go.gopls.incoming.custody_rerun.v1"
RUN_ID_PREFIX = "CALLS_GO_GOPLS_INCOMING_EXACT_V1_CUSTODY_RERUN_V1"
DISPOSITION = "INCOMING_CUSTODY_RERUN_ADDENDUM_AUTHORED_NOT_GO"
IMMUTABLE_DEPENDENCY_BASE = "c3d77247f5a2e06c0d6ac5b1b55a3df7dbea588a"
ADDENDUM_DIR = pathlib.Path(__file__).resolve().parent
REPO_ROOT = ADDENDUM_DIR.parents[2]
ADDENDUM_RELATIVE = ADDENDUM_DIR.relative_to(REPO_ROOT).as_posix()
CHECKSUM_EXCLUDED_PACKAGE_FILES = {"CHECKSUMS.sha256", "VERIFICATION.log", "TAMPER_TESTS.log"}
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
REQUIRED_EXECUTABLE_CONTRACT_FILES = {"capture_incoming_custody_rerun.py", "verify_custody_rerun_addendum.py"}
WORKLOAD_PACKAGE = REPO_ROOT / "docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1"
ORIGINAL_RUNNER = WORKLOAD_PACKAGE / "adr0011_workload_runner.py"
ORIGINAL_CONTRACT = WORKLOAD_PACKAGE / "contract.json"
TARGET_TSV = REPO_ROOT / "docs/qualification/adr0011-callhierarchy-target-corpus-v1/targets.tsv"

EXPECTED = {
    "target_count": 5900,
    "batch_size": 50,
    "batch_count": 118,
    "max_raw_response_bytes": 33554432,
    "source_revision": "4b910fc0c517cb7801fb51d17d295897230d11fa",
    "source_tree": "761474bf5d90bbe3468df62b1a68a1fa26dc2c82",
    "target_tsv_sha256": "8aa361546ab5245ae0ac2f945e3d19b36a48b745bfe7187e5baef6d6d9a14bd4",
}


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def digest_jsonable(obj: Any) -> str:
    return sha256_bytes(json.dumps(obj, sort_keys=True, separators=(",", ":")).encode("utf-8"))


def sha256_file(path: pathlib.Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def byte_len(path: pathlib.Path) -> int:
    return path.stat().st_size


def canonical(path: pathlib.Path) -> pathlib.Path:
    return path.expanduser().resolve(strict=True)


def write_json_atomic(path: pathlib.Path, obj: Any, *, private_mode: bool = False) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_name(path.name + ".tmp")
    tmp.write_text(json.dumps(obj, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    if private_mode:
        os.chmod(tmp, 0o600)
    os.replace(tmp, path)
    if private_mode:
        os.chmod(path, 0o600)


def load_json(path: pathlib.Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def run_command(argv: List[str], *, cwd: Optional[pathlib.Path] = None, env: Optional[Dict[str, str]] = None, timeout: int = 30) -> Dict[str, Any]:
    started = time.time()
    try:
        cp = subprocess.run(argv, cwd=str(cwd) if cwd else None, env=env, text=True, capture_output=True, timeout=timeout)
        return {
            "argv": argv,
            "cwd": str(cwd) if cwd else None,
            "exit_code": cp.returncode,
            "stdout": cp.stdout,
            "stderr": cp.stderr,
            "stdout_sha256": sha256_bytes(cp.stdout.encode("utf-8", errors="surrogateescape")),
            "stderr_sha256": sha256_bytes(cp.stderr.encode("utf-8", errors="surrogateescape")),
            "duration_seconds": round(time.time() - started, 6),
        }
    except Exception as exc:  # pragma: no cover - evidence path records failure payloads
        return {"argv": argv, "cwd": str(cwd) if cwd else None, "exception": repr(exc), "duration_seconds": round(time.time() - started, 6)}


def which_exact(name: str, env: Dict[str, str]) -> Optional[pathlib.Path]:
    found = shutil.which(name, path=env.get("PATH", ""))
    return canonical(pathlib.Path(found)) if found else None


def executable_record(path: pathlib.Path, *, role: str, version_argv: Optional[List[str]], cwd: pathlib.Path, env: Dict[str, str]) -> Dict[str, Any]:
    c = canonical(path)
    rec: Dict[str, Any] = {
        "role": role,
        "canonical_path": str(c),
        "byte_length": byte_len(c),
        "sha256": sha256_file(c),
    }
    if version_argv:
        rec["version"] = run_command(version_argv, cwd=cwd, env=env, timeout=30)
    return rec


def file_record(path: pathlib.Path, *, role: str, source: str) -> Dict[str, Any]:
    c = canonical(path)
    return {
        "role": role,
        "inventory_source": source,
        "canonical_path": str(c),
        "byte_length": byte_len(c),
        "sha256": sha256_file(c),
    }


def resolve_gopls(gopls: str, env: Dict[str, str]) -> pathlib.Path:
    p = pathlib.Path(gopls).expanduser()
    if p.is_absolute() or os.sep in gopls:
        return canonical(p)
    found = which_exact(gopls, env)
    if not found:
        raise RuntimeError(f"gopls not resolved from captured PATH: {gopls!r}")
    return found


def parse_go_tool_names(output: str) -> List[str]:
    names: List[str] = []
    for line in output.splitlines():
        s = line.strip()
        if not s or s.startswith("usage:") or s.startswith("go tool") or s.startswith("Known tools"):
            continue
        for part in s.replace("\t", " ").split():
            if part and part.replace("_", "").replace("-", "").isalnum():
                names.append(part)
    return sorted(set(names))


def resolve_env_tool(value: str, env: Dict[str, str]) -> Optional[pathlib.Path]:
    if not value:
        return None
    first = value.split()[0]
    p = pathlib.Path(first).expanduser()
    if p.is_absolute() or os.sep in first:
        return canonical(p) if p.exists() else None
    return which_exact(first, env)


def toolchain_inventory(go_exe: pathlib.Path, workspace: pathlib.Path, env: Dict[str, str]) -> Dict[str, Any]:
    go_env_result = run_command([str(go_exe), "env", "-json"], cwd=workspace, env=env, timeout=30)
    if go_env_result.get("exit_code") != 0:
        raise RuntimeError("go env -json failed; cannot close toolchain inventory")
    go_env: Dict[str, Any] = json.loads(go_env_result.get("stdout") or "{}")
    goroot = canonical(pathlib.Path(go_env.get("GOROOT", "")))
    gotooldir = canonical(pathlib.Path(go_env.get("GOTOOLDIR", "")))
    go_tool_result = run_command([str(go_exe), "tool"], cwd=workspace, env=env, timeout=30)
    if go_tool_result.get("exit_code") != 0:
        raise RuntimeError("go tool listing failed; cannot close toolchain inventory")
    tool_names = parse_go_tool_names(go_tool_result.get("stdout", "") + "\n" + go_tool_result.get("stderr", ""))

    records: List[Dict[str, Any]] = []
    seen: set[str] = set()
    missing_required: List[Dict[str, str]] = []

    def add(path: pathlib.Path, role: str, source: str, *, executable: bool = False) -> None:
        c = canonical(path)
        if not c.is_file():
            raise RuntimeError(f"required toolchain file is not a file: {c}")
        if executable and not os.access(c, os.X_OK):
            raise RuntimeError(f"required toolchain executable is not executable: {c}")
        key = str(c)
        if key in seen:
            return
        seen.add(key)
        records.append(file_record(c, role=role, source=source))

    add(go_exe, "go_executable_from_captured_PATH", "shutil.which('go', captured PATH)", executable=True)
    add(goroot / "VERSION", "go_env_GOROOT_VERSION_file", "go env GOROOT/VERSION", executable=False)
    for name in tool_names:
        add(gotooldir / name, f"go_tool_{name}", "go tool listing ∩ go env GOTOOLDIR", executable=True)
    for key in ("CC", "CXX", "AR", "PKG_CONFIG"):
        raw = str(go_env.get(key, ""))
        resolved = resolve_env_tool(raw, env)
        if raw and resolved is None:
            missing_required.append({"go_env_key": key, "value": raw, "reason": "nonempty go env tool did not resolve from captured PATH or absolute path"})
            continue
        if resolved is not None:
            add(resolved, f"go_env_{key}", f"go env {key} resolved using captured PATH when relative", executable=True)
    if missing_required:
        raise RuntimeError("required go env tools unresolved: " + json.dumps(missing_required, sort_keys=True))

    return {
        "schema": "lsp-trace.adr0011.calls.go.gopls.closed-toolchain-inventory.v1",
        "closed_inventory_scope": [
            "go executable resolved from exact captured PATH",
            "go env GOROOT/VERSION file",
            "all executable tools named by `go tool` that exist under exact go env GOTOOLDIR",
            "nonempty behavior-relevant external executables named by go env keys CC, CXX, AR, PKG_CONFIG resolved from exact captured PATH or absolute path",
        ],
        "non_claims": [
            "does not claim an uncontrolled recursive GOROOT tree hash",
            "does not claim files outside exact GOROOT/GOTOOLDIR/go-env executable closure are part of the closed inventory",
        ],
        "go_env_json": go_env,
        "go_env_command": go_env_result,
        "go_tool_command": go_tool_result,
        "goroot": str(goroot),
        "gotooldir": str(gotooldir),
        "invocable_tool_names_from_go_tool": tool_names,
        "required_records": records,
        "required_record_count": len(records),
        "complete": True,
    }


def file_uri(path: pathlib.Path) -> str:
    return path.resolve().as_uri()


def initialize_frame(workspace: pathlib.Path) -> Dict[str, Any]:
    params = {
        "processId": None,
        "rootUri": file_uri(workspace),
        "capabilities": {"textDocument": {"callHierarchy": {"dynamicRegistration": False}}},
        "workspaceFolders": [{"uri": file_uri(workspace), "name": workspace.name}],
    }
    payload = {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": params}
    body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
    framed = f"Content-Length: {len(body)}\r\n\r\n".encode("ascii") + body
    return {
        "method": "initialize",
        "rootUri": params["rootUri"],
        "workspaceFolders": params["workspaceFolders"],
        "client_callHierarchy_capability": params["capabilities"]["textDocument"]["callHierarchy"],
        "body_sha256": sha256_bytes(body),
        "framed_sha256": sha256_bytes(framed),
        "body_byte_length": len(body),
        "framed_byte_length": len(framed),
        "body_json": payload,
    }


def make_run_id(explicit: Optional[str]) -> str:
    if explicit:
        return explicit
    stamp = _dt.datetime.now(_dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    return f"{RUN_ID_PREFIX}_{stamp}"


def default_run_dir(run_id: str) -> pathlib.Path:
    return ADDENDUM_DIR / "runs" / run_id / "full-incoming"


def assert_addendum_confined(path: pathlib.Path) -> pathlib.Path:
    resolved = path.resolve()
    try:
        resolved.relative_to(ADDENDUM_DIR.resolve())
    except ValueError as exc:
        raise RuntimeError(f"output path escapes addendum package: {resolved}") from exc
    return resolved


def _reject_bad_component(part: str, *, rel: str) -> None:
    if part in {"", ".", ".."}:
        raise RuntimeError(f"invalid path component in run-relative path: {rel!r}")


def _relative_parts_no_ambiguous(rel: str) -> List[str]:
    if not isinstance(rel, str) or rel == "":
        raise RuntimeError("empty run-relative path")
    if rel.startswith("/") or rel.startswith("~"):
        raise RuntimeError(f"absolute/ambiguous run-relative path rejected: {rel!r}")
    parts = rel.split("/")
    for part in parts:
        _reject_bad_component(part, rel=rel)
    return parts


def _open_flags(*, directory: bool = False, final: bool = False) -> int:
    flags = os.O_RDONLY | getattr(os, "O_CLOEXEC", 0) | getattr(os, "O_NOFOLLOW", 0)
    if directory:
        flags |= getattr(os, "O_DIRECTORY", 0)
    if final:
        # Prevent FIFO/special-file open from blocking before fstat rejects nonregular files.
        flags |= getattr(os, "O_NONBLOCK", 0)
    return flags


def _open_absolute_dir_no_follow(path: pathlib.Path) -> int:
    absolute = pathlib.Path(path)
    if not absolute.is_absolute():
        absolute = pathlib.Path.cwd() / absolute
    raw_parts = absolute.parts
    if not raw_parts or raw_parts[0] != os.sep:
        raise RuntimeError(f"expected absolute directory path: {path}")
    fd = os.open(os.sep, _open_flags(directory=True))
    try:
        for part in raw_parts[1:]:
            _reject_bad_component(part, rel=str(path))
            next_fd = os.open(part, _open_flags(directory=True), dir_fd=fd)
            os.close(fd)
            fd = next_fd
        st = os.fstat(fd)
        if not stat.S_ISDIR(st.st_mode):
            raise RuntimeError(f"not a directory: {path}")
        return fd
    except Exception:
        os.close(fd)
        raise


def _open_relative_no_follow_fd(root_fd: int, rel: str, *, require_file: bool = True) -> int:
    parts = _relative_parts_no_ambiguous(rel)
    dir_fd = os.dup(root_fd)
    try:
        for part in parts[:-1]:
            next_fd = os.open(part, _open_flags(directory=True), dir_fd=dir_fd)
            st = os.fstat(next_fd)
            if not stat.S_ISDIR(st.st_mode):
                raise RuntimeError(f"intermediate component is not a real directory: {rel}")
            os.close(dir_fd)
            dir_fd = next_fd
        flags = _open_flags(directory=not require_file, final=True)
        fd = os.open(parts[-1], flags, dir_fd=dir_fd)
        st = os.fstat(fd)
        if require_file and not stat.S_ISREG(st.st_mode):
            os.close(fd)
            raise RuntimeError(f"run-relative path is not a regular file: {rel}")
        if not require_file and not stat.S_ISDIR(st.st_mode):
            os.close(fd)
            raise RuntimeError(f"run-relative path is not a directory: {rel}")
        return fd
    except OSError as exc:
        raise RuntimeError(f"no-follow run-relative open failed for {rel!r}: {exc}") from exc
    finally:
        os.close(dir_fd)


def _validated_run_root_fd(run_dir: pathlib.Path) -> int:
    run_dir_path = pathlib.Path(run_dir)
    if not run_dir_path.is_absolute():
        run_dir_path = pathlib.Path.cwd() / run_dir_path
    package_root = ADDENDUM_DIR.resolve(strict=True)
    try:
        run_dir_path.relative_to(package_root)
    except ValueError as exc:
        raise RuntimeError(f"run root is not lexically under addendum package: {run_dir_path}") from exc
    fd = _open_absolute_dir_no_follow(run_dir_path)
    package_fd = _open_absolute_dir_no_follow(package_root)
    try:
        root_stat = os.fstat(fd)
        package_stat = os.fstat(package_fd)
        root_lstat = os.stat(run_dir_path, follow_symlinks=False)
        if (root_stat.st_dev, root_stat.st_ino) != (root_lstat.st_dev, root_lstat.st_ino):
            raise RuntimeError("run root changed during no-follow validation")
        if not stat.S_ISDIR(root_stat.st_mode) or not stat.S_ISDIR(package_stat.st_mode):
            raise RuntimeError("invalid run/package directory descriptor")
    finally:
        os.close(package_fd)
    return fd


def package_relative(path: pathlib.Path) -> str:
    target = pathlib.Path(path)
    if not target.is_absolute():
        target = pathlib.Path.cwd() / target
    package_root = ADDENDUM_DIR.resolve(strict=True)
    target.relative_to(package_root)
    rel = target.relative_to(package_root).as_posix()
    root_fd = _open_absolute_dir_no_follow(package_root)
    try:
        fd = _open_relative_no_follow_fd(root_fd, rel, require_file=True)
        os.close(fd)
    finally:
        os.close(root_fd)
    return rel


def run_relative(run_dir: pathlib.Path, path: pathlib.Path) -> str:
    run_root_path = pathlib.Path(run_dir)
    if not run_root_path.is_absolute():
        run_root_path = pathlib.Path.cwd() / run_root_path
    target = pathlib.Path(path)
    if not target.is_absolute():
        target = pathlib.Path.cwd() / target
    rel = target.relative_to(run_root_path).as_posix()
    root_fd = _validated_run_root_fd(run_root_path)
    try:
        fd = _open_relative_no_follow_fd(root_fd, rel, require_file=True)
        os.close(fd)
    finally:
        os.close(root_fd)
    return rel


def resolve_run_relative(run_dir: pathlib.Path, rel: str, *, require_file: bool = True) -> pathlib.Path:
    root_fd = _validated_run_root_fd(run_dir)
    try:
        fd = _open_relative_no_follow_fd(root_fd, rel, require_file=require_file)
        os.close(fd)
    finally:
        os.close(root_fd)
    return pathlib.Path(run_dir) / pathlib.PurePosixPath(rel)


def read_run_relative_text(run_dir: pathlib.Path, rel: str, *, encoding: str = "utf-8") -> str:
    root_fd = _validated_run_root_fd(run_dir)
    try:
        fd = _open_relative_no_follow_fd(root_fd, rel, require_file=True)
        with os.fdopen(fd, "r", encoding=encoding) as f:
            return f.read()
    finally:
        os.close(root_fd)


def load_run_relative_json(run_dir: pathlib.Path, rel: str) -> Any:
    return json.loads(read_run_relative_text(run_dir, rel, encoding="utf-8"))


def sha256_run_relative(run_dir: pathlib.Path, rel: str) -> str:
    root_fd = _validated_run_root_fd(run_dir)
    try:
        fd = _open_relative_no_follow_fd(root_fd, rel, require_file=True)
        h = hashlib.sha256()
        with os.fdopen(fd, "rb") as f:
            for chunk in iter(lambda: f.read(1024 * 1024), b""):
                h.update(chunk)
        return h.hexdigest()
    finally:
        os.close(root_fd)


def byte_len_run_relative(run_dir: pathlib.Path, rel: str) -> int:
    root_fd = _validated_run_root_fd(run_dir)
    try:
        fd = _open_relative_no_follow_fd(root_fd, rel, require_file=True)
        try:
            return os.fstat(fd).st_size
        finally:
            os.close(fd)
    finally:
        os.close(root_fd)


def _assert_existing_ancestors_are_real_dirs(path: pathlib.Path) -> None:
    base = ADDENDUM_DIR.resolve(strict=True)
    resolved_parent = path.parent.resolve(strict=False)
    try:
        resolved_parent.relative_to(base)
    except ValueError as exc:
        raise RuntimeError(f"reserved path escapes addendum package: {path}") from exc
    cur = base
    for part in path.relative_to(base).parts[:-1]:
        cur = cur / part
        if cur.exists():
            if cur.is_symlink() or not cur.is_dir():
                raise RuntimeError(f"reservation ancestor is not a real directory: {cur}")


def reserve_fresh_run_dir(run_dir: pathlib.Path) -> pathlib.Path:
    """Atomically reserve a never-existing run directory; no resume after custody-only crash."""
    run_dir = assert_addendum_confined(run_dir)
    base = ADDENDUM_DIR.resolve(strict=True)
    if run_dir.exists() or run_dir.is_symlink():
        raise RuntimeError(f"run_dir already exists; refusing resume/overwrite: {run_dir}")
    run_id_dir = run_dir.parent if run_dir.name == "full-incoming" else run_dir
    if run_id_dir.exists() or run_id_dir.is_symlink():
        raise RuntimeError(f"run id directory already exists; refusing same-run-id resume: {run_id_dir}")
    _assert_existing_ancestors_are_real_dirs(run_id_dir)
    runs_root = run_id_dir.parent
    runs_root.mkdir(parents=True, exist_ok=True)
    if runs_root.is_symlink() or not runs_root.is_dir():
        raise RuntimeError(f"runs root is not a real directory: {runs_root}")
    run_id_dir.mkdir(mode=0o700, exist_ok=False)
    if run_dir != run_id_dir:
        run_dir.mkdir(mode=0o700, exist_ok=False)
    real = run_dir.resolve(strict=True)
    real.relative_to(base)
    if real.is_symlink() or not real.is_dir():
        raise RuntimeError(f"reserved run_dir is not a real directory: {run_dir}")
    return real


def git_text(repo_root: pathlib.Path, args: List[str], *, check: bool = True) -> str:
    cp = subprocess.run(["git", "-C", str(repo_root), *args], text=True, capture_output=True)
    if check and cp.returncode != 0:
        raise RuntimeError(f"git {' '.join(args)} failed: {cp.stderr.strip()}")
    return cp.stdout


def git_head(repo_root: pathlib.Path = REPO_ROOT) -> str:
    return git_text(repo_root, ["rev-parse", "HEAD"]).strip()


def git_paths(repo_root: pathlib.Path, args: List[str]) -> List[str]:
    return [line for line in git_text(repo_root, args).splitlines() if line]


def git_is_ancestor(repo_root: pathlib.Path, base: str, head: str) -> bool:
    cp = subprocess.run(["git", "-C", str(repo_root), "merge-base", "--is-ancestor", base, head], text=True, capture_output=True)
    if cp.returncode == 0:
        return True
    if cp.returncode == 1:
        return False
    raise RuntimeError(f"git merge-base failed: {cp.stderr.strip()}")


def load_package_checksums(addendum_dir: pathlib.Path) -> Dict[str, str]:
    checksums_path = addendum_dir / "CHECKSUMS.sha256"
    if not checksums_path.is_file() or checksums_path.is_symlink():
        raise RuntimeError("missing/nonregular CHECKSUMS.sha256")
    checksums: Dict[str, str] = {}
    for line_no, line in enumerate(checksums_path.read_text(encoding="utf-8").splitlines(), 1):
        if not line.strip():
            continue
        try:
            digest, rel = line.split("  ", 1)
        except ValueError as exc:
            raise RuntimeError(f"invalid checksum line {line_no}") from exc
        if not rel or rel.startswith("/") or ".." in pathlib.PurePosixPath(rel).parts or rel in checksums:
            raise RuntimeError(f"invalid checksum path at line {line_no}: {rel!r}")
        checksums[rel] = digest
    return checksums


def iter_addendum_package_files(addendum_dir: pathlib.Path) -> Iterable[pathlib.Path]:
    for path in sorted(addendum_dir.rglob("*")):
        rel_parts = path.relative_to(addendum_dir).parts
        if "__pycache__" in rel_parts:
            continue
        if path.is_symlink():
            raise RuntimeError(f"symlink forbidden in addendum package: {path.relative_to(addendum_dir).as_posix()}")
        if path.is_dir():
            continue
        if not path.is_file():
            raise RuntimeError(f"nonregular file forbidden in addendum package: {path.relative_to(addendum_dir).as_posix()}")
        yield path


def validate_addendum_package_integrity(addendum_dir: pathlib.Path = ADDENDUM_DIR) -> Dict[str, Any]:
    checksums = load_package_checksums(addendum_dir)
    seen: set[str] = set()
    for required in REQUIRED_PACKAGE_FILES | {"CHECKSUMS.sha256"}:
        path = addendum_dir / required
        if not path.exists() or path.is_symlink() or not path.is_file():
            raise RuntimeError(f"missing/nonregular required addendum file: {required}")
    executable_files: List[str] = []
    for path in iter_addendum_package_files(addendum_dir):
        rel = path.relative_to(addendum_dir).as_posix()
        mode = path.stat().st_mode
        if mode & (stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH):
            executable_files.append(rel)
            if rel not in REQUIRED_EXECUTABLE_CONTRACT_FILES:
                raise RuntimeError(f"extra executable contract file forbidden: {rel}")
        if rel in CHECKSUM_EXCLUDED_PACKAGE_FILES:
            continue
        seen.add(rel)
        if rel not in checksums:
            raise RuntimeError(f"addendum file missing from CHECKSUMS.sha256: {rel}")
        actual = sha256_file(path)
        if actual != checksums[rel]:
            raise RuntimeError(f"addendum checksum drift: {rel}")
    missing_executable = sorted(REQUIRED_EXECUTABLE_CONTRACT_FILES - set(executable_files))
    if missing_executable:
        raise RuntimeError("missing executable contract files: " + ", ".join(missing_executable))
    extra_checksum_entries = sorted(set(checksums) - seen)
    if extra_checksum_entries:
        raise RuntimeError("CHECKSUMS.sha256 references missing/extra files: " + ", ".join(extra_checksum_entries[:10]))
    return {
        "schema": "lsp-trace.adr0011.calls.go.gopls.incoming.addendum-package-integrity.v1",
        "package_file_count": len(seen),
        "package_file_list_sha256": sha256_bytes("\n".join(sorted(seen)).encode("utf-8")),
        "checksum_manifest_sha256": sha256_file(addendum_dir / "CHECKSUMS.sha256"),
        "required_executable_contract_files": sorted(REQUIRED_EXECUTABLE_CONTRACT_FILES),
    }


def validate_repository_execution_context(
    repo_root: pathlib.Path = REPO_ROOT,
    addendum_dir: pathlib.Path = ADDENDUM_DIR,
    immutable_base: str = IMMUTABLE_DEPENDENCY_BASE,
) -> Dict[str, Any]:
    repo_root = repo_root.resolve(strict=True)
    addendum_dir = addendum_dir.resolve(strict=True)
    try:
        addendum_rel = addendum_dir.relative_to(repo_root).as_posix()
    except ValueError as exc:
        raise RuntimeError("addendum package is outside repository root") from exc
    head = git_head(repo_root)
    if not git_is_ancestor(repo_root, immutable_base, head):
        raise RuntimeError(f"immutable dependency base is not an ancestor of HEAD: {immutable_base}")
    staged = git_paths(repo_root, ["diff", "--cached", "--name-only", "--"])
    if staged:
        raise RuntimeError("staged tracked changes forbidden before execution: " + ", ".join(staged[:10]))
    dirty = git_paths(repo_root, ["diff", "--name-only", "--"])
    if dirty:
        raise RuntimeError("dirty tracked changes forbidden before execution: " + ", ".join(dirty[:10]))
    changed = git_paths(repo_root, ["diff", "--name-only", f"{immutable_base}..{head}", "--"])
    prefix = addendum_rel + "/"
    unrelated = [path for path in changed if path != addendum_rel and not path.startswith(prefix)]
    if unrelated:
        raise RuntimeError("HEAD descendant changes outside committed addendum package forbidden: " + ", ".join(unrelated[:10]))
    integrity = validate_addendum_package_integrity(addendum_dir)
    return {
        "schema": "lsp-trace.adr0011.calls.go.gopls.incoming.repository-execution-context.v1",
        "immutable_dependency_base": immutable_base,
        "immutable_dependency_base_token": immutable_base[:12],
        "immutable_dependency_base_sha256": sha256_bytes(immutable_base.encode("utf-8")),
        "actual_head": head,
        "actual_head_token": head[:12],
        "actual_head_sha256": sha256_bytes(head.encode("utf-8")),
        "base_is_ancestor": True,
        "descendant_changes_confined_to_addendum_package": True,
        "addendum_package_relative": addendum_rel,
        "descendant_changed_path_count": len(changed),
        "descendant_changed_paths_sha256": sha256_bytes("\n".join(changed).encode("utf-8")),
        "tracked_dirty_paths": 0,
        "tracked_staged_paths": 0,
        "untracked_paths_authorized": False,
        "untracked_provider_candidate_read": False,
        "package_integrity": integrity,
    }


def load_original_runner():
    spec = importlib.util.spec_from_file_location("adr0011_original_workload_runner", ORIGINAL_RUNNER)
    if spec is None or spec.loader is None:
        raise RuntimeError("cannot import original workload runner")
    mod = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = mod
    spec.loader.exec_module(mod)
    return mod


def capture_complete_env() -> Tuple[Dict[str, str], Dict[str, str], str]:
    env = {str(k): str(v) for k, v in os.environ.items()}
    sorted_env = {k: env[k] for k in sorted(env)}
    return env, sorted_env, digest_jsonable(sorted_env)


PUBLIC_SUMMARY_ALLOWED_TOP_LEVEL = {
    "schema", "profile_id", "binds_to_generic_profile_id", "run_namespace", "run_id",
    "producer_disposition", "authority", "accepted", "completeness", "production_admission",
    "worktree_head_token", "worktree_head_sha256", "immutable_dependency_base_token",
    "immutable_dependency_base_sha256", "repository_state_policy", "historical_evidence_use",
    "contract_invariants", "artifact_digests",
    "launch_summary", "initialize_summary", "executable_summaries", "toolchain_summary",
    "runner_runtime_summary", "public_summary_policy",
}
PUBLIC_SUMMARY_FORBIDDEN_KEYS = {
    "complete_env", "go_env_json", "canonical_path", "path", "argv", "cwd", "rootUri",
    "workspaceFolders", "stdout", "stderr", "env_keys_sorted", "go_env_selected_behavior_relevant",
    "goroot", "gotooldir", "complete_env_private_record",
}


def command_public_summary(cmd: Dict[str, Any]) -> Dict[str, Any]:
    return {
        "exit_code": cmd.get("exit_code"),
        "stdout_sha256": cmd.get("stdout_sha256") or sha256_bytes(str(cmd.get("stdout", "")).encode("utf-8", errors="surrogateescape")),
        "stderr_sha256": cmd.get("stderr_sha256") or sha256_bytes(str(cmd.get("stderr", "")).encode("utf-8", errors="surrogateescape")),
        "stdout_byte_length": len(str(cmd.get("stdout", "")).encode("utf-8", errors="surrogateescape")),
        "stderr_byte_length": len(str(cmd.get("stderr", "")).encode("utf-8", errors="surrogateescape")),
        "exception_sha256": sha256_bytes(str(cmd.get("exception", "")).encode("utf-8", errors="surrogateescape")) if "exception" in cmd else None,
    }


def executable_public_summary(rec: Dict[str, Any]) -> Dict[str, Any]:
    out = {"byte_length": rec.get("byte_length"), "sha256": rec.get("sha256")}
    if "version" in rec:
        out["version"] = command_public_summary(rec["version"])
    return out


def assert_public_summary_allowlisted(obj: Any, *, path: str = "$") -> None:
    if isinstance(obj, dict):
        for key, value in obj.items():
            if key in PUBLIC_SUMMARY_FORBIDDEN_KEYS:
                raise RuntimeError(f"public summary forbidden key {key!r} at {path}")
            if isinstance(value, str) and (value.startswith("/") or "file:///" in value or "PATH=" in value):
                raise RuntimeError(f"public summary forbidden raw-looking value at {path}.{key}")
            assert_public_summary_allowlisted(value, path=f"{path}.{key}")
    elif isinstance(obj, list):
        for idx, item in enumerate(obj):
            assert_public_summary_allowlisted(item, path=f"{path}[{idx}]")


def build_public_summary(custody: Dict[str, Any], seal: Dict[str, Any], record_path: pathlib.Path) -> Dict[str, Any]:
    toolchain = custody["go_toolchain_inventory"]
    summary = {
        "schema": "lsp-trace.adr0011.calls.go.gopls.incoming.custody-rerun-public-summary.v3",
        "profile_id": custody["profile_id"],
        "binds_to_generic_profile_id": custody["binds_to_generic_profile_id"],
        "run_namespace": custody["run_namespace"],
        "run_id": custody["run_id"],
        "producer_disposition": custody["producer_disposition"],
        "authority": custody["authority"],
        "accepted": custody["accepted"],
        "completeness": custody["completeness"],
        "production_admission": custody["production_admission"],
        "worktree_head_token": custody["repository_state"]["actual_head_token"],
        "worktree_head_sha256": custody["repository_state"]["actual_head_sha256"],
        "immutable_dependency_base_token": custody["repository_state"]["immutable_dependency_base_token"],
        "immutable_dependency_base_sha256": custody["repository_state"]["immutable_dependency_base_sha256"],
        "repository_state_policy": {
            "base_is_ancestor": custody["repository_state"]["base_is_ancestor"],
            "descendant_changes_confined_to_addendum_package": custody["repository_state"]["descendant_changes_confined_to_addendum_package"],
            "descendant_changed_path_count": custody["repository_state"]["descendant_changed_path_count"],
            "descendant_changed_paths_sha256": custody["repository_state"]["descendant_changed_paths_sha256"],
            "package_integrity_schema": custody["repository_state"]["package_integrity"]["schema"],
            "untracked_paths_authorized": False,
        },
        "historical_evidence_use": custody["historical_evidence_use"],
        "contract_invariants": custody["contract_invariants"],
        "artifact_digests": {
            "private_prelaunch_record_sha256": seal["record_sha256"],
            "private_prelaunch_record_byte_length": seal["record_byte_length"],
            "environment_digest": seal["complete_env_sha256"],
            "environment_key_count": seal["complete_env_key_count"],
            "private_record_path_sha256": sha256_bytes(str(record_path).encode("utf-8")),
        },
        "launch_summary": {
            "argv_count": len(custody["launch"].get("argv", [])),
            "argv_sha256": sha256_bytes(json.dumps(custody["launch"].get("argv", []), sort_keys=True, separators=(",", ":")).encode("utf-8")),
            "cwd_sha256": sha256_bytes(custody["launch"].get("cwd", "").encode("utf-8")),
            "environment_digest": custody["launch"]["complete_env_sha256"],
            "environment_key_count": custody["launch"]["complete_env_key_count"],
            "env_binding_source": custody["launch"]["env_binding"]["source"],
        },
        "initialize_summary": {
            "body_sha256": custody["initialize"]["body_sha256"],
            "framed_sha256": custody["initialize"]["framed_sha256"],
            "body_byte_length": custody["initialize"]["body_byte_length"],
            "framed_byte_length": custody["initialize"]["framed_byte_length"],
            "root_uri_sha256": sha256_bytes(custody["initialize"]["rootUri"].encode("utf-8")),
            "workspace_folder_count": len(custody["initialize"].get("workspaceFolders", [])),
            "client_call_hierarchy_dynamic_registration": custody["initialize"]["client_callHierarchy_capability"].get("dynamicRegistration"),
        },
        "executable_summaries": {
            "gopls": executable_public_summary(custody["gopls_executable"]),
            "go": executable_public_summary(custody["go_executable"]),
            "python": executable_public_summary(custody["runner_runtime"]["python_executable"]),
        },
        "toolchain_summary": {
            "schema": toolchain["schema"],
            "complete": toolchain["complete"],
            "required_record_count": toolchain["required_record_count"],
            "required_records_digest": digest_jsonable(toolchain["required_records"]),
            "go_environment_digest": digest_jsonable(toolchain["go_env_json"]),
            "go_environment_key_count": len(toolchain["go_env_json"]),
            "go_tool_name_count": len(toolchain["invocable_tool_names_from_go_tool"]),
            "go_tool_names_digest": digest_jsonable(toolchain["invocable_tool_names_from_go_tool"]),
            "go_env_command": command_public_summary(toolchain["go_env_command"]),
            "go_tool_command": command_public_summary(toolchain["go_tool_command"]),
            "closed_inventory_scope_count": len(toolchain["closed_inventory_scope"]),
            "non_claim_count": len(toolchain["non_claims"]),
        },
        "runner_runtime_summary": {
            "python_version_sha256": sha256_bytes(custody["runner_runtime"]["python_version"].encode("utf-8")),
            "platform_sha256": sha256_bytes(custody["runner_runtime"]["platform"].encode("utf-8")),
            "wrapper_source_sha256": custody["runner_runtime"]["wrapper_source"]["sha256"],
            "wrapper_source_byte_length": custody["runner_runtime"]["wrapper_source"]["byte_length"],
            "original_runner_source_sha256": custody["runner_runtime"]["original_runner_source"]["sha256"],
            "original_runner_source_byte_length": custody["runner_runtime"]["original_runner_source"]["byte_length"],
            "original_contract_sha256": custody["runner_runtime"]["original_contract"]["sha256"],
            "original_contract_byte_length": custody["runner_runtime"]["original_contract"]["byte_length"],
        },
        "public_summary_policy": {
            "construction": "ALLOWLIST_ONLY",
            "raw_environment_values_present": False,
            "raw_environment_key_names_present": False,
            "raw_paths_present": False,
        },
    }
    if set(summary) - PUBLIC_SUMMARY_ALLOWED_TOP_LEVEL:
        raise RuntimeError("public summary contains non-allowlisted top-level fields")
    assert_public_summary_allowlisted(summary)
    return summary


def assert_contract_invariants(contract: Dict[str, Any]) -> Dict[str, Any]:
    inv = {
        "target_count": contract["corpus_identity"]["target_count"],
        "source_revision": contract["corpus_identity"]["source_revision"],
        "source_tree": contract["corpus_identity"]["source_tree"],
        "target_tsv_sha256": contract["corpus_identity"]["target_tsv"]["sha256"],
        "batch_size": contract["limits"]["batch_size"],
        "batch_count": contract["full_workload"]["batch_count"],
        "max_raw_response_bytes": contract["limits"]["max_raw_response_bytes"],
        "incoming_inference_from_outgoing_permitted": contract["directions"]["incoming"]["inference_from_outgoing_permitted"],
        "sequential_ordinals": contract["accounting"]["sequential_ordinals"],
        "strict_attempt_ordering": contract["accounting"]["strict_attempt_ordering"],
    }
    for key, expected in EXPECTED.items():
        if inv.get(key) != expected:
            raise RuntimeError(f"contract invariant drift: {key}: {inv.get(key)!r} != {expected!r}")
    if inv["incoming_inference_from_outgoing_permitted"] is not False:
        raise RuntimeError("incoming inference from outgoing must remain forbidden")
    return inv


def capture_prelaunch(run_dir: pathlib.Path, workspace: pathlib.Path, gopls: str, run_id: str) -> Dict[str, Any]:
    run_dir = assert_addendum_confined(run_dir)
    workspace = workspace.resolve(strict=True)
    repository_state = validate_repository_execution_context()
    env, sorted_env, env_digest = capture_complete_env()
    gopls_path = resolve_gopls(gopls, env)
    go_exe = which_exact("go", env)
    if not go_exe:
        raise RuntimeError("go executable not resolved from captured PATH")
    argv = [str(gopls_path), "serve"]
    contract = load_json(ORIGINAL_CONTRACT)
    contract_invariants = assert_contract_invariants(contract)
    custody = {
        "schema": "lsp-trace.adr0011.calls.go.gopls.incoming.custody-rerun-prelaunch.v3",
        "profile_id": PROFILE_ID,
        "binds_to_generic_profile_id": GENERIC_PROFILE_ID,
        "run_namespace": RUN_NAMESPACE,
        "run_id": run_id,
        "producer_disposition": DISPOSITION,
        "authority": 0,
        "accepted": False,
        "completeness": "UNKNOWN",
        "production_admission": "REFUSED_NOT_REQUESTED_NOT_GRANTED",
        "captured_at_unix": time.time(),
        "worktree_head": repository_state["actual_head"],
        "repository_state": repository_state,
        "historical_evidence_use": "FRESH_RERUN_ONLY_DO_NOT_REPRESENT_AS_OLD_RUN_EVIDENCE",
        "contract_invariants": contract_invariants,
        "gopls_executable": executable_record(gopls_path, role="gopls_executable", version_argv=[str(gopls_path), "version"], cwd=workspace, env=env),
        "launch": {
            "argv": argv,
            "cwd": str(workspace),
            "env_keys_sorted": list(sorted_env.keys()),
            "complete_env": sorted_env,
            "complete_env_sha256": env_digest,
            "complete_env_key_count": len(sorted_env),
            "sensitive_handling": "PRIVATE_VALUES_RETAINED_NOT_PUBLIC",
            "env_binding": {
                "source": "captured_env",
                "source_assignment": "captured_env = cap['captured_env']",
                "popen_call_requirement": "subprocess.Popen(..., env=captured_env, ...)",
                "same_object_digest": env_digest,
            },
        },
        "initialize": initialize_frame(workspace),
        "go_executable": executable_record(go_exe, role="go_executable_from_captured_PATH", version_argv=[str(go_exe), "version"], cwd=workspace, env=env),
        "go_toolchain_inventory": toolchain_inventory(go_exe, workspace, env),
        "runner_runtime": {
            "python_executable": executable_record(canonical(pathlib.Path(sys.executable)), role="python_executable", version_argv=[sys.executable, "--version"], cwd=workspace, env=env),
            "python_version": sys.version,
            "platform": platform.platform(),
            "wrapper_source": {"path": str(pathlib.Path(__file__).resolve()), "sha256": sha256_file(pathlib.Path(__file__).resolve()), "byte_length": byte_len(pathlib.Path(__file__).resolve())},
            "original_runner_source": {"path": str(ORIGINAL_RUNNER), "sha256": sha256_file(ORIGINAL_RUNNER), "byte_length": byte_len(ORIGINAL_RUNNER)},
            "original_contract": {"path": str(ORIGINAL_CONTRACT), "sha256": sha256_file(ORIGINAL_CONTRACT), "byte_length": byte_len(ORIGINAL_CONTRACT)},
        },
    }
    custody_dir = run_dir / "custody"
    record_path = custody_dir / "prelaunch-custody.private.json"
    write_json_atomic(record_path, custody, private_mode=True)
    record_sha = sha256_file(record_path)
    seal = {
        "schema": "lsp-trace.adr0011.calls.go.gopls.incoming.custody-rerun-seal.v3",
        "record_relative": run_relative(run_dir, record_path),
        "record_sha256": record_sha,
        "record_byte_length": byte_len(record_path),
        "complete_env_sha256": env_digest,
        "complete_env_key_count": len(sorted_env),
        "run_id": run_id,
        "sealed_before_gopls_launch": True,
        "private_values_retained": True,
    }
    write_json_atomic(custody_dir / "prelaunch-custody.seal.json", seal, private_mode=False)
    public_summary = build_public_summary(custody, seal, record_path)
    write_json_atomic(custody_dir / "prelaunch-custody.summary.json", public_summary, private_mode=False)
    return {"record_path": str(record_path), "seal": seal, "captured_env": sorted_env, "gopls_path": str(gopls_path), "argv": argv, "run_dir": str(run_dir)}


def write_popen_env_binding(run_dir: pathlib.Path, prelaunch_record: pathlib.Path, captured_env: Dict[str, str], *, test_only: bool, child_observed_env: Optional[Dict[str, str]] = None) -> pathlib.Path:
    prelaunch = load_json(prelaunch_record)
    env_digest = digest_jsonable({k: captured_env[k] for k in sorted(captured_env)})
    binding = {
        "schema": "lsp-trace.adr0011.calls.go.gopls.incoming.popen-env-binding.v1",
        "run_id": prelaunch["run_id"],
        "source": "env=captured_env",
        "test_only": test_only,
        "prelaunch_record": str(prelaunch_record),
        "prelaunch_complete_env_sha256": prelaunch["launch"]["complete_env_sha256"],
        "env_sha256_passed_to_popen": env_digest,
        "env_passed_to_popen": {k: captured_env[k] for k in sorted(captured_env)},
        "child_observed_env_sha256": digest_jsonable(child_observed_env) if child_observed_env is not None else None,
        "child_observed_env": child_observed_env,
        "equality": {
            "passed_equals_prelaunch_complete_env": captured_env == prelaunch["launch"]["complete_env"],
            "child_observed_equals_passed": (child_observed_env == {k: captured_env[k] for k in sorted(captured_env)}) if child_observed_env is not None else None,
        },
    }
    if not binding["equality"]["passed_equals_prelaunch_complete_env"]:
        raise RuntimeError("captured_env drifted before Popen binding")
    if child_observed_env is not None and not binding["equality"]["child_observed_equals_passed"]:
        raise RuntimeError("child-observed TEST_ONLY env differs from env passed to Popen")
    path = run_dir / "custody" / "popen-env-binding.private.json"
    write_json_atomic(path, binding, private_mode=True)
    return path


def test_only_env_probe(args: argparse.Namespace) -> Dict[str, Any]:
    run_id = make_run_id(args.run_id)
    run_dir = reserve_fresh_run_dir(pathlib.Path(args.run_dir).resolve() if args.run_dir else default_run_dir(run_id).resolve())
    workspace = pathlib.Path(args.workspace).resolve(strict=True)
    cap = capture_prelaunch(run_dir, workspace, args.gopls, run_id)
    captured_env = cap["captured_env"]
    probe = "import os,json; print(json.dumps({k: os.environ[k] for k in sorted(os.environ)}, sort_keys=True, separators=(',', ':')))"
    cp = subprocess.run([sys.executable, "-c", probe], cwd=str(workspace), env=captured_env, text=True, capture_output=True, timeout=30)
    if cp.returncode != 0:
        raise RuntimeError("TEST_ONLY env probe failed: " + cp.stderr)
    child_env = json.loads(cp.stdout)
    binding_path = write_popen_env_binding(run_dir, pathlib.Path(cap["record_path"]), captured_env, test_only=True, child_observed_env=child_env)
    notice = {
        "schema": "lsp-trace.adr0011.calls.go.gopls.incoming.test-only-notice.v1",
        "status": "TEST_ONLY_ENV_PROBE_NOT_DENOMINATOR_EVIDENCE_NO_GOPLS_LAUNCH",
        "may_masquerade_as_full_campaign_evidence": False,
        "gopls_launched": False,
        "target_denominator_evidence": False,
        "binding_private_record_relative": run_relative(run_dir, binding_path),
    }
    write_json_atomic(run_dir / "TEST_ONLY_NOTICE.json", notice)
    return {"status": notice["status"], "run_dir": str(run_dir), "custody_seal": cap["seal"], "popen_env_binding": str(binding_path), "run_id": run_id}


def run_wrapped(args: argparse.Namespace) -> Dict[str, Any]:
    if args.direction != "incoming":
        raise RuntimeError("custody rerun addendum is incoming-only")
    run_id = make_run_id(args.run_id)
    run_dir = reserve_fresh_run_dir(pathlib.Path(args.run_dir).resolve() if args.run_dir else default_run_dir(run_id).resolve())
    workspace = pathlib.Path(args.workspace).resolve(strict=True)
    cap = capture_prelaunch(run_dir, workspace, args.gopls, run_id)
    if args.capture_only:
        notice = {
            "schema": "lsp-trace.adr0011.calls.go.gopls.incoming.test-only-notice.v1",
            "status": "PRELAUNCH_CUSTODY_CAPTURED_TEST_ONLY_NO_GOPLS_LAUNCH",
            "may_masquerade_as_full_campaign_evidence": False,
            "gopls_launched": False,
            "target_denominator_evidence": False,
        }
        write_json_atomic(run_dir / "TEST_ONLY_NOTICE.json", notice)
        return {"status": notice["status"], "run_dir": str(run_dir), "custody_seal": cap["seal"], "run_id": run_id}

    base = load_original_runner()
    base.PACKAGE_DIR = ADDENDUM_DIR
    base.CONTRACT_PATH = ORIGINAL_CONTRACT
    base.REPO_ROOT = REPO_ROOT
    captured_env = cap["captured_env"]
    prelaunch_record = pathlib.Path(cap["record_path"])

    class CustodyLSPClient(base.LSPClient):  # type: ignore[misc]
        def __init__(self, argv, cwd, raw_dir, *, initialize_timeout, request_timeout, max_raw_response_bytes):
            write_popen_env_binding(run_dir, prelaunch_record, captured_env, test_only=False)
            self.proc = subprocess.Popen(argv, cwd=str(cwd), env=captured_env, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            self.raw_dir = raw_dir
            self.next_id = 1
            self.initialize_timeout = float(initialize_timeout)
            self.request_timeout = float(request_timeout)
            self.max_raw_response_bytes = int(max_raw_response_bytes)
            self.open_documents = {}

    def custody_environment(run_dir_arg, workspace_arg, gopls_arg, contract_arg):
        env_public = {
            "schema": "lsp-trace.adr0011.calls.go.gopls.environment.v2.custody-rerun-addendum-public",
            "profile_id": PROFILE_ID,
            "run_namespace": RUN_NAMESPACE,
            "run_id": run_id,
            "workspace_digest": sha256_bytes(str(pathlib.Path(workspace_arg).resolve()).encode("utf-8")),
            "runner_source": {"role_id": "wrapper_source", "sha256": sha256_file(pathlib.Path(__file__).resolve()), "byte_length": byte_len(pathlib.Path(__file__).resolve())},
            "original_runner_source": {"role_id": "original_workload_runner", "sha256": sha256_file(ORIGINAL_RUNNER), "byte_length": byte_len(ORIGINAL_RUNNER)},
            "contract_sha256": sha256_file(ORIGINAL_CONTRACT),
            "gopls_executable": {"role_id": "gopls_executable", "sha256": sha256_file(pathlib.Path(cap["gopls_path"])), "byte_length": byte_len(pathlib.Path(cap["gopls_path"]))},
            "gopls_launch": {"argv_count": len(cap["argv"]), "argv_digest": digest_jsonable(cap["argv"])},
            "private_custody_record_relative": run_relative(pathlib.Path(run_dir_arg), pathlib.Path(cap["record_path"])),
            "environment_digest": cap["seal"]["complete_env_sha256"],
            "prelaunch_custody_seal": cap["seal"],
            "authority": contract_arg["authority"],
        }
        base.write_json(pathlib.Path(run_dir_arg) / "provenance" / "environment.json", env_public)

    base.LSPClient = CustodyLSPClient
    base.capture_environment = custody_environment

    forwarded = argparse.Namespace(**vars(args))
    forwarded.gopls = cap["gopls_path"]
    forwarded.run_dir = str(run_dir)
    forwarded.full_targets = str(TARGET_TSV)
    forwarded.direction = "incoming"
    result = base.run_full_workload(forwarded, dry_run=False)
    result["run_namespace"] = RUN_NAMESPACE
    result["run_id"] = run_id
    result["custody_seal"] = cap["seal"]
    result["producer_disposition"] = DISPOSITION
    return result


def dry_full_plan(args: argparse.Namespace) -> Dict[str, Any]:
    if args.direction != "incoming":
        raise RuntimeError("custody rerun addendum is incoming-only")
    run_id = make_run_id(args.run_id)
    run_dir = reserve_fresh_run_dir(pathlib.Path(args.run_dir).resolve() if args.run_dir else default_run_dir(run_id).resolve())
    base = load_original_runner()
    base.PACKAGE_DIR = ADDENDUM_DIR
    base.CONTRACT_PATH = ORIGINAL_CONTRACT
    base.REPO_ROOT = REPO_ROOT
    forwarded = argparse.Namespace(**vars(args))
    forwarded.run_dir = str(run_dir)
    forwarded.full_targets = str(TARGET_TSV)
    forwarded.direction = "incoming"
    result = base.run_full_workload(forwarded, dry_run=True)
    notice = {
        "schema": "lsp-trace.adr0011.calls.go.gopls.incoming.test-only-notice.v1",
        "status": "TEST_ONLY_DRY_FULL_PLAN_NOT_DENOMINATOR_EVIDENCE",
        "may_masquerade_as_full_campaign_evidence": False,
        "gopls_launched": False,
        "target_denominator_evidence": False,
        "full_campaign_executed": False,
    }
    write_json_atomic(run_dir / "TEST_ONLY_NOTICE.json", notice)
    result["status"] = notice["status"]
    result["test_only_notice"] = notice
    result["run_namespace"] = RUN_NAMESPACE
    result["run_id"] = run_id
    return result



FINAL_DISPOSITIONS = {
    "SUCCEEDED_EMPTY", "SUCCEEDED_NON_EMPTY", "REFUSED_SOURCE_HASH_MISMATCH",
    "REFUSED_PREPARE_EMPTY", "REFUSED_PREPARE_AMBIGUOUS", "REFUSED_SELECTED_ITEM_TARGET_MISMATCH",
    "REFUSED_GOPLS_UNAVAILABLE", "FAILED_LSP_ERROR", "FAILED_TIMEOUT", "FAILED_EXCEPTION",
}
TERMINAL_DISPOSITIONS = FINAL_DISPOSITIONS | {"ABANDONED"}


def _run_dir_for_seal(path: pathlib.Path) -> pathlib.Path:
    run_dir = pathlib.Path(path)
    if not run_dir.is_absolute():
        run_dir = pathlib.Path.cwd() / run_dir
    fd = _validated_run_root_fd(run_dir)
    os.close(fd)
    return run_dir


def _seal_paths(run_dir: pathlib.Path) -> Tuple[pathlib.Path, pathlib.Path]:
    return run_dir.parent / f"{run_dir.name}.final-run.checksums.sha256", run_dir.parent / f"{run_dir.name}.final-run.seal.json"


def inventory_run_artifacts(run_dir: pathlib.Path) -> List[Dict[str, Any]]:
    run_dir = _run_dir_for_seal(run_dir)
    records: List[Dict[str, Any]] = []
    for path in sorted(run_dir.rglob("*")):
        rel = path.relative_to(run_dir).as_posix()
        if path.is_symlink():
            raise RuntimeError(f"symlink forbidden in sealed run artifacts: {rel}")
        if path.is_dir():
            continue
        if not path.is_file():
            raise RuntimeError(f"nonregular file forbidden in sealed run artifacts: {rel}")
        resolve_run_relative(run_dir, rel, require_file=True)
        records.append({"path": rel, "byte_length": byte_len_run_relative(run_dir, rel), "sha256": sha256_run_relative(run_dir, rel)})
    return records


def _load_json_if_exists(run_dir: pathlib.Path, rel: str) -> Any:
    try:
        return load_run_relative_json(run_dir, rel)
    except Exception as exc:
        raise RuntimeError(f"missing/nonregular required run artifact before read: {rel}") from exc


def _ledger_final_counts(run_dir: pathlib.Path, rel: str, *, expected_target_count: int, exact_ordinals: bool) -> Dict[str, Any]:
    by: Dict[int, List[Dict[str, Any]]] = {}
    for line_no, line in enumerate(read_run_relative_text(run_dir, rel, encoding="utf-8").splitlines(), 1):
        if not line.strip():
            continue
        event = json.loads(line)
        if event.get("direction") != "incoming":
            raise RuntimeError(f"non-incoming event in incoming seal ledger at line {line_no}")
        by.setdefault(int(event["ordinal"]), []).append(event)
    if len(by) != expected_target_count:
        raise RuntimeError(f"final ordinal count mismatch: {len(by)} != {expected_target_count}")
    if exact_ordinals and sorted(by) != list(range(expected_target_count)):
        raise RuntimeError("incoming ledger ordinals are not exact 0..5899")
    counts: Dict[str, int] = {}
    for ordinal, events in by.items():
        open_attempt: Optional[int] = None
        finals: List[str] = []
        final_seen = False
        for idx, event in enumerate(events):
            disp = event.get("disposition")
            attempt = int(event.get("attempt", 0))
            if final_seen:
                raise RuntimeError(f"append after final disposition at ordinal {ordinal} event {idx}")
            if disp == "STARTED":
                if open_attempt is not None:
                    raise RuntimeError(f"retry before terminal at ordinal {ordinal}")
                open_attempt = attempt
            elif disp in TERMINAL_DISPOSITIONS:
                if open_attempt is None:
                    raise RuntimeError(f"terminal without STARTED at ordinal {ordinal}")
                if attempt != open_attempt:
                    raise RuntimeError(f"terminal attempt mismatch at ordinal {ordinal}")
                open_attempt = None
                if disp in FINAL_DISPOSITIONS:
                    finals.append(disp)
                    final_seen = True
                    counts[disp] = counts.get(disp, 0) + 1
            else:
                raise RuntimeError(f"unknown disposition at ordinal {ordinal}: {disp}")
        if open_attempt is not None:
            raise RuntimeError(f"unfinalized open STARTED at ordinal {ordinal}")
        if len(finals) != 1:
            raise RuntimeError(f"ordinal {ordinal} final disposition count {len(finals)}")
    return {"ordinal_count": len(by), "final_disposition_counts": counts, "one_final_disposition_ordinals": len(by)}


def validate_run_for_seal(run_dir: pathlib.Path, *, test_only_synthetic: bool = False) -> Dict[str, Any]:
    run_dir = _run_dir_for_seal(run_dir)
    plan = _load_json_if_exists(run_dir, "full-plan.json")
    if plan.get("direction") != "incoming":
        raise RuntimeError("sealed run must be incoming-only")
    if test_only_synthetic:
        expected_target_count = int(plan.get("target_count"))
        exact_ordinals = False
    else:
        expected_target_count = EXPECTED["target_count"]
        exact_ordinals = True
        if plan.get("mode") != "RUN_FULL":
            raise RuntimeError("full-run seal requires RUN_FULL plan")
        for key in ("target_count", "batch_size", "batch_count"):
            if plan.get(key) != EXPECTED[key]:
                raise RuntimeError(f"sealed full-run plan invariant drift: {key}")
        if plan.get("target_tsv_sha256") != EXPECTED["target_tsv_sha256"]:
            raise RuntimeError("sealed full-run target TSV drift")
    required = [
        "custody/prelaunch-custody.private.json",
        "custody/prelaunch-custody.seal.json",
        "custody/prelaunch-custody.summary.json",
        "custody/popen-env-binding.private.json",
        "full-plan.json",
        "ledgers/incoming.attempts.jsonl",
        "verification.json",
        "provenance/environment.json",
    ]
    for rel in required:
        resolve_run_relative(run_dir, rel, require_file=True)
    resolve_run_relative(run_dir, "batches", require_file=False)
    resolve_run_relative(run_dir, "checkpoints", require_file=False)
    resolve_run_relative(run_dir, "raw", require_file=False)
    batch_files = sorted((run_dir / "batches").glob("batch-*.manifest.json"))
    checkpoint_files = sorted((run_dir / "checkpoints").glob("batch-*.checkpoint.json"))
    for bf in batch_files:
        resolve_run_relative(run_dir, bf.relative_to(run_dir).as_posix(), require_file=True)
    for cf in checkpoint_files:
        resolve_run_relative(run_dir, cf.relative_to(run_dir).as_posix(), require_file=True)
    if len(batch_files) != int(plan.get("batch_count")) or len(checkpoint_files) != int(plan.get("batch_count")):
        raise RuntimeError("batch/checkpoint count does not match plan")
    raw_files = []
    for p in (run_dir / "raw").rglob("*"):
        if p.is_symlink():
            raise RuntimeError(f"symlink forbidden in raw artifacts: {p.relative_to(run_dir).as_posix()}")
        if p.is_file():
            resolve_run_relative(run_dir, p.relative_to(run_dir).as_posix(), require_file=True)
            raw_files.append(p)
    if not raw_files:
        raise RuntimeError("sealed full run must include raw artifacts")
    verification = _load_json_if_exists(run_dir, "verification.json")
    if not test_only_synthetic and verification.get("status") != "RUN_VERIFICATION_PASSED":
        raise RuntimeError("full-run verification did not pass")
    ledger_counts = _ledger_final_counts(run_dir, "ledgers/incoming.attempts.jsonl", expected_target_count=expected_target_count, exact_ordinals=exact_ordinals)
    return {"plan": plan, "verification_status": verification.get("status"), "ledger_counts": ledger_counts}


def finalize_run(run_dir: pathlib.Path, *, test_only_synthetic: bool = False) -> Dict[str, Any]:
    run_dir = _run_dir_for_seal(run_dir)
    checksums_path, seal_path = _seal_paths(run_dir)
    if checksums_path.exists() or seal_path.exists() or checksums_path.is_symlink() or seal_path.is_symlink():
        raise RuntimeError("final seal/checksums already exist; refusing overwrite")
    validation = validate_run_for_seal(run_dir, test_only_synthetic=test_only_synthetic)
    inventory = inventory_run_artifacts(run_dir)
    checksums_body = "".join(f"{rec['sha256']}  {rec['path']}\n" for rec in inventory)
    tmp_checksums = checksums_path.with_name(checksums_path.name + ".tmp")
    tmp_checksums.write_text(checksums_body, encoding="utf-8")
    os.replace(tmp_checksums, checksums_path)
    seal = {
        "schema": "lsp-trace.adr0011.calls.go.gopls.incoming.final-run-seal.v1",
        "profile_id": PROFILE_ID,
        "binds_to_generic_profile_id": GENERIC_PROFILE_ID,
        "run_namespace": RUN_NAMESPACE,
        "producer_disposition": DISPOSITION,
        "authority": 0,
        "production_admission": "REFUSED_NOT_REQUESTED_NOT_GRANTED",
        "test_only_synthetic": test_only_synthetic,
        "run_dir_name": run_dir.name,
        "run_dir_sha256": sha256_bytes(str(run_dir).encode("utf-8")),
        "contract_sha256": sha256_file(ORIGINAL_CONTRACT),
        "runner_sha256": sha256_file(ORIGINAL_RUNNER),
        "target_tsv_sha256": sha256_file(TARGET_TSV),
        "custody_private_sha256": sha256_run_relative(run_dir, "custody/prelaunch-custody.private.json"),
        "popen_env_binding_sha256": sha256_run_relative(run_dir, "custody/popen-env-binding.private.json"),
        "checksums_file": checksums_path.name,
        "checksums_sha256": sha256_run_relative(run_dir.parent, checksums_path.name),
        "artifact_count": len(inventory),
        "artifact_inventory_digest": digest_jsonable(inventory),
        "final_counts": validation["ledger_counts"],
        "plan_digest": digest_jsonable(validation["plan"]),
        "verification_status": validation["verification_status"],
        "sealed_without_self_reference": True,
    }
    tmp_seal = seal_path.with_name(seal_path.name + ".tmp")
    tmp_seal.write_text(json.dumps(seal, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    os.replace(tmp_seal, seal_path)
    return {"status": "FINAL_RUN_SEALED_TEST_ONLY" if test_only_synthetic else "FINAL_RUN_SEALED", "seal": str(seal_path), "checksums": str(checksums_path), "seal_sha256": sha256_run_relative(run_dir.parent, seal_path.name), "checksums_sha256": sha256_run_relative(run_dir.parent, checksums_path.name), "artifact_count": len(inventory)}


def verify_sealed_run(run_dir: pathlib.Path, *, seal_path: Optional[pathlib.Path] = None, test_only_synthetic: bool = False) -> Dict[str, Any]:
    run_dir = _run_dir_for_seal(run_dir)
    checksums_path, default_seal_path = _seal_paths(run_dir)
    seal_path = seal_path or default_seal_path
    if pathlib.Path(seal_path).parent != run_dir.parent:
        raise RuntimeError("seal file must be the expected sidecar under the validated run parent")
    resolve_run_relative(run_dir.parent, pathlib.Path(seal_path).name, require_file=True)
    resolve_run_relative(run_dir.parent, checksums_path.name, require_file=True)
    seal = load_run_relative_json(run_dir.parent, pathlib.Path(seal_path).name)
    if bool(seal.get("test_only_synthetic")) != bool(test_only_synthetic):
        raise RuntimeError("seal synthetic/full mode mismatch")
    validation = validate_run_for_seal(run_dir, test_only_synthetic=test_only_synthetic)
    inventory = inventory_run_artifacts(run_dir)
    expected_body = "".join(f"{rec['sha256']}  {rec['path']}\n" for rec in inventory)
    actual_body = read_run_relative_text(run_dir.parent, checksums_path.name, encoding="utf-8")
    if actual_body != expected_body:
        raise RuntimeError("run checksums mismatch: changed, missing, or extra artifact")
    if sha256_run_relative(run_dir.parent, checksums_path.name) != seal.get("checksums_sha256"):
        raise RuntimeError("checksums file digest does not match seal")
    if digest_jsonable(inventory) != seal.get("artifact_inventory_digest"):
        raise RuntimeError("artifact inventory digest mismatch")
    if len(inventory) != seal.get("artifact_count"):
        raise RuntimeError("artifact count mismatch")
    if validation["ledger_counts"] != seal.get("final_counts"):
        raise RuntimeError("final count mismatch")
    return {"status": "FINAL_RUN_SEAL_VERIFIED_TEST_ONLY" if test_only_synthetic else "FINAL_RUN_SEAL_VERIFIED", "artifact_count": len(inventory), "checksums_sha256": sha256_run_relative(run_dir.parent, checksums_path.name), "seal_sha256": sha256_run_relative(run_dir.parent, pathlib.Path(seal_path).name)}


def main(argv: Optional[List[str]] = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--capture-only", action="store_true", help="TEST_ONLY: capture and seal prelaunch custody without launching gopls")
    ap.add_argument("--test-only-env-probe", action="store_true", help="TEST_ONLY: capture custody and prove Popen env equality with a Python child, not gopls")
    ap.add_argument("--dry-full-plan", action="store_true", help="TEST_ONLY: write incoming 5900-target dry plan inside addendum package; no gopls launch")
    ap.add_argument("--run-full", action="store_true", help="after independent review only: run full incoming workload with custody-complete prelaunch capture")
    ap.add_argument("--finalize-run", metavar="RUN_DIR", help="seal a completed generated run without self-reference")
    ap.add_argument("--verify-full-run", metavar="RUN_DIR", help="verify a completed sealed run")
    ap.add_argument("--seal", default=None, help="explicit seal path for --verify-full-run")
    ap.add_argument("--test-only-synthetic-seal", action="store_true", help="allow tiny TEST_ONLY synthetic seal fixtures; never denominator evidence")
    ap.add_argument("--direction", choices=["incoming"], default="incoming")
    ap.add_argument("--workspace", default=None, help="exact source workspace for rerun; required for capture/dry/run modes")
    ap.add_argument("--gopls", default="gopls", help="gopls executable name or path resolved with captured PATH")
    ap.add_argument("--run-id", default=None, help="fresh run id; defaults to timestamped custody-rerun id")
    ap.add_argument("--run-dir", default=None, help="output directory confined under this addendum package")
    args = ap.parse_args(argv)
    modes = [args.capture_only, args.test_only_env_probe, args.dry_full_plan, args.run_full, bool(args.finalize_run), bool(args.verify_full_run)]
    if sum(1 for x in modes if x) != 1:
        ap.error("choose exactly one of --capture-only, --test-only-env-probe, --dry-full-plan, --run-full, --finalize-run, or --verify-full-run")
    if (args.capture_only or args.test_only_env_probe or args.dry_full_plan or args.run_full) and not args.workspace:
        ap.error("--workspace is required for capture/dry/run modes")
    try:
        if args.finalize_run:
            result = finalize_run(pathlib.Path(args.finalize_run), test_only_synthetic=args.test_only_synthetic_seal)
        elif args.verify_full_run:
            result = verify_sealed_run(pathlib.Path(args.verify_full_run), seal_path=pathlib.Path(args.seal) if args.seal else None, test_only_synthetic=args.test_only_synthetic_seal)
        elif args.dry_full_plan:
            result = dry_full_plan(args)
        elif args.test_only_env_probe:
            result = test_only_env_probe(args)
        else:
            result = run_wrapped(args)
        print(json.dumps(result, indent=2, sort_keys=True))
        return 0
    except Exception as exc:
        print(json.dumps({"status": "INCOMING_CUSTODY_RERUN_WRAPPER_FAILED", "error": repr(exc)}, indent=2, sort_keys=True), file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
