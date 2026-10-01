#!/usr/bin/env python3
"""Offline V4 candidate vectors; never an issuer or capability verifier."""
import argparse
import hashlib
import json
import struct
from pathlib import Path

ORIG = Path(__file__).resolve().parents[1] / 'docs/qualification/originals'
OUT = ORIG / 'generic-lsp-v4-selector-vectors.proposed.json'
PINS = {
 'schema': ('adr0011-generic-envelope-v3.schema.json',21619,'7154843a373f8f55c9723b62b1691b7b9de7b6f66ea23082d504291f137701f1'),
 'transport': ('generic-lsp-exact-transport-v3.json',1555,'e2d326f1424a56afa7fbaafc7417ccec52f67e03b977e44c19bd3adb6d9ae01b'),
 'references': ('generic-lsp-references-exact-v4.json',1930,'0756ed60f6ec8582f29d6e1d02aff6f112dff9e0aba977472f6833911c41c077'),
 'definition': ('generic-lsp-definition-exact-v4.json',1839,'6155cc64a70c1d7928eb21edfec26df58a6740267141dbd94a30052cdd244a23'),
 'shared': ('generic-lsp-selector-applicability-v1.json',2247,'c73263cb1356abd9f2898e016ec31afbb6cb112924c881c0a1ef6034bb1d5a0d'),
 'v2_vectors': ('generic-lsp-v2-selector-vectors.json',2220,'6992776962db2a815c5f2a3ea9d96a89a7330ec40f6b862cd3894a1c13b7614e'),
 'v2_source': ('generic-lsp-source-selector-v2-proposed-vectors.json',5551,'7de0399f1fbd711abea08d26043492de4417574d8bdf13faf05577f14f48650f'),
 'v3_vectors': ('generic-lsp-v3-selector-vectors.proposed.json',4715,'d5a8e14631fbc10d4082ee1f8c2bfedd1b34c5821993986e301cb1f65141d0ee'),
 'v2_schema': ('adr0011-generic-envelope-v2.schema.json',13243,'f843389f811aea940ed5bf1d595f03dcb6de4fa97c4dafafc41ad83b5c1c7f8e'),
 'v2_transport': ('generic-lsp-exact-transport-v2.json',1206,'0519ef89b76d966141bece6ca24d9e186b6424113339a39fce68ae3f78ffa67f'),
 'v2_references': ('generic-lsp-references-exact-v2.json',724,'cd92d4167abc951432804991e576a52a43236f9061dfe21c412410a6a851ecc5'),
 'v2_definition': ('generic-lsp-definition-exact-v2.json',633,'f8fa1a0cb9f1379d69e347ef9573f6356d28f448f0cfb2529d5cd5747751a9af'),
 'v3_references': ('generic-lsp-references-exact-v3.json',1134,'8c76a71da5e888e278280bbb1802979b907ba7a4ad6b0aec910dfe06e24e225e'),
 'v3_definition': ('generic-lsp-definition-exact-v3.json',1043,'52dd415cb5a331462042103014991fcd4ff43a777d45ab5a9abd7d83a45a8051'),
}
def sha(b): return hashlib.sha256(b).hexdigest()
def lp(b):
 if isinstance(b,str): b=b.encode('utf8')
 return struct.pack('>Q',len(b))+b
def digest(fields): return 'sha256:'+sha(b''.join(map(lp,fields)))
def pinned(key):
 name,size,want=PINS[key];b=(ORIG/name).read_bytes()
 if size is not None and len(b)!=size or sha(b)!=want: raise ValueError('pin changed: '+key)
 return b
def common(method,role,artifact,docs,schema=None,policy=None,session='s',gen=1):
 return ['ADR0011-GENERIC-EXACT/4','GENERIC_LSP_'+method.upper()+'_EXACT_V4','textDocument/'+method,role,sha(docs['schema'] if schema is None else schema),sha(docs['transport']),sha(docs[method] if policy is None else policy),session,str(gen),sha(artifact)]
