#!/usr/bin/env python3
import json,pathlib,sys
P=pathlib.Path(__file__).resolve().parents[1]/'qualification/program-c/i-06-determinism-policy.v1.json'
def a(n,o):print(f"{n} result={'PASS' if o else 'FAIL'}");return o
def main():
 try:d=json.loads(P.read_text()) if P.is_file() and not P.is_symlink() else (_ for _ in()).throw(ValueError())
 except Exception:print('ASSERT_I06_PACKET result=FAIL\nI_06 PASS=0 FAIL=1 BLOCKED=0');return 1
 t=P.read_text(); req={'exact input digest','exact profile digest','exact profile version','candidate version or commit','parameters','seed','runtime identity','input order'}
 c=[('ASSERT_I06_SCHEMA',d.get('schema_version')=='lsp-trace.program-c-determinism-policy.v1'),('ASSERT_I06_EXACT_IDENTITY',set(d.get('identity',{}).get('required',[]))==req),('ASSERT_I06_REPEAT_AND_PERMUTATION',d.get('schedule')=={'repeats_per_seed':3,'permutations':1,'rule':'three repeats for each seed and one additional input-order permutation'}),('ASSERT_I06_CANONICAL_LABELS',all(x in t for x in ("Sort each community's node IDs ascending",'minimum node ID','full vector','canonical labels'))),('ASSERT_I06_GRAPH_SEMANTICS',all(x in t for x in ('direction','occurrence multiplicity','explicit weight'))),('ASSERT_I06_RUNTIME_EQUALITY',all(x in t for x in ('exact output byte equality','logical equality of canonical communities','byte equality is not claimed'))),('ASSERT_I06_FAIL_CLOSED',all(x in t for x in ('Any divergence','incomplete run fails closed'))),('ASSERT_I06_POLICY_ONLY','no candidate replay has run' in d.get('status','') and 'does not claim any candidate' in d.get('claim_ceiling',''))]
 p=sum(a(n,o) for n,o in c);f=len(c)-p;print(f'I_06 PASS={p} FAIL={f} BLOCKED=0');return int(f>0)
if __name__=='__main__':sys.exit(main())
