#!/usr/bin/env python3
"""Candidate V2 selector vectors only; never issues, admits or replays evidence."""
import hashlib
import json
from pathlib import Path
import struct
import sys

ROOT = Path(__file__).resolve().parents[1]
ORIGINALS = ROOT / "docs/qualification/originals"
OUTPUT = ORIGINALS / "generic-lsp-v2-selector-vectors.json"
PREFIX = "ADR0011-GENERIC-EXACT/2"


def sha(data):
    return hashlib.sha256(data).hexdigest()


def field(value):
    data = value.encode("utf-8") if isinstance(value, str) else value
    return struct.pack(">Q", len(data)) + data


def canonical(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"), allow_nan=False).encode()


def selector(method, role, artifact, schema, transport, policy, *, version="buffer:v1", line=0, ordinal=None, schema_override=None, policy_override=None):
    policy_name = "GENERIC_LSP_REFERENCES_EXACT_V2" if method == "textDocument/references" else "GENERIC_LSP_DEFINITION_EXACT_V2"
    parts = [PREFIX, policy_name, method, role, sha(schema_override if schema_override is not None else schema),
             sha(transport), sha(policy_override if policy_override is not None else policy), "s", "1", sha(artifact)]
    if role == "QUERY":
        parts.extend(["file:///a", version, sha(b"a"), "utf-16", str(line), "0"])
    elif role == "TARGET_EVENTS":
        parts.extend(["observed-key-1", sha(b"[]"), str(ordinal)])
    return "sha256:" + sha(b"".join(map(field, parts)))


def vectors():
    schema = (ORIGINALS / "adr0011-generic-envelope-v2.schema.json").read_bytes()
    transport = (ORIGINALS / "generic-lsp-exact-transport-v2.json").read_bytes()
    policies = {method: (ORIGINALS / f"generic-lsp-{method}-exact-v2.json").read_bytes()
                for method in ("references", "definition")}
    query = canonical({"role": "QUERY", "uri": "file:///a", "version": "buffer:v1", "line": 0, "character": 0})
    event = canonical({"role": "TARGET_EVENTS", "target_uri": "file:///x"})
    result = {"domain": PREFIX, "schema_sha256": sha(schema), "transport_sha256": sha(transport),
              "policy_sha256": {k: sha(v) for k, v in policies.items()},
              "query_artifact_utf8_hex": query.hex(), "event_artifact_utf8_hex": event.hex(), "selectors": {}}
    for suffix, method in (("references", "textDocument/references"), ("definition", "textDocument/definition")):
        policy = policies[suffix]
        for name, role, artifact, options in (
            ("query_version1_line0", "QUERY", query, {}),
            ("query_version2_line0", "QUERY", query, {"version": "buffer:v2"}),
            ("query_version1_line1", "QUERY", query, {"line": 1}),
            ("event_ordinal0", "TARGET_EVENTS", event, {"ordinal": 0}),
            ("event_ordinal1", "TARGET_EVENTS", event, {"ordinal": 1}),
            ("query_schema_substitution", "QUERY", query, {"schema_override": schema + b"\n"}),
            ("query_policy_substitution", "QUERY", query, {"policy_override": policy + b"\n"}),
        ):
            result["selectors"][suffix + "_" + name] = selector(method, role, artifact, schema, transport, policy, **options)
    assert len(set(result["selectors"].values())) == len(result["selectors"])
    return canonical(result) + b"\n"


if __name__ == "__main__":
    encoded = vectors()
    if sys.argv[1:] == ["--write"]:
        with OUTPUT.open("xb") as stream:
            stream.write(encoded)
    elif sys.argv[1:] == ["--check"]:
        if OUTPUT.read_bytes() != encoded:
            raise SystemExit("V2 selector vector mismatch")
    else:
        raise SystemExit("usage: adr0011-generic-v2-vectors.py --write|--check")
    print(f"{OUTPUT.relative_to(ROOT)} sha256:{sha(encoded)}")
