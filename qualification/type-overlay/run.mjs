#!/usr/bin/env node
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { runCampaign } from './campaign.mjs';
import { validateQualificationReport } from './validate.mjs';

function option(name) { const index = process.argv.indexOf(name); return index < 0 ? null : process.argv[index + 1]; }
const manifestPath = option('--manifest'), source = option('--source'), output = option('--output');
if (!manifestPath || !source || !output) throw new Error('usage: run.mjs --manifest PATH --source PINNED_REPOSITORY --output CALLER_SELECTED_PATH');
const manifest = JSON.parse(readFileSync(resolve(manifestPath)));
if (manifest.schema_version !== 'lsp-trace.type-overlay-campaign-manifest.v1') throw new Error('unsupported campaign manifest');
const adapterModule = await import(pathToFileURL(resolve(manifest.adapter.module)));
const observerModule = await import(pathToFileURL(resolve(manifest.observer.module)));
const adapter = adapterModule[manifest.adapter.export](manifest.adapter.options);
const observe = observerModule[manifest.observer.export](manifest.observer.options);
await runCampaign({ source: resolve(source), pinnedCommit: manifest.source.commit, adapter, observe, output: resolve(output), validate: validateQualificationReport });
console.log(`QUALIFICATION_ONLY published=${resolve(output)}`);
