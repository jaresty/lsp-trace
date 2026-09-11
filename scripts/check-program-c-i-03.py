#!/usr/bin/env python3
import json, pathlib, sys
ROOT = pathlib.Path(__file__).resolve().parents[1]
PATH = ROOT / "qualification/program-c/i-03-candidate-inventory.v1.json"
APPROVED_CANDIDATES = [
    {"name":"Gonum graph/community","version":"v0.17.0","maintainer":"The Gonum Authors","role":"eligible tagged native-Go Louvain comparator","surfaces":["Go package gonum.org/v1/gonum/graph/community"],"variants":["directed Louvain","undirected Louvain","multiplex Louvain"],"exports":["Q","Modularize","QMultiplex","ModularizeMultiplex"],"claims":["The v0.17.0 tag contains Louvain and no Leiden implementation."],"sources":["https://github.com/gonum/gonum/tree/v0.17.0/graph/community","https://pkg.go.dev/gonum.org/v1/gonum/graph/community@v0.17.0","https://github.com/gonum/gonum/releases/tag/v0.17.0"]},
    {"name":"Gonum unreleased Leiden","version":"commit 69ca49f456a7a38cf370131834a2178d9aae17fe","maintainer":"The Gonum Authors","role":"inventory-only and ineligible because it is not in the v0.17.0 tag","surfaces":["unreleased Go source"],"variants":["Leiden"],"sources":["https://github.com/gonum/gonum/commit/69ca49f456a7a38cf370131834a2178d9aae17fe"]},
    {"name":"Infomap","version":"v2.15.1; tag commit 0de17d0664c9c4ba2ae603e231a9e6a1e013af23","maintainers":["Martin Rosvall","Daniel Edler","The Infomap authors"],"role":"optional externally provisioned CLI/reference","surfaces":["native CLI","C++","Python","R","JavaScript","Node.js"],"variants":["directed","undirected","multilayer","memory","bipartite"],"sources":["https://github.com/mapequation/infomap/tree/v2.15.1","https://github.com/mapequation/infomap/commit/0de17d0664c9c4ba2ae603e231a9e6a1e013af23","https://github.com/mapequation/infomap/releases/tag/v2.15.1","https://www.mapequation.org/infomap/"]},
    {"name":"vtraag/leidenalg","version":"0.12.0","maintainer":"Vincent Traag and contributors","role":"rejected","surfaces":["C++","Python","igraph"],"variants":["Leiden"],"sources":["https://github.com/vtraag/leidenalg/tree/0.12.0","https://pypi.org/project/leidenalg/0.12.0/"]},
]
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
    checks=[
      ("ASSERT_I03_SCHEMA",d.get('schema_version')=='lsp-trace.program-c-candidate-inventory.v1'),
      ("ASSERT_I03_NO_EXECUTION",all(x in d.get('policy','') for x in ('No candidate was installed','imported','built','executed'))),
      ("ASSERT_I03_EXACT_CANDIDATES",d.get('candidates')==APPROVED_CANDIDATES),
      ("ASSERT_I03_CLAIM_CEILING",'makes no build, runtime, quality, license-compatibility, portability, determinism, resource, or admission claim' in d.get('claim_ceiling',''))]
    p=sum(emit(n,o) for n,o in checks); f=len(checks)-p
    print(f"I_03 PASS={p} FAIL={f} BLOCKED=0"); return int(f>0)
if __name__=='__main__': sys.exit(main())
