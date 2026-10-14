#!/usr/bin/env python3
"""Private ADR0011 Go/gopls CALLS workload evidence runner.

This is an evidence-only private tool confined to this package. It does not
grant production admission, Manager custody, profileGO, or completeness.
"""
from __future__ import annotations

import argparse
import csv
import hashlib
import json
import os
import pathlib
import select
import shutil
import subprocess
import sys
import time
from dataclasses import dataclass
from typing import Any, Dict, List, Optional, Tuple

PACKAGE_DIR = pathlib.Path(__file__).resolve().parent
REPO_ROOT = PACKAGE_DIR.parents[3]
CONTRACT_PATH = PACKAGE_DIR / "contract.json"

SEMANTIC_INVARIANTS = {
    "max_raw_response_bytes": 33554432,
    "request_timeout_seconds": 30,
    "initialize_timeout_seconds": 60,
    "smoke_max_targets_hard": 5,
    "batch_size": 50,
    "target_count": 5900,
    "batch_count": 118,
}

TERMINAL_DISPOSITIONS = {
    "ABANDONED",
    "SUCCEEDED_EMPTY",
    "SUCCEEDED_NON_EMPTY",
    "REFUSED_SOURCE_HASH_MISMATCH",
    "REFUSED_PREPARE_EMPTY",
    "REFUSED_PREPARE_AMBIGUOUS",
    "REFUSED_SELECTED_ITEM_TARGET_MISMATCH",
    "REFUSED_GOPLS_UNAVAILABLE",
    "FAILED_LSP_ERROR",
    "FAILED_TIMEOUT",
    "FAILED_EXCEPTION",
}
FINAL_ORDINAL_DISPOSITIONS = TERMINAL_DISPOSITIONS - {"ABANDONED"}


class ContractError(RuntimeError):
    pass


class LSPError(RuntimeError):
    pass


class LSPTimeout(LSPError):
    pass


