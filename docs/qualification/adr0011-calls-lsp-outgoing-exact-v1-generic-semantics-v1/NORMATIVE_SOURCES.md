# Normative source pins and excerpts

Status: `GENERIC_OUTGOING_SEMANTICS_AUTHORED_NOT_GO`; authority `0`.

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

## Outgoing-call member excerpt

```text
The request is sent from the client to the server to resolve outgoing calls for a given call hierarchy item.
```

```text
* method: `callHierarchy/outgoingCalls`
```

```typescript
export interface CallHierarchyOutgoingCallsParams extends
	WorkDoneProgressParams, PartialResultParams {
	item: CallHierarchyItem;
}
```

```text
* result: `CallHierarchyOutgoingCall[] | null` defined as follows:
```

```typescript
export interface CallHierarchyOutgoingCall {

	/**
	 * The item that is called.
	 */
	to: CallHierarchyItem;

	/**
	 * The range at which this item is called. This is the range relative to
	 * the caller, e.g the item passed to `callHierarchy/outgoingCalls` request.
	 */
	fromRanges: Range[];
}
```

```text
* partial result: `CallHierarchyOutgoingCall[]`
```

## Consequences

- The queried `item` is the prepared caller input to `callHierarchy/outgoingCalls`; generic target binding must retain the selected prepared item/ordinal before the outgoing request.
- A returned `to` is a full `CallHierarchyItem`; its required endpoint identity/display fields are `name`, `kind`, `uri`, `range`, and `selectionRange`.
- `tags`, `detail`, and `data` are optional endpoint metadata and must be retained when supplied, but they are not call-site occurrence ranges.
- `to.range` is the returned callee endpoint enclosing/display span, and `to.selectionRange` is the selected/revealed span contained by `to.range`. They identify/display the callee endpoint; they do not become CALLS occurrence rows.
- The outgoing member's `fromRanges` are the only normative call-site occurrence ranges: the spec says they are the ranges at which the returned item is called and are relative to the caller item passed to `callHierarchy/outgoingCalls`.
- Therefore, for queried caller `A` and returned `to=B`, the normalized CALLS relationship orientation is `A -> B`.
- The response may be complete top-level `CallHierarchyOutgoingCall[]` or `null`; this supports distinct top-level `[]` and `null` terminal cases when the generic terminal policy admits each.
- Inside a member, `to` and `fromRanges` are non-optional fields. Missing/null/malformed `to` or missing/null `fromRanges` is malformed for semantic admission.
- No excerpt requires `fromRanges: Range[]` to be non-empty. Therefore a present empty array is a valid explicit zero-range outgoing member unless a separate accepted contract imposes a stricter non-empty rule. No such stricter generic outgoing rule is accepted here.

## Non-authorities

- Go/gopls behavior is not used as semantic authority for this package.
- The existing untracked outgoing provider NO_GO package is not cited as semantic authority.
- The generic incoming package is structural precedent only and does not supply outgoing semantics where direction differs.
- Definition/Reference occurrence schemas or ledgers do not create CALLS occurrences.
