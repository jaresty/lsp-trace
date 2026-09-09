import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

const root = resolve(import.meta.dir, "..");
const adapterRoot = process.env.PI_MCP_ADAPTER_ROOT ?? "/Users/schwa/.pi/agent/npm/node_modules/pi-mcp-adapter";
const binary = process.env.LSP_TRACE_MCP_BINARY;
const fixturePath = resolve(root, "cmd/lsp-trace-mcp/testdata/d01-program-b-normative-graph.json");
if (!binary) throw new Error("ASSERT_ADAPTER_CANARY_BINARY_REQUIRED: set LSP_TRACE_MCP_BINARY to the built lsp-trace-mcp");

const manifest = JSON.parse(readFileSync(resolve(adapterRoot, "package.json"), "utf8"));
if (manifest.name !== "pi-mcp-adapter" || manifest.version !== "2.32.1") {
  throw new Error(`ASSERT_ADAPTER_VERSION: expected pi-mcp-adapter 2.32.1, got ${manifest.name ?? "missing"} ${manifest.version ?? "missing"}`);
}

const direct = await import(pathToFileURL(resolve(adapterRoot, "direct-tools.ts")).href);
const managerModule = await import(pathToFileURL(resolve(adapterRoot, "server-manager.ts")).href);
const cacheModule = await import(pathToFileURL(resolve(adapterRoot, "metadata-cache.ts")).href);
const shapeModule = await import(pathToFileURL(resolve(adapterRoot, "ts-shape.ts")).href);
const { resolveDirectTools, createDirectToolExecutor } = direct;
const { McpServerManager } = managerModule;
const { computeServerHash, serializeTools } = cacheModule;
const renderTsShape = shapeModule.renderTsShape;

const fixture = readFileSync(fixturePath);
const sha256 = (bytes: Uint8Array | string) => `sha256:${createHash("sha256").update(bytes).digest("hex")}`;
if (fixture.byteLength !== 3859 || sha256(fixture) !== "sha256:36f10abcad7d0f7780922f04064db032c77bf536e9c6ad864d759b57344b8cfb") {
  throw new Error(`ASSERT_D01_INPUT_BYTES: got bytes=${fixture.byteLength} digest=${sha256(fixture)}`);
}

