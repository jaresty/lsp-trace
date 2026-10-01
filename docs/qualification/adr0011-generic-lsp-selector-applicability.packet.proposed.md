# ADR0011 generic LSP 3.17 selector applicability — read-only decision packet

**Status: PROPOSED, NOT ACCEPTED.** Requested target: `ACCEPT_READ_ONLY_GENERIC_LSP_SELECTOR_APPLICABILITY_PACKET`. This is a shared applicability design for `textDocument/references` and `textDocument/definition`, not a new immutable policy original, B4 implementation, source issuer, admission decision, or provider qualification. Only `SUPPORTED` could eventually authorize a request, after all other guards. All fixtures below are **declarative expected results, not executed tests**. B3 remains private; Checkpoint B remains unaccepted.

## Pinned normative-source snapshot and conflicts

Source repository `https://github.com/microsoft/language-server-protocol`, `gh-pages` commit `f1b0378161a72018a2abeb5794532d5f6e2aebac` (2026-09-29). This pins the 3.17 page at a later repository snapshot, **not** the original release-time tree. Exact source-file SHA-256 (paths relative to repository):

| Source | SHA-256 | Relevant lines |
|---|---|---|
| `_specifications/lsp/3.17/specification.md` | `6553187ee72c18eb71d583ed25323843f66d70695097d2e0c1b1f0dad00e4363` | URI include; base JSON types |
| `_specifications/lsp/3.17/types/documentFilter.md` | `23cc68dff3320f89e45471e23a91ac6327385b1939006b8f67c1b331d7b93673` | 10–47: language, scheme, glob operators, at least one field |
| `_includes/messages/3.17/registerCapability.md` | `b41ffa7715056b899f7978e6442a7841b6300e54d59f1dcaba9245dab4b4d223` | 19–46, 92–107: IDs, methods, options, selector |
| `_includes/messages/3.17/unregisterCapability.md` | `d600b196f5a987db020f51e2e58f17ff3656178c82dd6eeb5a665868edbc7f13` | 13–39: ID, method, wire spelling `unregisterations` |
| `_specifications/lsp/3.17/language/references.md` | `0a2e757e18adf23bb4b9c6fd8a98668f7630fe3d98c1c39d23cc893c5a8a5029` | 20–38: `referencesProvider`, registration options |
| `_specifications/lsp/3.17/language/definition.md` | `44e9325c941057f09077085769384c967f193f85bd8c848d35e09bc076048df1` | 29–48: `definitionProvider`, registration options |
| `_specifications/lsp/3.17/notebookDocument/notebook.md` | `3781f3e3995d76ad26ec02dfbffc80130e2d29b6d6a98648575979aad5849e9c` | 131–213, 237–239: cell filters and opaque cell URI |
| `_includes/types/uri.md` | `900502290378b0c4b7514672597f6f97ee948a1cdcdc36ae81edb69ca8c29dbc` | 1–23: RFC 3986 reference; cautions about encoding and drive-letter case |

**Review-blocking semantic deltas from LSP 3.17:** `registerCapability.md:102–105` says a null `documentSelector` uses the selector provided on the **client side**; the requested local rule says null means **all text documents**. `documentFilter.md:42` calls a selector a combination of *one or more* filters; the requested local rule treats `[]` as valid, matching none. The source also says a server must not register the same capability statically and dynamically for the same selector (`registerCapability.md:5`); the requested static-plus-dynamic example is a private replay stress case, **not evidence of compliant server behavior**. A reviewer must explicitly decide whether these are intentional local overrides and under what client-side conditions; this packet must not label them LSP-conformant or silently enable them.

## Held inputs, JSON grammar and four outcomes

The verifier selects exact query URI UTF-8 bytes, language ID bytes **including explicit absence**, ordinary-text-document versus notebook-cell kind, workspace/session/generation, and (for notebook filters) independently held enclosing notebook type and URI or explicitly marks these unavailable. No filename/provider-derived language, normalized URI, inferred notebook URI, claimant event summary, or server assertion supplies an expectation. Context/transaction mismatches fail the chronology guard, not an inferred selector match. A notebook cell's document URI is opaque (`notebook.md:5–6,131`); ordinary text-document language filters can apply to cell text documents (`notebook.md:237–239`).

Strict JSON precondition: exact one complete UTF-8 JSON value; reject duplicate **decoded** keys (including `"language"` and `"\u006canguage"`) at every object depth, invalid encoding/syntax, and trailing non-whitespace bytes as `MALFORMED`. Preserve member presence and exact original bytes, never remarshal originals for custody. Known grammar for the two methods:

