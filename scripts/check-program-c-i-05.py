#!/usr/bin/env python3
import json,pathlib,sys
P=pathlib.Path(__file__).resolve().parents[1]/'qualification/program-c/i-05-portability-questions.v1.json'
def a(n,o):print(f"{n} result={'PASS' if o else 'FAIL'}");return o
def main():
 try:d=json.loads(P.read_text()) if P.is_file() and not P.is_symlink() else (_ for _ in()).throw(ValueError())
 except Exception:print('ASSERT_I05_PACKET result=FAIL\nI_05 PASS=0 FAIL=1 BLOCKED=0');return 1
 t=P.read_text();c=[('ASSERT_I05_SCHEMA',d.get('schema_version')=='lsp-trace.program-c-portability-questions.v1'),('ASSERT_I05_OS_ARCH_MATRIX',d.get('required_os')==['linux','darwin','windows'] and d.get('required_arch')==['amd64','arm64']),('ASSERT_I05_TOOLCHAIN_QUESTIONS',all(x in t for x in ('exact Go version','compiler/toolchain','CGO','package and packaging mode'))),('ASSERT_I05_NATIVE_AND_FOREIGN_BOUNDARIES',all(x in t for x in ('native Go','C++','OpenMP','Python','igraph','compiled-extension'))),('ASSERT_I05_NO_PATH_OR_INSTALL',all(x in t for x in ('prohibit PATH lookup','do not install anything until separately admitted'))),('ASSERT_I05_TYPED_UNSUPPORTED','UNSUPPORTED/INCOMPLETE' in t and 'never infer PASS' in t),('ASSERT_I05_PREEXECUTION_CEILING','no platform coordinate has been tested or passed' in d.get('status','') and 'reports no build' in d.get('claim_ceiling',''))]
 p=sum(a(n,o) for n,o in c);f=len(c)-p;print(f'I_05 PASS={p} FAIL={f} BLOCKED=0');return int(f>0)
if __name__=='__main__':sys.exit(main())
