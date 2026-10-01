#!/usr/bin/env python3
"""Offline candidate SOURCE preimage vectors only; no selector issuer or replay path."""
import argparse
import hashlib
import json
import struct
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
ORIGINALS = ROOT / "docs/qualification/originals"
OUT = ORIGINALS / "generic-lsp-source-selector-v2-proposed-vectors.json"
PINS = {
    "schema": ("adr0011-generic-envelope-v2.schema.json", "f843389f811aea940ed5bf1d595f03dcb6de4fa97c4dafafc41ad83b5c1c7f8e", 13243),
    "transport": ("generic-lsp-exact-transport-v2.json", "0519ef89b76d966141bece6ca24d9e186b6424113339a39fce68ae3f78ffa67f", 1206),
    "references": ("generic-lsp-references-exact-v2.json", "cd92d4167abc951432804991e576a52a43236f9061dfe21c412410a6a851ecc5", 724),
    "definition": ("generic-lsp-definition-exact-v2.json", "f8fa1a0cb9f1379d69e347ef9573f6356d28f448f0cfb2529d5cd5747751a9af", 633),
}


def sha(value):
    return hashlib.sha256(value).hexdigest()


def lp(value):
    if isinstance(value, str):
        value = value.encode("utf-8")
    return struct.pack(">Q", len(value)) + value


def original(key):
    name, expected, size = PINS[key]
    data = (ORIGINALS / name).read_bytes()
    if len(data) != size or sha(data) != expected:
        raise ValueError("accepted V2 original changed: " + key)
    return data


def tx(identity):
    session, generation, transaction = identity
    if not isinstance(session, str) or not isinstance(transaction, str) or not session or not transaction or type(generation) is not int or not (1 <= generation <= (1 << 64) - 1):
        raise ValueError("invalid independently held transaction")
    return "sha256:" + sha(b"".join(map(lp, ("ADR0011-GENERIC-TRANSACTION/1", session, str(generation), transaction))))


def selector(method, entry, *, schema=None, policy=None):
    schema = original("schema") if schema is None else schema
    transport = original("transport")
    policy = original(method) if policy is None else policy
    uri = bytes.fromhex(entry["uri_hex"])
    version = None if entry["version_hex"] is None else bytes.fromhex(entry["version_hex"])
    content = None if entry["content_hex"] is None else bytes.fromhex(entry["content_hex"])
    if not uri or version is None or content is None:
        raise ValueError("absent URI/version/source bytes")
    # Validate UTF-8, then keep the *original* bytes for the selector; never normalize.
    uri.decode("utf-8")
    version.decode("utf-8")
    if entry["custody"] not in ("OWNER_BUFFER", "MANAGED_VIRTUAL", "CLEAN_REGISTERED_WORKTREE", "IMMUTABLE_SOURCE_SNAPSHOT"):
        raise ValueError("unknown SOURCE custody")
    identity = entry["identity"]
    transaction_id = tx(identity)
    artifact = b"".join(map(lp, ("ADR0011-GENERIC-SOURCE-ARTIFACT/1", transaction_id, uri, "present", version,
                                  str(len(content)), sha(content), entry["custody"])))
    method_name = "textDocument/" + method
    profile = "GENERIC_LSP_" + method.upper() + "_EXACT_V2"
    preimage = b"".join(map(lp, ("ADR0011-GENERIC-EXACT/2", profile, method_name, "SOURCE", sha(schema), sha(transport),
                                   sha(policy), identity[0], str(identity[1]), sha(artifact), transaction_id)))
    return "sha256:" + sha(preimage)


def fixture():
    base = {"identity": ["s", 1, "tx-1"], "uri_hex": b"file:///query".hex(), "version_hex": b"buffer:v1".hex(),
            "content_hex": b"alpha\n".hex(), "custody": "OWNER_BUFFER"}
    target = {**base, "uri_hex": b"file:///target".hex(), "version_hex": "", "content_hex": "", "custody": "MANAGED_VIRTUAL"}
    cases = {"query": base, "target_present_empty_version_and_content": target}
    def alter(name, **changes):
        cases[name] = {**base, **changes}
    alter("uri_byte", uri_hex=b"file:///querY".hex())
    alter("version_byte", version_hex=b"buffer:v2".hex())
    alter("content_byte", content_hex=b"alphA\n".hex())
    alter("custody", custody="CLEAN_REGISTERED_WORKTREE")
    alter("transaction", identity=["s", 1, "tx-2"])
    alter("session", identity=["s2", 1, "tx-1"])
    alter("generation", identity=["s", 2, "tx-1"])
    # The transaction case retains exactly the same URI/version/content/custody.
    rejected = {
        "absent_version": {**base, "version_hex": None},
        "absent_content": {**base, "content_hex": None},
        "unknown_custody": {**base, "custody": "MADE_UP"},
        "invalid_uri_utf8": {**base, "uri_hex": "ff"},
        "invalid_version_utf8": {**base, "version_hex": "ff"},
        "noninteger_generation": {**base, "identity": ["s", 1.5, "tx-1"]},
        "boolean_generation": {**base, "identity": ["s", True, "tx-1"]},
    }
    vectors = {}
    for method in ("references", "definition"):
        for name, value in cases.items():
            vectors[method + "_" + name] = selector(method, value)
        vectors[method + "_schema_lf"] = selector(method, base, schema=original("schema") + b"\n")
        vectors[method + "_policy_lf"] = selector(method, base, policy=original(method) + b"\n")
        for name, value in rejected.items():
            try:
                selector(method, value)
            except (ValueError, UnicodeError):
                pass
            else:
                raise AssertionError(method + "/" + name + " unexpectedly received a selector")
    result = {"status": "UNACCEPTED_OFFLINE_CANDIDATE", "domain": "ADR0011-GENERIC-EXACT/2", "source_artifact_domain": "ADR0011-GENERIC-SOURCE-ARTIFACT/1",
              "transaction_domain": "ADR0011-GENERIC-TRANSACTION/1", "pins": {key: {"length": size, "sha256": expected} for key, (_, expected, size) in PINS.items()},
              "cases": cases, "selectors": vectors, "rejected_inputs": rejected}
    return (json.dumps(result, sort_keys=True, separators=(",", ":"), ensure_ascii=False) + "\n").encode()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    generated = fixture()
    if args.check:
        if OUT.read_bytes() != generated:
            raise SystemExit("SOURCE candidate vector bytes mismatch")
    else:
        OUT.write_bytes(generated)
    print(f"SOURCE_CANDIDATE_VECTORS_{'CHECKED' if args.check else 'WRITTEN'} sha256:{sha(generated)} bytes:{len(generated)}")
