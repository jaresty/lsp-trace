package acquisition

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"lsp-trace/internal/lsp"
)

// WireRequest carries the exact declared context and per-wire limits to a host
// adapter. The host validates its actual generation and bounds protocol messages
// and bytes before returning. No metadata lookup or generation inference occurs.
type WireRequest struct {
	Context     AcquisitionContext
	Method      string
	Params      json.RawMessage
	MaxBytes    int
	MaxMessages int
}
type RoundTrip func(context.Context, WireRequest) (json.RawMessage, error)
type callScope struct {
	context AcquisitionContext
	limits  Limits
}
type callScopeKey struct{}

type WireClient struct{ roundTrip RoundTrip }

func NewWireClient(roundTrip RoundTrip) *WireClient { return &WireClient{roundTrip: roundTrip} }
func (c *WireClient) call(ctx context.Context, method string, params, target any) (bool, error) {
	scope, ok := ctx.Value(callScopeKey{}).(callScope)
	if !ok || c == nil || c.roundTrip == nil {
		return false, errors.New("ACQUISITION_CONTEXT_UNAVAILABLE")
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return false, err
	}
	response, err := c.roundTrip(ctx, WireRequest{Context: scope.context, Method: method, Params: raw, MaxBytes: scope.limits.MaxResponseBytes, MaxMessages: scope.limits.MaxMessages})
	if err != nil {
		return false, err
	}
	if len(response) > scope.limits.MaxResponseBytes {
		return false, errors.New("WIRE_RESPONSE_BYTES")
	}
	if bytes.Equal(bytes.TrimSpace(response), []byte("null")) {
		return true, nil
	}
	if err = validateWireShape(method, response); err != nil {
		return false, err
	}
	decoder := json.NewDecoder(bytes.NewReader(response))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(target); err != nil {
		return false, fmt.Errorf("malformed %s: %w", method, err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return false, errors.New("multiple JSON response values")
	}
	return false, nil
}
func (c *WireClient) DocumentSymbols(ctx context.Context, p lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	var rows []json.RawMessage
	null, e := c.call(ctx, "textDocument/documentSymbol", p, &rows)
	if null || e != nil {
		return nil, e
	}
	flat := false
	for i, row := range rows {
		var discriminator struct {
			Location json.RawMessage `json:"location"`
		}
		if e := json.Unmarshal(row, &discriminator); e != nil {
			return nil, e
		}
		rowFlat := len(discriminator.Location) != 0
		if i == 0 {
			flat = rowFlat
		} else if rowFlat != flat {
			return nil, errors.New("malformed textDocument/documentSymbol: mixed result variants")
		}
	}

	out := make([]lsp.DocumentSymbol, 0, len(rows))
	for _, row := range rows {
		if !flat {
			var symbol wireSymbol
			if e := decodeStrict(row, &symbol); e != nil {
				return nil, e
			}
			out = append(out, symbol.symbol())
			continue
		}
		var symbol symbolInformationWire
		if e := decodeStrict(row, &symbol); e != nil {
			return nil, e
		}
		if symbol.Location.URI != p.TextDocument.URI {
			continue
		}
		out = append(out, lsp.DocumentSymbol{Name: symbol.Name, Kind: symbol.Kind, Range: symbol.Location.Range, SelectionRange: symbol.Location.Range, Flat: true})
	}
	return out, nil
}
func (c *WireClient) PrepareCallHierarchy(ctx context.Context, p lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	var items []lsp.CallHierarchyItem
	_, e := c.call(ctx, "textDocument/prepareCallHierarchy", p, &items)
	return items, e
}
func (c *WireClient) IncomingCalls(ctx context.Context, i lsp.CallHierarchyItem) ([]lsp.CallHierarchyIncomingCall, bool, error) {
	var calls []lsp.CallHierarchyIncomingCall
	n, e := c.call(ctx, "callHierarchy/incomingCalls", lsp.CallHierarchyIncomingCallsParams{Item: i}, &calls)
	return calls, n, e
}
func (c *WireClient) OutgoingCalls(ctx context.Context, i lsp.CallHierarchyItem) ([]lsp.CallHierarchyOutgoingCall, bool, error) {
	var calls []lsp.CallHierarchyOutgoingCall
	n, e := c.call(ctx, "callHierarchy/outgoingCalls", lsp.CallHierarchyOutgoingCallsParams{Item: i}, &calls)
	return calls, n, e
}

type wireSymbol struct {
	Name           string       `json:"name"`
	Detail         string       `json:"detail,omitempty"`
	Kind           int          `json:"kind"`
	Tags           []int        `json:"tags,omitempty"`
	Deprecated     bool         `json:"deprecated,omitempty"`
	Range          lsp.Range    `json:"range"`
	SelectionRange lsp.Range    `json:"selectionRange"`
	Children       []wireSymbol `json:"children,omitempty"`
}

type symbolInformationWire struct {
	Name       string `json:"name"`
	Kind       int    `json:"kind"`
	Tags       []int  `json:"tags,omitempty"`
	Deprecated bool   `json:"deprecated,omitempty"`
	Location   struct {
		URI   string    `json:"uri"`
		Range lsp.Range `json:"range"`
	} `json:"location"`
	ContainerName string `json:"containerName,omitempty"`
}

func (s wireSymbol) symbol() lsp.DocumentSymbol {
	out := lsp.DocumentSymbol{Name: s.Name, Detail: s.Detail, Kind: s.Kind, Range: s.Range, SelectionRange: s.SelectionRange}
	for _, child := range s.Children {
		out.Children = append(out.Children, child.symbol())
	}
	return out
}

func decodeStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("multiple JSON values")
	}
	return nil
}

