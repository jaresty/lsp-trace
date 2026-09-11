#!/usr/bin/env python3
"""Private, tuple-specific Program C Ember state-flow qualification."""
import argparse, hashlib, json, subprocess, sys
from pathlib import Path

PROFILE = "state-flow-v1"
COORDINATE = ("typescript", "ember", "ember-glint", "1.0.3")
RELATIONS = ("UPDATES_STATE",)
SCHEMA = "lsp-trace.program-c-profile-qualification-receipt.v1"
TOOL_VERSION = "program-c-ember-state-flow-qualifier/1"

def digest(data): return "sha256:" + hashlib.sha256(data).hexdigest()
def canonical(value): return json.dumps(value, sort_keys=True, separators=(",", ":")).encode()

def bytes_at_revision(root, revision, path):
    completed = subprocess.run(
        ["git", "-C", str(root), "show", f"{revision}:{path}"],
        stdout=subprocess.PIPE, stderr=subprocess.PIPE,
    )
    if completed.returncode != 0:
        raise ValueError(f"cannot read {path} at exact revision: {completed.stderr.decode(errors='replace')}")
    return completed.stdout

def matrix_at_revision(root, revision):
    return bytes_at_revision(root, revision, "qualification/program-c/profile-qualification.tsv")

def assert_tool_revision(root, revision):
    path = "scripts/qualify-program-c-ember-state-flow.py"
    if bytes_at_revision(root, revision, path) != (root / path).read_bytes():
        raise ValueError("qualifier bytes do not match exact repository revision")

def decode_frame(raw):
    header, sep, payload = raw.partition(b"\r\n\r\n")
    if not sep or not header.startswith(b"Content-Length: ") or int(header[16:]) != len(payload):
        raise ValueError("non-canonical provider frame")
    return json.loads(payload)

