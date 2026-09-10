import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

const root = resolve(import.meta.dir, "..");
const adapterRoot = process.env.PI_MCP_ADAPTER_ROOT;
const binary = process.env.LSP_TRACE_MCP_BINARY;
if (!adapterRoot || !binary) throw new Error("ASSERT_SLICE_ADAPTER_ENVIRONMENT_REQUIRED");

const direct = await import(pathToFileURL(resolve(adapterRoot, "direct-tools.ts")).href);
const managers = await import(pathToFileURL(resolve(adapterRoot, "server-manager.ts")).href);
const cacheModule = await import(pathToFileURL(resolve(adapterRoot, "metadata-cache.ts")).href);
const definition = { command: binary, args: ["--tool-profile", "compact"], cwd: root, directTools: ["lsp_trace_v1_slice"], exposeResources: false, lifecycle: "lazy" as const };
const config = { mcpServers: { slice: definition }, settings: { toolPrefix: "none" as const, directToolResultDetails: "bounded" as const } };
const manager = new managers.McpServerManager(root);
try {
  const connection = await manager.connect("slice", definition);
  const tools = connection.tools.filter((tool: any) => tool.name === "lsp_trace_v1_slice");
  const cache = { version: 1, servers: { slice: { configHash: cacheModule.computeServerHash(definition), tools: cacheModule.serializeTools(tools), resources: [], prompts: [], cachedAt: Date.now() } } };
  const specs = direct.resolveDirectTools(config, cache, "none");
  if (specs.length !== 1) throw new Error(`ASSERT_SLICE_ADAPTER_REGISTRATION: ${specs.length}`);
  const state: any = {
    owner: { signal: undefined }, manager, lifecycle: {}, config,
    toolMetadata: new Map(), resourceCounts: new Map(), promptMetadata: new Map(), promptMetadataLive: new Set(), serverInstructions: new Map(),
    failureTracker: new Map(), failureMessages: new Map(), approvedToolCalls: new Map(), completedUiSessions: [], uiServer: null,
  };
  const execute = direct.createDirectToolExecutor(() => state, () => null, specs[0]);
  const base = { session_id: "missing", generation: 1, start_mode: "at", uri: "file:///workspace/main.go" };
  for (const [name, selector] of [["SYMBOL", { symbol: "Target" }], ["POSITION", { line: 4, character: 7 }]] as const) {
    const result = await execute(`slice-${name.toLowerCase()}`, { ...base, ...selector }, undefined, undefined, {});
    const text = (result.content ?? []).map((part: any) => part.text ?? "").join("\n");
    if (result.details?.mcpResult?.structuredContent?.code !== "SESSION_NOT_FOUND" || !text.includes('"code":"SESSION_NOT_FOUND"')) {
      throw new Error(`ASSERT_SLICE_ADAPTER_${name}: ${JSON.stringify(result)}`);
    }
    console.log(`PASS ASSERT_SLICE_ADAPTER_${name}`);
  }
} finally {
  await manager.closeAll();
}
