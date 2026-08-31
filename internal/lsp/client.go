package lsp

import (
	"context"
	"encoding/json"
	"os"

	"lsp-trace/internal/jsonrpc"
)

type Client struct {
	conn             *jsonrpc.Conn
	InitializeResult InitializeResult
}

func NewClient(conn *jsonrpc.Conn) *Client { return &Client{conn: conn} }
func (c *Client) Initialize(ctx context.Context, rootURI string) error {
	params := InitializeParams{ProcessID: os.Getpid(), ClientInfo: ClientInfo{Name: "lsp-trace"}, RootURI: rootURI, Capabilities: map[string]interface{}{"textDocument": map[string]interface{}{"callHierarchy": map[string]interface{}{"dynamicRegistration": false}, "typeHierarchy": map[string]interface{}{"dynamicRegistration": false}}}}
	if err := c.conn.Call(ctx, "initialize", params, &c.InitializeResult); err != nil {
		return err
	}
	return c.conn.Notify("initialized", struct{}{})
}
func (c *Client) SupportsCallHierarchy() bool {
	return supportsProvider(c.InitializeResult.Capabilities.CallHierarchyProvider)
}
func (c *Client) SupportsTypeHierarchy() bool {
	return supportsProvider(c.InitializeResult.Capabilities.TypeHierarchyProvider)
}
func supportsProvider(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "false" {
		return false
	}
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		return b
	}
	var obj map[string]interface{}
	return json.Unmarshal(raw, &obj) == nil
}
func (c *Client) DidOpen(uri, languageID, text string) error {
	return c.conn.Notify("textDocument/didOpen", DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, LanguageID: languageID, Version: 1, Text: text}})
}
func (c *Client) PrepareCallHierarchy(ctx context.Context, p PrepareCallHierarchyParams) ([]CallHierarchyItem, error) {
	var raw json.RawMessage
	if err := c.conn.Call(ctx, "textDocument/prepareCallHierarchy", p, &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var items []CallHierarchyItem
	return items, json.Unmarshal(raw, &items)
}
func (c *Client) PrepareTypeHierarchy(ctx context.Context, p PrepareTypeHierarchyParams) ([]TypeHierarchyItem, error) {
	var raw json.RawMessage
	if err := c.conn.Call(ctx, "textDocument/prepareTypeHierarchy", p, &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var items []TypeHierarchyItem
	return items, json.Unmarshal(raw, &items)
}
func (c *Client) Subtypes(ctx context.Context, item TypeHierarchyItem) ([]TypeHierarchyItem, error) {
	var raw json.RawMessage
	if err := c.conn.Call(ctx, "typeHierarchy/subtypes", TypeHierarchySubtypesParams{Item: item}, &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var items []TypeHierarchyItem
	return items, json.Unmarshal(raw, &items)
}
func (c *Client) IncomingCalls(ctx context.Context, item CallHierarchyItem) ([]CallHierarchyIncomingCall, bool, error) {
	var raw json.RawMessage
	if err := c.conn.Call(ctx, "callHierarchy/incomingCalls", CallHierarchyIncomingCallsParams{Item: item}, &raw); err != nil {
		return nil, false, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, true, nil
	}
	var calls []CallHierarchyIncomingCall
	return calls, false, json.Unmarshal(raw, &calls)
}
func (c *Client) Shutdown(ctx context.Context) error {
	if err := c.conn.Call(ctx, "shutdown", nil, nil); err != nil {
		return err
	}
	return c.conn.Notify("exit", nil)
}
