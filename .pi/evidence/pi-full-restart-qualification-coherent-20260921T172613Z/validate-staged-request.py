#!/usr/bin/env python3
"""Fail-closed validation for the ADR 0007 staged qualification request."""

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path

EXPECTED = {
    "sources": [
        "internal/censuscontinuation/pipeline.go",
        "internal/censuscontinuation/capture.go",
    ],
    "down_depth": 1,
    "up_depth": 0,
    "max_nodes": 10000,
    "batch_targets": 16,
    "timeout_ms": 60000,
    "request_timeout_ms": 30000,
    "continuation": {
        "kind": "ADR_0007_FEATURE_CATALOG",
        "stop_after": "DESCRIBE_REQUESTS",
    },
}
SEMANTIC_KEYS = frozenset(EXPECTED)


def canonical(value: object) -> bytes:
    return json.dumps(value, sort_keys=True, separators=(",", ":")).encode()


def semantic_request(value: object) -> dict:
    if not isinstance(value, dict):
        raise ValueError("request must be a JSON object")
    missing = sorted(SEMANTIC_KEYS - value.keys())
    if missing:
        raise ValueError("missing/defaulted semantic fields: " + ", ".join(missing))
    sources = value.get("sources")
    if sources == ["."] or (isinstance(sources, list) and "." in sources):
        raise ValueError('repository-wide source "." is forbidden')
    return {key: value[key] for key in EXPECTED}


def load_handoff_request(path: Path) -> object:
    text = path.read_text()
    matches = re.findall(r"```json\s*(\{.*?\})\s*```", text, flags=re.DOTALL)
    if len(matches) != 1:
        raise ValueError(f"HANDOFF must contain exactly one JSON request block; found {len(matches)}")
    return json.loads(matches[0])


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("request", type=Path)
    parser.add_argument("--handoff", type=Path)
    args = parser.parse_args()

    request = json.loads(args.request.read_text())
    actual = semantic_request(request)
    expected_bytes = canonical(EXPECTED)
    actual_bytes = canonical(actual)
    expected_fingerprint = hashlib.sha256(expected_bytes).hexdigest()
    actual_fingerprint = hashlib.sha256(actual_bytes).hexdigest()
    if actual_bytes != expected_bytes:
        raise ValueError(
            "semantic fingerprint mismatch: "
            f"actual=sha256:{actual_fingerprint} expected=sha256:{expected_fingerprint}"
        )
    if args.handoff is not None:
        handoff = semantic_request(load_handoff_request(args.handoff))
        if canonical(handoff) != expected_bytes:
            raise ValueError("HANDOFF semantic fingerprint mismatch")
        if canonical(handoff) != actual_bytes:
            raise ValueError("HANDOFF/request semantic fingerprint mismatch")
    print(f"REQUEST_FINGERPRINT_VALID sha256:{expected_fingerprint}")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError, json.JSONDecodeError) as error:
        print(f"REQUEST_FINGERPRINT_INVALID: {error}", file=sys.stderr)
        raise SystemExit(1)
