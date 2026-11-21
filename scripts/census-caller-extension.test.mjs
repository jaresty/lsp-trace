import test from 'node:test';
import assert from 'node:assert/strict';
import { resolve } from 'node:path';

test('optional Pi extension loads, registers actual supported tools/events, and refuses lost ownership', {skip:!process.env.PI_MODULE_ROOT}, async () => {
 const root=process.env.PI_MODULE_ROOT;
 const {createJiti}=await import(resolve(root,'jiti/lib/jiti.cjs'));
 const jiti=createJiti(import.meta.url,{alias:{typebox:resolve(root,'typebox/build/index.mjs')}});
 const {default:extension}=await jiti.import(new URL('../.pi/extensions/census-caller.ts',import.meta.url).pathname);
 const events=new Map();let tool;
 extension({on:(name,handler)=>events.set(name,handler),registerTool:t=>{tool=t;}});
 assert.equal(tool.name,'census_inventory');assert.equal(tool.executionMode,'sequential');
 for (const event of ['session_shutdown','session_before_tree','agent_settled']) assert.equal(typeof events.get(event),'function');
 const ctx={cwd:process.cwd(),model:{provider:'controlled',id:'fixture'},sessionManager:{getSessionId:()=> 'controlled-caller'}};
 await assert.rejects(tool.execute('no-owner',{action:'next'},undefined,undefined,ctx),/No live census child/);
 await assert.rejects(tool.execute('same-context',{action:'start',subagents:true,interpreter:'same',reviewer:'same'},undefined,undefined,ctx),/distinct context IDs/);
 await assert.rejects(tool.execute('missing-executable',{action:'start',executable:'/not-a-census-fixture-binary'},undefined,undefined,ctx),/ENOENT|closed/);
 await events.get('session_shutdown')();
});