def source(method,entry,docs,schema=None,policy=None):
 session,gen,transaction=entry['identity']
 if type(gen) is not int or gen<1 or not session or not transaction: raise ValueError('bad TX')
 uri=bytes.fromhex(entry['uri_hex']);version=None if entry['version_hex'] is None else bytes.fromhex(entry['version_hex']);content=None if entry['content_hex'] is None else bytes.fromhex(entry['content_hex'])
 if not uri or version is None or content is None: raise ValueError('absent SOURCE')
 uri.decode('utf8');version.decode('utf8')
 if entry['custody'] not in ('OWNER_BUFFER','MANAGED_VIRTUAL','CLEAN_REGISTERED_WORKTREE','IMMUTABLE_SOURCE_SNAPSHOT'): raise ValueError('bad custody')
 tx=digest(['ADR0011-GENERIC-TRANSACTION/1',session,str(gen),transaction])
 artifact=b''.join(map(lp,['ADR0011-GENERIC-SOURCE-ARTIFACT/1',tx,uri,'present',version,str(len(content)),sha(content),entry['custody']]))
 return digest(common(method,'SOURCE',artifact,docs,schema,policy,session,gen)+[tx])
def frame(obj):
 body=json.dumps(obj,sort_keys=True,separators=(',',':')).encode()
 return b'Content-Length: '+str(len(body)).encode()+b'\r\n\r\n'+body
def exchange_artifact(tx,write,exchanges,initialized_notification,observed):
 fields=['ADR0011-GENERIC-CAPABILITY-EXCHANGE-ARTIFACT/2',tx,str(write),str(len(exchanges))]
 for x in exchanges:
  fields.extend([x['request_direction'],x['request_frame'],x['request_id'],x['request_method'],str(x['request_ordinal']),x['response_direction'],x['response_frame'],x['response_id'],str(x['response_ordinal']),x['status']])
 direction,ordinal,raw=initialized_notification
 fields.extend([direction,str(ordinal),raw])
 fields.append(str(len(observed)))
 for direction,ordinal,raw in observed: fields.extend([direction,str(ordinal),raw])
 return b''.join(map(lp,fields))
def strict_frame(raw):
 header,sep,body=raw.partition(b'\r\n\r\n')
 if not sep or not header.startswith(b'Content-Length: ') or not header[16:].isdigit() or int(header[16:])!=len(body):raise ValueError('invalid complete frame')
 def unique(pairs):
  d={}
  for k,v in pairs:
   if k in d:raise ValueError('duplicate decoded key')
   d[k]=v
  return d
 def reject_non_json_constant(token):
  raise ValueError('non-JSON constant: '+token)
 return json.loads(body.decode('utf8'),object_pairs_hook=unique,parse_constant=reject_non_json_constant)
def validate_exchange(x,observed,write):
 request=strict_frame(x['request_frame'])
 if not isinstance(request,dict):raise ValueError('request is not an object')
 rid=request.get('id')
 if type(rid) not in (str,int) or rid=='' or request.get('jsonrpc')!='2.0' or request.get('method')!=x['request_method'] or request.get('method') not in ('initialize','client/registerCapability','client/unregisterCapability') or not isinstance(request.get('params'),dict) or 'result' in request or 'error' in request:raise ValueError('invalid request')
 typed=('number:' if type(rid) is int else 'string:')+str(rid)
 if x['request_id']!=typed:raise ValueError('request ID mirror')
 method=x['request_method'];req_dir='CLIENT_TO_SERVER' if method=='initialize' else 'SERVER_TO_CLIENT';resp_dir='SERVER_TO_CLIENT' if method=='initialize' else 'CLIENT_TO_SERVER'
 if x['request_direction']!=req_dir or x['request_ordinal']<0:raise ValueError('request direction/ordinal')
 required=[(req_dir,x['request_ordinal'],x['request_frame'])]
 if x['response_frame']:
  response=strict_frame(x['response_frame'])
  if not isinstance(response,dict) or response.get('jsonrpc')!='2.0' or 'method' in response or 'params' in response:raise ValueError('invalid response object')
  if type(response.get('id')) is not type(rid) or response.get('id')!=rid or x['response_id']!=typed or x['response_direction']!=resp_dir or x['response_ordinal']<=x['request_ordinal']:raise ValueError('mismatched or reversed response')
  if ('result' in response)==('error' in response):raise ValueError('response outcome shape')
  if 'result' in response:
   result=response['result']
   if method=='initialize' and (not isinstance(result,dict) or not isinstance(result.get('capabilities'),dict)):raise ValueError('invalid initialize result')
   if method!='initialize' and result is not None:raise ValueError('invalid capability acknowledgement')
   if 'response_result' not in x or json.dumps(x['response_result'],sort_keys=True,separators=(',',':'))!=json.dumps(result,sort_keys=True,separators=(',',':')) or x.get('response_error') is not None:raise ValueError('result mirror mismatch')
  else:
   error=response['error']
   if not isinstance(error,dict) or type(error.get('code')) is not int or not isinstance(error.get('message'),str) or set(error)-{'code','message','data'}:raise ValueError('invalid JSON-RPC error')
   if 'response_error' not in x or json.dumps(x['response_error'],sort_keys=True,separators=(',',':'))!=json.dumps(error,sort_keys=True,separators=(',',':')) or x.get('response_result') is not None:raise ValueError('error mirror mismatch')
  if x['status']!=('ERROR' if 'error' in response else 'SUCCESS'):raise ValueError('response status mirror')
  required.append((resp_dir,x['response_ordinal'],x['response_frame']))
 else:
  if x['status']!='PENDING' or x['response_id']!='ABSENT' or x['response_direction']!='ABSENT' or x['response_ordinal']!=-1 or x.get('response_result') is not None or x.get('response_error') is not None:raise ValueError('pending mirror')
 if sorted(observed,key=lambda o:o[1])!=observed or observed!=required:raise ValueError('missing/extra/duplicate observed capability frame')
 return x['status']=='SUCCESS' and x['response_ordinal']<write
