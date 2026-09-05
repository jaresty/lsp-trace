#!/usr/bin/env node
import { analyze, decodeFrame, encodeFrame } from '../src/provider.mjs';
const chunks=[];
for await (const chunk of process.stdin) chunks.push(chunk);
try { process.stdout.write(encodeFrame(await analyze(decodeFrame(Buffer.concat(chunks))))); }
catch (error) { process.stderr.write(`${error.stack||error}\n`); process.exitCode=1; }
