import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

export const IDENTITY='source-constrained-synthetic-provider';
export const VERSION='1.0.0-provisional';
export const EVIDENCE_CLASS='SOURCE_CONSTRAINED_SYNTHETIC';
export const DISCOVERY_STATUS='PROVISIONAL_DISCOVERY';
const ledger=JSON.parse(await readFile(new URL('../provenance.json',import.meta.url),'utf8'));
const hash=x=>createHash('sha256').update(typeof x==='string'?x:JSON.stringify(x,Object.keys(x).sort())).digest('hex');
const patterns={INVOKES_TASK:/this\.uploadComplete\.perform\s*\(/,TRIGGERS_RELOAD:/await upload\.reload\s*\(/};
export const metadata=Object.freeze({identity:IDENTITY,version:VERSION,selectable:true,selection:'EXPLICIT_ONLY',evidence_class:EVIDENCE_CLASS,discovery_status:DISCOVERY_STATUS,capabilities:{relations:['INVOKES_TASK','TRIGGERS_RELOAD'],languages:['javascript'],frameworks:[]}});
export async function analyze(request){
 if(request.provider_id!==IDENTITY) throw new Error('explicit provider identity required');
 const uri=request.seed?.uri; if(!uri?.startsWith('file://')) throw new Error('file seed required');
 const source=await readFile(fileURLToPath(uri),'utf8'); const digest=hash(source);
 const revision=request.document_custody?.workspace_revision?.value||digest;
 const observations=[];
 for(const kind of [...new Set(request.relations||[])].sort()){
  const p=ledger.relations[kind]; if(!p||!patterns[kind].test(source)) continue;
  const anchor={uri,revision,blob:digest,range:{start:{line:0,character:0},end:{line:0,character:1}}};
  const logical={kind,from:p.anchor.symbol,to:p.chain.at(-1),evidence_class:EVIDENCE_CLASS,discovery_status:DISCOVERY_STATUS,anchors:[anchor],confidence:'EXACT',supports:['source_dependency_relation'],does_not_support:['runtime_execution','whole_source_completeness'],declaration_chain:p.chain,dependencies:p.dependencies||[p.dependency],negative_boundaries:p.negative_boundaries};
  observations.push({...logical,observation_id:`sha256:${hash(JSON.stringify(logical))}`});
 }
 return {provider:{name:IDENTITY,version:VERSION},protocol:{name:'lsp-trace.provider-observations',version:'1'},adapter:{name:'source-constrained-synthetic',version:'1'},authority:'PROVIDER_REPORTED',coverage:{status:'COMPLETE_WITHIN_BOUNDS',denominator:[uri],covered:[uri],CoveredCount:1},documents:[{document_id:'original',original_uri:uri,content_sha256:digest,revision:{kind:'git',value:revision,blob:digest,custody:'PROVIDER_PROVED'},coordinates:'ORIGINAL'}],observations,request_id:`${request.session.session_id}:${request.session.generation}:${uri}`};
}
export function decodeFrame(buffer){const i=buffer.indexOf('\r\n\r\n');if(i<0)throw new Error('framing');const h=buffer.subarray(0,i).toString();if(!/^Content-Length: (0|[1-9]\d*)$/.test(h))throw new Error('framing');const body=buffer.subarray(i+4);if(body.length!==Number(h.slice(16)))throw new Error('length');return JSON.parse(body);}
export function encodeFrame(value){const body=Buffer.from(JSON.stringify(value));return Buffer.concat([Buffer.from(`Content-Length: ${body.length}\r\n\r\n`),body]);}
