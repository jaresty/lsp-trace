# Caller omitted module-resolution policy claim

Status: CLAIMED
Baseline: `e5a24696b83218403a009a702a48950e5d1f36bc`

## Goal

Apply a generic analysis-only Node-compatible TypeScript module-resolution policy to caller projects only when their config omits `compilerOptions.moduleResolution`, without package/source special cases, paths, or fallback declarations.

## Derivation

1. TypeScript 5.9.2 parses a target/decorators-only JavaScript config without a Node-compatible resolution mode, so installed package declarations remain unresolved with TS2792.
2. Explicit caller configuration is authoritative and remains unchanged.
3. On omission, `module: NodeNext` requires `NodeNext`, `module: Node16` requires `Node16`, and other or omitted module kinds use `Node10`, which resolves installed Node packages without changing caller module semantics.
4. Compiler diagnostics remain explicit `BLOCKED`; unsafe `any`/`unknown` remain `BLOCKED`; checked same-spelling absence alone may be `EMPTY`.

## RED

Before production mutation:

`node --test --test-name-pattern='ASSERT_CALLER_OMITTED_MODULE_RESOLUTION' providers/ember-glint/test/caller-project-loader.test.mjs`

executed the caller-owned synthetic project and failed `ASSERT_CALLER_OMITTED_MODULE_RESOLUTION_USES_NODE_COMPATIBLE_POLICY`: expected `COMPLETE`, observed `BLOCKED`, with `TS2792 ... Cannot find module 'ember-concurrency'. Did you mean to set the 'moduleResolution' option to 'nodenext', or to add aliases to the 'paths' option?`

## GREEN

- Persistent omitted-resolution assertion: 1/1 pass.
- Caller loader matrix: 9/9 pass, covering omitted/default, omitted NodeNext, explicit Classic, explicit NodeNext, explicit Bundler, installed/missing/wrong declarations, same-spelling, any, and unknown controls.
- Packed caller MCP ledger: 12 cases, 24 logical operations, 48/48 replay attempts; retained evidence guards 4/4.
- Provider package: 89/89.
- B05 Frame 6: 24/24 attempts; 12 seeds and 24 stages.
- Full Go: 3318 tests in 41 packages.
- Race Go: 3318 tests in 41 packages.
- CI contract: all assertions pass.
- Release check: pass, including parity, omission compatibility, dry builds, and archive exclusion.
- GoReleaser v2 check: one configuration validated.

## Pinned Market View replay

Requested immutable UI commit: `326718ae733cb26097bd30246276cecd371a4e79`.

No discovered local checkout retained the commit object. A fresh disposable clone of `https://github.com/NAISorg/market-view-ui.git` failed checkout with `fatal: unable to read tree (326718ae733cb26097bd30246276cecd371a4e79)`. Therefore materialization and resolved-identity replay remain explicitly BLOCKED; no alternate revision or fallback declaration was substituted, and the source checkout was not modified.
