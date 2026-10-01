#!/usr/bin/env python3
"""Proposal-only independent selector vectors; never issues an ADR0011 original."""
import hashlib
import json
from pathlib import Path
import struct
import sys

ROOT = Path(__file__).resolve().parents[1]
ORIGINALS = ROOT / "docs/qualification/originals"
OUT = ORIGINALS / "generic-lsp-final-selector-vectors.json"
SCHEMA = (ORIGINALS / "adr0011-generic-envelope-v1.schema.json").read_bytes()
TRANSPORT = (ORIGINALS / "generic-lsp-exact-transport-v1.json").read_bytes()
REFERENCES = (ORIGINALS / "generic-lsp-references-exact-v1.json").read_bytes()
PREFIX = "ADR0011-GENERIC-EXACT/1"


def digest(data):
    return hashlib.sha256(data).hexdigest()


def field(value):
    data = value.encode("utf-8") if isinstance(value, str) else value
    return struct.pack(">Q", len(data)) + data


def canonical(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"), allow_nan=False).encode("utf-8")


def selector(role, artifact, *, version="buffer:v1", line=0, ordinal=None, schema_hash=None, policy_hash=None):
    method = "textDocument/references"
    uri = "file:///a"
    source_hash = digest(b"a")
    parts = [PREFIX, "GENERIC_LSP_REFERENCES_EXACT_V1", method, role,
             schema_hash or digest(SCHEMA), digest(TRANSPORT), policy_hash or digest(REFERENCES),
             "s", "1", digest(artifact)]
    if role == "QUERY":
        parts.extend([uri, version, source_hash, "utf-16", str(line), "0"])
    if role == "TARGET_EVENTS":
        parts.extend(["observed-key-1", digest(b"[]"), str(ordinal)])
    return "sha256:" + digest(b"".join(map(field, parts)))


def no_duplicate(raw):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError("duplicate decoded key")
            result[key] = value
        return result
    return json.loads(raw, object_pairs_hook=unique)


query = canonical({"role": "QUERY", "uri": "file:///a", "version": "buffer:v1", "line": 0, "character": 0})
event = canonical({"role": "TARGET_EVENTS", "target_uri": "file:///x"})
unicode_bytes = canonical({"\ue000": "p", "\U0001f600": "s"})
assert unicode_bytes == '{"\ue000":"p","\U0001f600":"s"}'.encode("utf-8")
try:
    no_duplicate('{"uri":1,"\\u0075ri":2}')
    raise AssertionError("duplicate key was accepted")
except ValueError as e:
    assert str(e) == "duplicate decoded key"

vectors = {
    "domain": PREFIX,
    "schema_sha256": digest(SCHEMA),
    "transport_sha256": digest(TRANSPORT),
    "references_sha256": digest(REFERENCES),
    "query_artifact_utf8_hex": query.hex(),
    "event_artifact_utf8_hex": event.hex(),
    "unicode_canonical_utf8_hex": unicode_bytes.hex(),
    "escaped_duplicate_json": '{"uri":1,"\\u0075ri":2}',
    "selectors": {
        "query_version1_line0": selector("QUERY", query),
        "query_version2_line0": selector("QUERY", query, version="buffer:v2"),
        "query_version1_line1": selector("QUERY", query, line=1),
        "event_ordinal0": selector("TARGET_EVENTS", event, ordinal=0),
        "event_ordinal1": selector("TARGET_EVENTS", event, ordinal=1),
        "query_schema_substitution": selector("QUERY", query, schema_hash=digest(SCHEMA + b"\n")),
        "query_policy_substitution": selector("QUERY", query, policy_hash=digest(REFERENCES + b"\n")),
    },
}
assert len(set(vectors["selectors"].values())) == len(vectors["selectors"])
encoded = canonical(vectors) + b"\n"
if sys.argv[1:] == ["--write"]:
    if OUT.exists() and OUT.read_bytes() != encoded:
        raise SystemExit("existing vector file differs; do not overwrite without review")
    OUT.write_bytes(encoded)
elif sys.argv[1:] == ["--check"]:
    if OUT.read_bytes() != encoded:
        raise SystemExit("Python selector fixture mismatch")
else:
    raise SystemExit("usage: adr0011-generic-final-vectors.py --write|--check")
print(f"{OUT.relative_to(ROOT)} sha256:{digest(encoded)}")
