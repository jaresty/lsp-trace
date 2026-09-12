// Package traceops implements the intent-oriented MCP trace facade by adapting
// exact targets to the existing managed Graph Provenance V5 acquisition.
package traceops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"sort"
	"time"

	"lsp-trace/acquisitionops"
	"lsp-trace/incomingops"
	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/seedformat"
	"lsp-trace/internal/session"
	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

const Operation operation.Name = "trace"
const maxPositions = 64

type Runtime interface {
	Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure)
	RoundTrip(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult
	Records() []sessionruntime.Record
}

type documentRuntime interface {
	PrepareDocument(context.Context, sessionruntime.DocumentRequest) sessionruntime.DocumentResult
}

type Position struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}
type input struct {
	SessionID        string     `json:"session_id"`
	Generation       uint64     `json:"generation,omitempty"`
	URI              string     `json:"uri"`
	Symbol           string     `json:"symbol,omitempty"`
	Positions        []Position `json:"positions,omitempty"`
	LanguageID       string     `json:"language_id,omitempty"`
	DownDepth        *int       `json:"down_depth,omitempty"`
	UpDepth          *int       `json:"up_depth,omitempty"`
	MaxNodes         *int       `json:"max_nodes,omitempty"`
	TimeoutMS        *int       `json:"timeout_ms,omitempty"`
	RequestTimeoutMS *int       `json:"request_timeout_ms,omitempty"`
	TopmostSiblings  bool       `json:"topmost_siblings,omitempty"`
}

type acquirer interface {
	Execute(context.Context, operation.Request) (operation.Result, *operation.Failure)
}
type Executor struct {
	runtime     Runtime
	acquisition acquirer
}

func NewExecutor(r Runtime) *Executor {
	return &Executor{runtime: r, acquisition: acquisitionops.NewExecutor(r)}
}

func decode(raw []byte, dst any) error {
	if len(raw) > acquisitionops.MaxInputBytes {
		return fmt.Errorf("trace input exceeds %d bytes", acquisitionops.MaxInputBytes)
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("one JSON object required")
	}
	return nil
}
func value(p *int, fallback int) int {
	if p != nil {
		return *p
	}
	return fallback
}
func fail(code string, err error, diagnostics ...string) (operation.Result, *operation.Failure) {
	return operation.Result{}, &operation.Failure{Code: code, Err: err, Diagnostics: diagnostics}
}

func (e *Executor) Execute(parent context.Context, op operation.Request) (operation.Result, *operation.Failure) {
	if e == nil || e.runtime == nil || op.Name != Operation {
		return fail(operation.FailureNotImplemented, operation.ErrNotImplemented)
	}
	var in input
	if err := decode(op.Input, &in); err != nil {
		return fail(operation.FailureInvalidInput, err)
	}
	if (in.Symbol == "") == (len(in.Positions) == 0) {
		return fail(operation.FailureInvalidInput, fmt.Errorf("exactly one target mode is required: symbol or positions"))
	}
	if len(in.Positions) > maxPositions {
		return fail(operation.FailureInvalidInput, fmt.Errorf("positions must contain at most %d entries", maxPositions))
	}
	resolvedID, generation, sf := incomingops.ResolveSession(e.runtime, in.SessionID, in.Generation)
	if sf != "" {
		return fail(string(sf), nil)
	}
	metadata, sf := e.runtime.Metadata(resolvedID, generation)
	if sf != "" {
		return fail(string(sf), nil)
	}
	if !metadata.CallHierarchySupport {
		return fail(string(graph.UnsupportedCallHierarchy), nil)
	}
	if in.Symbol != "" && !metadata.DocumentSymbolSupport {
		return fail("UNSUPPORTED_DOCUMENT_SYMBOL", nil)
	}
	switch metadata.PositionEncoding {
	case "utf-8", "utf-16", "utf-32":
	default:
		return fail("UNSUPPORTED_POSITION_ENCODING", fmt.Errorf("retained position encoding %q is unsupported", metadata.PositionEncoding))
	}
	timeout, requestTimeout := value(in.TimeoutMS, 5000), value(in.RequestTimeoutMS, 1000)
	ctx, cancel := context.WithTimeout(parent, time.Duration(timeout)*time.Millisecond)
	defer cancel()
	positions := append([]Position(nil), in.Positions...)
	var preparedCapability sessionruntime.PreparedDocumentCapability
	var preparedBinding sessionruntime.PreparedOperationBinding
	prepared := false
	if d, ok := e.runtime.(documentRuntime); ok {
		doc, capability, prepareErr := sessionruntime.PrepareDocumentForOperation(ctx, d, sessionruntime.DocumentRequest{SessionID: resolvedID, Generation: generation, URI: in.URI, LanguageID: in.LanguageID, CaptureSupply: true}, op.RequestID)
		if doc.Failure != "" {
			return fail(string(doc.Failure), nil)
		}
		if prepareErr != nil {
			return fail("DOCUMENT_SUPPLY_UNAVAILABLE", prepareErr)
		}
		in.LanguageID = doc.LanguageID
		preparedCapability, preparedBinding, prepared = capability, capability.Binding(), true
	}
	if in.Symbol != "" {
		p, f := resolveSymbol(ctx, e.runtime, resolvedID, generation, in.URI, in.Symbol, requestTimeout)
		if f != nil {
			return operation.Result{}, f
		}
		positions = []Position{p}
	}
	targets := make([]acquisitionops.Target, len(positions))
	for i, p := range positions {
		line, character := p.Line, p.Character
		id := fmt.Sprintf("trace-%03d", i)
		if i == 0 {
			id = "root"
		}
		targets[i] = acquisitionops.Target{ID: id, Locator: acquisition.Locator{URI: in.URI, Line: &line, Character: &character, LanguageID: in.LanguageID}, DownDepth: in.DownDepth, UpDepth: in.UpDepth}
	}
	manifest := acquisitionops.Manifest{SchemaVersion: acquisitionops.ManifestVersion, CoordinateConvention: "zero-based-session", Root: targets[0], RequiredTargets: targets[1:], Expansion: acquisitionops.Expansion{TopmostSiblings: in.TopmostSiblings}, Limits: acquisitionops.Limits{MaxNodes: in.MaxNodes, TimeoutMS: in.TimeoutMS, RequestTimeoutMS: in.RequestTimeoutMS}}
	request := acquisitionops.Input{SessionID: resolvedID, Generation: generation, SeedManifest: manifest, OutputVersion: "lsp-trace.graph-provenance.v5"}
	raw, err := json.Marshal(request)
	if err != nil {
		return fail(operation.FailureInternal, err)
	}
	seedSpec, workspace, err := explicitSeedSpec(e.runtime, resolvedID, generation, in, positions)
	if err != nil {
		return fail(operation.FailureInvalidInput, err)
	}
	admission, err := acquisitionops.NewExplicitTraceAdmission(seedSpec, workspace, op.RequestID, request)
	if err != nil {
		return fail(operation.FailureInvalidInput, err)
	}
	requestOp := operation.Request{Name: acquisitionops.SliceV3, RequestID: op.RequestID, Input: raw, PublicationRoot: op.PublicationRoot, ArtifactStore: op.ArtifactStore}
	if prepared {
		if acquisition, ok := e.acquisition.(interface {
			ExecuteExplicitTrace(context.Context, operation.Request, acquisitionops.ExplicitTraceAdmission, sessionruntime.PreparedDocumentCapability, sessionruntime.PreparedOperationBinding) (operation.Result, *operation.Failure)
		}); ok {
			return acquisition.ExecuteExplicitTrace(ctx, requestOp, admission, preparedCapability, preparedBinding)
		}
	}
	return fail(operation.FailureInternal, fmt.Errorf("explicit trace acquisition unavailable"))
}

