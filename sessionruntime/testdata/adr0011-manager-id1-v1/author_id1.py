#!/usr/bin/env python3
"""Private synthetic ID1 fixture author; NOT a manager capture or authority claim.

Historical algorithms inspected (not executed): ROOT transcript write events
233ce2aca6e630dfee476f1a4c8563a431d918bb3bd78c3d15819d0ae5df27d5
21a1fca7e6a923e2b41303efa02828cacaa267bcb1904fe23a38af96bc618f82.
Each case is staged originals -> claimants -> responses, after source-first
independent EXPECTATION.json files have already been pinned and reviewed.
"""
import argparse
import hashlib
import json
import struct
from pathlib import Path

ROOT = Path('/Users/schwa/dev/lsp-trace')
E = ROOT / '.pi/evidence'
OUT = E / 'adr0011-composed-b4-manager-id1-held-v1'
RAW = E / 'adr0011-definition-b4b-held-originals-v1/CORE'
TARGETS = E / 'adr0011-definition-response-held-originals-v1-corrected/targets'
DOC = ROOT / 'docs/qualification/originals'
PINS = {
    E/'adr0011-composed-b4-manager-id1-derivation-v1/DERIVATION.md': '6180b736af5ec7e70e366425e8e658ec48ba64e3aae40ccdb19ac6487888d069',
    E/'adr0011-definition-bridge-independent-oracle-v1/oracle.json': 'd82cf78ae7a8c024d22a5eac3b31e1a3c8e5a8f6dfc9c26630f97f5296432b59',
    RAW/'source.bytes': '33fe4fbf4871969c874b738e0912e1199bee4781213bf50ed99bb62a19777374',
    TARGETS/'target-a.go': '2b61fbe6058fe0c78aeb9ba3a52a92efc4ccb9fc3e9280275b44c85f9bb15a3c',
    TARGETS/'target-b.go': '318cb48346e30c507f390c86e5dceaaac490a02f8df0deff7f205db558266176',
    DOC/'adr0011-generic-envelope-v4.schema.json': 'df4908187030e3d3110bd3745290eb25d8a3934ffb2a85c5d47d8d22dc28f6f3',
    DOC/'generic-lsp-exact-transport-v4.json': 'f3e25fab98cc4b8b96b76978cf56220ccc81f4ef41455ab0401041ea281d0bc8',
    DOC/'generic-lsp-definition-exact-v5.json': 'da62f5a161532fcb7c79dd93b0dca940a460c6e0c5fab83e8987de21c91b4dd3',
}
EXPECTED = {
    'A': ('0770c2ce9cb397e5a0bda77b3ddf3f0978aa65d0ea94d50d87a05777518ee48e', 'sk1:fcbe2d08e21ac9e5dd973cf650d6b6e001fad9873942df4cecdb9bccb52246ab', 'b4-a', 'b4-manager-a-definition-id1', 'b4-owner-a-id1', 'b4-query-a'),
    'B': ('5632d69969c8e2e9f54ef692b93137624f351bf1d6018b6cc5a583ebdecff763', 'sk1:4c9c6b713fe794d9bceb0926dec94939517eca5cbec20feffbe1a5f150c1ee30', 'b4-b', 'b4-manager-b-definition-id1', 'b4-owner-b-id1', 'b4-query-b'),
}
METHOD, URI, WORKSPACE, VERSION = 'textDocument/definition', 'file:///w/definition.go', 'file:///w', 'buffer:v1'
PARAMS = b'{"position":{"character":5,"line":1},"textDocument":{"uri":"file:///w/definition.go"}}'
# Literal independently predicted response results; not parsed from expectations.
RESULT = {
    'A': b'{"uri":"file:///w/target-a.go","range":{"start":{"line":1,"character":5},"end":{"line":1,"character":12}}}',
    'B': b'[{"uri":"file:///w/target-a.go","range":{"start":{"line":1,"character":5},"end":{"line":1,"character":12}}},{"uri":"file:///w/target-b.go","range":{"start":{"line":2,"character":5},"end":{"line":2,"character":12}}}]',
}
CAP_ORIGINS = ('00-initialize_request.frame', '01-initialize_response.frame', '02-initialized_notification.frame', '03-registration_request.frame', '05-registration_response.frame')
CAP_DIRS = ('CLIENT_TO_SERVER', 'SERVER_TO_CLIENT', 'CLIENT_TO_SERVER', 'SERVER_TO_CLIENT', 'CLIENT_TO_SERVER')


