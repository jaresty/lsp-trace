#!/usr/bin/env python3
"""Deterministic verifier for ADR0011 incoming-calls v1 final qualification package."""
import argparse, hashlib, json, pathlib, shutil, subprocess, sys, tempfile

ROOT = pathlib.Path(__file__).resolve().parent
CHECKSUMS = ROOT / "CHECKSUMS.sha256"
RESULT = ROOT / "RESULT.json"
MANIFEST = ROOT / "MANIFEST.json"


def sha(path: pathlib.Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def load_checksums(root: pathlib.Path):
    checks = {}
    for line in (root / "CHECKSUMS.sha256").read_text().splitlines():
        if not line.strip():
            continue
        digest, rel = line.split("  ", 1)
        checks[rel] = digest
    return checks


def verify(root: pathlib.Path, require_tamper_record: bool = True) -> dict:
    checks = load_checksums(root)
    assert "CHECKSUMS.sha256" not in checks, "CHECKSUMS must not self-reference"
    for rel, digest in checks.items():
        path = root / rel
        assert path.is_file(), f"missing checksummed file: {rel}"
        got = sha(path)
        assert got == digest, f"checksum mismatch: {rel}: {got} != {digest}"

    manifest = json.loads((root / "MANIFEST.json").read_text())
    result = json.loads((root / "RESULT.json").read_text())
    stageb = json.loads((root / "STAGEB_COUNTS.json").read_text())
    source_hashes = json.loads((root / "SOURCE_POST_HASHES_EXACT14.json").read_text())

    assert manifest["authority"] is False and manifest["production_go"] is False and manifest["self_go"] is False
    assert result["status"] == "IMPLEMENTATION_QUALIFICATION_AUTHORED_NOT_GO"
    assert result["authority"] is False and result["production_go"] is False and result["self_go"] is False
    assert result["head"] == "49334ce23de37c0094f50d21d57ab251a46130ca"
    assert result["four_witnesses"]["exact32_runs"] >= 3
    assert result["four_witnesses"]["plus_one_runs"] >= 3
    assert result["four_witnesses"]["partial_external_runs"] == 1
    assert result["four_witnesses"]["early_terminal_runs"] == 1
    assert result["four_witnesses"]["distinct_processes"] == result["four_witnesses"]["total_runs"]
    assert result["four_witnesses"]["plus_one_zero_write_all"] is True
    assert result["four_witnesses"]["plus_one_same_session_reuse_all"] is True
    assert result["stageb_counts"] == {"actual_rows": 91, "necessary": 78, "reuse": 11, "false_positive": 2, "unresolved": 0, "duplicates": 0, "omissions": 0}
    assert stageb == result["stageb_counts"]
    assert source_hashes["count"] == 14 and source_hashes["all_match_manifest"] is True
    assert result["source_post_hashes_exact14"]["count"] == 14
    assert result["source_post_hashes_exact14"]["all_match_manifest"] is True
    assert result["diff_check_exit_code"] == 0
    assert all(cmd["exit_code"] == 0 for cmd in result["commands"]), "one or more recorded commands failed"
    if require_tamper_record:
        assert result["tamper_guard_self_test"]["detected_checksum_tamper"] is True
    return {"status": "VERIFIED", "files": len(checks), "result_sha256": sha(root / "RESULT.json")}


def self_test(root: pathlib.Path) -> dict:
    with tempfile.TemporaryDirectory(prefix="adr0011-final-qual-tamper-") as tmp:
        dst = pathlib.Path(tmp) / "pkg"
        shutil.copytree(root, dst)
        target = dst / "MANIFEST.json"
        raw = target.read_text()
        target.write_text(raw.replace('"authority": false', '"authority": true', 1))
        try:
            verify(dst, require_tamper_record=False)
        except Exception as exc:
            return {"detected_checksum_tamper": True, "error": str(exc)}
        raise AssertionError("tamper self-test did not fail")


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--allow-missing-tamper-record", action="store_true", help="authoring-only: verify before RESULT records the self-test")
    args = ap.parse_args()
    out = verify(ROOT, require_tamper_record=not args.allow_missing_tamper_record)
    if args.self_test:
        out["self_test"] = self_test(ROOT)
    print(json.dumps(out, indent=2, sort_keys=True))
    return 0

if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:
        print(json.dumps({"status": "FAILED", "error": str(exc)}, indent=2, sort_keys=True))
        raise
