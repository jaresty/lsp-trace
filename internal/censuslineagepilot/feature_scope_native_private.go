package censuslineagepilot

import (
	"context"
	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/lsp"
)

// Keep WireClient's strict native contracts, but extend the admission lock and
// terminal guarantee through its post-callback validators. No directional lease
// callback is installed. Pre-response refusals do not manufacture consumed bytes.
type scopeNativeClient struct {
	adapter *scopeRuntimeAdapter
	wire    acquisition.Client
	symbols func() []lsp.DocumentSymbol
}

func (c scopeNativeClient) DocumentSymbols(ctx context.Context, p lsp.DocumentSymbolParams) (rows []lsp.DocumentSymbol, err error) {
	err = c.adapter.invoke(func() error {
		var e error
		rows, e = c.wire.DocumentSymbols(ctx, p)
		if e == nil {
			rows = c.symbols()
		}
		return e
	})
	return
}
func (c scopeNativeClient) PrepareCallHierarchy(ctx context.Context, p lsp.PrepareCallHierarchyParams) (rows []lsp.CallHierarchyItem, err error) {
	err = c.adapter.invoke(func() error { var e error; rows, e = c.wire.PrepareCallHierarchy(ctx, p); return e })
	return
}
func (c scopeNativeClient) IncomingCalls(ctx context.Context, item lsp.CallHierarchyItem) (rows []lsp.CallHierarchyIncomingCall, null bool, err error) {
	err = c.adapter.invoke(func() error { var e error; rows, null, e = c.wire.IncomingCalls(ctx, item); return e })
	return
}
func (c scopeNativeClient) OutgoingCalls(ctx context.Context, item lsp.CallHierarchyItem) (rows []lsp.CallHierarchyOutgoingCall, null bool, err error) {
	err = c.adapter.invoke(func() error { var e error; rows, null, e = c.wire.OutgoingCalls(ctx, item); return e })
	return
}
