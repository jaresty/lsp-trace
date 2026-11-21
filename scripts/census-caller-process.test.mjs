import test from 'node:test';
import assert from 'node:assert/strict';
import { CensusCallerProcess } from './census-caller-process.mjs';

const protocol = 'lsp-trace.census-caller.v1';
// Test-only process; it does not implement or replace the census controller.
const peer = `
 const net=require('node:net'); const s=new net.Socket({fd:3});
 let b=Buffer.alloc(0);
 function send(x){const v=Buffer.from(JSON.stringify(x)),h=Buffer.alloc(4);h.writeUInt32BE(v.length);s.write(Buffer.concat([h,v]));}
 s.on('data',c=>{b=Buffer.concat([b,c]);while(b.length>=4&&b.length>=4+b.readUInt32BE()){let n=b.readUInt32BE(),x=JSON.parse(b.subarray(4,n+4));b=b.subarray(n+4);if(x.hello)send({Event:'NEEDS_HOST',value:'still owned'});else {send({Event:'TERMINAL',Payload:x.Payload});s.end();}}});
`;
test('host tool returns while the same child remains owned; later answer preserves bytes and joins', async () => {
 const p=new CensusCallerProcess(process.execPath,['-e',peer]);
 try {
  p.send({hello:true});
  assert.equal((await p.next()).Event,'NEEDS_HOST');
  assert.equal(p.exit,undefined);
  const payload=Buffer.from('{"status":"COMPLETE"}\n').toString('base64');
  p.send({Payload:payload});
  assert.equal(Buffer.from((await p.next()).Payload,'base64').toString(),'{"status":"COMPLETE"}\n');
  assert.equal((await p.closed).code,0);
 } finally {await p.stop();}
});
test('host shutdown joins a waiting child, including repeated shutdown', async () => {
 const p=new CensusCallerProcess(process.execPath,['-e',peer]);
 p.send({hello:true});await p.next();
 await p.stop();await p.stop();
 assert.ok(p.exit);
});
test('tool abort joins the child instead of leaving an unowned process', async () => {
 const p=new CensusCallerProcess(process.execPath,['-e',peer]);
 const abort=new AbortController();
 const pending=p.next(abort.signal);abort.abort();
 await assert.rejects(pending,/aborted/);
 await p.stop();assert.ok(p.exit);
});
test('oversized reply is refused before writing', async () => {
 const p=new CensusCallerProcess(process.execPath,['-e',peer]);
 try {assert.throws(()=>p.send({value:'x'.repeat(96*1024)}),/size refused/);} finally {await p.stop();}
});

for (const refusal of ['missing','malformed','stale','disconnect','withheld']) test(`actual Go caller refuses ${refusal}`, {skip:!process.env.CENSUS_GO_TEST_BINARY}, async () => {
 const p=new CensusCallerProcess(process.env.CENSUS_GO_TEST_BINARY,['-test.run=^TestCensusFolderManagedHostProcess$'],{env:{...process.env,CENSUS_MANAGED_PROCESS_FIXTURE:'1',CENSUS_MANAGED_PROCESS_REFUSAL:refusal}});
 try {
  if(refusal!=='missing') {
   const pin='sha256:'+'a'.repeat(64);
   p.send({Protocol:protocol,Subagents:false,Interpreter:'scripted',Reviewer:'scripted',Pins:{worker_sha256:pin,model_sha256:pin,grammar_sha256:pin,prompt_sha256:''}});
   assert.equal((await p.next()).Event,'ACTIVATION_READY');
   const event=await p.next();assert.equal(event.Event,'NEEDS_HOST');
   if(refusal==='disconnect') p.stream.end();
   if(refusal==='malformed') p.send({unexpected:'not an allowed reply'});
   if(refusal==='stale') {const f=event.Frame;p.send({Protocol:protocol,RunID:f.RunID,Role:f.Role,RequestDigest:f.RequestDigest,Sequence:f.Sequence+1,Payload:Buffer.from('{}').toString('base64')});}
   // withheld deliberately leaves the original frame deadline to expire.
  }
  assert.equal((await p.closed).code,0,p.stdout+p.stderr);
 } finally {await p.stop();}
});

