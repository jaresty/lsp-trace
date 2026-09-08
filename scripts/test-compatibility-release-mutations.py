#!/usr/bin/env python3
import json
import os
import pathlib
import shutil
import subprocess
import tempfile

REPO = pathlib.Path(__file__).resolve().parent.parent
WRAPPER = REPO / "scripts" / "test-compatibility-release-evidence.sh"
FIXTURE_NAME = "synthetic-compatibility-graph.v1.json"
EXPECTED = {
    "ASSERT_FR22_SYNTHETIC_AUTHORITY",
    "ASSERT_FR22_FIXTURE_SOURCE_IDENTITY",
    "ASSERT_FR22_FIXTURE_BYTES_PINNED",
    "ASSERT_FR22_SOURCE_READY_DEPLOYMENT_UNKNOWN",
}


def run(command, *, cwd, env=None):
    return subprocess.run(command, cwd=cwd, env=env, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)


def require(name, condition, detail=""):
    if not condition:
        raise AssertionError(f"FAIL {name}: {detail}")
    print(f"PASS {name}")


def stage():
    root = pathlib.Path(tempfile.mkdtemp(prefix="lsp-trace-fr22-"))
    shutil.copytree(REPO / "scripts", root / "scripts")
    shutil.copytree(REPO / "qualification" / "compatibility-release", root / "qualification" / "compatibility-release")
    return root


def mutate_matrix(root, mutation):
    path = root / "qualification" / "compatibility-release" / "matrix.v1.json"
    data = json.loads(path.read_text())
    mutation(data)
    path.write_text(json.dumps(data, indent=2) + "\n")


repo_result = run([str(WRAPPER)], cwd=REPO)
tmp_result = run([str(WRAPPER)], cwd=pathlib.Path(tempfile.gettempdir()))
require("MUTATION_FR22_CWD_PORTABLE", repo_result.returncode == 0 and tmp_result.returncode == 0 and repo_result.stdout == tmp_result.stdout, tmp_result.stdout)

root = stage()
(root / "scripts" / "qualify-compatibility-release.sh").unlink()
missing = run([str(root / "scripts" / "test-compatibility-release-evidence.sh")], cwd=tempfile.gettempdir())
require("MUTATION_FR22_MISSING_RUNNER_FAILS_CLOSED", missing.returncode != 0 and "qualification runner unavailable:" in missing.stdout and missing.stdout.index("qualification runner unavailable:") < missing.stdout.index("qualification output omitted assertion"), missing.stdout)

root = stage()
wrapper_link = root / "test-compatibility-release-evidence-link.sh"
wrapper_link.symlink_to(root / "scripts" / "test-compatibility-release-evidence.sh")
symlinked = run([str(wrapper_link)], cwd=tempfile.gettempdir())
require("MUTATION_FR22_WRAPPER_SYMLINK_REJECTED", symlinked.returncode != 0 and "qualification wrapper unsafe symlink:" in symlinked.stdout, symlinked.stdout)

root = stage()
fixture = root / "qualification" / "compatibility-release" / FIXTURE_NAME
fixture.write_bytes(fixture.read_bytes() + b" ")
mutate_matrix(root, lambda d: d["transitions"][0]["input"].update({"sha256": __import__("hashlib").sha256(fixture.read_bytes()).hexdigest(), "byte_length": len(fixture.read_bytes())}))
cochange = run([str(root / "scripts" / "qualify-compatibility-release.sh")], cwd=root)
require("MUTATION_FR22_FIXTURE_HASH_COCHANGE_REJECTED", cochange.returncode != 0 and "FAIL ASSERT_FR22_FIXTURE_SOURCE_IDENTITY" in cochange.stdout, cochange.stdout)

mutations = [
    ("MUTATION_FR22_AUTHORITY_LABEL_REJECTED", lambda d: d["transitions"][0]["producer"].update({"authority": "HISTORICAL_RETAINED"}), "ASSERT_FR22_SYNTHETIC_AUTHORITY"),
    ("MUTATION_FR22_BYTE_LENGTH_REJECTED", lambda d: d["transitions"][0]["input"].update({"byte_length": 99}), "ASSERT_FR22_FIXTURE_BYTES_PINNED"),
    ("MUTATION_FR22_SOURCE_IDENTITY_REJECTED", lambda d: d["transitions"][0]["input"].update({"source_blob": "0" * 40}), "ASSERT_FR22_FIXTURE_SOURCE_IDENTITY"),
]
for name, mutation, assertion in mutations:
    root = stage()
    mutate_matrix(root, mutation)
    result = run([str(root / "scripts" / "qualify-compatibility-release.sh")], cwd=root)
    require(name, result.returncode != 0 and f"FAIL {assertion}" in result.stdout, result.stdout)

print("PASS FR22_MUTATION_GUARDS")
