# @lsp-trace/ember-glint-provider

This directory is the independently installable and versioned package boundary for Ember/Glint provider work.

It is intentionally separate from the core Go module: core `lsp-trace` must not import this package, and core release archives must not bundle it. Provider executables are installed independently and supplied to hosts by absolute path.

## Current scope

This boundary currently contains package metadata only. Analyzer implementation, protocol command/framing, generic conformance, provider inventory, readiness, and production qualification belong to other frames and are not implemented here.

Existing reusable prototypes remain in their sibling-owned locations until their owning analyzer or protocol frames move them. This package does not duplicate those sources or claim their behavior.

## Installation

From this repository checkout:

```sh
npm install ./providers/ember-glint
```

Published versions can be installed by package name once released independently:

```sh
npm install @lsp-trace/ember-glint-provider
```
