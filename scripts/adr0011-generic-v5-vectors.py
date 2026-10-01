#!/usr/bin/env python3
"""Offline V5 candidate vectors; never an issuer or capability verifier."""
import argparse
import hashlib
import json
import re
import struct
from decimal import Decimal
from pathlib import Path

ORIG = Path(__file__).resolve().parents[1] / 'docs/qualification/originals'
OUT = ORIG / 'generic-lsp-v5-selector-vectors.proposed.json'
PINS = {
 'schema': ('adr0011-generic-envelope-v4.schema.json',21409,'df4908187030e3d3110bd3745290eb25d8a3934ffb2a85c5d47d8d22dc28f6f3'),
 'transport': ('generic-lsp-exact-transport-v4.json',1784,'f3e25fab98cc4b8b96b76978cf56220ccc81f4ef41455ab0401041ea281d0bc8'),
 'references': ('generic-lsp-references-exact-v5.json',2210,'7c46893fecf0dda7a4942b8f33d92f0c2f49793d003c7eab4e8995aeddf8da36'),
 'definition': ('generic-lsp-definition-exact-v5.json',2119,'da62f5a161532fcb7c79dd93b0dca940a460c6e0c5fab83e8987de21c91b4dd3'),
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
 'v4_schema': ('adr0011-generic-envelope-v3.schema.json',21619,'7154843a373f8f55c9723b62b1691b7b9de7b6f66ea23082d504291f137701f1'),
 'v4_transport': ('generic-lsp-exact-transport-v3.json',1555,'e2d326f1424a56afa7fbaafc7417ccec52f67e03b977e44c19bd3adb6d9ae01b'),
 'v4_references': ('generic-lsp-references-exact-v4.json',1930,'0756ed60f6ec8582f29d6e1d02aff6f112dff9e0aba977472f6833911c41c077'),
 'v4_definition': ('generic-lsp-definition-exact-v4.json',1839,'6155cc64a70c1d7928eb21edfec26df58a6740267141dbd94a30052cdd244a23'),
 'v4_vectors': ('generic-lsp-v4-selector-vectors.proposed.json',7588,'46a761e1ac3041aca7569018bf4062790ade6981c3ed6adb8880dce94bbdd92e'),
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
 return ['ADR0011-GENERIC-EXACT/5','GENERIC_LSP_'+method.upper()+'_EXACT_V5','textDocument/'+method,role,sha(docs['schema'] if schema is None else schema),sha(docs['transport']),sha(docs[method] if policy is None else policy),session,str(gen),sha(artifact)]
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
 def bounded_number(token):
  if len(token.encode())>4096:raise ValueError('oversized numeric ID token')
  match=re.search(r'[eE]([+-]?[0-9]+)$',token)
  if match and (len(match.group(1).lstrip('+-'))>6 or abs(int(match.group(1)))>100000):raise ValueError('oversized decimal exponent')
  return Decimal(token)
 return json.loads(body.decode('utf8'),object_pairs_hook=unique,parse_int=bounded_number,parse_float=bounded_number,parse_constant=reject_non_json_constant)
def original_id_token(frame_bytes):
 # Find the top-level decoded key in the ORIGINAL UTF-8 frame body; slicing
 # the lexical value retains quotes, escapes, and numeric spelling. Never remarshal.
 body=frame_bytes.partition(b'\r\n\r\n')[2]
 text=body.decode('utf8')
 decoder=json.JSONDecoder(parse_int=Decimal,parse_float=Decimal,parse_constant=lambda c:(_ for _ in ()).throw(ValueError('non-JSON constant: '+c)))
 def spaces(i):
  while i<len(text) and text[i] in ' \t\r\n':i+=1
  return i
 i=spaces(0)
 if i>=len(text) or text[i]!='{':raise ValueError('invalid JSON-RPC object')
 i+=1
 while True:
  i=spaces(i)
  if i>=len(text) or text[i]=='}':break
  key,i=decoder.raw_decode(text,i)
  if type(key) is not str:raise ValueError('non-string JSON-RPC key')
  i=spaces(i)
  if i>=len(text) or text[i]!=':':raise ValueError('invalid JSON-RPC member')
  i=spaces(i+1);start=i
  _,i=decoder.raw_decode(text,i)
  if key=='id':return text[start:i].encode('utf8')
  i=spaces(i)
  if i>=len(text) or text[i]!=',':break
  i+=1
 raise ValueError('missing original ID token')
def validate_exchange(x,observed,write):
 request=strict_frame(x['request_frame'])
 if len(original_id_token(x['request_frame']))>4096:raise ValueError('oversized original request ID token')
 if not isinstance(request,dict):raise ValueError('request is not an object')
 rid=request.get('id')
 if type(rid) not in (str,Decimal) or type(rid) is str and len(rid.encode('utf8'))>1024 or request.get('jsonrpc')!='2.0' or request.get('method')!=x['request_method'] or request.get('method') not in ('initialize','client/registerCapability','client/unregisterCapability') or not isinstance(request.get('params'),dict) or 'result' in request or 'error' in request:raise ValueError('invalid request')
 typed=('number:' if type(rid) is Decimal else 'string:')+str(rid)
 if x['request_id']!=typed:raise ValueError('request ID mirror')
 method=x['request_method'];req_dir='CLIENT_TO_SERVER' if method=='initialize' else 'SERVER_TO_CLIENT';resp_dir='SERVER_TO_CLIENT' if method=='initialize' else 'CLIENT_TO_SERVER'
 if x['request_direction']!=req_dir or x['request_ordinal']<0:raise ValueError('request direction/ordinal')
 required=[(req_dir,x['request_ordinal'],x['request_frame'])]
 if x['response_frame']:
  response=strict_frame(x['response_frame'])
  if len(original_id_token(x['response_frame']))>4096:raise ValueError('oversized original response ID token')
  if not isinstance(response,dict) or response.get('jsonrpc')!='2.0' or 'method' in response or 'params' in response:raise ValueError('invalid response object')
  response_id=response.get('id')
  if type(response_id) is str and len(response_id.encode('utf8'))>1024:raise ValueError('oversized decoded response ID')
  if type(response_id) is not type(rid) or response_id!=rid or x['response_id']!=('number:' if type(response_id) is Decimal else 'string:')+str(response_id) or x['response_direction']!=resp_dir or x['response_ordinal']<=x['request_ordinal']:raise ValueError('mismatched or reversed response')
  if ('result' in response)==('error' in response):raise ValueError('response outcome shape')
  if 'result' in response:
   result=response['result']
   if method=='initialize' and (not isinstance(result,dict) or not isinstance(result.get('capabilities'),dict)):raise ValueError('invalid initialize result')
   if method!='initialize' and result is not None:raise ValueError('invalid capability acknowledgement')
   if 'response_result' not in x or json.dumps(x['response_result'],sort_keys=True,separators=(',',':'))!=json.dumps(result,sort_keys=True,separators=(',',':')) or x.get('response_error') is not None:raise ValueError('result mirror mismatch')
  else:
   error=response['error']
   if not isinstance(error,dict) or type(error.get('code')) is not Decimal or error['code']!=error['code'].to_integral_value() or not isinstance(error.get('message'),str) or set(error)-{'code','message','data'}:raise ValueError('invalid JSON-RPC error')
   mirror={**error,'code':int(error['code'])}
   if 'response_error' not in x or json.dumps(x['response_error'],sort_keys=True,separators=(',',':'))!=json.dumps(mirror,sort_keys=True,separators=(',',':')) or x.get('response_result') is not None:raise ValueError('error mirror mismatch')
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
def test_id_boundaries():
 def escaped(n,padding=0):return b'"'+b'\\u0061'*n+b'a'*padding+b'"'
 def framed(body):return b'Content-Length: '+str(len(body)).encode()+b'\r\n\r\n'+body
 def candidate(token,response_token=None):
  response_token=token if response_token is None else response_token
  req=framed(b'{"jsonrpc":"2.0","id":'+token+b',"method":"client/registerCapability","params":{"registrations":[]}}')
  resp=framed(b'{"jsonrpc":"2.0","id":'+response_token+b',"result":null}')
  value=json.loads(token.decode('utf8'),parse_int=Decimal,parse_float=Decimal)
  returned=json.loads(response_token.decode('utf8'),parse_int=Decimal,parse_float=Decimal)
  prefix='number:' if type(value) is Decimal else 'string:'
  x={'request_direction':'SERVER_TO_CLIENT','request_frame':req,'request_id':prefix+str(value),'request_method':'client/registerCapability','request_ordinal':1,'response_direction':'CLIENT_TO_SERVER','response_frame':resp,'response_id':prefix+str(returned),'response_ordinal':2,'status':'SUCCESS','response_result':None,'response_error':None}
  return x,[('SERVER_TO_CLIENT',1,req),('CLIENT_TO_SERVER',2,resp)]
 for name,token,other,valid in [
  ('escaped_raw_4096',escaped(682,2),None,True),
  ('escaped_raw_4097',escaped(682,3),None,False),
  ('escaped_raw_4202',escaped(700),None,False),
  ('response_raw_4097',b'"'+b'a'*685+b'"',escaped(682,3),False),
  ('decoded_1024',b'"'+b'a'*1024+b'"',None,True),
  ('multibyte_decoded_1024',b'"'+'é'.encode('utf8')*512+b'"',None,True),
  ('multibyte_decoded_1026',b'"'+'é'.encode('utf8')*513+b'"',None,False),
  ('decoded_1025',b'"'+b'a'*1025+b'"',None,False),
  ('small_decoded_oversized_raw',escaped(683),None,False),
  ('numeric_raw_4096',b'1'*4096,None,True),
  ('numeric_raw_4097',b'1'*4097,None,False),
  ('equivalent_decimal',b'1',b'1.0',True),
  ('equivalent_exponent',b'1',b'1e0',True),
  ('equivalent_escaped_string',escaped(1),b'"a"',True),
 ]:
  x,held=candidate(token,other)
  try:validate_exchange(x,held,10)
  except ValueError:
   if valid:raise AssertionError('valid ID boundary rejected: '+name)
  else:
   if not valid:raise AssertionError('invalid ID boundary admitted: '+name)
def query_artifact(tx,query_sel,source_sel,uri,language,kind='ORDINARY',notebook_type=None,notebook_uri=None,notebook_source=None,workspace='file:///workspace'):
 fields=['ADR0011-GENERIC-QUERY-APPLICABILITY-ARTIFACT/1',tx,query_sel,source_sel,uri,'present' if language is not None else 'absent',language or b'',kind,'present' if notebook_type is not None else 'absent',notebook_type or b'','present' if notebook_uri is not None else 'absent',notebook_uri or b'',notebook_source or '',workspace,'s','1','tx-1']
 return b''.join(map(lp,fields))
def vectors():
 docs={k:pinned(k) for k in PINS}
 for m in ('references','definition'):
  obj=json.loads(docs[m]);assert obj['shared_applicability_sha256']==PINS['shared'][2] and obj['selector_domain']=='ADR0011-GENERIC-EXACT/5'
 old=json.loads(docs['v2_vectors']);sources=json.loads(docs['v2_source']);v3=json.loads(docs['v3_vectors']);query=bytes.fromhex(old['query_artifact_utf8_hex']);event=bytes.fromhex(old['event_artifact_utf8_hex'])
 result={'status':'UNACCEPTED_OFFLINE_CANDIDATE','domain':'ADR0011-GENERIC-EXACT/5','pins':{k:{'length':len(docs[k]),'sha256':sha(docs[k])} for k in PINS},'selectors':{},'historical_collision_count':0}
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
 test_id_boundaries()
 if len(sels)!=54 or len(set(sels.values()))!=54:raise AssertionError('V5 vector coverage/distinctness')
 historical=set(old['selectors'].values())|set(sources['selectors'].values())|set(v3['selectors'].values())|set(json.loads(docs['v4_vectors'])['selectors'].values())
 if historical & set(sels.values()):raise AssertionError('historical collision')
 return (json.dumps(result,sort_keys=True,separators=(',',':'))+'\n').encode()
if __name__=='__main__':
 ap=argparse.ArgumentParser();ap.add_argument('--check',action='store_true');a=ap.parse_args();out=vectors()
 if a.check:
  if OUT.read_bytes()!=out:raise SystemExit('V5 candidate vector mismatch')
 else:OUT.write_bytes(out)
 print('V5_CANDIDATE_VECTORS_'+ ('CHECKED' if a.check else 'WRITTEN')+' sha256:'+sha(out)+' bytes:'+str(len(out)))