def digest(b): return hashlib.sha256(b).hexdigest()
def sha(b): return 'sha256:' + digest(b)
def lp(*items):
    chunks = (x if isinstance(x, bytes) else str(x).encode() for x in items)
    return b''.join(struct.pack('>Q', len(x)) + x for x in chunks)
def pretty(value): return (json.dumps(value, sort_keys=True, indent=2, ensure_ascii=False) + '\n').encode()
def compact(value): return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=False).encode()
def pinned(path, want):
    data = path.read_bytes()
    assert digest(data) == want, (str(path), digest(data), want)
    return data

def write_once(path, data):
    assert isinstance(data, bytes)
    path.parent.mkdir(parents=True, exist_ok=True)
    if path.exists():
        assert path.read_bytes() == data, ('immutable mismatch', str(path))
    else:
        with path.open('xb') as f: f.write(data)
    assert path.read_bytes() == data
    print(path.relative_to(OUT), len(data), digest(data))

def frame(body): return b'Content-Length: ' + str(len(body)).encode() + b'\r\n\r\n' + body
def message(raw):
    header, sep, body = raw.partition(b'\r\n\r\n')
    assert sep and header == b'Content-Length: ' + str(len(body)).encode()
    return json.loads(body)
def desc(data, ref): return {'length': len(data), 'sha256': sha(data), 'private_ref': ref}
def dep(role, selector, raw=None, digest_value=None):
    assert (raw is None) != (digest_value is None)
    return {'role': role, 'selector': selector, 'digest': sha(raw) if raw is not None else 'sha256:' + digest_value}

def keys(case):
    expectation, session, profile, transaction, owner, occurrence = EXPECTED[case]
    return dict(case=case, manager='manager-'+case.lower(), profile=profile, session=session, generation=1,
                key={'generation': 1, 'id': 1}, transaction=transaction, completed_owner_key=owner,
                method=METHOD, uri=URI, workspace=WORKSPACE, source_version=VERSION,
                source_custody='OWNER_BUFFER', position_encoding='utf-16', line=1, character=5,
                language='go', document_kind='ORDINARY', query_occurrence_id=occurrence,
                client_selector='[{"scheme":"file"}]', expected_sha256=expectation,
                profile_selector={'trust_domain':'fixture', 'workspace':'/w', 'profile':profile,
                                  'environment_reference':'local', 'options':[]},
                scope='PRIVATE_SYNTHETIC_NOT_OBSERVED_OR_AUTHENTICATED')

def validate_expectation(case):
    k=keys(case)
    b=pinned(OUT/case/'EXPECTATION.json', k['expected_sha256'])
    obj=json.loads(b)
    assert obj['case']==case and not obj['custody']['accepted'] and obj['custody']['authority']==0
    assert obj['custody']['completeness']=='UNKNOWN'
    rows=obj['targets']
    assert len(rows)==(1 if case=='A' else 2) and obj['candidate_count']==len(rows)
    source_a=pinned(TARGETS/'target-a.go', PINS[TARGETS/'target-a.go'])
    source_b=pinned(TARGETS/'target-b.go', PINS[TARGETS/'target-b.go'])
    assert source_a.splitlines()[1][5:12]==b'alphaZZ' and source_b.splitlines()[2][5:12]==b'betaZZZ'
    for i,r in enumerate(rows):
        name='target-a.go' if i==0 else 'target-b.go'
        assert r['ordinal']==i and r['kind']=='LOCATION' and r['uri']=='file:///w/'+name
        assert r['target_range'] is None and r['target_source_sha256']==PINS[TARGETS/name]
        assert r['selection_range']=={'start':{'line': 1 if i==0 else 2,'character':5},
                                      'end':{'line': 1 if i==0 else 2,'character':12}}
    return k

def external_pins():
    return {str(p):digest(pinned(p,d)) for p,d in PINS.items()}

