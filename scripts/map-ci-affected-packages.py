#!/usr/bin/env python3
"""Advisory, fail-closed affected-package mapper; CI remains full by default."""
import argparse, json, os, subprocess
from pathlib import Path

FORCE_PREFIXES=(".github/workflows/","scripts/ci-","scripts/check-ci","scripts/release","docs/qualification/","internal/schema/","internal/mcpcontract/","internal/census")
FORCE_FILES={"go.mod","go.sum"}
FORCE_EVENTS={"merge_group","workflow_dispatch","push","release"}

def go_metadata(goos):
 env=os.environ.copy(); env["GOOS"]=goos
 raw=subprocess.run(["go","list","-deps","-test","-json","./..."],env=env,text=True,stdout=subprocess.PIPE,check=True).stdout
 dec=json.JSONDecoder(); pos=0; packages=[]
 while pos < len(raw):
  while pos<len(raw) and raw[pos].isspace(): pos+=1
  if pos>=len(raw): break
  obj,pos=dec.raw_decode(raw,pos)
  if obj.get("Module") and obj.get("ImportPath") and obj.get("Dir"):
   packages.append({"ImportPath":obj["ImportPath"],"Dir":obj["Dir"],"Deps":obj.get("Deps",[])})
 return packages

def decide(meta,root,changed,event):
 if event in FORCE_EVENTS: return {"mode":"full","reason":"forced event"}
 if not changed: return {"mode":"full","reason":"uncertain empty change set"}
 if any(p in FORCE_FILES or p.startswith(FORCE_PREFIXES) for p in changed): return {"mode":"full","reason":"forced path"}
 package_records=meta["packages"]
 owners=set()
 for path in changed:
  if not path.endswith(".go"): return {"mode":"full","reason":f"uncertain non-Go path: {path}"}
  absolute=str(Path(root,path))
  candidates=[p for p in package_records if absolute==p["Dir"] or absolute.startswith(p["Dir"].rstrip('/')+'/')]
  if not candidates: return {"mode":"full","reason":f"unmapped Go path: {path}"}
  owners.add(max(candidates,key=lambda p:len(p["Dir"]))["ImportPath"])
 affected=set(owners)
 changed_flag=True
 while changed_flag:
  changed_flag=False
  for package in package_records:
   if package["ImportPath"] not in affected and affected.intersection(package.get("Deps",[])):
    affected.add(package["ImportPath"]); changed_flag=True
 return {"mode":"affected","reason":"changed Go owners plus reverse dependencies","platforms":{o:sorted(affected.intersection(v)) for o,v in meta["manifests"].items()}}

def main():
 p=argparse.ArgumentParser(); p.add_argument('--metadata'); p.add_argument('--repo-root',default=os.getcwd()); p.add_argument('--event',default='pull_request'); p.add_argument('--changed',action='append'); p.add_argument('--merge-base'); a=p.parse_args()
 if a.metadata: meta=json.loads(Path(a.metadata).read_text())
 else:
  manifest=json.loads(Path('scripts/ci-shards.json').read_text())
  records={}
  for goos in ('linux','darwin'):
   for record in go_metadata(goos): records[record['ImportPath']]=record
  meta={"packages":list(records.values()),"manifests":{o:[x for s in manifest['platforms'][o]['shards'] for x in s['packages']] for o in ('linux','darwin')}}
 changed=a.changed
 if changed is None and a.merge_base:
  changed=subprocess.run(['git','diff','--name-only',a.merge_base+'...HEAD'],text=True,stdout=subprocess.PIPE,check=True).stdout.splitlines()
 print(json.dumps(decide(meta,a.repo_root,changed or [],a.event),sort_keys=True))
if __name__=='__main__': main()