def validate_transaction(initialize,events,initialized_notification,observed,write):
 def pair(x):
  rows=[(x['request_direction'],x['request_ordinal'],x['request_frame'])]
  if x['response_frame']:rows.append((x['response_direction'],x['response_ordinal'],x['response_frame']))
  validate_exchange(x,rows,write)
  return rows
 if initialize['request_method']!='initialize' or initialize['status']!='SUCCESS' or initialize['response_ordinal']>=write:raise ValueError('incomplete initialization')
 expected=pair(initialize)
 direction,ordinal,raw=initialized_notification
 notification=strict_frame(raw)
 if direction!='CLIENT_TO_SERVER' or ordinal<=initialize['response_ordinal'] or ordinal>=write or type(notification) is not dict or set(notification)!={'jsonrpc','method','params'} or notification['jsonrpc']!='2.0' or notification['method']!='initialized' or notification['params']!={}:raise ValueError('invalid initialized notification')
 expected.append(initialized_notification)
 for x in events:
  if x['request_method'] not in ('client/registerCapability','client/unregisterCapability') or x['request_ordinal']<=ordinal or x['request_ordinal']>=write:raise ValueError('premature capability request')
  rows=pair(x)
  if x['response_frame'] and x['response_ordinal']<=ordinal:raise ValueError('premature capability response')
  expected.extend(rows)
 expected.sort(key=lambda item:item[1])
 if len(set(o[1] for o in expected))!=len(expected) or observed!=expected:raise ValueError('missing/extra/duplicate held frame')
def query_artifact(tx,query_sel,source_sel,uri,language,kind='ORDINARY',notebook_type=None,notebook_uri=None,notebook_source=None,workspace='file:///workspace'):
 fields=['ADR0011-GENERIC-QUERY-APPLICABILITY-ARTIFACT/1',tx,query_sel,source_sel,uri,'present' if language is not None else 'absent',language or b'',kind,'present' if notebook_type is not None else 'absent',notebook_type or b'','present' if notebook_uri is not None else 'absent',notebook_uri or b'',notebook_source or '',workspace,'s','1','tx-1']
 return b''.join(map(lp,fields))
