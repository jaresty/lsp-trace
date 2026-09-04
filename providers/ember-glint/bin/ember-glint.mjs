#!/usr/bin/env node
import { stdin, stdout } from 'node:process';
import { createProvider, decodeFrame, encodeFrame } from '../src/protocol.mjs';

const chunks = [];
for await (const chunk of stdin) chunks.push(chunk);
const request = decodeFrame(Buffer.concat(chunks));
const response = await createProvider().handle(request);
stdout.write(encodeFrame(response));
