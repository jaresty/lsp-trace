// Package structuralcontextsymbolops resolves one workspace symbol solely as a
// locator before delegating unchanged analysis to structural context.
package structuralcontextsymbolops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"lsp-trace/incomingops"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/operation"
)

const Operation operation.Name = "structural_context_symbol"

type Delegate interface {
	Execute(context.Context, operation.Request) (operation.Result, *operation.Failure)
}

type Executor struct {
	runtime  incomingops.Runtime
	delegate Delegate
}

func NewExecutor(runtime incomingops.Runtime, delegate Delegate) *Executor {
	return &Executor{runtime: runtime, delegate: delegate}
}

type analysis struct {
	Kind      string `json:"kind"`
	Direction string `json:"direction,omitempty"`
	Depth     int    `json:"depth,omitempty"`
}
type request struct {
	SessionID        string   `json:"session_id"`
	Generation       uint64   `json:"generation"`
	Symbol           string   `json:"symbol"`
	DownDepth        int      `json:"down_depth"`
	UpDepth          int      `json:"up_depth"`
	MaxNodes         int      `json:"max_nodes"`
	TimeoutMS        int64    `json:"timeout_ms"`
	RequestTimeoutMS int64    `json:"request_timeout_ms"`
	MaxMessages      int      `json:"max_messages,omitempty"`
	MaxBytes         int64    `json:"max_bytes,omitempty"`
	Analysis         analysis `json:"analysis"`
}
type delegatedRequest struct {
	SessionID        string   `json:"session_id"`
	Generation       uint64   `json:"generation"`
	URI              string   `json:"uri"`
	Line             *uint32  `json:"line"`
	Character        *uint32  `json:"character"`
	DownDepth        int      `json:"down_depth"`
	UpDepth          int      `json:"up_depth"`
	MaxNodes         int      `json:"max_nodes"`
	TimeoutMS        int64    `json:"timeout_ms"`
	RequestTimeoutMS int64    `json:"request_timeout_ms"`
	MaxMessages      int      `json:"max_messages,omitempty"`
	MaxBytes         int64    `json:"max_bytes,omitempty"`
	Analysis         analysis `json:"analysis"`
}

func (e *Executor) Execute(parent context.Context, op operation.Request) (operation.Result, *operation.Failure) {
	if e == nil || e.runtime == nil || e.delegate == nil || op.Name != Operation {
		return fail(operation.FailureNotImplemented, operation.ErrNotImplemented)
	}
	var in request
	if err := decodeClosed(op.Input, &in); err != nil {
		return fail(operation.FailureInvalidInput, err)
	}
	if in.SessionID == "" || in.Generation == 0 || in.Symbol == "" || in.TimeoutMS < 1 || in.RequestTimeoutMS < 1 {
		return fail(operation.FailureInvalidInput, errors.New("required symbol request fields are invalid"))
	}
	id, generation, sf := incomingops.ResolveSession(e.runtime, in.SessionID, in.Generation)
	if sf != "" {
		return fail(string(sf), nil)
	}
	metadata, sf := e.runtime.Metadata(id, generation)
	if sf != "" {
		return fail(string(sf), nil)
	}
	if !metadata.WorkspaceSymbolSupport {
		return fail("UNSUPPORTED", nil)
	}
	workspace := workspaceRoot(e.runtime, id, generation)
	ctx, cancel := context.WithTimeout(parent, time.Duration(in.TimeoutMS)*time.Millisecond)
	defer cancel()
	if in.MaxMessages == 0 {
		in.MaxMessages = 64
	}
	if in.MaxBytes == 0 {
		in.MaxBytes = 4 << 20
	}
	client := incomingops.NewSessionClientWithWireLimits(e.runtime, id, generation, time.Duration(in.RequestTimeoutMS)*time.Millisecond, incomingops.WireLimits{MaxMessages: in.MaxMessages, MaxBytes: in.MaxBytes})
	symbols, err := client.WorkspaceSymbols(ctx, lsp.WorkspaceSymbolParams{Query: in.Symbol})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fail("REQUEST_TIMEOUT", err)
		}
		if errors.Is(err, context.Canceled) {
			return fail("CANCELLED", err)
		}
		return fail("WORKSPACE_SYMBOL_MALFORMED", err)
	}
	matches := make([]lsp.WorkspaceSymbol, 0, 1)
	for _, symbol := range symbols {
		if symbol.Name == in.Symbol {
			matches = append(matches, symbol)
		}
	}
	if len(matches) == 0 {
		return failWithDiagnostics("WORKSPACE_SYMBOL_ABSENT", fmt.Errorf("no exact workspace symbol match"), []string{fmt.Sprintf("exact_matches=0 returned_candidates=%d", len(symbols))})
	}
	if len(matches) > 1 {
		return failWithDiagnostics("WORKSPACE_SYMBOL_AMBIGUOUS", fmt.Errorf("multiple exact workspace symbol matches"), []string{fmt.Sprintf("exact_matches=%d diagnostics_bounded=true", len(matches))})
	}
	location := matches[0].Location
	if location.Range == nil || !validRange(*location.Range) || !concreteConfinedDocument(workspace, location.URI) {
		code := "WORKSPACE_SYMBOL_MALFORMED"
		if location.Range != nil && validRange(*location.Range) {
			code = "WORKSPACE_SYMBOL_OUTSIDE_WORKSPACE"
		}
		return fail(code, errors.New("workspace symbol location is not one concrete confined document"))
	}
	line, character := location.Range.Start.Line, location.Range.Start.Character
	raw, err := json.Marshal(delegatedRequest{SessionID: id, Generation: generation, URI: location.URI, Line: &line, Character: &character, DownDepth: in.DownDepth, UpDepth: in.UpDepth, MaxNodes: in.MaxNodes, TimeoutMS: in.TimeoutMS, RequestTimeoutMS: in.RequestTimeoutMS, MaxMessages: in.MaxMessages, MaxBytes: in.MaxBytes, Analysis: in.Analysis})
	if err != nil {
		return fail(operation.FailureInternal, err)
	}
	return e.delegate.Execute(parent, operation.Request{Name: operation.Name("structural_context"), Input: raw})
}

func workspaceRoot(runtime incomingops.Runtime, id string, generation uint64) string {
	for _, record := range runtime.Records() {
		if record.SessionID == id && record.Generation == generation {
			return record.Routing.WorkspaceRoot
		}
	}
	return ""
}
func concreteConfinedDocument(workspace, raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.String() != raw || workspace == "" {
		return false
	}
	path, err := url.PathUnescape(u.EscapedPath())
	if err != nil {
		return false
	}
	path = filepath.Clean(path)
	root := filepath.Clean(workspace)
	if path == root {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
func validRange(r lsp.Range) bool {
	return r.End.Line > r.Start.Line || r.End.Line == r.Start.Line && r.End.Character >= r.Start.Character
}
func decodeClosed(raw []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return errors.New("one JSON value required")
	}
	return nil
}
func fail(code string, err error) (operation.Result, *operation.Failure) {
	return operation.Result{}, &operation.Failure{Code: code, Err: err}
}
func failWithDiagnostics(code string, err error, diagnostics []string) (operation.Result, *operation.Failure) {
	return operation.Result{}, &operation.Failure{Code: code, Err: err, Diagnostics: diagnostics}
}
