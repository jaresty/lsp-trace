I want my responses formatted with a "token derivation" structure. The axis taxonomy and token definitions below are verbatim and authoritative — use them as written.

=== REQUEST 依頼 ===
The response creates new content that did not previously exist, based on the input and constraints.

Correct fresh V6 capture so outside-workspace endpoint URIs remain nominated and become typed SOURCE_UNAVAILABLE, while only canonical in-workspace URIs enter the global managed-preparation barrier; all eligible documents must prepare before any resolver call. Preserve exact session/generation custody, ordering, no fallback/additional roots, no URI leakage, and historical behavior. Use the fresh DOCUMENT_OUTSIDE_WORKSPACE 12/11/17/11 observation and existing V6 missing-document behavior as the counterfactual witness. Derive the smallest RED-first implementation seam and stop conditions.

=== AXES 軸 (token types — each governs a different dimension) ===
- topology
- completeness: Depth of coverage — from a quick pass to exhaustive treatment.
- method: Reasoning approach — how to think through the problem. Up to five can be combined.
Axis interaction: completeness sets the depth at which each method step runs; scope sets what the method reasons about; method shapes how it is sequenced. Derive the combined stance across axes before producing output.

=== TOKENS 役割 ===
Issue all `bar help token <slug>` calls in a single Bash tool call joined with `&&` before writing any `Loaded:` lines.

For each slug in this TOKENS section's bullet list (excluding `persona = (none)`), run `bar help token <slug>` as a Bash tool call. You may append `--skip "<phrase>"` where `<phrase>` is a verbatim substring of that token's Definition or Heuristics known from this session — the binary emits a one-line confirmation when the phrase matches, or full output when it does not. Either output satisfies the tool-result requirement. A `Loaded: <slug>` line is valid if and only if a tool-result block whose first line is `# Token: <slug>` or `# Token: <slug> (confirmed: "...")` appears in this transcript above this TOKENS section than the `Loaded:` line, and the `Loaded:` line's `when:` value is a verbatim complete semicolon-delimited phrase from the `Heuristics` line of that tool-result block (or the confirmed phrase when a confirmation line appeared), and the `not:` value is a different verbatim complete semicolon-delimited phrase from the same `Heuristics` line.

The `Loaded:` line form is: `Loaded: <slug> (when: "<phrase>" — not: "<phrase>" — because: <reason this token applies to this invocation>)`. The `because:` value must name why this token is being applied — a token present in the TOKENS list is always applied; its presence is the decision. A `because:` value that questions applicability, concludes the token is unnecessary, or declines to apply it is a validity violation — the token is mandatory if listed.

Do not skip based on memory or inference — the call is always required.

In a parallel batch, the `Loaded:` lines for all slugs in the batch must appear as consecutive lines in the next assistant output block — one per slug, in the order their tool-result blocks appeared, beginning with the next line of assistant output that is not itself a tool-result block.

Before writing `Token loads complete.`, write: `Loads verified: <slug1>, ... (<N> of <N>)` where N is the count of distinct valid `Loaded: <slug>` lines appearing above this line in the transcript, each for a slug in this TOKENS section's bullet list. Then write `Token loads complete.`

In the Token derivations block, for each active token write: `[slug]: "[verbatim Heuristics phrase from the tool-result block whose first line is # Token: <slug>, appearing above the valid Loaded: line for this slug]" → "[verbatim Description text up to the first ' —' or first '.'; if neither appears, use full Description]" as applied here: [manifestation in this response]`. A derivations line whose `→` clause does not appear verbatim in that tool-result block does not satisfy this requirement.

`Token loads complete.` is not a turn-end signal and no user message may appear between it and the Token derivations block.
- topology = witness 観  → bar help token witness
- completeness = full 全  → bar help token full
- method = ground 地  → bar help token ground
- method = gate 閘  → bar help token gate
- method = falsify 偽  → bar help token falsify
- method = atomic 粒  → bar help token atomic
- persona = (none)

=== COMPOSITION RULES 合成 (CO-PRESENCE) ===
↓ [Additional rules that apply because specific token combinations are co-present. Applied on top of TOKENS.]
Issue all `bar help composition <slug>` calls in a single parallel batch before writing any `Composition active:` lines.

For each composition line below, run `bar help composition <slug>` as a tool call. You may append `--skip "<phrase>"` where `<phrase>` is a verbatim substring of the composition prose known from this session — the binary emits a one-line confirmation when the phrase matches, or full output when it does not. Either output satisfies the tool-result requirement. The call is always required — do not skip based on memory or inference. After the tool-result block appears, write `Loaded: <slug>` — a `Loaded:` line not preceded by a tool-result block for that slug does not satisfy this requirement. After writing `Loaded: <slug>`, immediately write `Composition active: <slug> (when: "<verbatim condition from the composition rule body that applies to this invocation>" — because: <reason this specific invocation activates it>)` — a `Composition active:` line not appearing immediately after a `Loaded: <slug>` line does not satisfy this requirement; a `Composition active:` line whose `when:` value is not a verbatim substring of the composition rule body returned by `bar help composition <slug>` does not satisfy this requirement. All `Loaded:` and `Composition active:` pairs for every slug listed here must appear before the `Token loads complete.` line from the TOKENS section above. Each composition is a binding constraint on this response — its rules apply throughout.
- ground+falsify  → bar help composition ground+falsify
- falsify+atomic  → bar help composition falsify+atomic
- gate+atomic  → bar help composition gate+atomic

=== FORMAT 形式 ===
Before task content, write a token derivation block. Begin with the literal line 'Token derivations:' — this marks the start of the derivation span. For each active token, write one line of the form '[token-name]: [effect]' where the effect names how this token changes the response relative to a version without it. For each method token, add a second line: 'What it requires here: [procedure specific to this task content]'. Then write a combined stance paragraph — the paragraph is non-hollow when it contains at least one clause of the form 'without [token-name], this response would [specific change in content, reasoning, or structure]'. Write 'Derived stance complete.' to close the derivation span. No tool call result blocks appear between 'Token derivations:' and 'Derived stance complete.' — a tool call result block in that span renders the derivation non-compliant.

=== META INTERPRETATION ===
The response appends a section beginning with '## Model interpretation' after all task content. This section contains: a summary of interpretive choices, key assumptions as short bullets (exception: do not name any directional token by name — its effect should be evident from the response flow), gaps and up to three verification items, one improved framing sentence, and at most one line of the form 'Suggestion: <axis>=<token>'. A Suggestion line is permitted only when the definition text of the suggested token appears in a tool call result block or user message earlier in this transcript — not by name recognition alone. If the token catalog's heuristics and distinctions are available in context (for example, via bar help or bar lookup output), prefer those over the definition alone: read the heuristics to confirm the intent matches, and read the distinctions to confirm no other token is a better fit. If neither heuristics and distinctions nor a definition are present in context, omit the Suggestion line entirely. When suggesting, prefer an existing axis name (for example, completeness, scope, method, form) and a single existing axis token (for example, deep, narrow, bullets). Do not include multiple options (no lists, pipes, or slashes). No directional token slug appears by name in this section. This section must not appear in any SUBJECT or ADDENDUM block of a subsequent bar build invocation.
