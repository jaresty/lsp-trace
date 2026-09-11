#!/usr/bin/env python3
import hashlib,json,pathlib,sys
ROOT=pathlib.Path(__file__).resolve().parents[1]
V1=ROOT/'qualification/program-c/i-04-license-inputs.v1.json'
V2=ROOT/'qualification/program-c/i-04-license-inputs.v2.json'
LICENSE=ROOT/'qualification/program-c/gonum-leiden-69ca49f.LICENSE'
V1_SHA='177f8d988df26bd8bce60908ce38fe11d206476f2d005e386a1573bbc53a22a1'
LICENSE_SHA='b44d9e394ba3efc15de4e5a8ebd843bd8d6325f6ffbb0a4ff6db7181aa15dda3'
REV='69ca49f456a7a38cf370131834a2178d9aae17fe'
def emit(n,o): print(f"{n} result={'PASS' if o else 'FAIL'}"); return o
def main():
 try:
  if any(p.is_symlink() or not p.is_file() for p in (V1,V2,LICENSE)): raise ValueError()
  v1=V1.read_bytes(); lic=LICENSE.read_bytes(); d=json.loads(V2.read_text())
 except Exception: print('ASSERT_I04_PACKET result=FAIL\nI_04 PASS=0 FAIL=1 BLOCKED=0'); return 1
 e=d.get('entry',{})
 checks=[
  ('ASSERT_I04_V1_IMMUTABLE',hashlib.sha256(v1).hexdigest()==V1_SHA),
  ('ASSERT_I04_V2_SCHEMA',d.get('schema_version')=='lsp-trace.program-c-license-inputs.v2'),
  ('ASSERT_I04_PREDECESSOR',d.get('predecessor')=={'path':'qualification/program-c/i-04-license-inputs.v1.json','sha256':'sha256:'+V1_SHA}),
  ('ASSERT_I04_EXACT_CANDIDATE',e.get('module')=='gonum.org/v1/gonum' and e.get('revision')==REV and e.get('pseudo_version')=='v0.17.1-0.20260426204603-69ca49f456a7'),
  ('ASSERT_I04_EXACT_LICENSE',e.get('license')=='BSD-3-Clause' and e.get('retained_license_path')=='qualification/program-c/gonum-leiden-69ca49f.LICENSE' and e.get('license_sha256')=='sha256:'+LICENSE_SHA and hashlib.sha256(lic).hexdigest()==LICENSE_SHA),
  ('ASSERT_I04_QUALIFICATION_ONLY','qualification-only' in e.get('mode','') and 'not a root or production dependency' in e.get('mode','')),
  ('ASSERT_I04_NO_LEGAL_CONCLUSION','no legal compatibility conclusion' in d.get('policy','') and 'no other revision or distribution mode' in d.get('claim_ceiling','')),
 ]
 p=sum(emit(n,o) for n,o in checks); f=len(checks)-p; print(f'I_04 PASS={p} FAIL={f} BLOCKED=0'); return int(f>0)
if __name__=='__main__':sys.exit(main())
