import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

export const IDENTITY='source-constrained-synthetic-provider';
export const VERSION='1.0.0-provisional';
export const EVIDENCE_CLASS='SOURCE_CONSTRAINED_SYNTHETIC';
export const DISCOVERY_STATUS='PROVISIONAL_DISCOVERY';
const sha256=x=>createHash('sha256').update(x).digest('hex');
export const metadata=Object.freeze({identity:IDENTITY,version:VERSION,selectable:true,selection:'EXPLICIT_ONLY',evidence_class:EVIDENCE_CLASS,discovery_status:DISCOVERY_STATUS,capabilities:{relations:['PASSES_CALLBACK','INVOKES_TASK','TRIGGERS_RELOAD','UPDATES_STATE','RENDERS_FROM'],languages:['go'],frameworks:[]}});
const patterns={
 PASSES_CALLBACK:/PROVISIONAL_PASSES_CALLBACK_POSITIVE/,
 INVOKES_TASK:/PROVISIONAL_INVOKES_TASK_POSITIVE/,
 TRIGGERS_RELOAD:/PROVISIONAL_TRIGGERS_RELOAD_POSITIVE/,
 UPDATES_STATE:/PROVISIONAL_UPDATES_STATE_POSITIVE/,
 RENDERS_FROM:/PROVISIONAL_RENDERS_FROM_POSITIVE/,
};
const roles={PASSES_CALLBACK:['CALLABLE_REFERENCE','CALLBACK_PARAMETER'],INVOKES_TASK:['TASK_INVOCATION','TASK'],TRIGGERS_RELOAD:['RELOAD_REQUEST','RELOAD_TARGET'],UPDATES_STATE:['STATE_PRODUCER','STATE_VALUE'],RENDERS_FROM:['RENDER_EXPRESSION','READ_VALUE']};
export async function analyze(request){
 if(request.provider_id!==`${IDENTITY}@${VERSION}`) throw new Error('explicit provider identity required');
 const uri=request.seed?.uri; if(!uri?.startsWith('file://')) throw new Error('file seed required');
 const source=await readFile(fileURLToPath(uri));
 const commit=request.document_custody?.workspace_revision?.commit||request.document_custody?.workspace_revision?.value;
 if(!/^[0-9a-f]{40}$/.test(commit||'')) throw new Error('strict git commit required');
 const documentID='qualification-seed';
 const observations=[];
 for(const kind of [...new Set(request.relations||[])].sort()){
  if(!patterns[kind]?.test(source.toString())) continue;
  observations.push({kind,from:{node_id:`${kind}:source`,role:roles[kind][0]},to:{node_id:`${kind}:target`,role:roles[kind][1]},original_anchor:{document_id:documentID,uri,revision:commit,blob:sha256(source),range:{start:{line:0,character:0},end:{line:0,character:1}}},supports:['source_dependency_relation'],does_not_support:['runtime_execution','callback_invocation','repaint','feature_identity','whole_source_completeness']});
 }
 return {provider:{name:IDENTITY,version:VERSION},protocol:{name:'lsp-trace.provider-observations',version:'1'},adapter:{name:'lsp-trace-observation-adapter',version:'1'},request_id:`${request.session.session_id}:${request.session.generation}:${uri}`,authority:'PROVIDER_REPORTED',coverage:{Status:'COMPLETE_WITHIN_BOUNDS',Denominator:[documentID],Covered:[documentID],CoveredCount:1},documents:[{document_id:documentID,original_uri:uri,content_sha256:sha256(source),revision:{kind:'git',value:commit,blob:sha256(source),custody:'PROVIDER_PROVED'},coordinates:'ORIGINAL'}],observations};
}
export function decodeFrame(buffer){const i=buffer.indexOf('\r\n\r\n');if(i<0)throw new Error('framing');const h=buffer.subarray(0,i).toString();if(!/^Content-Length: (0|[1-9]\d*)$/.test(h))throw new Error('framing');const body=buffer.subarray(i+4);if(body.length!==Number(h.slice(16)))throw new Error('length');return JSON.parse(body);}
export function encodeFrame(value){const body=Buffer.from(JSON.stringify(value));return Buffer.concat([Buffer.from(`Content-Length: ${body.length}\r\n\r\n`),body]);}