```text
initialize.result.capabilities.<referencesProvider|definitionProvider>
  := absent | null | false | true | object
static object := { workDoneProgress?: boolean, ...retained extensions }
registration := { id: string, method: string, registerOptions?: object, ...extensions }
known registerOptions := { documentSelector?: null | filter[],
                           workDoneProgress?: boolean, ...retained extensions }
known ordinary filter := { language?: string, scheme?: string, pattern?: string,
                            ...retained extensions }; at least one known key required
known notebook-cell filter := { notebook: string | notebookFilter,
                                language?: string, ...retained extensions }
notebookFilter := object with at least one typed notebookType:string,
                  scheme:string, or pattern:string (LSP 3.17 union)
unregistration := { id: string, method: string }
```

A non-object static value other than absent/null/false/true is `MALFORMED`; a static object with wrong type for a known field is `MALFORMED`. A static valid method-options object and `true` support all otherwise eligible text documents. Unknown extensions are retained and ignored only when independently established inert for applicability; a field that may change applicability yields `UNKNOWN`, never silent wildcard. Static absent/null/false is `UNSUPPORTED` by **local selection** (LSP declares `boolean | Options`, not null).

For a matching dynamic method, absent `registerOptions` or absent `documentSelector` is `UNKNOWN`; non-object options, wrong known-field types, duplicate decoded keys and invalid known-filter shapes are `MALFORMED`. Proposed local null selector is `SUPPORTED` for all text documents (including cell text documents); `[]` is `UNSUPPORTED`; nonempty arrays use OR over filters, AND over present recognized conditions within a filter. A known filter `{}` or non-object array element is `MALFORMED`; unknown future selector/filter forms are `UNKNOWN` unless a known field has a syntactically invalid type. Matching another method never contributes support to this method. Do not treat absence, explicit null, false, true and an options object as interchangeable.

`MALFORMED` takes precedence for a relevant pre-WRITE original with known invalid grammar, even when static support exists: no exact chronology receipt may be issued from a malformed chain. A well-formed unregister with the wrong ID or method is **not malformed LSP JSON** (`unregisterCapability.md:17–28`); the proposed four-outcome model classifies it as `MALFORMED` only by a **local invalid-chronology rule**, with a distinct diagnostic cause and no exact replay. This local classification requires separate approval; it is not asserted by LSP 3.17. Otherwise static `true`/valid object or one applicable active dynamic registration yields `SUPPORTED`; absent/false/static-null plus only proven nonmatches yields `UNSUPPORTED`; unresolved relevant dynamic applicability yields `UNKNOWN` unless a separately established static contribution supports the method. Only `SUPPORTED` can authorize. A late event affects later snapshots only. An unknown extension with no established inertness cannot turn a filter into a positive match.

## Filter evidence ceiling (no invented matcher)

| Dimension | Pinned LSP statement | This packet's conservative applicability and fixture |
|---|---|---|
| `language` | `DocumentFilter.language?: string` (`documentFilter.md:10–14`). | Exact held language bytes can be compared without guessing; absent held language + required filter → `UNKNOWN`, exact unequal → `UNSUPPORTED`. Case folding is not selected: case-different values → `UNKNOWN` unless a reviewed case rule is pinned. |
| `scheme` | URI scheme and filter field (`documentFilter.md:16–19`; `uri.md:1–19` links RFC 3986). | URI parsing, mixed-case scheme handling and parser identity are not pinned here. A scheme-dependent filter is `UNKNOWN` until a separately pinned URI parser/RFC rule and tests establish extraction and case comparison. |
| `pattern` | `*` within segment; `?` one character within segment; `**` any segments; `{}` OR; `[]` range; `[!...]` negated range (`documentFilter.md:21–36`). | These are only high-level operators. Escape syntax, wildcard Unicode unit, separator/leading slash, brace nesting, malformed pattern grammar, URI path extraction and percent-decoding are not fully specified in the pinned source. **Every pattern-dependent positive match remains `UNKNOWN`**, even for `*.ts` and apparent paths; do not use a generic filesystem glob as an oracle. |
| percent-encoding | `uri.md:19–23` warns both `file:///c:/...` and `file:///C%3A/...` are valid and must not be presumed equivalent. | Raw-vs-decoded path and percent-hex case are not selected: `file:///C%3A/x.ts` versus `file:///c:/x.ts` with a path glob → `UNKNOWN`. No URI normalization. |
| combinations | Each present recognized ordinary-filter field constrains that filter; multiple filters are OR (local algorithm consistent with selector array/filter examples, not a fully specified spec matcher). | False AND term → filter `UNSUPPORTED`; otherwise unresolved term → `UNKNOWN`; all established true → `SUPPORTED`. Across filters: any supported → supported; else any unknown → unknown; else unsupported. A known malformed element makes the complete selector `MALFORMED`. |
| notebook-cell | `notebook.md:133–196` specifies notebook type/scheme/pattern and optional cell language; `:131` forbids inferring scheme/path from cell URI. | Ordinary document + notebook-only filter → `UNSUPPORTED`; cell with held notebook type and matching literal type may support only when all other conditions are established. Missing enclosing notebook original, scheme or pattern uncertainty → `UNKNOWN`; `'*'` for notebook/language is source-defined at `:145–156`. |
| extension | LSP object types allow extensions; exact applicability effects are not frozen here. | Known invalid type → `MALFORMED`; unknown future selector/filter form or applicability-changing option → `UNKNOWN`; only independently established inert extension may be retained and ignored. |

