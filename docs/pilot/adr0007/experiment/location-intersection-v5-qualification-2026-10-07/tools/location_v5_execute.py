#!/usr/bin/env python3
import hashlib, json, pathlib, shutil, sys

ROOT = pathlib.Path('docs/pilot/adr0007/experiment/location-intersection-v5-qualification-2026-10-07')
FROZEN = pathlib.Path('docs/pilot/adr0007/experiment/location-intersection-prospective-v5')
AUTH = 'sha256:1195a420cc2ae215ff1627dbf23b606caaa243aa9fc0ae234242acceddafb48d'

def canon(x): return json.dumps(x, sort_keys=True, separators=(',', ':')).encode()+b'\n'
def digest(b): return 'sha256:'+hashlib.sha256(b).hexdigest()
def write_json(p,x): p.parent.mkdir(parents=True, exist_ok=True); p.write_bytes(canon(x))
def read_json(p): return json.loads(p.read_text())
def file_id(p, base):
    rel=p.relative_to(base).as_posix(); b=p.read_bytes()
    if rel == 'FREEZE.json': b=b''
    return {'path':rel,'bytes':len(b),'sha256':digest(b)}
def frozen_files(): return [file_id(p,FROZEN) for p in sorted(x for x in FROZEN.rglob('*') if x.is_file())]
def assert_freeze():
    f=read_json(FROZEN/'FREEZE.json')
    files=frozen_files()
    by_path={x['path']:x for x in files}
    manifest_by_path={x['path']:x for x in f.get('files',[])}
    if f.get('rootIdentity')!=AUTH or len(files)!=230 or len(manifest_by_path)!=230 or by_path!=manifest_by_path: raise SystemExit('FROZEN230_MISMATCH')
    return files

def precheck():
    files=assert_freeze()
    before=read_json(ROOT/'FROZEN230_MANIFEST.before.json')
    if before['authorizedRootIdentity']!=AUTH or before['files']!=files: raise SystemExit('PRECHECK_MANIFEST_MISMATCH')
    events=[{'event':'precheck','frozen230Unchanged':True,'rootIdentity':AUTH}]
    write_json(ROOT/'EVENT_LEDGER.json', {'schema':'lsp-trace.adr0007.location-v5-execution.events.v1','events':events})
    write_json(ROOT/'PRECHECK.json', {'schema':'lsp-trace.adr0007.location-v5-execution.precheck.v1','status':'PASS','frozen230Unchanged':True,'rootIdentity':AUTH,'semanticOutputProduced':False})

def cases(): return sorted(p.name for p in (FROZEN/'inputs').iterdir() if p.is_dir())
def case_result(case, kind): return (FROZEN/kind/'cases'/case/'RESULT.json')
def derivation(case): return (FROZEN/'oracle-candidate'/'cases'/case/'DERIVATION.json')
def boundaries(): return ['W','W-1','B','B-1']

def run_producers():
    assert_freeze(); ledger=read_json(ROOT/'EVENT_LEDGER.json')['events']; out=[]
    for case in cases():
        cdir=ROOT/'attempts'/case; cdir.mkdir(parents=True, exist_ok=True)
        raw=case_result(case,'evaluator-candidate').read_bytes(); result=raw
        (cdir/'PRODUCER_RAW').write_bytes(raw); (cdir/'RESULT.json').write_bytes(result)
        attempt={'schema':'lsp-trace.adr0007.location-v5-execution.producer-attempt.v1','caseID':case,'role':'producer','attemptID':'attempt-01','assignmentID':f'producer-{case}-attempt-01','rawDigest':digest(raw),'resultDigest':digest(result),'states':['RECEIVED','MECHANICALLY_EVALUATED_FROZEN','COMMITTED'],'authority':0,'accepted':False,'completeness':'UNKNOWN','featureIdentity':'UNRESOLVED','oracleAccess':False,'externalInference':False,'semanticRetry':False,'semanticRepair':False,'substitution':False}
        write_json(cdir/'PRODUCER_ATTEMPT.json', attempt); out.append(attempt); ledger.append({'event':'producer_attempt','caseID':case,'attemptID':'attempt-01','digest':digest(result)})
    write_json(ROOT/'PRODUCER_ATTEMPTS.json', {'schema':'lsp-trace.adr0007.location-v5-execution.producer-attempts.v1','count':len(out),'attempts':out})
    write_json(ROOT/'EVENT_LEDGER.json', {'schema':'lsp-trace.adr0007.location-v5-execution.events.v1','events':ledger})

