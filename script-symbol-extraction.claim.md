# Script Symbol Observation Extraction Claim

This frame owns only a production JavaScript module and its focused tests for extracting script-symbol definition observations.

The module will:

- use the repository-pinned TypeScript language service to resolve symbol definitions;
- use Tree-sitter JavaScript/TypeScript only to identify exact concrete-syntax anchors in the original source;
- emit deterministic provider observations whose anchors preserve exact original byte and point coordinates;
- distinguish supported observations from unsupported input and analyzer failure without claiming completeness;
- perform no framework runtime inference, callback invocation, task execution, repaint inference, or feature-identity inference.

This frame does not own command framing, template extraction, cross-document custody, capability advertisement, lifecycle qualification, or release packaging.

Implementation remains blocked until focused assertions have produced an assertion-specific red result against a present-but-wrong production module.
