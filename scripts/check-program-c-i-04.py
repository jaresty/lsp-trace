#!/usr/bin/env python3
import json,pathlib,sys
P=pathlib.Path(__file__).resolve().parents[1]/'qualification/program-c/i-04-license-inputs.v1.json'
def a(n,o): print(f"{n} result={'PASS' if o else 'FAIL'}"); return o
def main():
 try:
  if P.is_symlink() or not P.is_file(): raise ValueError()
  d=json.loads(P.read_text()); t=P.read_text()
 except Exception: print('ASSERT_I04_PACKET result=FAIL\nI_04 PASS=0 FAIL=1 BLOCKED=0'); return 1
 c=[('ASSERT_I04_SCHEMA',d.get('schema_version')=='lsp-trace.program-c-license-inputs.v1'),('ASSERT_I04_GONUM_BSD',all(x in t for x in ('Gonum v0.17.0','BSD-3-Clause','native-Go linked module','blob/v0.17.0/LICENSE'))),('ASSERT_I04_INFOMAP_GPL_EXTERNAL',all(x in t for x in ('Infomap v2.15.1','GPL-3.0-or-later','LICENSE_GPLv3.txt','externally provisioned CLI/reference only','never linked or bundled'))),('ASSERT_I04_LEIDENALG_BOUNDARY',all(x in t for x in ('leidenalg 0.12.0','pyproject.toml','LICENSE.txt','_c_leiden','libleidenalg','igraph','rejected'))),('ASSERT_I04_EXACT_URLS',all(all(u.startswith('https://') for u in e['sources']) for e in d.get('entries',[]))),('ASSERT_I04_NO_LEGAL_CONCLUSION','no legal advice, approval, compatibility' in d.get('claim_ceiling',''))]
 p=sum(a(n,o) for n,o in c); f=len(c)-p; print(f'I_04 PASS={p} FAIL={f} BLOCKED=0'); return int(f>0)
if __name__=='__main__':sys.exit(main())
