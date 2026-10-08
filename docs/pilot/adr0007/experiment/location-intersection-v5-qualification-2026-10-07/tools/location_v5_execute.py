#!/usr/bin/env python3
import hashlib, json, pathlib, sys

ROOT = pathlib.Path('docs/pilot/adr0007/experiment/location-intersection-v5-qualification-2026-10-07')
FROZEN = pathlib.Path('docs/pilot/adr0007/experiment/location-intersection-prospective-v5')
AUTH = 'sha256:1195a420cc2ae215ff1627dbf23b606caaa243aa9fc0ae234242acceddafb48d'

def digest(b): return 'sha256:'+hashlib.sha256(b).hexdigest()
def read_json(p): return json.loads(p.read_text())
def file_id(p, base):
    rel=p.relative_to(base).as_posix(); b=p.read_bytes()
    if rel == 'FREEZE.json': b=b''
    return {'path':rel,'bytes':len(b),'sha256':digest(b)}
def frozen_files(): return [file_id(p,FROZEN) for p in sorted(x for x in FROZEN.rglob('*') if x.is_file())]
def assert_freeze():
    f=read_json(FROZEN/'FREEZE.json')
    files=frozen_files(); by_path={x['path']:x for x in files}; manifest_by_path={x['path']:x for x in f.get('files',[])}
    if f.get('rootIdentity')!=AUTH or len(files)!=230 or len(manifest_by_path)!=230 or by_path!=manifest_by_path: raise SystemExit('FROZEN230_MISMATCH')
    return files

def precheck():
    files=assert_freeze(); before=read_json(ROOT/'FROZEN230_MANIFEST.before.json')
    if before['authorizedRootIdentity']!=AUTH or before['files']!=files: raise SystemExit('PRECHECK_MANIFEST_MISMATCH')
    print('PREDISPATCH_PRECHECK_PASS '+AUTH)

cmd=sys.argv[1] if len(sys.argv)>1 else ''
if cmd=='assert-freeze':
    assert_freeze(); print('FROZEN230_UNCHANGED '+AUTH)
elif cmd=='precheck':
    precheck()
elif cmd in {'producers','reviewers','reconcile','run','verify'}:
    raise SystemExit('SEMANTIC_COMMAND_DISABLED_USE_GO_LOCATIONEXECUTIONV5')
else:
    raise SystemExit('usage: location_v5_execute.py assert-freeze|precheck')