def vectors():
 docs={k:pinned(k) for k in PINS}
 for m in ('references','definition'):
  obj=json.loads(docs[m]);assert obj['shared_applicability_sha256']==PINS['shared'][2] and obj['selector_domain']=='ADR0011-GENERIC-EXACT/4'
 old=json.loads(docs['v2_vectors']);sources=json.loads(docs['v2_source']);v3=json.loads(docs['v3_vectors']);query=bytes.fromhex(old['query_artifact_utf8_hex']);event=bytes.fromhex(old['event_artifact_utf8_hex'])
 result={'status':'UNACCEPTED_OFFLINE_CANDIDATE','domain':'ADR0011-GENERIC-EXACT/4','pins':{k:{'length':len(docs[k]),'sha256':sha(docs[k])} for k in PINS},'selectors':{},'historical_collision_count':0}
 sels=result['selectors']
 for m in ('references','definition'):
  cases=[('query_version1_line0','QUERY',query,['file:///a','buffer:v1',sha(b'a'),'utf-16','0','0'],None,None),('query_version2_line0','QUERY',query,['file:///a','buffer:v2',sha(b'a'),'utf-16','0','0'],None,None),('query_version1_line1','QUERY',query,['file:///a','buffer:v1',sha(b'a'),'utf-16','1','0'],None,None),('event_ordinal0','TARGET_EVENTS',event,['observed-key-1',sha(b'[]'),'0'],None,None),('event_ordinal1','TARGET_EVENTS',event,['observed-key-1',sha(b'[]'),'1'],None,None),('query_schema_substitution','QUERY',query,['file:///a','buffer:v1',sha(b'a'),'utf-16','0','0'],docs['schema']+b'\n',None),('query_policy_substitution','QUERY',query,['file:///a','buffer:v1',sha(b'a'),'utf-16','0','0'],None,docs[m]+b'\n')]
  for name,role,artifact,extra,schema,policy in cases:sels[m+'_'+name]=digest(common(m,role,artifact,docs,schema,policy)+extra)
  for name,entry in sources['cases'].items():sels[m+'_source_'+name]=source(m,entry,docs)
  for name,kw in [('schema_lf',{'schema':docs['schema']+b'\n'}),('policy_lf',{'policy':docs[m]+b'\n'})]:sels[m+'_source_'+name]=source(m,sources['cases']['query'],docs,**kw)
  for name,entry in sources['rejected_inputs'].items():
   try:source(m,entry,docs)
   except (ValueError,UnicodeError):pass
   else:raise AssertionError('invalid SOURCE: '+name)
  tx=digest(['ADR0011-GENERIC-TRANSACTION/1','s','1','tx-1']);src_sel=sels[m+'_source_query'];qsel=sels[m+'_query_version1_line0']
  for name,uri,lang,kind,nt,nu,ns in [('ordinary',b'file:///query',b'go','ORDINARY',None,None,None),('uri_byte',b'file:///querY',b'go','ORDINARY',None,None,None),('notebook',b'file:///cell',b'python','NOTEBOOK_CELL',b'jupyter',b'file:///book','sha256:'+sha(b'held-notebook-source'))]:
   art=query_artifact(tx,qsel,src_sel,uri,lang,kind,nt,nu,ns);sels[m+'_query_applicability_'+name]=digest(common(m,'QUERY_APPLICABILITY',art,docs)+[tx])
  register=frame({'jsonrpc':'2.0','id':1,'method':'client/registerCapability','params':{'registrations':[{'id':'r','method':'textDocument/'+m,'registerOptions':{'documentSelector':[{'language':'go'}]}}]}})
  unregister=frame({'jsonrpc':'2.0','id':1,'method':'client/unregisterCapability','params':{'unregisterations':[{'id':'r','method':'textDocument/'+m}]}})
  initialize=frame({'jsonrpc':'2.0','id':1,'method':'initialize','params':{}})
  success=frame({'jsonrpc':'2.0','id':1,'result':None});init_success=frame({'jsonrpc':'2.0','id':1,'result':{'capabilities':{}}});failure=frame({'jsonrpc':'2.0','id':1,'error':{'code':-32603,'message':'failed'}})
  initialized=('CLIENT_TO_SERVER',2,frame({'jsonrpc':'2.0','method':'initialized','params':{}}))
  init={'request_direction':'CLIENT_TO_SERVER','request_frame':initialize,'request_id':'number:1','request_method':'initialize','request_ordinal':0,'response_direction':'SERVER_TO_CLIENT','response_frame':init_success,'response_id':'number:1','response_ordinal':1,'status':'SUCCESS','response_result':{'capabilities':{}},'response_error':None}
  for name,req,method,response,status,ordinal,write in [
   ('success',register,'client/registerCapability',success,'SUCCESS',4,10),
   ('error',register,'client/registerCapability',failure,'ERROR',4,10),
   ('pending',register,'client/registerCapability',b'','PENDING',-1,10),
   ('initialize_success',initialize,'initialize',init_success,'SUCCESS',1,10),
   ('unregister_success',unregister,'client/unregisterCapability',success,'SUCCESS',4,10),
   ('late_success',register,'client/registerCapability',success,'SUCCESS',11,10),
  ]:
   exchanges=[init];events=[]
   observed=[('CLIENT_TO_SERVER',0,initialize),('SERVER_TO_CLIENT',1,init_success),initialized]
   if name!='initialize_success':
    x={'request_direction':'SERVER_TO_CLIENT','request_frame':req,'request_id':'number:1','request_method':method,'request_ordinal':3,'response_direction':'CLIENT_TO_SERVER' if response else 'ABSENT','response_frame':response,'response_id':'number:1' if response else 'ABSENT','response_ordinal':ordinal,'status':status,'response_result':None,'response_error':{'code':-32603,'message':'failed'} if status=='ERROR' else None}
    events=[x];exchanges.append(x);observed.append(('SERVER_TO_CLIENT',3,req))
    if response:observed.append(('CLIENT_TO_SERVER',ordinal,response))
   validate_transaction(init,events,initialized,observed,write)
   art=exchange_artifact(tx,write,exchanges,initialized,observed);sels[m+'_capability_exchange_'+name]=digest(common(m,'CAPABILITY_EVENTS',art,docs)+[tx])
  nan_request=frame({'jsonrpc':'2.0','id':1,'method':'client/registerCapability','params':{'registrations':[],'x':float('nan')}})
  nan_response=frame({'jsonrpc':'2.0','id':1,'result':None,'x':float('nan')})
  bad={'request_direction':'SERVER_TO_CLIENT','request_frame':register,'request_id':'number:1','request_method':'client/registerCapability','request_ordinal':1,'response_direction':'CLIENT_TO_SERVER','response_frame':success,'response_id':'number:1','response_ordinal':2,'status':'SUCCESS','response_result':None,'response_error':None}
  for name,change,stream in [
   ('mismatched_id',{'response_frame':frame({'jsonrpc':'2.0','id':2,'result':None})},None),
   ('boolean_response_id',{'response_frame':frame({'jsonrpc':'2.0','id':True,'result':None})},None),
   ('response_with_request_method',{'response_frame':frame({'jsonrpc':'2.0','id':1,'method':'client/registerCapability','result':None})},None),
   ('non_json_nan_request',{'request_frame':nan_request},[('SERVER_TO_CLIENT',1,nan_request),('CLIENT_TO_SERVER',2,success)]),
   ('non_json_nan_response',{'response_frame':nan_response},[('SERVER_TO_CLIENT',1,register),('CLIENT_TO_SERVER',2,nan_response)]),
   ('null_error',{'response_frame':frame({'jsonrpc':'2.0','id':1,'error':None}),'status':'ERROR','response_error':{'code':-32603,'message':'failed'}},None),
   ('boolean_error_code',{'response_frame':frame({'jsonrpc':'2.0','id':1,'error':{'code':True,'message':'failed'}}),'status':'ERROR','response_error':{'code':True,'message':'failed'}},None),
   ('missing_error_message',{'response_frame':frame({'jsonrpc':'2.0','id':1,'error':{'code':-32603}}),'status':'ERROR','response_error':{'code':-32603}},None),
   ('error_mirror_mismatch',{'response_frame':frame({'jsonrpc':'2.0','id':1,'error':{'code':-32603,'message':'failed'}}),'status':'ERROR','response_error':{'code':-32602,'message':'failed'}},None),
   ('reversed_response',{'response_ordinal':0},None),
   ('wrong_direction',{'response_direction':'SERVER_TO_CLIENT'},None),
   ('duplicate_response',{},[('SERVER_TO_CLIENT',1,register),('CLIENT_TO_SERVER',2,success),('CLIENT_TO_SERVER',3,success)]),
   ('missing_response_frame',{},[('SERVER_TO_CLIENT',1,register)]),
  ]:
   x={**bad,**change};observed=stream if stream is not None else [('SERVER_TO_CLIENT',1,register),('CLIENT_TO_SERVER',x['response_ordinal'],x['response_frame'])]
   try:validate_exchange(x,observed,10)
   except ValueError:pass
   else:raise AssertionError('invalid capability exchange admitted: '+name)
 if len(sels)!=54 or len(set(sels.values()))!=54:raise AssertionError('V4 vector coverage/distinctness')
 historical=set(old['selectors'].values())|set(sources['selectors'].values())|set(v3['selectors'].values())
 if historical & set(sels.values()):raise AssertionError('historical collision')
 return (json.dumps(result,sort_keys=True,separators=(',',':'))+'\n').encode()
if __name__=='__main__':
 ap=argparse.ArgumentParser();ap.add_argument('--check',action='store_true');a=ap.parse_args();out=vectors()
 if a.check:
  if OUT.read_bytes()!=out:raise SystemExit('V4 candidate vector mismatch')
 else:OUT.write_bytes(out)
 print('V4_CANDIDATE_VECTORS_'+('CHECKED' if a.check else 'WRITTEN')+' sha256:'+sha(out)+' bytes:'+str(len(out)))