// Optional actual Go normal-census process fixture. Build only with the controlled
// tag; the helper replaces LSP wire transport, never native/controller dispatch.
for (const subagents of [false,true]) test(`actual Go normal-folder managed child (${subagents?'SUBAGENTS':'SAME_THREAD'})`, {skip:!process.env.CENSUS_GO_TEST_BINARY}, async () => {
 const p=new CensusCallerProcess(process.env.CENSUS_GO_TEST_BINARY,['-test.run=^TestCensusFolderManagedHostProcess$'],{env:{...process.env,CENSUS_MANAGED_PROCESS_FIXTURE:'1'}});
 const pin='sha256:'+'a'.repeat(64), actor='scripted-interpreter', reviewer=subagents?'scripted-reviewer':actor;
 let count=0, proposal='', terminal;
 try {
  p.send({Protocol:protocol,Subagents:subagents,Interpreter:actor,Reviewer:reviewer,Pins:{worker_sha256:pin,model_sha256:pin,grammar_sha256:pin,prompt_sha256:''}});
  for(let n=0;n<18;n++) {
   const event=await p.next();
   if(event.Event==='ACTIVATION_READY') {assert.equal(event.ReviewMode,subagents?'SUBAGENTS':'SAME_THREAD');continue;}
   if(event.Event==='TERMINAL') {terminal=event.Terminal;break;}
   assert.equal(event.Event,'NEEDS_HOST');assert.equal(p.exit,undefined);
   const f=event.Frame;let payload;
   if(f.Operation==='Describe') {
    count++;
    const semantic={verdict:'COMPLETE',target_role:'controlled checkout role',consumer_need:{status:'UNRESOLVED',value:''},provided_behavior:{value:'controlled checkout relation',consumer_relative:false},boundary_contribution:'fixture only',limitations:['scripted transport fixture, not live model execution']};
    // Exact existing adapter field order, including zero-valued telemetry;
    // these scripted values are not authenticated model execution measurements.
    payload=JSON.stringify({status:'COMPLETE',text:JSON.stringify(semantic),tokens:0,load_ms:0,run_ms:0,cancelled:false,decode_status:0,grammar:'',context_tokens:0})+'\n';
   } else {
    const c=f.Payload;
    assert.equal(c.evidence.length,count);
    let groups=c.evidence.length>1?[{id:'checkout',label:'checkout',reason:'controlled grouping',members:c.evidence.slice(0,-1).map(e=>e.ID)}]:[];
    let affected=[];
    if(f.Operation==='Reviewer') {assert.equal(c.predecessor,proposal);groups=c.groups.map(g=>({...g,label:'separately reviewed checkout',reason:'controlled separate Reviewer exchange'}));affected=groups.map(g=>g.id);}
    else {assert.equal(f.Operation,'GroupDecision');proposal=c.revision;}
    payload=JSON.stringify({predecessor:c.revision,actor:f.Operation==='Reviewer'?reviewer:actor,reason:'scripted separate review',affected,groups});
   }
   p.send({Protocol:protocol,RunID:f.RunID,Role:f.Role,RequestDigest:f.RequestDigest,Sequence:f.Sequence,Payload:Buffer.from(payload).toString('base64')});
  }
  assert.ok(terminal);assert.equal(terminal.Status,'PROVISIONAL');assert.equal(terminal.SuccessfulDescriptions,2);assert.equal(terminal.Authority,0);assert.equal(terminal.Accepted,false);
  assert.equal((await p.closed).code,0,p.stderr+p.stdout);
 } catch (error) { throw new Error(`${error.message}\nChild stdout: ${p.stdout}\nChild stderr: ${p.stderr}`,{cause:error}); }
 finally {await p.stop();}
});
