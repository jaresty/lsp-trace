#!/usr/bin/env python3
"""Strict read-only preflight over explicitly supplied final qualification inputs."""

import argparse
import hashlib
import json
import stat
import subprocess
import sys
from pathlib import Path


def fail(message: str) -> None:
    raise ValueError(message)


def digest(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def require_file(path: Path, label: str) -> None:
    if not path.is_file() or path.is_symlink():
        fail(f"{label} must be a regular non-symlink file: {path}")


def require_mode(path: Path, expected: int, label: str) -> None:
    actual = stat.S_IMODE(path.stat().st_mode)
    if actual != expected:
        fail(f"{label} mode is {actual:04o}, expected {expected:04o}: {path}")


def walk(value, path="$"):
    yield path, value
    if isinstance(value, dict):
        for key, child in value.items():
            yield from walk(child, f"{path}.{key}")
    elif isinstance(value, list):
        for index, child in enumerate(value):
            yield from walk(child, f"{path}[{index}]")


def diagnostic_sinks(configs) -> list[str]:
    hits = []
    for config_name, value in configs:
        for pointer, item in walk(value):
            leaf = pointer.rsplit(".", 1)[-1].lower()
            disabled = item in (None, False, "", [], {}) or (isinstance(item, str) and item.upper() in {"OFF", "DISABLED", "NONE"})
            if ("diagnostic" in leaf or leaf in {"trace_sink", "event_sink"}) and not disabled:
                hits.append(f"{config_name}:{pointer}={item!r}")
    return hits


def unwrap_session_list(value):
    if isinstance(value, dict) and isinstance(value.get("structuredContent"), dict):
        value = value["structuredContent"]
    if isinstance(value, dict) and isinstance(value.get("result"), dict):
        value = value["result"]
    if not isinstance(value, dict):
        fail("retained session-list result must be an object")
    return value


def normalize_sessions(value):
    result = unwrap_session_list(value)
    raw = result.get("Sessions", result.get("sessions"))
    if not isinstance(raw, list):
        fail("retained session-list result must contain Sessions")
    normalized = []
    for index, record in enumerate(raw):
        if not isinstance(record, dict):
            fail(f"Sessions[{index}] must be an object")
        routing = record.get("routing")
        if routing is None:
            routing = record
        if not isinstance(routing, dict):
            fail(f"Sessions[{index}] routing must be an object")
        alias = routing.get("alias")
        readiness = routing.get("readiness")
        instance = record.get("server_instance", routing.get("server_instance"))
        if not isinstance(alias, str) or not isinstance(readiness, str):
            fail(f"Sessions[{index}] routing must contain alias/readiness")
        if not isinstance(instance, dict):
            fail(f"Sessions[{index}] must retain associated server_instance")
        normalized.append((alias, readiness, instance))
    census = result.get("Census", result.get("census"))
    if not isinstance(census, dict):
        fail("retained session-list result must contain Census")
    workers = census.get("Workers", census.get("workers"))
    if workers != 0:
        fail(f"retained session-list Census.Workers must equal integer zero, got {workers!r}")
    return normalized


def normalized_sha(value):
    if not isinstance(value, str):
        return None
    return value.removeprefix("sha256:").lower()


def parse_pin(spec: str):
    parts = spec.split("=", 2)
    if len(parts) != 3 or not parts[0] or not parts[1] or len(parts[2]) != 64:
        fail("--schema must be LABEL=PATH=64_HEX_SHA256")
    label, raw_path, expected = parts
    try:
        int(expected, 16)
    except ValueError:
        fail(f"invalid SHA-256 for {label}")
    return label, Path(raw_path), expected.lower()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--cli", type=Path, required=True)
    parser.add_argument("--cli-sha256", required=True)
    parser.add_argument("--mcp", type=Path, required=True)
    parser.add_argument("--mcp-sha256", required=True)
    parser.add_argument("--mcp-config", type=Path, required=True)
    parser.add_argument("--mcp-config-sha256", required=True)
    parser.add_argument("--bootstrap-config", type=Path, required=True)
    parser.add_argument("--bootstrap-config-sha256", required=True)
    parser.add_argument("--schema", action="append", default=[], metavar="LABEL=PATH=SHA256")
    parser.add_argument("--root", action="append", type=Path, required=True)
    parser.add_argument("--session-list", type=Path, required=True)
    parser.add_argument("--request", type=Path, required=True)
    parser.add_argument("--validator", type=Path, required=True)
    parser.add_argument("--handoff", type=Path, required=True)
    parser.add_argument("--diagnostics-enabled", action="store_true")
    args = parser.parse_args()

    if len(args.root) < 2:
        fail("at least two final roots are required")
    resolved_roots = [root.resolve(strict=True) for root in args.root]
    if len(set(resolved_roots)) != len(resolved_roots):
        fail("final roots must be pairwise distinct")
    for root in args.root:
        if not root.is_dir() or root.is_symlink():
            fail(f"root must be a non-symlink directory: {root}")
        require_mode(root, 0o700, "root")
        if any(root.iterdir()):
            fail(f"root must be empty: {root}")

    files = [
        ("cli", args.cli, args.cli_sha256),
        ("mcp", args.mcp, args.mcp_sha256),
        ("mcp-config", args.mcp_config, args.mcp_config_sha256),
        ("bootstrap-config", args.bootstrap_config, args.bootstrap_config_sha256),
    ]
    files.extend(parse_pin(spec) for spec in args.schema)
    if not args.schema:
        fail("at least one pinned schema is required")
    for label, path, expected in files:
        require_file(path, label)
        actual = digest(path)
        if actual != normalized_sha(expected):
            fail(f"{label} SHA-256 mismatch: actual={actual} expected={normalized_sha(expected)}")

    require_mode(args.mcp_config, 0o600, "MCP config")
    require_mode(args.bootstrap_config, 0o600, "bootstrap config")
    mcp_config = json.loads(args.mcp_config.read_text())
    bootstrap = json.loads(args.bootstrap_config.read_text())
    sinks = diagnostic_sinks((("mcp-config", mcp_config), ("bootstrap", bootstrap)))
    if sinks and not args.diagnostics_enabled:
        fail("diagnostic sinks are configured without --diagnostics-enabled: " + "; ".join(sinks))

    sessions = normalize_sessions(json.loads(args.session_list.read_text()))
    for required in ("project", "nais-discovery-cue"):
        matches = [entry for entry in sessions if entry[0] == required]
        if len(matches) != 1:
            fail(f"expected exactly one session alias {required}; found {len(matches)}")
        if matches[0][1] != "READY":
            fail(f"session {required} readiness is {matches[0][1]!r}, expected 'READY'")
    expected_mcp = normalized_sha(args.mcp_sha256)
    for alias, _, instance in sessions:
        observed = normalized_sha(instance.get("executable_sha256"))
        if observed != expected_mcp:
            fail(f"session {alias} server_instance executable_sha256 mismatch: actual={observed!r} expected={expected_mcp}")

    command = [sys.executable, str(args.validator), str(args.request), "--handoff", str(args.handoff)]
    checked = subprocess.run(command, check=False, capture_output=True, text=True)
    expected_line = "REQUEST_FINGERPRINT_VALID sha256:246a63cddcc552144d60578a4f127e34eddb970f496f863cd6a3509011783cb9"
    if checked.returncode != 0 or checked.stdout.strip() != expected_line or checked.stderr:
        fail("request validator did not produce the exact expected fingerprint verdict")

    print("PREFLIGHT_VALID")
    print(expected_line)
    print("REQUIRED_SESSIONS_READY project nais-discovery-cue")
    print(f"ADDITIONAL_SESSIONS_PERMITTED {len(sessions) - 2}")
    print(f"SERVER_INSTANCE_MCP_DIGEST_VALID {len(sessions)}")
    print("WORKERS_ZERO retained_pre_result_session_list")
    print("DIAGNOSTIC_SINKS " + ("EXPLICITLY_ENABLED" if sinks else "NONE"))
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError, json.JSONDecodeError) as error:
        print(f"PREFLIGHT_INVALID: {error}", file=sys.stderr)
        raise SystemExit(1)
