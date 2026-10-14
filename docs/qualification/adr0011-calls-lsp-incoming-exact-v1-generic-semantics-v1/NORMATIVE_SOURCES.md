# Normative source pins and excerpts

Status: `GENERIC_INCOMING_SEMANTICS_AUTHORED_NOT_GO`; authority `0`.

## LSP 3.17 call hierarchy source

Pinned source:

- Repository: `microsoft/language-server-protocol`
- Ref/commit: `f1b0378161a72018a2abeb5794532d5f6e2aebac`
- Path: `_specifications/lsp/3.17/language/callHierarchy.md`
- URL: `https://raw.githubusercontent.com/microsoft/language-server-protocol/f1b0378161a72018a2abeb5794532d5f6e2aebac/_specifications/lsp/3.17/language/callHierarchy.md`
- Byte length observed: `6812`
- SHA-256 observed: `5fd617bd3f3d6cb5d6dbd754febeeaebe6525808ff20bbb4ca85cee6c832eaf3`

## CallHierarchyItem endpoint contract excerpt

```text
* result: `CallHierarchyItem[] | null` defined as follows:
```

```typescript
export interface CallHierarchyItem {
	/**
	 * The name of this item.
	 */
	name: string;

	/**
	 * The kind of this item.
	 */
	kind: SymbolKind;

	/**
	 * Tags for this item.
	 */
	tags?: SymbolTag[];

	/**
	 * More detail for this item, e.g. the signature of a function.
	 */
	detail?: string;

	/**
	 * The resource identifier of this item.
	 */
	uri: DocumentUri;

	/**
	 * The range enclosing this symbol not including leading/trailing whitespace
	 * but everything else, e.g. comments and code.
	 */
	range: Range;

	/**
	 * The range that should be selected and revealed when this symbol is being
	 * picked, e.g. the name of a function. Must be contained by the
	 * [`range`](#CallHierarchyItem.range).
	 */
	selectionRange: Range;

	/**
	 * A data entry field that is preserved between a call hierarchy prepare and
	 * incoming calls or outgoing calls requests.
	 */
	data?: LSPAny;
}
```

## Incoming-call member excerpt

```text
The request is sent from the client to the server to resolve incoming calls for a given call hierarchy item.
```

```text
* method: `callHierarchy/incomingCalls`
```

```typescript
export interface CallHierarchyIncomingCallsParams extends
	WorkDoneProgressParams, PartialResultParams {
	item: CallHierarchyItem;
}
```

```text
* result: `CallHierarchyIncomingCall[] | null` defined as follows:
```

```typescript
export interface CallHierarchyIncomingCall {

	/**
	 * The item that makes the call.
	 */
	from: CallHierarchyItem;

	/**
	 * The ranges at which the calls appear. This is relative to the caller
	 * denoted by [`this.from`](#CallHierarchyIncomingCall.from).
	 */
	fromRanges: Range[];
}
```

```text
* partial result: `CallHierarchyIncomingCall[]`
```

## Consequences

- A returned `from` is a full `CallHierarchyItem`; its required endpoint identity/display fields are `name`, `kind`, `uri`, `range`, and `selectionRange`.
- `tags`, `detail`, and `data` are optional endpoint metadata and must be retained when supplied, but they are not call-site occurrence ranges.
- `from.range` is the returned caller endpoint enclosing/display span, and `from.selectionRange` is the selected/revealed span contained by `from.range`. They identify/display the caller endpoint; they do not become CALLS occurrence rows.
- The incoming member's `fromRanges` are the only normative call-site occurrence ranges: "The ranges at which the calls appear" and "relative to the caller denoted by this.from".
- The response may be complete top-level `CallHierarchyIncomingCall[]` or `null`; this supports distinct top-level `[]` and `null` terminal cases when the generic terminal policy admits each.
- Inside a member, `from` and `fromRanges` are non-optional fields. Missing/null/malformed `from` or missing/null `fromRanges` is malformed for semantic admission.
- No excerpt requires `fromRanges: Range[]` to be non-empty. Therefore a present empty array is a valid explicit zero-range member unless a separate accepted contract imposes a stricter non-empty rule. No such stricter generic incoming rule is accepted here.

## Non-authorities

- Go/gopls behavior is not used as semantic authority for this package.
- Definition/Reference occurrence schemas or ledgers do not create CALLS occurrences.
- The failed provider candidate in the worktree is not cited as normative source.