def run_provider(provider, fixture, relation, revision, generation):
    source = fixture.read_bytes()
    request = {"schema_version":"lsp-trace.provider-collector-request.v1","provider_id":"ember-glint@1","adapter_id":"lsp-trace-observation-adapter@1","session":{"session_id":f"program-c:{generation}:{relation}","generation":1},"seed":{"uri":fixture.resolve().as_uri()},"relations":[relation],"languages":["typescript"],"frameworks":["ember"],"document_custody":{"workspace_revision":{"kind":"git","value":revision}},"limits":{"max_nodes":10000,"request_timeout_ms":30000}}
    frame = canonical(request)
    framed = f"Content-Length: {len(frame)}\r\n\r\n".encode() + frame
    def execute():
        completed = subprocess.run([str(provider)], input=framed, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if completed.returncode != 0:
            raise ValueError(f"{relation}: provider failed: {completed.stderr.decode(errors='replace')}")
        return completed.stdout
    first = execute()
    second = execute()
    if first != second: raise ValueError(f"{relation}: replay bytes differ")
    response = decode_frame(first)
    observations = [o for o in response.get("observations", []) if o.get("kind") == relation]
    if response.get("coverage", {}).get("status") != "COMPLETE_WITHIN_BOUNDS" or not observations:
        raise ValueError(f"{relation}: no positive complete evidence")
    return source, framed, first, response, len(observations)

def build(root, revision, matrix_bytes):
    profiles_path = root / "qualification/program-c/projection-profiles.v1.json"
    matrix_path = root / "qualification/program-c/profile-qualification.tsv"
    package_path = root / "providers/ember-glint/package.json"
    provider = root / "providers/ember-glint/bin/ember-glint.mjs"
    profile_doc = json.loads(profiles_path.read_text())
    package = json.loads(package_path.read_text())
    if package.get("name") != "@lsp-trace/ember-glint-provider" or package.get("version") != "1.0.3": raise ValueError("provider package identity/version mismatch")
    profile = next(p for p in profile_doc["profiles"] if p["name"] == PROFILE)
    if tuple(r["kind"] for r in profile["relations"]) != RELATIONS: raise ValueError("profile relation mismatch")
    matrix_rows = [line.split("\t") for line in matrix_bytes.decode().splitlines() if line and not line.startswith("#")]
    row = next(r for r in matrix_rows if tuple(r[:5]) == (PROFILE,) + COORDINATE)
    if row[6:] != ["MISSING", "-", "-"]: raise ValueError("target matrix row must be pre-publication MISSING")
    generation = hashlib.sha256((revision + digest(matrix_bytes)).encode()).hexdigest()[:24]
    fixtures = {
      "UPDATES_STATE": root / "providers/ember-glint/fixtures/updates-state-positive.ts",
    }
    inputs = {"profile_manifest": digest(profiles_path.read_bytes()), "qualification_matrix_prepublication": digest(matrix_bytes), "provider_package_manifest": digest(package_path.read_bytes()), "provider_executable": digest(provider.read_bytes()), "qualifier_script": digest(Path(__file__).read_bytes())}
    evidence, result_parts = [], []
    for relation in RELATIONS:
        source, request, response, envelope, count = run_provider(provider, fixtures[relation], relation, revision, generation)
        if envelope.get("provider") != {"name":"ember-glint","version":"1"}: raise ValueError(f"{relation}: observed provider identity mismatch")
        inputs[f"{relation.lower()}_fixture"] = digest(source)
        inputs[f"{relation.lower()}_request"] = digest(request)
        inputs[f"{relation.lower()}_result"] = digest(response)
        result_parts.append(response)
        evidence.append({"relation":relation,"status":"PASS","count":count,"custody":"PROVIDER_PROVED","replay":"EXACT_BYTES"})
    command = ["python3","scripts/qualify-program-c-ember-state-flow.py","--repository-revision","$REPOSITORY_REVISION","--output","$RECEIPT"]
    bindings = {"REPOSITORY_REVISION":revision,"RECEIPT":"qualification/program-c/state-flow-v1-typescript-ember-ember-glint-1.0.3.receipt.json"}
    return {"schema_version":SCHEMA,"result":"PASS","current":True,"repository_revision":revision,"selection":{"status":"CURRENT","supersedes":None},"profile":{"name":PROFILE,"logical_digest":profile["logical_digest"],"relations":list(RELATIONS)},"coordinate":{"language":"typescript","framework":"ember","provider":"ember-glint","provider_version":"1.0.3","observed_provider_identity":"ember-glint","observed_provider_version":"1.0.3"},"policy_matrix_identity":{"profile_manifest":"qualification/program-c/projection-profiles.v1.json","profile_manifest_sha256":inputs["profile_manifest"],"qualification_matrix":"qualification/program-c/profile-qualification.tsv","qualification_matrix_prepublication_sha256":inputs["qualification_matrix_prepublication"],"matrix_row":row[:6]+["PASS"]},"tool_identity":{"name":"lsp-trace","revision":revision,"version_output":f"lsp-trace {TOOL_VERSION} revision={revision} modified=false"},"exact_command":command,"command_bindings":bindings,"run_identity":{"run_id":f"program-c-ember-state-flow-{generation}","generation":generation},"input_sha256":inputs,"result_sha256":digest(canonical({"domain":"program-c-ember-state-flow-results-v1","digests":[digest(part) for part in result_parts]})),"relation_evidence":evidence,"claim_ceiling":{"scope":"EXACT_COORDINATE_AND_INPUTS_ONLY","whole_source_complete":False,"source_authenticated":False,"cross_coordinate_transfer":False}}

def main():
    p=argparse.ArgumentParser(); p.add_argument("--repository-revision", required=True); p.add_argument("--output", required=True); p.add_argument("--matrix-prepublication")
    a=p.parse_args(); root=Path(__file__).resolve().parents[1]
    if len(a.repository_revision)!=40 or any(c not in "0123456789abcdef" for c in a.repository_revision): raise SystemExit("repository revision must be exact lowercase SHA-1")
    assert_tool_revision(root, a.repository_revision)
    matrix_bytes = Path(a.matrix_prepublication).read_bytes() if a.matrix_prepublication else matrix_at_revision(root, a.repository_revision)
    receipt=build(root,a.repository_revision,matrix_bytes)
    payload=json.dumps(receipt,indent=2,separators=(",", ": ")).encode()+b"\n"
    out=Path(a.output); out.parent.mkdir(parents=True,exist_ok=True)
    if out.exists():
        if out.read_bytes() != payload:
            raise SystemExit("refusing to overwrite immutable receipt with different bytes")
    else:
        out.write_bytes(payload)
    print(f"ASSERT_PROGRAM_C_EMBER_STATE_FLOW_QUALIFICATION result=PASS counts="+",".join(f"{e['relation']}:{e['count']}" for e in receipt['relation_evidence']))
if __name__=="__main__": main()
