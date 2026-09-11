#!/usr/bin/env python3
import json, pathlib, sys
ROOT = pathlib.Path(__file__).resolve().parents[1]
PATH = ROOT / "qualification/program-c/i-03-candidate-inventory.v1.json"
def emit(name, ok):
    print(f"{name} result={'PASS' if ok else 'FAIL'}")
    return ok
def main():
    try:
        if PATH.is_symlink() or not PATH.is_file(): raise ValueError()
        d=json.loads(PATH.read_text())
    except Exception:
        print("ASSERT_I03_PACKET result=FAIL")
        print("I_03 PASS=0 FAIL=1 BLOCKED=0"); return 1
    text=PATH.read_text(); cs=d.get('candidates',[])
    checks=[
      ("ASSERT_I03_SCHEMA",d.get('schema_version')=='lsp-trace.program-c-candidate-inventory.v1'),
      ("ASSERT_I03_NO_EXECUTION",all(x in d.get('policy','') for x in ('No candidate was installed','imported','built','executed'))),
      ("ASSERT_I03_GONUM_TAG",all(x in text for x in ('v0.17.0','directed Louvain','undirected Louvain','multiplex Louvain','"Q"','"Modularize"','"QMultiplex"','"ModularizeMultiplex"','contains Louvain and no Leiden'))),
      ("ASSERT_I03_UNRELEASED_LEIDEN",all(x in text for x in ('69ca49f456a7a38cf370131834a2178d9aae17fe','inventory-only and ineligible'))),
      ("ASSERT_I03_INFOMAP",all(x in text for x in ('0de17d0664c9c4ba2ae603e231a9e6a1e013af23','native CLI','C++','Python','R','JavaScript','Node.js','directed','undirected','multilayer','memory','bipartite'))),
      ("ASSERT_I03_LEIDENALG_REJECTED",all(x in text for x in ('vtraag/leidenalg','0.12.0','Vincent Traag','rejected','igraph'))),
      ("ASSERT_I03_MAINTAINERS_AND_URLS",all(('maintainer' in c or 'maintainers' in c) and c.get('sources') and all(u.startswith('https://') for u in c['sources']) for c in cs)),
      ("ASSERT_I03_CLAIM_CEILING",'makes no build, runtime, quality, license-compatibility, portability, determinism, resource, or admission claim' in d.get('claim_ceiling',''))]
    p=sum(emit(n,o) for n,o in checks); f=len(checks)-p
    print(f"I_03 PASS={p} FAIL={f} BLOCKED=0"); return int(f>0)
if __name__=='__main__': sys.exit(main())
