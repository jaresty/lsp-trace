#!/usr/bin/env python3
from pathlib import Path
import hashlib
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[3]
PKG = Path(__file__).resolve().parent
HEAD = "87b5c9c76c93861bd2759313e93e757260aea8d3"
TOKEN = "ADR0011_DIRECTIONAL_RAW_CUSTODY_C15_LEDGER_PARTITION_GO"
REQUIRED = {
    "README.md", "ARCHITECTURE.md", "C15_ENUMERATION_AND_MIGRATION.md",
    "WRITESET_AND_TESTS.md", "SOURCE_HASHES.tsv", "REVIEW_REQUEST.md",
    "verify_package.py", "SHA256SUMS",
}

def fail(msg):
    print(f"FAIL {msg}")
    raise SystemExit(1)

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

if subprocess.check_output(["git", "-C", str(ROOT), "rev-parse", "HEAD"], text=True).strip() != HEAD:
    fail("HEAD")
if {p.name for p in PKG.iterdir() if p.is_file()} != REQUIRED:
    fail("FILE_SET")

for line in (PKG / "SOURCE_HASHES.tsv").read_text().splitlines()[1:]:
    kind, expected, rel, role = line.split("\t", 3)
    path = ROOT / rel
    if not path.is_file() or sha(path) != expected:
        fail(f"SOURCE_HASH {rel}")

manifest = {}
for line in (PKG / "SHA256SUMS").read_text().splitlines():
    digest, name = line.split("  ", 1)
    if name == "SHA256SUMS" or name not in REQUIRED:
        fail("MANIFEST_ENTRY")
    manifest[name] = digest
if set(manifest) != REQUIRED - {"SHA256SUMS"}:
    fail("MANIFEST_SET")
for name, expected in manifest.items():
    if sha(PKG / name) != expected:
        fail(f"PACKAGE_HASH {name}")

all_text = "\n".join((PKG / n).read_text() for n in REQUIRED if n.endswith((".md", ".tsv")))
for needle in [
    TOKEN, "AUTHORED_NOT_GO", "67,108,864", "33,554,432", "24,600",
    "sessionruntime/b4_explicit_bytes_c15_private.go", "Stage A", "Stage B",
    "STOP", "RESTART", "Shutdown", "CALLS_ONLY", "authority `0`",
]:
    if needle not in all_text:
        fail(f"REQUIRED_TEXT {needle}")
print(f"OK {TOKEN} AUTHORED_NOT_GO")
