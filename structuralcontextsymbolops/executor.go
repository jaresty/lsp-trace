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
	"sort"
	"strings"
	"time"

	"lsp-trace/incomingops"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/transientstructural"
)

const (
	Operation   operation.Name = "structural_context_symbol"
	OperationV2 operation.Name = "structural_context_symbol_v2"
)

const (
	defaultDownDepth        = 2
	defaultUpDepth          = 2
	defaultMaxNodes         = 100
	defaultTimeoutMS        = 5000
	defaultRequestTimeoutMS = 1000
	defaultMaxMessages      = 64
	defaultMaxBytes         = 4 << 20
)

type Delegate interface {
	Execute(context.Context, operation.Request) (operation.Result, *operation.Failure)
}

type Executor struct {
	runtime           incomingops.Runtime
	delegate          Delegate
	operation         operation.Name
	delegateOperation operation.Name
}

func NewExecutor(runtime incomingops.Runtime, delegate Delegate) *Executor {
	return &Executor{runtime: runtime, delegate: delegate, operation: Operation, delegateOperation: operation.Name("structural_context")}
}

func NewV2Executor(runtime incomingops.Runtime, delegate Delegate) *Executor {
	return &Executor{runtime: runtime, delegate: delegate, operation: OperationV2, delegateOperation: operation.Name("structural_context_v2")}
}

// NewUnifiedV2Executor binds exact-symbol lookup as an internal target-resolution
// branch of the canonical structural_context_v2 operation. It does not register
// or route either historical symbol-only product operation.
func NewUnifiedV2Executor(runtime incomingops.Runtime, delegate Delegate) *Executor {
	return &Executor{runtime: runtime, delegate: delegate, operation: operation.Name("structural_context_v2"), delegateOperation: operation.Name("structural_context_v2")}
}

type analysis struct {
	Kind      string `json:"kind"`
	Direction string `json:"direction,omitempty"`
	Depth     int    `json:"depth,omitempty"`
}
type request struct {
	SessionID        string          `json:"session_id"`
	Generation       uint64          `json:"generation"`
	Symbol           string          `json:"symbol"`
	DownDepth        *int            `json:"down_depth"`
	UpDepth          *int            `json:"up_depth"`
	MaxNodes         *int            `json:"max_nodes"`
	TimeoutMS        *int64          `json:"timeout_ms"`
	RequestTimeoutMS *int64          `json:"request_timeout_ms"`
	MaxMessages      int             `json:"max_messages,omitempty"`
	MaxBytes         int64           `json:"max_bytes,omitempty"`
	Projection       json.RawMessage `json:"projection,omitempty"`
	Analysis         analysis        `json:"analysis"`
}
type delegatedRequest struct {
	SessionID        string          `json:"session_id"`
	Generation       uint64          `json:"generation"`
	URI              string          `json:"uri"`
	Symbol           string          `json:"symbol"`
	DownDepth        int             `json:"down_depth"`
	UpDepth          int             `json:"up_depth"`
	MaxNodes         int             `json:"max_nodes"`
	TimeoutMS        int64           `json:"timeout_ms"`
	RequestTimeoutMS int64           `json:"request_timeout_ms"`
	MaxMessages      int             `json:"max_messages,omitempty"`
	MaxBytes         int64           `json:"max_bytes,omitempty"`
	Projection       json.RawMessage `json:"projection,omitempty"`
	Analysis         analysis        `json:"analysis"`
}