const cases = [
  { key: "analysis", tool: "lsp_trace_v2_bounded_retained_analysis", operation: "ANALYSIS", bytes: 2304, payload: "sha256:5e261e80646dc8a5ac56a6d6a90a07566d0e64f8e8ee043d06eed3067ce60905", result: "sha256:5ca624baab9d0838fb278ed26aacb4dd9c66fb12fc79c49d345bc7d4ae32820f" },
  { key: "metrics", tool: "lsp_trace_v2_bounded_retained_metrics", operation: "METRICS", bytes: 1859, payload: "sha256:a9be70c90c176b788689aded899e348bcfd65de928532c4322b02065f5835438", result: "sha256:7724e93f28f0c4eea3ab954a27527d505e5444263c9d4788da94baef2fd41f99" },
  { key: "ranking", tool: "lsp_trace_v2_bounded_retained_ranking", operation: "RANKING", bytes: 1958, payload: "sha256:37f7f723abbdaa0ae1d86a7a6a59881bac2352760fcdedda43920ff766763536", result: "sha256:fa804fddf0944ca874500e87670a8d42e821ba1b55a70af41be0a0d704c4d0e7" },
] as const;
const targetNames = new Set(cases.map(c => c.tool));
const definition = { command: binary, cwd: root, directTools: [...targetNames], exposeResources: false, lifecycle: "lazy" as const };
const config = { mcpServers: { canary: definition }, settings: { toolPrefix: "none" as const, directToolResultDetails: "bounded" as const } };
const manager = new McpServerManager(root);
const connection = await manager.connect("canary", definition);
try {
  const tools = connection.tools.filter((tool: any) => targetNames.has(tool.name));
  if (tools.length !== 3) throw new Error(`ASSERT_DIRECT_TOOL_DISCOVERY: expected 3, got ${tools.length}`);
  const cache = { version: 1, servers: { canary: { configHash: computeServerHash(definition), tools: serializeTools(tools), resources: [], prompts: [], cachedAt: Date.now() } } };
  const specs = resolveDirectTools(config, cache, "none");
  if (specs.length !== 3) throw new Error(`ASSERT_RESOLVE_DIRECT_TOOLS: expected 3, got ${specs.length}`);

  const registered = new Map<string, { parameters: unknown; execute: Function }>();
  const state: any = {
    owner: { signal: undefined }, manager, lifecycle: {}, config,
    toolMetadata: new Map(), resourceCounts: new Map(), promptMetadata: new Map(), promptMetadataLive: new Set(), serverInstructions: new Map(),
    failureTracker: new Map(), failureMessages: new Map(), approvedToolCalls: new Map(), completedUiSessions: [], uiServer: null,
  };
  for (const spec of specs) {
    registered.set(spec.prefixedName, { parameters: spec.inputSchema, execute: createDirectToolExecutor(() => state, () => null, spec) });
    const variants = (spec.inputSchema as any)?.oneOf;
    const rendered = renderTsShape(spec.inputSchema);
    if (!Array.isArray(variants) || variants.length !== 2 || variants.some((v: any) => v?.type !== "object" || typeof v?.properties !== "object") || !rendered || rendered.includes("unknown | unknown")) {
      throw new Error(`ASSERT_PRESENTATION_USEFUL_OBJECT_SCHEMA ${spec.originalName}: ${rendered}`);
    }
  }
  console.log("PASS ASSERT_ADAPTER_VERSION pi-mcp-adapter@2.32.1");
  console.log("PASS ASSERT_DIRECT_TOOL_REGISTRATION resolveDirectTools=3 registered=3");
  console.log("PASS ASSERT_PRESENTATION_USEFUL_OBJECT_SCHEMA typed-complete-oneOf=3");

  for (const c of cases) {
    const registration = registered.get(c.tool)!;
    const missing = await registration.execute(`missing-${c.key}`, { operation: c.operation, filter: ["CALLS"], max_work: 10000 }, undefined, undefined, {});
    const missingText = missing.content?.map((part: any) => part.text ?? "").join("\n") ?? "";
    if (!missing.details?.error || !/input|publication_selector|oneOf|required/i.test(missingText)) {
      throw new Error(`ASSERT_MISSING_CARRIER_CLEAR_REJECTION ${c.key}: ${JSON.stringify(missing)}`);
    }
    const args = { operation: c.operation, filter: ["CALLS"], max_work: 10000, input: JSON.parse(fixture.toString("utf8")) };
    const adapted = await registration.execute(`valid-${c.key}`, args, undefined, undefined, {});
    if (adapted.details?.error) throw new Error(`ASSERT_VALID_CARRIER_ADAPTER_SUCCESS ${c.key}: ${JSON.stringify(adapted)}`);
    const adaptedText = adapted.content?.find((part: any) => part.type === "text")?.text;
    const adaptedEnvelope = JSON.parse(adaptedText);
    const payload = Buffer.from(adaptedEnvelope.content, "utf8");
    if (adaptedEnvelope.outcome !== "COMPLETE" || adaptedEnvelope.operation_status !== "SUCCEEDED") {
      throw new Error(`ASSERT_OPERATION_TERMINAL ${c.key}: ${adaptedEnvelope.outcome}/${adaptedEnvelope.operation_status}`);
    }
    const result = JSON.parse(payload.toString("utf8"));
    if (payload.byteLength !== c.bytes || sha256(payload) !== c.payload || result.Digest !== c.result || result.Status !== "COMPLETE") {
      throw new Error(`ASSERT_CANONICAL_PAYLOAD ${c.key}: bytes=${payload.byteLength} payload=${sha256(payload)} result=${result.Digest} status=${result.Status}`);
    }
    const currentConnection = manager.getConnection("canary");
    if (!currentConnection || currentConnection.status !== "connected") throw new Error(`ASSERT_RAW_STDIO_CONNECTION ${c.key}: unavailable`);
    const raw = await currentConnection.client.callTool({ name: c.tool, arguments: args });
    if (raw.isError) throw new Error(`ASSERT_RAW_STDIO_SUCCESS ${c.key}: ${JSON.stringify(raw)}`);
    const rawEnvelope = JSON.parse((raw.content as any[]).find(part => part.type === "text")?.text);
    const rawPayload = Buffer.from(rawEnvelope.content, "utf8");
    if (!rawPayload.equals(payload)) throw new Error(`ASSERT_RAW_STDIO_BYTE_PARITY ${c.key}: adapter=${sha256(payload)} raw=${sha256(rawPayload)}`);
    console.log(`PASS ASSERT_MISSING_CARRIER_CLEAR_REJECTION ${c.key}`);
    console.log(`PASS ASSERT_VALID_CARRIER_ADAPTER_SUCCESS ${c.key}`);
    console.log(`PASS ASSERT_COMPUTATION_REACHED ${c.key} outcome=COMPLETE operation_status=SUCCEEDED`);
    console.log(`PASS ASSERT_CANONICAL_PAYLOAD ${c.key} bytes=${c.bytes} payload_sha256=${c.payload} result_digest=${c.result} digest_scope=EXACT_SERIALIZED_OUTPUT_BYTES`);
    console.log(`PASS ASSERT_RAW_STDIO_BYTE_PARITY ${c.key} sha256=${c.payload}`);
  }
  console.log("ADAPTER CANARY PASS");
} finally {
  await manager.closeAll();
}