def originals(case):
    k=validate_expectation(case)
    assert len(PARAMS)==86 and json.loads(PARAMS)=={'textDocument':{'uri':URI},'position':{'line':1,'character':5}}
    c=OUT/case
    # The pre-call declaration is the first new asset, never inferred from a response.
    write_once(c/'DECLARATION.json', pretty(k))
    write_once(c/'source.bytes', pinned(RAW/'source.bytes', PINS[RAW/'source.bytes']))
    for name in ('target-a.go','target-b.go'):
        write_once(c/name, pinned(TARGETS/name,PINS[TARGETS/name]))
    write_once(c/'request.params', PARAMS)
    write_once(c/'client-selector.json', b'[{"scheme":"file"}]')
    for i,name in enumerate(CAP_ORIGINS):
        old=(RAW/name).read_bytes()
        m=message(old)
        assert m['jsonrpc']=='2.0' and (m.get('method') in ('initialize','initialized','client/registerCapability') or 'result' in m)
        write_once(c/f'cap-{i}.frame', old)
    # Go lspwire.Message JSON struct order, unlike the historical ID141/142 map order.
    request=b'{"jsonrpc":"2.0","id":1,"method":"textDocument/definition","params":'+PARAMS+b'}'
    assert len(request)==155 and message(frame(request))['id']==1
    write_once(c/'request.frame',frame(request))
    src=(c/'source.bytes').read_bytes()
    query={'character':5,'encoding':'utf-16','generation':1,'line':1,'method':METHOD,
           'session':k['session'],'sourceSha256':sha(src),
           'target':{'ordinal':5,'ownerKey':k['completed_owner_key'],'role':'WRITE',
                     'wireId':{'type':'number','value':1}},
           'transaction':k['transaction'],'uri':URI,'version':VERSION,'workspace':WORKSPACE}
    write_once(c/'query.original',compact(query)+b'\n')
    print('ORIGINALS_READY',case)

def selector(role, artifact, k, *suffix):
    s=pinned(DOC/'adr0011-generic-envelope-v4.schema.json',PINS[DOC/'adr0011-generic-envelope-v4.schema.json'])
    t=pinned(DOC/'generic-lsp-exact-transport-v4.json',PINS[DOC/'generic-lsp-exact-transport-v4.json'])
    p=pinned(DOC/'generic-lsp-definition-exact-v5.json',PINS[DOC/'generic-lsp-definition-exact-v5.json'])
    return sha(lp('ADR0011-GENERIC-EXACT/5','GENERIC_LSP_DEFINITION_EXACT_V5',METHOD,
                  role,digest(s),digest(t),digest(p),k['session'],'1',digest(artifact),*suffix))