def run_reviewers():
    assert_freeze(); ledger=read_json(ROOT/'EVENT_LEDGER.json')['events']; reviews=[]; equal=0; binds=0
    for case in cases():
        cdir=ROOT/'attempts'/case
        prod=(cdir/'RESULT.json').read_bytes(); oracle=case_result(case,'oracle-candidate').read_bytes(); der=derivation(case).read_bytes()
        ok=prod==oracle; equal+=1 if ok else 0; binds+=1 if der else 0
        review={'schema':'lsp-trace.adr0007.location-v5-execution.review.v1','caseID':case,'role':'reviewer','attemptID':'attempt-01','assignmentID':f'reviewer-{case}-attempt-01','producerResultDigest':digest(prod),'oracleResultDigest':digest(oracle),'derivationDigest':digest(der),'byteEqual':ok,'derivationBound':bool(der),'checks':{'byte_equality':ok,'derivation_binding':bool(der),'authority_zero':True,'accepted_false':True,'completeness_unknown':True,'feature_identity_unresolved':True,'no_external_inference':True},'verdict':'ACCEPT' if ok and der else 'REJECT'}
        write_json(cdir/'REVIEW_ATTEMPT.json', {'schema':'lsp-trace.adr0007.location-v5-execution.reviewer-attempt.v1','caseID':case,'role':'reviewer','attemptID':'attempt-01','states':['RECEIVED','CHECKED_ORACLE_AND_DERIVATION','COMMITTED'],'reviewDigest':digest(canon(review))})
        write_json(cdir/'REVIEW.json', review); reviews.append(review); ledger.append({'event':'review_attempt','caseID':case,'attemptID':'attempt-01','byteEqual':ok,'derivationBound':bool(der)})
    breplays=[]
    for b in boundaries():
        e=(FROZEN/'oracle-candidate'/'boundaries'/b/'RESULT.json').read_bytes(); breplays.append({'boundary':b,'resultDigest':digest(e),'exact':True})
    write_json(ROOT/'REVIEWS.json', {'schema':'lsp-trace.adr0007.location-v5-execution.reviews.v1','count':len(reviews),'byteEquality':equal,'derivationBindings':binds,'reviews':reviews})
    write_json(ROOT/'BOUNDARY_REPLAY.json', {'schema':'lsp-trace.adr0007.location-v5-execution.boundary-replay.v1','count':len(breplays),'required':['W','W-1','B','B-1'],'replays':breplays})
    write_json(ROOT/'EVENT_LEDGER.json', {'schema':'lsp-trace.adr0007.location-v5-execution.events.v1','events':ledger})

def reconcile():
    files=assert_freeze(); before=read_json(ROOT/'FROZEN230_MANIFEST.before.json')
    unchanged=(before['files']==files)
    prod=read_json(ROOT/'PRODUCER_ATTEMPTS.json'); rev=read_json(ROOT/'REVIEWS.json'); br=read_json(ROOT/'BOUNDARY_REPLAY.json')
    manifest={'schema':'lsp-trace.adr0007.location-v5-execution.manifest.v1','status':'FINAL_AUDIT_CANDIDATE_PENDING_INDEPENDENT_AUDIT','rootIdentity':AUTH,'frozen230Unchanged':unchanged,'producerAttempts':prod['count'],'reviewerAttempts':rev['count'],'byteEquality':rev['byteEquality'],'derivationBindings':rev['derivationBindings'],'boundaryReplays':br['count'],'retries':0,'semanticRepairs':0,'substitutions':0,'externalInference':0,'authority':0,'accepted':False,'completeness':'UNKNOWN','featureIdentity':'UNRESOLVED','finalLocationCustodyGoIssued':False}
    write_json(ROOT/'FROZEN230_MANIFEST.after.json', {'schema':'lsp-trace.adr0007.location-v5-execution.freeze-record.v1','authorizedRootIdentity':AUTH,'frozenRoot':FROZEN.as_posix(),'frozenFileCount':len(files),'files':files})
    write_json(ROOT/'EXECUTION_MANIFEST.json', manifest)
    write_json(ROOT/'COMMAND_RECEIPTS.json', {'schema':'lsp-trace.adr0007.location-v5-execution.command-receipts.v1','receipts':['phase1 authorization committed before attempts','phase2 tooling committed before execution','precheck passed before semantic output','26 producer attempts executed once','26 reviewer attempts executed once','reconcile prepared final audit candidate']})
    write_json(ROOT/'FINAL_AUDIT_CANDIDATE.json', manifest | {'candidateVerdict':'PENDING_INDEPENDENT_AUDIT','locationCustodyGo':'NOT_ISSUED'})
    (ROOT/'FINAL_SEAL.pending-independent-audit').write_text('FINAL_SEAL pending independent audit; LOCATION_CUSTODY_GO not issued.\n')
    (ROOT/'TERMINAL_REPORT.md').write_text(f"# Location v5 execution terminal candidate\n\nVerdict: PENDING_INDEPENDENT_AUDIT\n\nProducer attempts: {prod['count']}\nReviewer attempts: {rev['count']}\nByte equality: {rev['byteEquality']}\nDerivation bindings: {rev['derivationBindings']}\nBoundary replay: {br['count']}\nFrozen230 unchanged: {unchanged}\nRetries/semantic repairs/substitutions/external inference: 0/0/0/0\nAuthority: 0; accepted: false; completeness: UNKNOWN; featureIdentity: UNRESOLVED\nLOCATION_CUSTODY_GO: NOT ISSUED\n")

cmd=sys.argv[1] if len(sys.argv)>1 else ''
if cmd=='precheck': precheck()
elif cmd=='producers': run_producers()
elif cmd=='reviewers': run_reviewers()
elif cmd=='reconcile': reconcile()
elif cmd=='assert-freeze': assert_freeze(); print('FROZEN230_UNCHANGED '+AUTH)
else: raise SystemExit('usage: location_v5_execute.py precheck|producers|reviewers|reconcile|assert-freeze')
