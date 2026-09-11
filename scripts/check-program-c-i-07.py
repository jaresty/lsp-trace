#!/usr/bin/env python3
import json,pathlib,sys
P=pathlib.Path(__file__).resolve().parents[1]/'qualification/program-c/i-07-resource-envelope.v1.json'
def a(n,o):print(f"{n} result={'PASS' if o else 'FAIL'}");return o
def main():
 try:d=json.loads(P.read_text()) if P.is_file() and not P.is_symlink() else (_ for _ in()).throw(ValueError())
 except Exception:print('ASSERT_I07_PACKET result=FAIL\nI_07 PASS=0 FAIL=1 BLOCKED=0');return 1
 t=P.read_text();L=d.get('limits',{});expected={'fixtures_max':6,'nodes_per_fixture_max':10000,'directed_weighted_edges_per_fixture_max':100000,'wall_seconds_per_run':60,'memory_mib_per_run':512,'runs_per_seed_candidate_fixture_max':3,'input_order_permutations':1};fail={'timeout','memory exhaustion','unsupported measurement','nonzero exit','panic','noncanonical output'}
 c=[('ASSERT_I07_SCHEMA',d.get('schema_version')=='lsp-trace.program-c-resource-envelope.v1'),('ASSERT_I07_EXACT_LIMITS',L==expected),('ASSERT_I07_TYPED_FAILURES',set(d.get('typed_failure_or_incomplete',[]))==fail and 'fails closed' in t),('ASSERT_I07_PORTABLE_ENFORCEMENT',all(x in t for x in ('platform-native','process-tree','linux','darwin','windows','externally controlled'))),('ASSERT_I07_NO_DEGRADATION',set(d.get('prohibitions',[]))=={'never sample','never truncate','never fallback','never approximate','never substitute algorithm or input'}),('ASSERT_I07_PREEXECUTION_CEILING','enforcement has not run' in d.get('status','') and 'does not claim a candidate run' in d.get('claim_ceiling',''))]
 p=sum(a(n,o) for n,o in c);f=len(c)-p;print(f'I_07 PASS={p} FAIL={f} BLOCKED=0');return int(f>0)
if __name__=='__main__':sys.exit(main())