def claims(case):
    k=validate_expectation(case); c=OUT/case
    assert json.loads(pinned(c/'DECLARATION.json',digest(pretty(k))))==k
    src=pinned(c/'source.bytes',PINS[RAW/'source.bytes'])
    q=(c/'query.original').read_bytes(); assert json.loads(q)['target']=={'ordinal':5,'ownerKey':k['completed_owner_key'],'role':'WRITE','wireId':{'type':'number','value':1}}
    request=(c/'request.frame').read_bytes(); assert message(request)=={'jsonrpc':'2.0','id':1,'method':METHOD,'params':json.loads(PARAMS)}
    assert (c/'request.params').read_bytes()==PARAMS
    frames=[(CAP_DIRS[i],(c/f'cap-{i}.frame').read_bytes()) for i in range(5)]
    assert [message(f).get('id') for _,f in frames]==[10,10,None,'definition-register','definition-register']
    assert message(frames[1][1])['result']=={'capabilities':{'definitionProvider':False}}
    assert message(frames[3][1])['params']['registrations'][0]['registerOptions']['documentSelector'] is None
    assert message(frames[4][1])['result'] is None
    tx=sha(lp('ADR0011-GENERIC-TRANSACTION/1',k['session'],'1',k['transaction']))
    src_art=lp('ADR0011-GENERIC-SOURCE-ARTIFACT/1',tx,URI,'present',VERSION,str(len(src)),digest(src),'OWNER_BUFFER')
    source_sel=selector('SOURCE',src_art,k,tx)
    query_sel=selector('QUERY',q,k,URI,VERSION,digest(src),'utf-16','1','5')
    app=lp('ADR0011-GENERIC-QUERY-APPLICABILITY-ARTIFACT/1',tx,query_sel,source_sel,
           URI,'present','go','ORDINARY','absent','','absent','','',WORKSPACE,k['session'],'1',k['transaction'])
    app_sel=selector('QUERY_APPLICABILITY',app,k,tx)
    # Synthetic capability chronology: observed five predecessor frames, WRITE ordinal 5.
    observed=[]; fields=[b'ADR0011-GENERIC-CAPABILITY-EXCHANGE-ARTIFACT/2',tx.encode(),b'5',b'2']
    exchanges=[]
    for request_idx,response_idx in ((0,1),(3,4)):
        direc,wire=frames[request_idx]; rdirec,rwire=frames[response_idx]
        m=message(wire); rm=message(rwire); wire_id=m['id']
        assert rm['id']==wire_id and type(rm['id']) is type(wire_id) and 'result' in rm and 'error' not in rm
        canonical=('number:' if type(wire_id) is int else 'string:')+str(wire_id)
        claim={'request_direction':direc,'request_frame':desc(wire,f'held:{case}/cap-{request_idx}.frame'),
               'request_frame_ordinal':request_idx,'request_id':wire_id,'request_method':m['method'],
               'request_params':m['params'],'response_direction':rdirec,
               'response_frame':desc(rwire,f'held:{case}/cap-{response_idx}.frame'),
               'response_frame_ordinal':response_idx,'response_id':rm['id'],
               'response_status':'SUCCESS','response_result':rm['result'],
               'response_error':None,'target_write_frame_ordinal':5}
        exchanges.append(claim)
        fields += [direc.encode(),wire,canonical.encode(),m['method'].encode(),str(request_idx).encode(),
                   rdirec.encode(),rwire,canonical.encode(),str(response_idx).encode(),b'SUCCESS']
    fields += [frames[2][0].encode(),b'2',frames[2][1],b'5']
    for i,(direction,wire) in enumerate(frames):
        observed.append({'direction':direction,'frame':desc(wire,f'held:{case}/cap-{i}.frame'),'frame_ordinal':i})
        fields += [direction.encode(),str(i).encode(),wire]
    assert len(fields)==43
    cap=lp(*fields); cap_sel=selector('CAPABILITY_EVENTS',cap,k,tx)
    ident={'session':k['session'],'generation':1,'transaction':k['transaction']}
    def env(role,original,ref,preds,payload):
        return {'identity':ident,'original':desc(original,ref),'payload':payload,'predecessors':preds,'role':role}
    policy_sha=PINS[DOC/'generic-lsp-definition-exact-v5.json']
    schema_sha=PINS[DOC/'adr0011-generic-envelope-v4.schema.json']
    cap_env=env('CAPABILITY_EVENTS',cap,f'held:{case}/capability-artifact.bin',[],
        {'initialize':exchanges[0],'events':[exchanges[1]],
         'initialized_notification':{'direction':frames[2][0],
              'frame':desc(frames[2][1],f'held:{case}/cap-2.frame'),'frame_ordinal':2},
         'observed_frames':observed,'target_write_frame_ordinal':5})
    source_env=env('SOURCE',src,f'held:{case}/source.bytes',[],
        {'uri':URI,'version':VERSION,'custody':'OWNER_BUFFER',
         'source_bytes':desc(src,f'held:{case}/source.bytes')})
    query_env=env('QUERY',q,f'held:{case}/query.original',
        [dep('POLICY','private-context:definition-policy-v5',digest_value=policy_sha),
         dep('SCHEMA','private-context:envelope-v4',digest_value=schema_sha),
         dep('SOURCE',source_sel,raw=src)],
        {'uri':URI,'version':VERSION,'method':METHOD,'line':1,'character':5,
         'encoding':'utf-16','source':sha(src)})
    app_env=env('QUERY_APPLICABILITY',app,f'held:{case}/query-applicability.original',
        [dep('QUERY',query_sel,raw=q),dep('SOURCE',source_sel,raw=src)],
        {'workspace':WORKSPACE,'session':k['session'],'generation':1,'transaction':k['transaction'],
         'query_selector':query_sel,'query_source_selector':source_sel,
         'uri_bytes':desc(URI.encode(),f'held:{case}/uri.bytes'),
         'language_id_present':True,'language_id_bytes':desc(b'go',f'held:{case}/language.bytes'),
         'document_kind':'ORDINARY','notebook_type_bytes':None,'notebook_uri_bytes':None,
         'notebook_source_selector':None})
    write_env=env('REQUEST_WRITE',request,f'held:{case}/request.frame',
        [dep('QUERY',query_sel,raw=q),dep('QUERY_APPLICABILITY',app_sel,raw=app),
         dep('CAPABILITY_EVENTS',cap_sel,raw=cap),
         dep('POLICY','private-context:definition-policy-v5',digest_value=policy_sha)],
        {'params':desc(PARAMS,f'held:{case}/request.params'),
         'frame':desc(request,f'held:{case}/request.frame'),
         'actual_key':k['completed_owner_key'],'completed':True,'completed_frame_ordinal':5})
    for name,data in [('source-artifact.bin',src_art),('query-applicability.original',app),
                      ('capability-artifact.bin',cap),('uri.bytes',URI.encode()),('language.bytes',b'go')]:
        write_once(c/name,data)
    for name,obj in [('capability-envelope.json',cap_env),('source-envelope.json',source_env),
                     ('query-envelope.json',query_env),('query-applicability-envelope.json',app_env),
                     ('request-write-envelope.json',write_env)]:
        write_once(c/name,pretty(obj))
    write_once(c/'CLAIM_SELECTORS.json',pretty({'transaction':tx,'SOURCE':source_sel,'QUERY':query_sel,
       'QUERY_APPLICABILITY':app_sel,'CAPABILITY_EVENTS':cap_sel,'synthetic_frame_ordinals':[0,1,2,3,4,5],
       'manager_observed_capability_frames':False,'a4_runtime_validation':'PENDING_INDEPENDENT_REVIEW'}))
    print('CLAIMS_READY',case)

