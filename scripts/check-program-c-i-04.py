#!/usr/bin/env python3
import json,pathlib,sys
P=pathlib.Path(__file__).resolve().parents[1]/'qualification/program-c/i-04-license-inputs.v1.json'
APPROVED_ENTRIES = [
 {'candidate':'Gonum v0.17.0','license':'BSD-3-Clause','mode':'intended native-Go linked module','status':'preferred input','sources':['https://github.com/gonum/gonum/blob/v0.17.0/LICENSE','https://github.com/gonum/gonum/tree/v0.17.0']},
 {'candidate':'Infomap v2.15.1','license':'GPL-3.0-or-later (descriptive input from tagged LICENSE_GPLv3.txt)','mode':'optional externally provisioned CLI/reference only; never linked or bundled','status':'requires separate provisioning and authorized license review','sources':['https://github.com/mapequation/infomap/blob/v2.15.1/LICENSE_GPLv3.txt','https://github.com/mapequation/infomap/tree/v2.15.1']},
 {'candidate':'vtraag/leidenalg 0.12.0','license':'GPL-3.0-or-later (pyproject license and LICENSE descriptive inputs)','mode':'compiled Python C++ extension _c_leiden linked with libleidenalg and igraph','status':'rejected','sources':['https://github.com/vtraag/leidenalg/blob/0.12.0/pyproject.toml','https://github.com/vtraag/leidenalg/blob/0.12.0/LICENSE','https://github.com/vtraag/leidenalg/tree/0.12.0/src']},
]
def a(n,o): print(f"{n} result={'PASS' if o else 'FAIL'}"); return o
def main():
 try:
  if P.is_symlink() or not P.is_file(): raise ValueError()
  d=json.loads(P.read_text())
 except Exception: print('ASSERT_I04_PACKET result=FAIL\nI_04 PASS=0 FAIL=1 BLOCKED=0'); return 1
 c=[('ASSERT_I04_SCHEMA',d.get('schema_version')=='lsp-trace.program-c-license-inputs.v1'),('ASSERT_I04_EXACT_ENTRIES',d.get('entries')==APPROVED_ENTRIES),('ASSERT_I04_NO_LEGAL_CONCLUSION','no legal advice, approval, compatibility' in d.get('claim_ceiling',''))]
 p=sum(a(n,o) for n,o in c); f=len(c)-p; print(f'I_04 PASS={p} FAIL={f} BLOCKED=0'); return int(f>0)
if __name__=='__main__':sys.exit(main())
