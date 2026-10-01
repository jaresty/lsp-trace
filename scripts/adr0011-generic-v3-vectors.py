#!/usr/bin/env python3
"""Offline V3 QUERY/TARGET_EVENTS/SOURCE candidates. Never issues selectors."""
import argparse
import hashlib
import json
import struct
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
ORIG = ROOT / "docs/qualification/originals"
OUT = ORIG / "generic-lsp-v3-selector-vectors.proposed.json"
PINS = {
    "schema": ("adr0011-generic-envelope-v2.schema.json", 13243, "f843389f811aea940ed5bf1d595f03dcb6de4fa97c4dafafc41ad83b5c1c7f8e"),
    "transport": ("generic-lsp-exact-transport-v2.json", 1206, "0519ef89b76d966141bece6ca24d9e186b6424113339a39fce68ae3f78ffa67f"),
    "shared": ("generic-lsp-selector-applicability-v1.json", 2247, "c73263cb1356abd9f2898e016ec31afbb6cb112924c881c0a1ef6034bb1d5a0d"),
    "references": ("generic-lsp-references-exact-v3.json", 1134, "8c76a71da5e888e278280bbb1802979b907ba7a4ad6b0aec910dfe06e24e225e"),
    "definition": ("generic-lsp-definition-exact-v3.json", 1043, "52dd415cb5a331462042103014991fcd4ff43a777d45ab5a9abd7d83a45a8051"),
    "v2_vectors": ("generic-lsp-v2-selector-vectors.json", 2220, "6992776962db2a815c5f2a3ea9d96a89a7330ec40f6b862cd3894a1c13b7614e"),
    "v2_source_vectors": ("generic-lsp-source-selector-v2-proposed-vectors.json", 5551, "7de0399f1fbd711abea08d26043492de4417574d8bdf13faf05577f14f48650f"),
}

def sha(b): return hashlib.sha256(b).hexdigest()
def lp(b):
    b = b.encode() if isinstance(b, str) else b
    return struct.pack(">Q",len(b))+b
def digest(fields): return "sha256:"+sha(b"".join(map(lp,fields)))
def pinned(key):
    name, size, expected=PINS[key]
    b=(ORIG/name).read_bytes()
    if len(b)!=size or sha(b)!=expected: raise ValueError("original pin changed: "+key)
    return b

def source(method, entry, originals, *, schema_override=None, policy_override=None):
    session,gen,transaction=entry["identity"]
    if not isinstance(session,str) or not session or not isinstance(transaction,str) or not transaction or type(gen) is not int or not 1<=gen<1<<64: raise ValueError("invalid transaction")
    uri=bytes.fromhex(entry["uri_hex"])
    version=None if entry["version_hex"] is None else bytes.fromhex(entry["version_hex"])
    content=None if entry["content_hex"] is None else bytes.fromhex(entry["content_hex"])
    if not uri or version is None or content is None: raise ValueError("missing original")
    uri.decode("utf8");version.decode("utf8")
    if entry["custody"] not in ("OWNER_BUFFER","MANAGED_VIRTUAL","CLEAN_REGISTERED_WORKTREE","IMMUTABLE_SOURCE_SNAPSHOT"):raise ValueError("unknown custody")
    tx=digest(["ADR0011-GENERIC-TRANSACTION/1",session,str(gen),transaction])
    artifact=sha(b"".join(map(lp,["ADR0011-GENERIC-SOURCE-ARTIFACT/1",tx,uri,"present",version,str(len(content)),sha(content),entry["custody"]])))
    return digest(common(method,"SOURCE",artifact,originals,session=session,generation=gen,schema_override=schema_override,policy_override=policy_override)+[tx])

def common(method,role,artifact_digest,originals,*,session="s",generation=1,schema_override=None,policy_override=None):
    schema=originals["schema"] if schema_override is None else schema_override
    policy=originals[method] if policy_override is None else policy_override
    return ["ADR0011-GENERIC-EXACT/3","GENERIC_LSP_"+method.upper()+"_EXACT_V3","textDocument/"+method,role,sha(schema),sha(originals["transport"]),sha(policy),session,str(generation),artifact_digest]

def vectors():
    originals={key:pinned(key) for key in PINS}
    for method in ("references","definition"):
        obj=json.loads(originals[method]); assert obj["shared_applicability_sha256"]==PINS["shared"][2] and obj["selector_domain"]=="ADR0011-GENERIC-EXACT/3"
    old=json.loads(originals["v2_vectors"]); old_source=json.loads(originals["v2_source_vectors"])
    query=bytes.fromhex(old["query_artifact_utf8_hex"]);event=bytes.fromhex(old["event_artifact_utf8_hex"])
    result={"status":"UNACCEPTED_OFFLINE_CANDIDATE","domain":"ADR0011-GENERIC-EXACT/3","pins":{k:{"length":v[1],"sha256":v[2]} for k,v in PINS.items()},"selectors":{},"v2_collision_count":0}
    for method in ("references","definition"):
        for name,role,artifact,extra,schema_override,policy_override in [
            ("query_version1_line0","QUERY",query,["file:///a","buffer:v1",sha(b"a"),"utf-16","0","0"],None,None),
            ("query_version2_line0","QUERY",query,["file:///a","buffer:v2",sha(b"a"),"utf-16","0","0"],None,None),
            ("query_version1_line1","QUERY",query,["file:///a","buffer:v1",sha(b"a"),"utf-16","1","0"],None,None),
            ("event_ordinal0","TARGET_EVENTS",event,["observed-key-1",sha(b"[]"),"0"],None,None),
            ("event_ordinal1","TARGET_EVENTS",event,["observed-key-1",sha(b"[]"),"1"],None,None),
            ("query_schema_substitution","QUERY",query,["file:///a","buffer:v1",sha(b"a"),"utf-16","0","0"],originals["schema"]+b"\n",None),
            ("query_policy_substitution","QUERY",query,["file:///a","buffer:v1",sha(b"a"),"utf-16","0","0"],None,originals[method]+b"\n"),
        ]:
            result["selectors"][method+"_"+name]=digest(common(method,role,sha(artifact),originals,schema_override=schema_override,policy_override=policy_override)+extra)
        for name,entry in old_source["cases"].items(): result["selectors"][method+"_source_"+name]=source(method,entry,originals)
        for name,override in (("schema_lf",{"schema_override":originals["schema"]+b"\n"}),("policy_lf",{"policy_override":originals[method]+b"\n"})):
            result["selectors"][method+"_source_"+name]=source(method,old_source["cases"]["query"],originals,**override)
        for name,entry in old_source["rejected_inputs"].items():
            try: source(method,entry,originals)
            except (ValueError,UnicodeError): pass
            else: raise AssertionError(method+"/"+name+" rejected input issued selector")
    if len(result["selectors"])!=36 or len(set(result["selectors"].values()))!=36: raise AssertionError("V3 selector cardinality/collision")
    if set(result["selectors"].values()) & (set(old["selectors"].values())|set(old_source["selectors"].values())): raise AssertionError("V2 selector collision")
    return (json.dumps(result,sort_keys=True,separators=(",",":"))+"\n").encode()

if __name__=="__main__":
    parser=argparse.ArgumentParser();parser.add_argument("--check",action="store_true");args=parser.parse_args()
    output=vectors()
    if args.check:
        if OUT.read_bytes()!=output: raise SystemExit("V3 candidate vectors mismatch")
    else: OUT.write_bytes(output)
    print(f"V3_CANDIDATE_VECTORS_{'CHECKED' if args.check else 'WRITTEN'} sha256:{sha(output)} bytes:{len(output)}")