// Check presence separately from Go zero-values; otherwise absent coordinates
// are indistinguishable from a valid zero-based position. Bounded JSON nesting
// is also enforced before recursive standard-library decoding or normalization.
func validateWireShape(method string, raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	depth := 0
	for {
		tok, e := decoder.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		if d, ok := tok.(json.Delim); ok {
			switch d {
			case '[', '{':
				depth++
				if depth > 128 {
					return errors.New("WIRE_NESTING_LIMIT")
				}
			case ']', '}':
				depth--
			}
		}
	}
	var rows []map[string]json.RawMessage
	if e := json.Unmarshal(raw, &rows); e != nil {
		return e
	}
	type entry struct {
		object map[string]json.RawMessage
		symbol bool
	}
	stack := []entry{}
	for _, row := range rows {
		switch method {
		case "textDocument/documentSymbol":
			if location, ok := row["location"]; ok {
				for _, key := range []string{"name", "kind"} {
					if v, present := row[key]; !present || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
						return fmt.Errorf("missing %s", key)
					}
				}
				var fields map[string]json.RawMessage
				if e := json.Unmarshal(location, &fields); e != nil {
					return e
				}
				if uri, present := fields["uri"]; !present || bytes.Equal(bytes.TrimSpace(uri), []byte("null")) {
					return errors.New("missing uri")
				}
				if e := requiredRange(fields["range"]); e != nil {
					return e
				}
				continue
			}
			stack = append(stack, entry{row, true})
		case "textDocument/prepareCallHierarchy":
			stack = append(stack, entry{row, false})
		default:
			field := "from"
			if method == "callHierarchy/outgoingCalls" {
				field = "to"
			}
			var item map[string]json.RawMessage
			if e := json.Unmarshal(row[field], &item); e != nil {
				return e
			}
			stack = append(stack, entry{item, false})
			var ranges []json.RawMessage
			if v, ok := row["fromRanges"]; !ok {
				return errors.New("missing fromRanges")
			} else if trimmed := bytes.TrimSpace(v); len(trimmed) == 0 || trimmed[0] != '[' {
				return errors.New("fromRanges must be a non-null array")
			} else if e := json.Unmarshal(v, &ranges); e != nil {
				return e
			}
			for _, r := range ranges {
				if e := requiredRange(r); e != nil {
					return e
				}
			}
		}
	}
	for len(stack) > 0 {
		entry := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		row := entry.object
		for _, key := range []string{"name", "kind", "range", "selectionRange"} {
			if len(row[key]) == 0 || bytes.Equal(row[key], []byte("null")) {
				return fmt.Errorf("missing %s", key)
			}
		}
		if !entry.symbol && len(row["uri"]) == 0 {
			return errors.New("missing uri")
		}
		for _, key := range []string{"range", "selectionRange"} {
			if e := requiredRange(row[key]); e != nil {
				return e
			}
		}
		if entry.symbol && len(row["children"]) > 0 {
			var children []map[string]json.RawMessage
			if e := json.Unmarshal(row["children"], &children); e != nil {
				return e
			}
			for _, ch := range children {
				stack = append(stack, struct {
					object map[string]json.RawMessage
					symbol bool
				}{ch, true})
			}
		}
	}
	return nil
}
func requiredRange(raw json.RawMessage) error {
	var r map[string]json.RawMessage
	if e := json.Unmarshal(raw, &r); e != nil {
		return e
	}
	for _, key := range []string{"start", "end"} {
		var p map[string]json.RawMessage
		if e := json.Unmarshal(r[key], &p); e != nil {
			return e
		}
		for _, coord := range []string{"line", "character"} {
			v, ok := p[coord]
			if !ok || bytes.Equal(v, []byte("null")) {
				return errors.New("missing range coordinate")
			}
			var n uint32
			if e := json.Unmarshal(v, &n); e != nil {
				return e
			}
		}
	}
	return nil
}

// WithDocumentSupply wraps a client only when supply is available. Every actual
// invocation of supply is budgeted and its returned observation is retained.
func WithDocumentSupply(client Client, supply func(context.Context, AcquisitionContext, Locator) (Supply, error)) Client {
	if supply == nil {
		return client
	}
	return suppliedClient{Client: client, supply: supply}
}

type suppliedClient struct {
	Client
	supply func(context.Context, AcquisitionContext, Locator) (Supply, error)
}

func (c suppliedClient) PrepareDocument(ctx context.Context, a AcquisitionContext, l Locator) (Supply, error) {
	return c.supply(ctx, a, l)
}