func explicitSeedSpec(runtime Runtime, id string, generation uint64, in input, positions []Position) ([]byte, string, error) {
	workspace := ""
	for _, record := range runtime.Records() {
		if record.SessionID == id && record.Generation == generation {
			if workspace != "" {
				return nil, "", fmt.Errorf("ambiguous host workspace")
			}
			workspace = record.Profile.Workspace().String()
		}
	}
	if workspace == "" {
		return nil, "", fmt.Errorf("host workspace unavailable")
	}
	u, err := url.Parse(in.URI)
	if err != nil || u.Scheme != "file" || u.Host != "" {
		return nil, "", fmt.Errorf("canonical file URI required")
	}
	rel, err := filepath.Rel(workspace, filepath.FromSlash(u.Path))
	if err != nil || rel == ".." || filepath.IsAbs(rel) {
		return nil, "", fmt.Errorf("trace URI is outside managed workspace")
	}
	path := filepath.ToSlash(rel)
	file := seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: make([]seedformat.Seed, len(positions))}
	for i, p := range positions {
		label := fmt.Sprintf("trace-%03d", i)
		if i == 0 {
			label = "root"
		}
		file.Seeds[i] = seedformat.Seed{Type: seedformat.PositionType, Position: &seedformat.Position{Label: label, Path: path, Line: uint64(p.Line) + 1, Column: uint64(p.Character) + 1}}
	}
	encoded, err := seedformat.EncodeCanonical(file, workspace)
	return encoded, workspace, err
}

func resolveSymbol(ctx context.Context, runtime Runtime, id string, generation uint64, uri, name string, requestTimeout int) (Position, *operation.Failure) {
	client := incomingops.NewSessionClient(runtime, id, generation, time.Duration(requestTimeout)*time.Millisecond)
	symbols, err := client.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
	if err != nil {
		_, f := fail("DOCUMENT_SYMBOL_FAILED", err)
		return Position{}, f
	}
	var matches []lsp.Position
	stack := append([]lsp.DocumentSymbol(nil), symbols...)
	for len(stack) > 0 {
		s := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if s.Name == name {
			if !incomingops.ValidDocumentSymbolTarget(s) {
				_, f := fail("DOCUMENT_SYMBOL_MALFORMED_RANGE", fmt.Errorf("document symbol %q has invalid ranges", name))
				return Position{}, f
			}
			matches = append(matches, s.SelectionRange.Start)
		}
		stack = append(stack, s.Children...)
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Line != matches[j].Line {
			return matches[i].Line < matches[j].Line
		}
		return matches[i].Character < matches[j].Character
	})
	if len(matches) == 0 {
		_, f := fail("DOCUMENT_SYMBOL_ABSENT", fmt.Errorf("document symbol %q matched 0 symbols", name), "use an exact symbol name or positions")
		return Position{}, f
	}
	if len(matches) > 1 {
		shown := len(matches)
		if shown > 8 {
			shown = 8
		}
		candidates := make([]string, shown)
		for i := 0; i < shown; i++ {
			candidates[i] = fmt.Sprintf("line=%d,character=%d", matches[i].Line, matches[i].Character)
		}
		_, f := fail("DOCUMENT_SYMBOL_AMBIGUOUS", fmt.Errorf("document symbol %q matched %d symbols", name, len(matches)), fmt.Sprintf("total=%d omitted=%d candidates=%v; use positions", len(matches), len(matches)-shown, candidates))
		return Position{}, f
	}
	return Position{Line: matches[0].Line, Character: matches[0].Character}, nil
}
