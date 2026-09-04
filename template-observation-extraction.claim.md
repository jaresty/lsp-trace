# Template Observation Extraction Claim

Baseline: `0702d12`
Frame: Template Observation Extraction

## Ownership

This work owns only a production JavaScript module and its focused tests for extracting provider observations from Ember template AST evidence.

It does not own command framing, a TypeScript adapter, cross-document custody, provider orchestration, packaging, qualification evidence, or release admission.

## Claimed behavior

- Parse templates with the repository-pinned Ember Template Compiler.
- Emit only syntax-level observations constructible from qualified template AST nodes.
- Preserve exact original-template source anchors.
- Emit explicit support and non-support claims in deterministic source order.
- Distinguish unsupported input from extraction failure.
- Advertise only relation kinds the extractor constructs from qualified AST evidence.

## Non-claims

The extractor does not claim callback execution, task execution, runtime behavior, repaint behavior, feature identity, rendering execution, relation absence beyond its bounded evidence, or source completeness.

No Go Ember parser is introduced.
