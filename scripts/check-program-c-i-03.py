#!/usr/bin/env python3
import hashlib, json, pathlib, sys
ROOT = pathlib.Path(__file__).resolve().parents[1]
V1 = ROOT / "qualification/program-c/i-03-candidate-inventory.v1.json"
V2 = ROOT / "qualification/program-c/i-03-candidate-inventory.v2.json"
V1_SHA = "bea2dca0a82aad0518c0eab0acb233cebe5ec766b2e75b58cbcc4a3c33991f2e"
REV = "69ca49f456a7a38cf370131834a2178d9aae17fe"
COMMAND = f"go mod download -json gonum.org/v1/gonum@{REV}"
def emit(name, ok):
    print(f"{name} result={'PASS' if ok else 'FAIL'}")
    return ok
def main():
    try:
        if any(p.is_symlink() or not p.is_file() for p in (V1,V2)): raise ValueError("regular files required")
        v1 = V1.read_bytes(); d = json.loads(V2.read_text())
    except Exception:
        print("ASSERT_I03_PACKET result=FAIL\nI_03 PASS=0 FAIL=1 BLOCKED=0"); return 1
    a=d.get("authorized_candidate",{})
    checks=[
      ("ASSERT_I03_V1_IMMUTABLE", hashlib.sha256(v1).hexdigest()==V1_SHA),
      ("ASSERT_I03_V2_SCHEMA", d.get("schema_version")=="lsp-trace.program-c-candidate-inventory.v2"),
      ("ASSERT_I03_PREDECESSOR", d.get("predecessor")=={"path":"qualification/program-c/i-03-candidate-inventory.v1.json","sha256":"sha256:"+V1_SHA}),
      ("ASSERT_I03_EXACT_REVISION", a.get("module")=="gonum.org/v1/gonum" and a.get("revision")==REV),
      ("ASSERT_I03_EXACT_ACQUISITION", a.get("acquisition")==COMMAND and a.get("allowed_network")=="Go module mechanism for this exact module and revision only"),
      ("ASSERT_I03_QUALIFICATION_ONLY", a.get("role")=="qualification-only Leiden comparator" and a.get("surfaces")==["private separate qualification module"] and a.get("maximum_outcome")=="IMPLEMENTATION_CANDIDATE"),
      ("ASSERT_I03_NO_FALLBACK", "Any different revision or any general network fallback requires new explicit authorization" in d.get("policy","") and "No other candidate or revision gains acquisition or execution authorization" in d.get("other_candidates","")),
    ]
    p=sum(emit(n,o) for n,o in checks); f=len(checks)-p
    print(f"I_03 PASS={p} FAIL={f} BLOCKED=0"); return int(f>0)
if __name__=='__main__': sys.exit(main())