func (e *Executor) Execute(parent context.Context, op operation.Request) (operation.Result, *operation.Failure) {
	if e == nil || e.runtime == nil || e.delegate == nil || op.Name != e.operation {
		return fail(operation.FailureNotImplemented, operation.ErrNotImplemented)
	}
	var in request
	if err := decodeClosed(op.Input, &in); err != nil {
		return fail(operation.FailureInvalidInput, err)
	}
	if in.SessionID == "" || in.Generation == 0 || in.Symbol == "" {
		return fail(operation.FailureInvalidInput, errors.New("required symbol request fields are invalid"))
	}
	downDepth := defaultDownDepth
	if in.DownDepth != nil {
		downDepth = *in.DownDepth
	}
	upDepth := defaultUpDepth
	if in.UpDepth != nil {
		upDepth = *in.UpDepth
	}
	maxNodes := defaultMaxNodes
	if in.MaxNodes != nil {
		maxNodes = *in.MaxNodes
	}
	timeoutMS := int64(defaultTimeoutMS)
	if in.TimeoutMS != nil {
		timeoutMS = *in.TimeoutMS
	}
	requestTimeoutMS := int64(defaultRequestTimeoutMS)
	if in.RequestTimeoutMS != nil {
		requestTimeoutMS = *in.RequestTimeoutMS
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
	ctx, cancel := context.WithTimeout(parent, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	if in.MaxMessages == 0 {
		in.MaxMessages = defaultMaxMessages
	}
	if in.MaxBytes == 0 {
		in.MaxBytes = defaultMaxBytes
	}
	client := incomingops.NewSessionClientWithWireLimits(e.runtime, id, generation, time.Duration(requestTimeoutMS)*time.Millisecond, incomingops.WireLimits{MaxMessages: in.MaxMessages, MaxBytes: in.MaxBytes})
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
		if e.operation == operation.Name("structural_context_v2") {
			diagnostic := ambiguousTargetDiagnostic(workspace, symbols, matches, maxNodes)
			return operation.Result{}, &operation.Failure{
				Code: "AMBIGUOUS_TARGET",
				Err: &transientstructural.DomainFailure{
					Phase:            transientstructural.PhasePreflight,
					State:            transientstructural.StateAmbiguousTarget,
					TargetDiagnostic: diagnostic,
				},
			}
		}
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
	raw, err := json.Marshal(delegatedRequest{SessionID: id, Generation: generation, URI: location.URI, Symbol: in.Symbol, DownDepth: downDepth, UpDepth: upDepth, MaxNodes: maxNodes, TimeoutMS: timeoutMS, RequestTimeoutMS: requestTimeoutMS, MaxMessages: in.MaxMessages, MaxBytes: in.MaxBytes, Projection: in.Projection, Analysis: in.Analysis})
	if err != nil {
		return fail(operation.FailureInternal, err)
	}
	return e.delegate.Execute(parent, operation.Request{Name: e.delegateOperation, Input: raw})
}

func ambiguousTargetDiagnostic(workspace string, symbols, matches []lsp.WorkspaceSymbol, cap int) *transientstructural.TargetDiagnostic {
	observed := len(symbols)
	d := &transientstructural.TargetDiagnostic{ExactMatches: len(matches), TotalSymbols: observed, OmittedSymbols: max(0, observed-len(matches)), Action: transientstructural.TargetActionFailAmbiguous}
	accepted := make([]transientstructural.TargetCandidate, 0, len(matches))
	excluded := 0
	for _, symbol := range matches {
		if symbol.Location.Range == nil || !validRange(*symbol.Location.Range) || !concreteConfinedDocument(workspace, symbol.Location.URI) {
			excluded++
			continue
		}
		r := *symbol.Location.Range
		accepted = append(accepted, transientstructural.TargetCandidate{URI: symbol.Location.URI, Name: symbol.Name, Kind: symbol.Kind, Container: symbol.ContainerName, Range: transientstructural.TargetCandidateRange{Start: transientstructural.TargetCandidatePosition{Line: int(r.Start.Line), Character: int(r.Start.Character)}, End: transientstructural.TargetCandidatePosition{Line: int(r.End.Line), Character: int(r.End.Character)}}})
	}
	sort.Slice(accepted, func(i, j int) bool {
		a, b := accepted[i], accepted[j]
		if a.URI != b.URI {
			return a.URI < b.URI
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Container != b.Container {
			return a.Container < b.Container
		}
		if a.Range.Start.Line != b.Range.Start.Line {
			return a.Range.Start.Line < b.Range.Start.Line
		}
		if a.Range.Start.Character != b.Range.Start.Character {
			return a.Range.Start.Character < b.Range.Start.Character
		}
		if a.Range.End.Line != b.Range.End.Line {
			return a.Range.End.Line < b.Range.End.Line
		}
		return a.Range.End.Character < b.Range.End.Character
	})
	unique := accepted[:0]
	for _, c := range accepted {
		if len(unique) == 0 || c != unique[len(unique)-1] {
			unique = append(unique, c)
		}
	}
	deduplicated := len(accepted) - len(unique)
	if cap < 1 {
		cap = defaultMaxNodes
	}
	cap = min(cap, defaultMaxNodes)
	truncated := max(0, len(unique)-cap)
	returned := unique
	if len(returned) > cap {
		returned = returned[:cap]
	}
	d.CandidateAccounting = &transientstructural.TargetCandidateAccounting{Observed: &observed, Accepted: len(accepted), Returned: len(returned), Excluded: excluded, Deduplicated: deduplicated, Truncated: truncated}
	if len(returned) > 0 {
		d.Candidates = returned
	}
	return d
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
