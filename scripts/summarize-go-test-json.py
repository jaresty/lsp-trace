#!/usr/bin/env python3
"""Summarize one captured `go test -json` stream without rerunning tests."""
import argparse
import datetime as dt
import json
from pathlib import Path


def parse_time(value):
    if not value:
        return None
    return dt.datetime.fromisoformat(value.replace("Z", "+00:00"))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--os", required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--job", required=True)
    parser.add_argument("--shard", required=True)
    parser.add_argument("--exit-code", required=True, type=int)
    args = parser.parse_args()
    events = []
    malformed_lines = 0
    for line in Path(args.input).read_text(errors="replace").splitlines():
        if not line.strip():
            continue
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            malformed_lines += 1
            continue
        if isinstance(event, dict):
            events.append(event)
        else:
            malformed_lines += 1
    cached = set()
    timed_out = set()
    for event in events:
        package = event.get("Package")
        output = event.get("Output", "")
        if package and "(cached)" in output:
            cached.add(package)
        if package and ("test timed out" in output.lower() or "timeout" in output.lower() and "panic" in output.lower()):
            timed_out.add(package)
    terminal = {}
    for event in events:
        package = event.get("Package")
        action = event.get("Action")
        if package and action in {"pass", "fail", "skip"} and "Test" not in event:
            terminal[package] = {
                "package": package,
                "status": "timeout" if package in timed_out else action,
                "elapsed": event.get("Elapsed"),
                "cached": package in cached,
            }
    times = [parse_time(e.get("Time")) for e in events]
    times = [value for value in times if value is not None]
    total = (max(times) - min(times)).total_seconds() if len(times) >= 2 else None
    status = "timeout" if timed_out else ("failure" if args.exit_code else "success")
    summary = {
        "os": args.os,
        "commit": args.commit,
        "job": args.job,
        "shard": args.shard,
        "status": status,
        "exit_code": args.exit_code,
        "packages": [terminal[p] for p in sorted(terminal)],
        "shard_total_seconds": total,
        "setup_seconds": None,
        "cached": bool(cached),
        "malformed_lines": malformed_lines,
        "partial": malformed_lines > 0,
    }
    Path(args.output).write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n")


if __name__ == "__main__":
    main()