@dataclass(frozen=True)
class Target:
    ordinal: int
    id: str
    workspace_revision: str
    source_path: str
    source_sha256: str
    byte_start: int
    byte_end: int
    start_line: int
    start_utf16: int
    end_line: int
    end_utf16: int
    package: str
    receiver: str
    symbol: str
    binding_mode: str


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def sha256_file(path: pathlib.Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def read_json(path: pathlib.Path) -> Any:
    with path.open("r", encoding="utf-8") as f:
        return json.load(f)


def write_json(path: pathlib.Path, data: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_text(json.dumps(data, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    os.replace(tmp, path)


def append_jsonl(path: pathlib.Path, obj: Dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a", encoding="utf-8") as f:
        f.write(json.dumps(obj, sort_keys=True) + "\n")


def file_uri(path: pathlib.Path) -> str:
    return path.resolve().as_uri()


def load_contract() -> Dict[str, Any]:
    return read_json(CONTRACT_PATH)


def assert_under_package(path: pathlib.Path) -> None:
    resolved = path.resolve()
    try:
        resolved.relative_to(PACKAGE_DIR.resolve())
    except ValueError as exc:
        raise ContractError(f"path escapes package: {resolved}") from exc


def confined_run_dir(path: pathlib.Path) -> pathlib.Path:
    resolved = path.resolve()
    assert_under_package(resolved)
    return resolved


def confined_child(root: pathlib.Path, *parts: str) -> pathlib.Path:
    root_resolved = confined_run_dir(root)
    child = root_resolved.joinpath(*parts)
    child.parent.mkdir(parents=True, exist_ok=True)
    assert_under_package(child.parent)
    return child


def load_targets_tsv(path: pathlib.Path, limit: Optional[int] = None) -> List[Target]:
    targets: List[Target] = []
    with path.open("r", encoding="utf-8", newline="") as f:
        reader = csv.DictReader(f, delimiter="\t")
        for row in reader:
            targets.append(Target(
                ordinal=len(targets),
                id=row["id"],
                workspace_revision=row["workspace_revision"],
                source_path=row["source_path"],
                source_sha256=row["source_sha256"],
                byte_start=int(row["byte_start"]),
                byte_end=int(row["byte_end"]),
                start_line=int(row["start_line"]),
                start_utf16=int(row["start_utf16"]),
                end_line=int(row["end_line"]),
                end_utf16=int(row["end_utf16"]),
                package=row.get("package", ""),
                receiver=row.get("receiver", ""),
                symbol=row.get("symbol", ""),
                binding_mode=row.get("binding_mode", ""),
            ))
            if limit is not None and len(targets) >= limit:
                break
    return targets


def semantic_invariant_errors(contract: Dict[str, Any]) -> List[str]:
    limits = contract.get("limits", {})
    corpus = contract.get("corpus_identity", {})
    full = contract.get("full_workload", {})
    errors: List[str] = []
    checks = {
        "max_raw_response_bytes": limits.get("max_raw_response_bytes"),
        "request_timeout_seconds": limits.get("request_timeout_seconds"),
        "initialize_timeout_seconds": limits.get("initialize_timeout_seconds"),
        "smoke_max_targets_hard": limits.get("smoke_max_targets_hard"),
        "batch_size": limits.get("batch_size"),
        "target_count": corpus.get("target_count"),
        "batch_count": full.get("batch_count"),
    }
    for key, expected in SEMANTIC_INVARIANTS.items():
        if checks.get(key) != expected:
            errors.append(f"semantic invariant drift: {key} expected {expected}, got {checks.get(key)}")
    if full.get("direction_required") is not True:
        errors.append("full workload direction_required must be true")
    if full.get("run_full_requires_explicit_flag") is not True:
        errors.append("full workload must require explicit run-full flag")
    if full.get("uses_smoke_hard_limit") is not False:
        errors.append("full workload must not use smoke hard limit")
    return errors


def target_corpus_tsv_path(contract: Dict[str, Any], explicit: Optional[str] = None) -> pathlib.Path:
    if explicit:
        return pathlib.Path(explicit).resolve()
    return (REPO_ROOT / contract["corpus_identity"]["target_tsv"]["path"]).resolve()


def load_exact_full_targets(contract: Dict[str, Any], explicit: Optional[str] = None) -> List[Target]:
    path = target_corpus_tsv_path(contract, explicit)
    expected_sha = contract["corpus_identity"]["target_tsv"]["sha256"]
    actual_sha = sha256_file(path)
    if actual_sha != expected_sha:
        raise ContractError(f"target TSV sha256 mismatch: {actual_sha}")
    targets = load_targets_tsv(path)
    expected_count = SEMANTIC_INVARIANTS["target_count"]
    if len(targets) != expected_count:
        raise ContractError(f"full target denominator mismatch: expected {expected_count}, got {len(targets)}")
    for expected_ordinal, target in enumerate(targets):
        if target.ordinal != expected_ordinal:
            raise ContractError(f"target order mismatch at ordinal {expected_ordinal}")
    return targets


def digest_jsonable(obj: Any) -> str:
    return sha256_bytes(json.dumps(obj, sort_keys=True, separators=(",", ":")).encode("utf-8"))


def batch_records(targets: List[Target], batch_size: int = 50) -> List[Dict[str, Any]]:
    if batch_size != SEMANTIC_INVARIANTS["batch_size"]:
        raise ContractError(f"batch_size must be {SEMANTIC_INVARIANTS['batch_size']}")
    if len(targets) != SEMANTIC_INVARIANTS["target_count"]:
        raise ContractError("batch planning requires exact 5900-target denominator")
    batches: List[Dict[str, Any]] = []
    for start in range(0, len(targets), batch_size):
        members = targets[start:start + batch_size]
        payload = [{"ordinal": t.ordinal, "id": t.id, "source_path": t.source_path, "symbol": t.symbol, "byte_start": t.byte_start, "byte_end": t.byte_end} for t in members]
        batches.append({
            "batch_index": len(batches),
            "batch_count": SEMANTIC_INVARIANTS["batch_count"],
            "start_ordinal": members[0].ordinal,
            "end_ordinal_inclusive": members[-1].ordinal,
            "target_count": len(members),
            "input_digest": digest_jsonable(payload),
            "target_ids_digest": digest_jsonable([t.id for t in members]),
        })
    if len(batches) != SEMANTIC_INVARIANTS["batch_count"]:
        raise ContractError(f"batch count mismatch: {len(batches)}")
    return batches


def ledger_digest_for_batch(run_dir: pathlib.Path, direction: str, batch: Dict[str, Any], contract: Dict[str, Any]) -> str:
    ledger = run_dir / contract["directions"][direction]["ledger"]
    events = read_ledger_events(ledger)
    start = batch["start_ordinal"]
    end = batch["end_ordinal_inclusive"]
    selected = [e for e in events if start <= int(e.get("ordinal", -1)) <= end]
    return digest_jsonable(selected)


def write_full_plan(run_dir: pathlib.Path, direction: str, targets: List[Target], contract: Dict[str, Any], *, dry_run: bool) -> Dict[str, Any]:
    if direction not in ("incoming", "outgoing"):
        raise ContractError("full workload direction must be exactly incoming or outgoing")
    run_dir = confined_run_dir(run_dir)
    batches = batch_records(targets, int(contract["limits"]["batch_size"]))
    batches_dir = confined_run_dir(run_dir / "batches")
    checkpoints_dir = confined_run_dir(run_dir / "checkpoints")
    batches_dir.mkdir(parents=True, exist_ok=True)
    checkpoints_dir.mkdir(parents=True, exist_ok=True)
    plan = {
        "schema": "lsp-trace.adr0011.calls.go.gopls.full-plan.v1",
        "mode": "DRY_FULL_PLAN" if dry_run else "RUN_FULL",
        "direction": direction,
        "target_count": len(targets),
        "batch_size": contract["limits"]["batch_size"],
        "batch_count": len(batches),
        "authority": 0,
        "production_admission": "REFUSED_NOT_REQUESTED_NOT_GRANTED",
        "target_tsv_sha256": contract["corpus_identity"]["target_tsv"]["sha256"],
        "batches_digest": digest_jsonable(batches),
    }
    write_json(confined_child(run_dir, "full-plan.json"), plan)
    for batch in batches:
        output_digest = sha256_bytes(b"") if dry_run else ledger_digest_for_batch(run_dir, direction, batch, contract)
        manifest = dict(batch)
        manifest.update({"direction": direction, "dry_run": dry_run, "output_digest": output_digest})
        write_json(confined_child(batches_dir, f"batch-{batch['batch_index']:03d}.manifest.json"), manifest)
        checkpoint = {
            "schema": "lsp-trace.adr0011.calls.go.gopls.batch-checkpoint.v1",
            "direction": direction,
            "batch_index": batch["batch_index"],
            "status": "PLANNED" if dry_run else "PENDING_OR_RECORDED",
            "input_digest": batch["input_digest"],
            "output_digest": output_digest,
            "finalized": False if dry_run else None,
        }
        write_json(confined_child(checkpoints_dir, f"batch-{batch['batch_index']:03d}.checkpoint.json"), checkpoint)
    return {"status": "FULL_PLAN_VALIDATED" if dry_run else "FULL_PLAN_WRITTEN", "plan": plan, "run_dir": str(run_dir)}


def verify_contract(package_dir: pathlib.Path = PACKAGE_DIR) -> Dict[str, Any]:
    contract = read_json(package_dir / "contract.json")
    schema = read_json(package_dir / "schemas" / "contract.schema.json")
    errors: List[str] = []

    if contract.get("schema") != "lsp-trace.adr0011.calls.go.gopls.workload_study.contract.v1":
        errors.append("schema mismatch")
    if contract.get("status") != "CONTRACT_AUTHORED_FULL_WORKLOAD_NOT_RUN":
        errors.append("status must not claim workload execution")
    authority = contract.get("authority", {})
    if authority.get("manager_custody_claimed") is not False:
        errors.append("manager custody must be false")
    if authority.get("production_admission") != "REFUSED_NOT_REQUESTED_NOT_GRANTED":
        errors.append("production admission must be refused")
    if authority.get("evidence_authority") != 0:
        errors.append("authority must be 0")
    if authority.get("completeness") != "UNKNOWN":
        errors.append("completeness must be UNKNOWN")

    corpus = contract.get("corpus_identity", {})
    if corpus.get("qualification_corpus_commit") != "7f09e5357b117d20449fc16097631e32d4eead7f":
        errors.append("qualification corpus commit mismatch")
    if corpus.get("source_revision") != "4b910fc0c517cb7801fb51d17d295897230d11fa":
        errors.append("source revision mismatch")
    if corpus.get("source_tree") != "761474bf5d90bbe3468df62b1a68a1fa26dc2c82":
        errors.append("source tree mismatch")
    if corpus.get("target_corpus_manifest_sha256") != "316afe2501aebe4195894d1334be566ae49eb32c88276ebee7b2e4d86062f44d":
        errors.append("target manifest mismatch")
    if corpus.get("target_count") != 5900:
        errors.append("target count must be 5900")
    for forbidden in ("requested_commit_prefix", "source_tree_prefix", "manifest_prefix"):
        if forbidden in corpus:
            errors.append(f"prefix field forbidden: {forbidden}")

    first = contract.get("position_mapping", {}).get("first_target", {})
    if (first.get("byte_start") != 1117 or first.get("symbol_byte_offset") != 1122 or
            first.get("range_start_lsp_position") != {"line": 35, "character": 0} or
            first.get("expected_lsp_position") != {"line": 35, "character": 5}):
        errors.append("first target byte range/symbol mapping contract mismatch")

    dirs = contract.get("directions", {})
    if dirs.get("incoming", {}).get("inference_from_outgoing_permitted") is not False:
        errors.append("incoming must not infer from outgoing")
    if dirs.get("outgoing", {}).get("inference_from_incoming_permitted") is not False:
        errors.append("outgoing must not infer from incoming")

    terminal_contract = set(contract.get("terminal_attempt_dispositions", []))
    final_contract = set(contract.get("final_ordinal_dispositions", []))
    if "STARTED" in terminal_contract or "STARTED" in final_contract:
        errors.append("STARTED cannot be terminal/final")
    if "ABANDONED" in final_contract:
        errors.append("ABANDONED is not a final ordinal disposition")
    if not TERMINAL_DISPOSITIONS.issubset(terminal_contract):
        errors.append("terminal attempt disposition taxonomy incomplete")
    if not FINAL_ORDINAL_DISPOSITIONS.issubset(final_contract):
        errors.append("final ordinal disposition taxonomy incomplete")

    limits = contract.get("limits", {})
    if limits.get("batch_size") != 50:
        errors.append("batch size must be 50")
    if limits.get("existing_ceilings_changed") is not False:
        errors.append("existing ceilings changed")
    if int(limits.get("max_raw_response_bytes", 0)) <= 0:
        errors.append("max_raw_response_bytes must be positive")
    errors.extend(semantic_invariant_errors(contract))

    full = contract.get("full_workload", {})
    if full.get("batch_size") != 50 or full.get("batch_count") != 118:
        errors.append("full workload batch contract must be 50 x 118")
    if full.get("target_count") != 5900:
        errors.append("full workload target_count must be 5900")

    lifecycle = contract.get("document_lifecycle", {})
    if lifecycle.get("did_open_cache") != "PER_URI_VERSION_NO_DUPLICATE_DIDOPEN":
        errors.append("didOpen cache policy missing")
    if lifecycle.get("did_close") != "DETERMINISTIC_ON_SHUTDOWN_OR_BOUNDARY":
        errors.append("didClose lifecycle policy missing")

    smoke = contract.get("smoke", {})
    if smoke.get("full_workload_execution_permitted") is not False:
        errors.append("full workload execution must be forbidden")
    fixture = package_dir / smoke.get("fixture_tsv", "")
    if not fixture.exists():
        errors.append("smoke fixture missing")
    else:
        rows = load_targets_tsv(fixture)
        if not (1 <= len(rows) <= limits.get("smoke_max_targets_hard", 5)):
            errors.append("smoke fixture must contain 1..hard-limit targets")

    if schema.get("$id") != contract.get("schema"):
        errors.append("schema $id must equal contract schema")

    checksums = package_dir / "CHECKSUMS.sha256"
    checksum_verified = False
    if checksums.exists():
        checksum_verified = verify_checksums(checksums, package_dir, errors)

    if errors:
        raise ContractError("; ".join(errors))
    return {
        "status": "CONTRACT_CHECK_PASSED",
        "package": str(package_dir),
        "contract_sha256": sha256_file(package_dir / "contract.json"),
        "schema_sha256": sha256_file(package_dir / "schemas" / "contract.schema.json"),
        "checksums_verified": checksum_verified,
    }


def verify_checksums(checksums: pathlib.Path, package_dir: pathlib.Path, errors: List[str]) -> bool:
    for line in checksums.read_text(encoding="utf-8").splitlines():
        if not line.strip():
            continue
        digest, rel = line.split("  ", 1)
        if rel == "CHECKSUMS.sha256":
            continue
        path = package_dir / rel
        if not path.exists():
            errors.append(f"checksum path missing: {rel}")
            continue
        actual = sha256_file(path)
        if actual != digest:
            errors.append(f"checksum mismatch: {rel}")
    return not errors


def verify_source_bytes(workspace: pathlib.Path, target: Target) -> Tuple[bool, str, bytes]:
    path = workspace / target.source_path
    if not path.exists():
        return False, "missing source file", b""
    data = path.read_bytes()
    digest = sha256_bytes(data)
    return digest == target.source_sha256, digest, data


def byte_offset_to_lsp_utf16(source: bytes, byte_offset: int) -> Dict[str, int]:
    if byte_offset < 0 or byte_offset > len(source):
        raise ContractError(f"byte offset out of range: {byte_offset}")
    try:
        prefix = source[:byte_offset].decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ContractError(f"byte offset does not align to UTF-8 prefix: {byte_offset}") from exc
    line = prefix.count("\n")
    segment = prefix.rsplit("\n", 1)[-1]
    character = len(segment.encode("utf-16-le")) // 2
    return {"line": line, "character": character}


def target_symbol_byte_offset(source: bytes, target: Target) -> int:
    if target.byte_start < 0 or target.byte_end > len(source) or target.byte_start >= target.byte_end:
        raise ContractError(f"invalid target byte range: {target.byte_start}-{target.byte_end}")
    if not target.symbol:
        return target.byte_start
    needle = target.symbol.encode("utf-8")
    window = source[target.byte_start:target.byte_end]
    index = window.find(needle)
    if index < 0:
        raise ContractError(f"target symbol not found in verified byte range: {target.symbol}")
    return target.byte_start + index


def target_lsp_position(workspace: pathlib.Path, target: Target) -> Tuple[Optional[Dict[str, int]], Dict[str, Any]]:
    ok, observed, data = verify_source_bytes(workspace, target)
    proof = {"source_path": target.source_path, "expected_sha256": target.source_sha256, "observed_sha256": observed}
    if not ok:
        return None, proof
    symbol_offset = target_symbol_byte_offset(data, target)
    pos = byte_offset_to_lsp_utf16(data, symbol_offset)
    proof.update({"byte_start": target.byte_start, "symbol_byte_offset": symbol_offset, "lsp_position": pos})
    return pos, proof


def run_command(argv: List[str], cwd: Optional[pathlib.Path] = None, timeout: int = 30) -> Dict[str, Any]:
    started = time.time()
    try:
        cp = subprocess.run(argv, cwd=str(cwd) if cwd else None, text=True, capture_output=True, timeout=timeout)
        return {"argv": argv, "exit_code": cp.returncode, "stdout": cp.stdout, "stderr": cp.stderr, "duration_seconds": round(time.time() - started, 6)}
    except Exception as exc:
        return {"argv": argv, "exception": repr(exc), "duration_seconds": round(time.time() - started, 6)}


def capture_environment(run_dir: pathlib.Path, workspace: pathlib.Path, gopls: str, contract: Dict[str, Any]) -> None:
    gopls_path = shutil.which(gopls) or gopls
    gopls_digest = None
    if os.path.exists(gopls_path) and os.path.isfile(gopls_path):
        try:
            gopls_digest = sha256_file(pathlib.Path(gopls_path))
        except OSError:
            gopls_digest = None
    provenance = {
        "schema": "lsp-trace.adr0011.calls.go.gopls.environment.v1",
        "captured_at_unix": time.time(),
        "workspace": str(workspace.resolve()),
        "runner": str(pathlib.Path(__file__).resolve()),
        "contract_sha256": sha256_file(CONTRACT_PATH),
        "gopls_path": gopls_path,
        "gopls_binary_sha256": gopls_digest,
        "gopls_version": run_command([gopls, "version"], timeout=contract["limits"]["request_timeout_seconds"]),
        "go_version": run_command(["go", "version"], timeout=contract["limits"]["request_timeout_seconds"]),
        "go_env_json": run_command(["go", "env", "-json"], cwd=workspace, timeout=contract["limits"]["request_timeout_seconds"]),
        "authority": contract["authority"],
    }
    write_json(run_dir / "provenance" / "environment.json", provenance)


class LSPClient:
    def __init__(self, argv: List[str], cwd: pathlib.Path, raw_dir: pathlib.Path, *, initialize_timeout: float, request_timeout: float, max_raw_response_bytes: int):
        self.proc = subprocess.Popen(argv, cwd=str(cwd), stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.raw_dir = raw_dir
        self.next_id = 1
        self.initialize_timeout = float(initialize_timeout)
        self.request_timeout = float(request_timeout)
        self.max_raw_response_bytes = int(max_raw_response_bytes)
        self.open_documents: Dict[str, Dict[str, Any]] = {}

    def close(self) -> None:
        if self.proc.poll() is None:
            try:
                self.close_documents()
                self.request("shutdown", None, "control", -1, timeout_seconds=self.request_timeout)
                self.notify("exit", None, "control", -1)
            except Exception:
                pass
            try:
                self.proc.terminate()
                self.proc.wait(timeout=5)
            except Exception:
                self.proc.kill()

    def close_documents(self) -> None:
        for uri in sorted(list(self.open_documents)):
            rec = self.open_documents.pop(uri)
            self.notify("textDocument/didClose", {"textDocument": {"uri": uri}}, rec["direction"], rec["ordinal"])

    def open_document(self, uri: str, text: str, direction: str, ordinal: int) -> Dict[str, Any]:
        digest = sha256_bytes(text.encode("utf-8", errors="surrogateescape"))
        existing = self.open_documents.get(uri)
        if existing and existing["text_sha256"] == digest:
            return {"didOpen_sent": False, "version": existing["version"], "text_sha256": digest}
        if existing:
            self.notify("textDocument/didClose", {"textDocument": {"uri": uri}}, existing["direction"], existing["ordinal"])
            version = existing["version"] + 1
        else:
            version = 1
        self.notify("textDocument/didOpen", {"textDocument": {"uri": uri, "languageId": "go", "version": version, "text": text}}, direction, ordinal)
        self.open_documents[uri] = {"version": version, "text_sha256": digest, "direction": direction, "ordinal": ordinal}
        return {"didOpen_sent": True, "version": version, "text_sha256": digest}

    def _write_message(self, payload: Dict[str, Any], direction: str, ordinal: int, label: str) -> Dict[str, Any]:
        body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
        framed = f"Content-Length: {len(body)}\r\n\r\n".encode("ascii") + body
        if not self.proc.stdin:
            raise LSPError("stdin unavailable")
        self.proc.stdin.write(framed)
        self.proc.stdin.flush()
        return self._record(direction, ordinal, f"{label}.request", framed, payload.get("method"))

    def _read_some(self, fd: int, max_bytes: int, deadline: float, partial: bytes, direction: str, ordinal: int, label: str) -> bytes:
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            self._record(direction, ordinal, f"{label}.response.timeout", partial, None, partial=True, error="timeout")
            raise LSPTimeout("LSP response timeout")
        ready, _, _ = select.select([fd], [], [], remaining)
        if not ready:
            self._record(direction, ordinal, f"{label}.response.timeout", partial, None, partial=True, error="timeout")
            raise LSPTimeout("LSP response timeout")
        chunk = os.read(fd, max_bytes)
        if not chunk:
            self._record(direction, ordinal, f"{label}.response.closed", partial, None, partial=True, error="closed")
            raise LSPError("gopls closed stdout")
        return chunk

    def _read_message(self, direction: str, ordinal: int, label: str, timeout_seconds: float) -> Tuple[Dict[str, Any], Dict[str, Any]]:
        if not self.proc.stdout:
            raise LSPError("stdout unavailable")
        fd = self.proc.stdout.fileno()
        deadline = time.monotonic() + timeout_seconds
        header = b""
        while b"\r\n\r\n" not in header:
            if len(header) >= self.max_raw_response_bytes:
                self._record(direction, ordinal, f"{label}.response.too_large", header, None, partial=True, error="header_exceeds_max_raw_response_bytes")
                raise LSPError("max_raw_response_bytes exceeded before header complete")
            header += self._read_some(fd, 1, deadline, header, direction, ordinal, label)
        headers, _ = header.split(b"\r\n\r\n", 1)
        length = None
        for line in headers.split(b"\r\n"):
            if line.lower().startswith(b"content-length:"):
                length = int(line.split(b":", 1)[1].strip())
        if length is None:
            self._record(direction, ordinal, f"{label}.response.malformed", header, None, partial=True, error="missing_content_length")
            raise LSPError("missing Content-Length")
        if len(header) + length > self.max_raw_response_bytes:
            self._record(direction, ordinal, f"{label}.response.too_large", header, None, partial=True, error="body_exceeds_max_raw_response_bytes")
            raise LSPError("max_raw_response_bytes exceeded by declared body")
        body = b""
        while len(body) < length:
            body += self._read_some(fd, min(65536, length - len(body)), deadline, header + body, direction, ordinal, label)
        framed = header + body
        rec = self._record(direction, ordinal, f"{label}.response", framed, None)
        try:
            parsed = json.loads(body.decode("utf-8"))
        except Exception as exc:
            raise LSPError(f"malformed JSON response: {exc}") from exc
        return parsed, rec

    def _record(self, direction: str, ordinal: int, label: str, data: bytes, method: Optional[str], *, partial: bool = False, error: Optional[str] = None) -> Dict[str, Any]:
        safe_label = label.replace("/", "_")
        out_dir = self.raw_dir / direction / f"{ordinal:05d}"
        out_dir.mkdir(parents=True, exist_ok=True)
        digest = sha256_bytes(data)
        path = out_dir / f"{safe_label}.{digest[:16]}.bin"
        path.write_bytes(data)
        sidecar = {
            "schema": "lsp-trace.adr0011.calls.go.gopls.raw-message.v1",
            "direction": direction,
            "ordinal": ordinal,
            "label": label,
            "method": method,
            "path": str(path),
            "sha256": digest,
            "byte_length": len(data),
            "partial": partial,
        }
        if error:
            sidecar["error"] = error
        write_json(path.with_suffix(path.suffix + ".json"), sidecar)
        return sidecar

    def request(self, method: str, params: Any, direction: str, ordinal: int, *, timeout_seconds: Optional[float] = None) -> Dict[str, Any]:
        msg_id = self.next_id
        self.next_id += 1
        timeout = self.request_timeout if timeout_seconds is None else float(timeout_seconds)
        payload = {"jsonrpc": "2.0", "id": msg_id, "method": method, "params": params}
        self._write_message(payload, direction, ordinal, method.replace("/", "_"))
        while True:
            response, _ = self._read_message(direction, ordinal, method.replace("/", "_"), timeout)
            if response.get("id") == msg_id:
                if "error" in response:
                    raise LSPError(json.dumps(response["error"], sort_keys=True))
                return response.get("result")

    def notify(self, method: str, params: Any, direction: str, ordinal: int) -> None:
        self._write_message({"jsonrpc": "2.0", "method": method, "params": params}, direction, ordinal, method.replace("/", "_"))


def _pos_tuple(pos: Dict[str, int]) -> Tuple[int, int]:
    return int(pos["line"]), int(pos["character"])


def position_in_range(pos: Dict[str, int], rng: Dict[str, Any]) -> bool:
    start = _pos_tuple(rng["start"])
    end = _pos_tuple(rng["end"])
    p = _pos_tuple(pos)
    return start <= p < end


def item_contains_position(item: Dict[str, Any], pos: Dict[str, int]) -> bool:
    for key in ("selectionRange", "range"):
        rng = item.get(key)
        if rng and position_in_range(pos, rng):
            return True
    return False


def select_prepared_item(items: Any, target: Target, uri: str, position: Dict[str, int]) -> Tuple[Optional[Dict[str, Any]], str]:
    if not items:
        return None, "REFUSED_PREPARE_EMPTY"
    candidates = []
    for item in items:
        if item.get("uri") != uri:
            continue
        if target.symbol and item.get("name") != target.symbol:
            continue
        if not item_contains_position(item, position):
            continue
        candidates.append(item)
    if len(candidates) == 1:
        return candidates[0], "OK"
    if len(candidates) == 0:
        return None, "REFUSED_SELECTED_ITEM_TARGET_MISMATCH"
    return None, "REFUSED_PREPARE_AMBIGUOUS"


def ledger_event(direction: str, target: Target, disposition: str, attempt: int, extra: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
    event = {
        "schema": "lsp-trace.adr0011.calls.go.gopls.attempt.v1",
        "direction": direction,
        "ordinal": target.ordinal,
        "target_id": target.id,
        "attempt": attempt,
        "disposition": disposition,
        "timestamp_unix": time.time(),
        "authority": 0,
        "production_admission": "REFUSED_NOT_REQUESTED_NOT_GRANTED",
    }
    if extra:
        event.update(extra)
    return event


def read_ledger_events(ledger: pathlib.Path) -> List[Dict[str, Any]]:
    if not ledger.exists():
        return []
    return [json.loads(line) for line in ledger.read_text(encoding="utf-8").splitlines() if line.strip()]


def prepare_attempt(ledger: pathlib.Path, direction: str, target: Target) -> int:
    target_events = [e for e in read_ledger_events(ledger) if int(e.get("ordinal", -1)) == target.ordinal]
    if any(e.get("disposition") in FINAL_ORDINAL_DISPOSITIONS for e in target_events):
        raise ContractError(f"{direction}:{target.ordinal}: cannot append after final disposition")
    max_attempt = max([int(e.get("attempt", 0)) for e in target_events] or [0])
    open_attempt: Optional[int] = None
    for event in target_events:
        disp = event.get("disposition")
        if disp == "STARTED":
            if open_attempt is not None:
                raise ContractError(f"{direction}:{target.ordinal}: retry without ABANDONED")
            open_attempt = int(event.get("attempt", max_attempt))
        elif disp in TERMINAL_DISPOSITIONS:
            open_attempt = None
    if open_attempt is not None:
        append_jsonl(ledger, ledger_event(direction, target, "ABANDONED", open_attempt, {"reason": "restart_after_open_started"}))
    return max_attempt + 1


def run_smoke(args: argparse.Namespace) -> Dict[str, Any]:
    contract = load_contract()
    workspace = pathlib.Path(args.workspace).resolve()
    run_dir = confined_run_dir(pathlib.Path(args.run_dir))
    if args.max_targets > contract["limits"]["smoke_max_targets_hard"]:
        raise ContractError("smoke max exceeds hard limit")
    targets = load_targets_tsv(pathlib.Path(args.targets), args.max_targets)
    if not targets:
        raise ContractError("no smoke targets")
    directions = ["incoming", "outgoing"] if args.direction == "both" else [args.direction]

    capture_environment(run_dir, workspace, args.gopls, contract)

    client: Optional[LSPClient] = None
    try:
        client = LSPClient(
            [args.gopls, "serve"],
            workspace,
            confined_run_dir(run_dir / "raw"),
            initialize_timeout=contract["limits"]["initialize_timeout_seconds"],
            request_timeout=contract["limits"]["request_timeout_seconds"],
            max_raw_response_bytes=contract["limits"]["max_raw_response_bytes"],
        )
        init = {
            "processId": None,
            "rootUri": file_uri(workspace),
            "capabilities": {"textDocument": {"callHierarchy": {"dynamicRegistration": False}}},
            "workspaceFolders": [{"uri": file_uri(workspace), "name": workspace.name}],
        }
        client.request("initialize", init, "generic", -1, timeout_seconds=contract["limits"]["initialize_timeout_seconds"])
        client.notify("initialized", {}, "generic", -1)

        for direction in directions:
            ledger = run_dir / contract["directions"][direction]["ledger"]
            for target in targets:
                attempt = prepare_attempt(ledger, direction, target)
                append_jsonl(ledger, ledger_event(direction, target, "STARTED", attempt))
                try:
                    position, source_proof = target_lsp_position(workspace, target)
                    if position is None:
                        append_jsonl(ledger, ledger_event(direction, target, "REFUSED_SOURCE_HASH_MISMATCH", attempt, source_proof))
                        continue
                    src = workspace / target.source_path
                    text = src.read_text(encoding="utf-8", errors="surrogateescape")
                    uri = file_uri(src)
                    open_info = client.open_document(uri, text, direction, target.ordinal)
                    prepare = client.request("textDocument/prepareCallHierarchy", {"textDocument": {"uri": uri}, "position": position}, direction, target.ordinal)
                    item, selection = select_prepared_item(prepare, target, uri, position)
                    if item is None:
                        append_jsonl(ledger, ledger_event(direction, target, selection, attempt, {"source_proof": source_proof, "open_info": open_info}))
                        continue
                    method = "callHierarchy/incomingCalls" if direction == "incoming" else "callHierarchy/outgoingCalls"
                    result = client.request(method, {"item": item}, direction, target.ordinal)
                    disposition = "SUCCEEDED_NON_EMPTY" if result else "SUCCEEDED_EMPTY"
                    append_jsonl(ledger, ledger_event(direction, target, disposition, attempt, {"result_count": len(result or []), "source_proof": source_proof, "open_info": open_info}))
                except LSPTimeout as exc:
                    append_jsonl(ledger, ledger_event(direction, target, "FAILED_TIMEOUT", attempt, {"error": str(exc)}))
                except LSPError as exc:
                    append_jsonl(ledger, ledger_event(direction, target, "FAILED_LSP_ERROR", attempt, {"error": str(exc)}))
                except Exception as exc:
                    append_jsonl(ledger, ledger_event(direction, target, "FAILED_EXCEPTION", attempt, {"error": repr(exc)}))
    except FileNotFoundError as exc:
        for direction in directions:
            ledger = run_dir / contract["directions"][direction]["ledger"]
            for target in targets:
                attempt = prepare_attempt(ledger, direction, target)
                append_jsonl(ledger, ledger_event(direction, target, "STARTED", attempt))
                append_jsonl(ledger, ledger_event(direction, target, "REFUSED_GOPLS_UNAVAILABLE", attempt, {"error": repr(exc)}))
    finally:
        if client is not None:
            try:
                client.close()
            except Exception:
                pass

    verification = verify_run(run_dir, directions=directions, ordinals=[t.ordinal for t in targets])
    write_json(confined_child(run_dir, "verification.json"), verification)
    return {"status": "SMOKE_COMPLETED", "run_dir": str(run_dir), "verification": verification}


def run_full_workload(args: argparse.Namespace, *, dry_run: bool) -> Dict[str, Any]:
    contract = load_contract()
    errors = semantic_invariant_errors(contract)
    if errors:
        raise ContractError("; ".join(errors))
    if args.direction not in ("incoming", "outgoing"):
        raise ContractError("--run-full/--dry-full-plan requires --direction incoming or --direction outgoing")
    run_dir = confined_run_dir(pathlib.Path(args.run_dir))
    targets = load_exact_full_targets(contract, args.full_targets)
    plan_result = write_full_plan(run_dir, args.direction, targets, contract, dry_run=dry_run)
    if dry_run:
        return plan_result

    workspace = pathlib.Path(args.workspace).resolve()
    capture_environment(run_dir, workspace, args.gopls, contract)
    client: Optional[LSPClient] = None
    try:
        client = LSPClient(
            [args.gopls, "serve"],
            workspace,
            confined_run_dir(run_dir / "raw"),
            initialize_timeout=contract["limits"]["initialize_timeout_seconds"],
            request_timeout=contract["limits"]["request_timeout_seconds"],
            max_raw_response_bytes=contract["limits"]["max_raw_response_bytes"],
        )
        init = {
            "processId": None,
            "rootUri": file_uri(workspace),
            "capabilities": {"textDocument": {"callHierarchy": {"dynamicRegistration": False}}},
            "workspaceFolders": [{"uri": file_uri(workspace), "name": workspace.name}],
        }
        client.request("initialize", init, "generic", -1, timeout_seconds=contract["limits"]["initialize_timeout_seconds"])
        client.notify("initialized", {}, "generic", -1)
        ledger = run_dir / contract["directions"][args.direction]["ledger"]
        for batch in batch_records(targets, int(contract["limits"]["batch_size"])):
            batch_targets = targets[batch["start_ordinal"]:batch["end_ordinal_inclusive"] + 1]
            for target in batch_targets:
                attempt = prepare_attempt(ledger, args.direction, target)
                append_jsonl(ledger, ledger_event(args.direction, target, "STARTED", attempt, {"batch_index": batch["batch_index"]}))
                try:
                    position, source_proof = target_lsp_position(workspace, target)
                    if position is None:
                        append_jsonl(ledger, ledger_event(args.direction, target, "REFUSED_SOURCE_HASH_MISMATCH", attempt, source_proof))
                        continue
                    src = workspace / target.source_path
                    text = src.read_text(encoding="utf-8", errors="surrogateescape")
                    uri = file_uri(src)
                    open_info = client.open_document(uri, text, args.direction, target.ordinal)
                    prepare = client.request("textDocument/prepareCallHierarchy", {"textDocument": {"uri": uri}, "position": position}, args.direction, target.ordinal)
                    item, selection = select_prepared_item(prepare, target, uri, position)
                    if item is None:
                        append_jsonl(ledger, ledger_event(args.direction, target, selection, attempt, {"source_proof": source_proof, "open_info": open_info, "batch_index": batch["batch_index"]}))
                        continue
                    method = "callHierarchy/incomingCalls" if args.direction == "incoming" else "callHierarchy/outgoingCalls"
                    result = client.request(method, {"item": item}, args.direction, target.ordinal)
                    disposition = "SUCCEEDED_NON_EMPTY" if result else "SUCCEEDED_EMPTY"
                    append_jsonl(ledger, ledger_event(args.direction, target, disposition, attempt, {"result_count": len(result or []), "source_proof": source_proof, "open_info": open_info, "batch_index": batch["batch_index"]}))
                except LSPTimeout as exc:
                    append_jsonl(ledger, ledger_event(args.direction, target, "FAILED_TIMEOUT", attempt, {"error": str(exc), "batch_index": batch["batch_index"]}))
                except LSPError as exc:
                    append_jsonl(ledger, ledger_event(args.direction, target, "FAILED_LSP_ERROR", attempt, {"error": str(exc), "batch_index": batch["batch_index"]}))
                except Exception as exc:
                    append_jsonl(ledger, ledger_event(args.direction, target, "FAILED_EXCEPTION", attempt, {"error": repr(exc), "batch_index": batch["batch_index"]}))
            manifest_path = confined_child(run_dir, "batches", f"batch-{batch['batch_index']:03d}.manifest.json")
            manifest = read_json(manifest_path)
            manifest["output_digest"] = ledger_digest_for_batch(run_dir, args.direction, batch, contract)
            write_json(manifest_path, manifest)
            checkpoint_path = confined_child(run_dir, "checkpoints", f"batch-{batch['batch_index']:03d}.checkpoint.json")
            checkpoint = read_json(checkpoint_path)
            checkpoint.update({"status": "BATCH_RECORDED", "finalized": True, "output_digest": manifest["output_digest"]})
            write_json(checkpoint_path, checkpoint)
    finally:
        if client is not None:
            try:
                client.close()
            except Exception:
                pass
    verification = verify_run(run_dir, directions=[args.direction], ordinals=[t.ordinal for t in targets])
    write_json(confined_child(run_dir, "verification.json"), verification)
    return {"status": "FULL_WORKLOAD_COMPLETED", "run_dir": str(run_dir), "verification": verification, "plan": plan_result["plan"]}


def verify_run(run_dir: pathlib.Path, directions: Optional[List[str]] = None, ordinals: Optional[List[int]] = None) -> Dict[str, Any]:
    run_dir = confined_run_dir(run_dir)
    contract = load_contract()
    directions = directions or ["incoming", "outgoing"]
    errors: List[str] = []
    summary: Dict[str, Any] = {"directions": {}, "errors": errors}

    for direction in directions:
        ledger = run_dir / contract["directions"][direction]["ledger"]
        if not ledger.exists():
            errors.append(f"missing ledger: {direction}")
            continue
        events = read_ledger_events(ledger)
        by_ordinal: Dict[int, List[Dict[str, Any]]] = {}
        for event in events:
            by_ordinal.setdefault(int(event["ordinal"]), []).append(event)
        expected = ordinals if ordinals is not None else sorted(by_ordinal)
        direction_summary = {}
        for ordinal in expected:
            evs = by_ordinal.get(int(ordinal), [])
            expected_attempt = 1
            open_attempt: Optional[int] = None
            terminal_attempts: List[str] = []
            finals: List[str] = []
            final_seen = False
            for idx, event in enumerate(evs):
                disp = event.get("disposition")
                attempt = int(event.get("attempt", 0))
                if final_seen:
                    errors.append(f"{direction}:{ordinal}: append after final disposition at event {idx}: {disp}")
                    continue
                if disp == "STARTED":
                    if open_attempt is not None:
                        errors.append(f"{direction}:{ordinal}: STARTED before terminal for attempt {open_attempt}")
                    if attempt != expected_attempt:
                        errors.append(f"{direction}:{ordinal}: expected STARTED attempt {expected_attempt}, got {attempt}")
                    open_attempt = attempt
                elif disp in TERMINAL_DISPOSITIONS:
                    if open_attempt is None:
                        errors.append(f"{direction}:{ordinal}: terminal without STARTED: {disp}")
                    elif attempt != open_attempt:
                        errors.append(f"{direction}:{ordinal}: terminal attempt {attempt} does not match open attempt {open_attempt}")
                    terminal_attempts.append(disp)
                    open_attempt = None
                    expected_attempt = attempt + 1
                    if disp in FINAL_ORDINAL_DISPOSITIONS:
                        finals.append(disp)
                        final_seen = True
                else:
                    errors.append(f"{direction}:{ordinal}: unknown disposition {disp}")
            if open_attempt is not None:
                errors.append(f"{direction}:{ordinal}: open STARTED without terminal")
            if len(finals) != 1:
                errors.append(f"{direction}:{ordinal}: expected exactly one final ordinal disposition, got {len(finals)}")
            direction_summary[str(ordinal)] = {"events": len(evs), "terminal_attempts": terminal_attempts, "finals": finals}
        summary["directions"][direction] = direction_summary

    summary["status"] = "RUN_VERIFICATION_PASSED" if not errors else "RUN_VERIFICATION_FAILED"
    if errors:
        raise ContractError("; ".join(errors))
    return summary


def main(argv: Optional[List[str]] = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--contract-check", action="store_true", help="verify private contract and package guards")
    parser.add_argument("--verify-run", metavar="RUN_DIR", help="verify append-only run accounting")
    parser.add_argument("--smoke", action="store_true", help="run bounded smoke using real gopls LSP")
    parser.add_argument("--dry-full-plan", action="store_true", help="validate exact 5900-target full plan without acquisition")
    parser.add_argument("--run-full", action="store_true", help="explicitly run the full 5900-target workload; direction required")
    parser.add_argument("--workspace", default=".", help="private source workspace for smoke/full run")
    parser.add_argument("--targets", default=str(PACKAGE_DIR / "fixtures" / "targets.smoke.tsv"), help="target TSV for bounded smoke")
    parser.add_argument("--full-targets", default=None, help="exact full target TSV; defaults to frozen corpus path from contract")
    parser.add_argument("--direction", choices=["incoming", "outgoing", "both"], default="both")
    parser.add_argument("--run-dir", default=str(PACKAGE_DIR / "runs" / f"smoke-{int(time.time())}"))
    parser.add_argument("--max-targets", type=int, default=1)
    parser.add_argument("--gopls", default="gopls")
    args = parser.parse_args(argv)

    try:
        if args.contract_check:
            print(json.dumps(verify_contract(), indent=2, sort_keys=True))
            return 0
        if args.verify_run:
            print(json.dumps(verify_run(pathlib.Path(args.verify_run)), indent=2, sort_keys=True))
            return 0
        if args.dry_full_plan:
            print(json.dumps(run_full_workload(args, dry_run=True), indent=2, sort_keys=True))
            return 0
        if args.run_full:
            print(json.dumps(run_full_workload(args, dry_run=False), indent=2, sort_keys=True))
            return 0
        if args.smoke:
            print(json.dumps(run_smoke(args), indent=2, sort_keys=True))
            return 0
        parser.error("choose --contract-check, --verify-run, --dry-full-plan, --run-full, or --smoke")
    except ContractError as exc:
        print(json.dumps({"status": "CONTRACT_OR_VERIFICATION_FAILED", "error": str(exc)}, indent=2, sort_keys=True), file=sys.stderr)
        return 2
    except Exception as exc:
        print(json.dumps({"status": "RUNNER_FAILED", "error": repr(exc)}, indent=2, sort_keys=True), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
