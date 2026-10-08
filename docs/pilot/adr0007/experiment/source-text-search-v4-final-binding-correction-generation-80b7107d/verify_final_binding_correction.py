#!/usr/bin/env python3
import json, hashlib, pathlib, re, sys, copy
ROOT=pathlib.Path(__file__).resolve().parents[5]
NS=pathlib.Path(__file__).resolve().parent
SHA=re.compile(r'^sha256:[0-9a-f]{64}$')
def h(p): return 'sha256:'+hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest()
def load(n): return json.loads((NS/n).read_text())
def fail(m): raise SystemExit('VERIFY_FAIL: '+m)
def check_sha(s):
    if not isinstance(s,str) or not SHA.match(s): fail('bad sha pattern '+repr(s))
def rec_ok(r):
    check_sha(r['sha256']); p=ROOT/r['path'];
    if not p.exists(): fail('missing '+str(p))
    if h(p)!=r['sha256']: fail('digest mismatch '+str(p))
    if p.stat().st_size!=r['bytes']: fail('byte mismatch '+str(p))
def main():
    auth=load('AUTHORIZED_BINDINGS.json'); audit=load('FINAL_AUDIT.json'); manifest=load('CAMPAIGN_MANIFEST.json'); seal=load('CAMPAIGN_SEAL.json')
    for s in [auth['predecessor_bindings']['terminal_event_sha256'],auth['predecessor_bindings']['ledger_sha256']]: check_sha(s)
    if auth['predecessor_bindings']['terminal_event_sha256'].startswith('sha256:bea1b32') and len(auth['predecessor_bindings']['terminal_event_sha256']) != 71: fail('terminal prefix/truncation accepted')
    if auth['predecessor_bindings']['ledger_sha256'].startswith('sha256:a4e6549') and len(auth['predecessor_bindings']['ledger_sha256']) != 71: fail('ledger prefix/truncation accepted')
    rec_ok(auth['predecessor_bindings']['ledger_file'])
    if auth['predecessor_bindings']['ledger_file']['sha256'] != auth['predecessor_bindings']['ledger_sha256']: fail('ledger substitution')
    for r in auth['prior_independent_review_generation']['files'].values(): rec_ok(r)
    js=auth['prior_independent_review_generation']['judgment_records']
    if len(js)!=50: fail('judgment count')
    seen=set()
    for j in js:
        check_sha(j['sha256']);
        if j['sha256'] in seen: fail('duplicate judgment digest')
        seen.add(j['sha256'])
        p=ROOT/j['path']
        if h(p)!=j['sha256']: fail('judgment substitution '+str(p))
    if audit['prior_independent_review_generation']['all_50_judgment_digests'] != auth['prior_independent_review_generation']['all_50_judgment_digests']: fail('audit/auth judgment divergence')
    ev=(NS/'EVENTS.ndjson').read_text().splitlines()
    names=[json.loads(x)['event'] for x in ev]
    if names != ['BINDING_CORRECTION_AUTHORIZED','AUDIT','SEAL']: fail('event chain changed '+repr(names))
    if seal['preseal_ledger_sha256'] != 'sha256:'+hashlib.sha256(('\n'.join((NS/'EVENTS.ndjson').read_text().splitlines()[:2])+'\n').encode()).hexdigest(): fail('seal preseal ledger mismatch')
    for key in ['audit_sha256','manifest_sha256']:
        check_sha(seal[key])
    if seal['audit_sha256'] != h(NS/'FINAL_AUDIT.json'): fail('seal audit mismatch')
    if seal['manifest_sha256'] != h(NS/'CAMPAIGN_MANIFEST.json'): fail('seal manifest mismatch')
    if auth.get('not_go') is not True or auth.get('not_public') is not True: fail('custody/public self claim')
    print('VERIFY_OK final-binding-correction full-sha predecessor/prior/judgment bindings validated; negative prefix/truncation/substitution checks active')
if __name__=='__main__': main()
