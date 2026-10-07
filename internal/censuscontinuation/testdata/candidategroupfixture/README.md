# Real-community handoff record

The historical failed real-community evaluation remains part of this fixture. The successor evidence is additive: it does not erase, reinterpret, or convert that failure into an accepted candidate, feature, or usefulness result.

The failed handoff used Program C's human-facing `location` as if it were a managed structural-request position. That field is intentionally one-based and is derived from the node display range, so it is not a machine locator. In the retained examples this produced the wrong coordinates for real members.

Program C now retains `location` unchanged for display and adds `target` for machine use. `target` is derived only from the graph node's `selection_range.start`; it carries a canonical fragment-free file URI, zero-based line and character, `utf-16`, `coordinate_base: 0`, `range_role: SELECTION_RANGE`, and the Program C node ID and kind. Managed structural adapters consume only this target. They do not parse `location` or `portable_locators` as URIs.

Exact targets outside the managed workspace terminate as `SOURCE_UNAVAILABLE` with reason `DOCUMENT_OUTSIDE_WORKSPACE` before document-symbol, call-hierarchy, locator, or source-projection activity. This preserves workspace-only disclosure and does not inspect SDK source.