def responses(case):
    k=validate_expectation(case); c=OUT/case
    assert (c/'request-write-envelope.json').exists(), 'no claimant before response'
    raw=RESULT[case]; parsed=json.loads(raw)
    locations=[parsed] if case=='A' else parsed
    expected=json.loads(pinned(c/'EXPECTATION.json',k['expected_sha256']))['targets']
    assert len(locations)==len(expected)
    for location,row in zip(locations,expected):
        assert location=={'uri':row['uri'],'range':row['selection_range']}
    body=b'{"jsonrpc":"2.0","id":1,"result":'+raw+b'}'
    assert message(frame(body))=={'jsonrpc':'2.0','id':1,'result':parsed}
    write_once(c/'response.result',raw)
    write_once(c/'response.json',body)
    write_once(c/'response.frame',frame(body))
    print('RESPONSE_PREDICTION_READY_NOT_OBSERVED',case)

def manifest():
    for case in ('A','B'):
        k=validate_expectation(case); c=OUT/case
        for needed in ('DECLARATION.json','source.bytes','request.params','request.frame','query.original',
                       'query-applicability.original','capability-artifact.bin','capability-envelope.json',
                       'source-envelope.json','query-envelope.json','query-applicability-envelope.json',
                       'request-write-envelope.json','response.result','response.json','response.frame','CLAIM_SELECTORS.json'):
            assert (c/needed).is_file(), (case,needed)
        assert json.loads((c/'response.json').read_bytes())['id']==1
        assert json.loads((c/'request-write-envelope.json').read_bytes())['identity']['transaction']==k['transaction']
    files=sorted(p for p in OUT.rglob('*') if p.is_file() and p.name!='manifest.json')
    assets=[{'path':str(p.relative_to(OUT)),'length':p.stat().st_size,'sha256':digest(p.read_bytes())} for p in files]
    predecessors=[{'path':str(p),'sha256':digest(p.read_bytes())} for p in sorted(PINS)]
    data=pretty({'status':'PRIVATE_SYNTHETIC_MANAGER_ID1_ORIGINALS_PENDING_INDEPENDENT_REVIEW',
                 'custody':{'authority':0,'accepted':False,'completeness':'UNKNOWN',
                            'producer_authentication':'NO_PRODUCER_AUTHENTICATION'},
                 'scope':'A/B separately held predictions; not manager response or B4a/CAP validation',
                 'expectation_precedes_response':True,'manager_response_observed':False,
                 'manager_capability_frames_observed':False,'a4_b4a_b4b_runtime_validation':'PENDING',
                 'design_sha256':'a7a8f7bf1927cfc530113e9e29cdd35e15f1dd044b4b29e37bd1e1134b2a0876',
                 'predecessors':predecessors,'assets':assets,
                 'limitations':['historical builders supply algorithm examples, not ID1 custody',
                    'all responses are predictions; no manager-issued lease or selected response yet',
                    'no Go scaffold, semantic RED, implementation, issuance or qualification']})
    write_once(OUT/'manifest.json',data)
    print('MANIFEST_READY_PENDING_INDEPENDENT_REVIEW',len(assets),digest(data))

if __name__=='__main__':
    parser=argparse.ArgumentParser()
    parser.add_argument('stage',choices=['originals','claims','responses','manifest'])
    parser.add_argument('case',nargs='?',choices=['A','B'])
    args=parser.parse_args()
    external_pins()
    if args.stage=='manifest':
        assert args.case is None
        manifest()
    else:
        assert args.case is not None
        {'originals':originals,'claims':claims,'responses':responses}[args.stage](args.case)