## Declarative fixture matrix (both methods unless stated)

Every row assumes valid pinned held query/context and complete originals except its named alteration. `R` = references, `D` = definition; no executable test or credential is claimed.

| Fixture | Held originals or query delta | Expected result |
|---|---|---|
| S1–S5 | each method static absent / null / false / true / `{}` | `UNSUPPORTED` / `UNSUPPORTED` / `UNSUPPORTED` / `SUPPORTED` / `SUPPORTED` |
| S6–S8 | static string; `{ "workDoneProgress": "yes" }`; `{ "workDoneProgress": false, "x-inert": 1 }` only if inertness selected | `MALFORMED`; `MALFORMED`; `SUPPORTED` conditional on reviewed inertness, otherwise `UNKNOWN` |
| D1–D5 | matching registration options missing; options null; selector omitted; selector null; selector `[]` | `UNKNOWN`; `MALFORMED`; `UNKNOWN`; proposed local `SUPPORTED`; proposed local `UNSUPPORTED` |
| D6–D9 | selector `[{"language":"go"}]`, held `go`; held language absent; held `rust`; definition registration queried as references | `SUPPORTED`; `UNKNOWN`; `UNSUPPORTED`; `UNSUPPORTED` |
| D10–D13 | selector `[{"scheme":"file"}]`, URI `FILE:///x`; selector `[{"pattern":"**/*.go"}]`, URI `file:///x.go`; `{}` filter; future `{"vendorSelector":{}}` | `UNKNOWN`; `UNKNOWN`; `MALFORMED`; `UNKNOWN` |
| D14–D17 | `[{"language":"rust"},{"language":"go"}]` held `go`; `[{"language":"go","scheme":"file"}]` held `go` with unpinned URI parser; `[null]`; duplicate decoded keys in JSON bytes `{ "language":"go", "\u006canguage":"rust" }` | `SUPPORTED` by OR; `UNKNOWN`; `MALFORMED`; `MALFORMED` |
| D18–D20 | ordinary text document and notebook-only filter; cell with held notebook type `jupyter-notebook` and `{notebook:"jupyter-notebook",language:"python"}`; cell with no held parent | `UNSUPPORTED`; `SUPPORTED` when held cell language `python`; `UNKNOWN` |
| G1–G8 | `*`, `**`, `?`, `[0-9]`, `[!0-9]`, `{ts,js}`, backslash escape, brace nesting applied to URI path | `UNKNOWN` for path applicability until exact path/escape/unit grammar selected; recognized syntax is not proof of a match |
| U1–U2 | `file:///C%3A/x.ts` versus `file:///c:/x.ts` with path glob; URI `FILE:///x.ts` versus filter scheme `file` | `UNKNOWN`; `UNKNOWN` |
| C1–C4 | register R `r1` with proposed local `{documentSelector:null}` before WRITE then unregister ID+R method, static absent; same with static true; unregister `r1`+D method; unregister unknown ID+R method | `UNSUPPORTED`; static `SUPPORTED` (synthetic nonconformant coexistence); local invalid-chronology `MALFORMED` (well-formed wire, no exact replay), original registration not removed; same local invalid-chronology outcome |
| C5–C7 | register R `r1` with proposed local `{documentSelector:null}` after WRITE (static absent); pre-WRITE D registration with that selector queried as R; reorder held complete event originals with unchanged claimant summaries | `UNSUPPORTED` at earlier WRITE; `UNSUPPORTED`; chronology mismatch → `MALFORMED`/no exact replay, not claimant-selected support |
| C8–C10 | omit/extra held event; cross-transaction held event; duplicate decoded `"id"` in a nested registration | no exact replay; no exact replay; `MALFORMED` |

Actual B4 must retain exact initialize response/result and ordered complete register/unregister original bytes, event/frame ordinals and held WRITE boundary, and derive these results **before** comparing any claimant chronology summary. The matrix cannot itself prove those originals exist. Separate independent review and a human contract decision must resolve the marked local spec conflicts and UNKNOWN families before any policy original or B4 code is proposed.
