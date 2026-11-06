package referencegrouping

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programccompose"
	"lsp-trace/internal/strictjson"
)

const PrivateNativeEvidenceSchemaV2 = "lsp-trace.private.native-evidence.v2"

var ErrPrivateNativeEvidenceV2 = errors.New("private native evidence reconstruction refused")

type PrivateNativeRawJoinV2 struct {
	OriginalRequestOrdinal, ElementOrdinal, RangeOrdinal, ViewIndex, EdgeLocalOrdinal int
	Direction, RelationID                                                             string
	Range                                                                             graph.Range
}

// Byte slices preserve lexical originals; RawMessage is not used for wire bytes.
type PrivateNativeRequestV2 struct {
	Ordinal               int
	Method                string
	RequestID, Generation uint64
	Params, Result        []byte
	RequestFrame          []byte
	ResponseFrames        [][]byte
	Wire                  directionalraw.CompletionMetadataPrivate
	Observations          []directionalraw.Observation
	Status                string
}
type PrivateNativeEvidenceV2 struct {
	Schema           string
	Parent           acquisition.PrivateDirectionalParentV2
	Requests         []PrivateNativeRequestV2
	Invocation       graph.Invocation
	Sources          [2]graphprovenance.EvidenceV2
	Inputs           []programccompose.Input
	OriginalOrdinals [2][]int
	RawJoins         []PrivateNativeRawJoinV2
}

// PrivateNativeEncodedBoundV2 is a conservative structural JSON bound, checked
// before encoding/copying. It includes fields omitted by normal JSON tags, which
// also covers EvidenceV2's custom redundant acquisition/graph representation.
// It is a retention bound, not RAM/CPU/IO accounting or provider authority.
func PrivateNativeEncodedBoundV2(value any, limit uint64) (uint64, bool) {
	var walk func(reflect.Value, int) (uint64, bool)
	add := func(a, b uint64) (uint64, bool) {
		if a > limit || b > limit-a {
			return 0, false
		}
		return a + b, true
	}
	walk = func(v reflect.Value, depth int) (uint64, bool) {
		if depth > 64 {
			return 0, false
		}
		if !v.IsValid() {
			return 4, true
		}
		if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return 4, true
			}
			return walk(v.Elem(), depth+1)
		}
		switch v.Kind() {
		case reflect.String:
			n := uint64(v.Len())
			if n > (limit-2)/6 {
				return 0, false
			}
			return 2 + 6*n, true
		case reflect.Bool:
			return 5, true
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
			return 32, true
		case reflect.Slice, reflect.Array:
			if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 {
				n := uint64(v.Len())
				if v.Type() == reflect.TypeOf(json.RawMessage{}) {
					if n > (limit-4)/6 {
						return 0, false
					}
					return 4 + 6*n, true
				}
				if n > limit {
					return 0, false
				}
				return 4 + 4*((n+2)/3), true
			}
			total := uint64(4)
			for i := 0; i < v.Len(); i++ {
				n, ok := walk(v.Index(i), depth+1)
				if !ok {
					return 0, false
				}
				total, ok = add(total, n+1)
				if !ok {
					return 0, false
				}
			}
			return total, true
		case reflect.Struct:
			total := uint64(4)
			for i := 0; i < v.NumField(); i++ {
				f := v.Type().Field(i)
				if f.PkgPath != "" {
					continue
				}
				name := strings.Split(f.Tag.Get("json"), ",")[0]
				if name == "" || name == "-" {
					name = f.Name
				}
				n, ok := walk(v.Field(i), depth+1)
				if !ok {
					return 0, false
				}
				total, ok = add(total, n+uint64(6*len(name)+8))
				if !ok {
					return 0, false
				}
			}
			return total, true
		case reflect.Map:
			if v.Type().Key().Kind() != reflect.String {
				return 0, false
			}
			total := uint64(4)
			it := v.MapRange()
			for it.Next() {
				n, ok := walk(it.Value(), depth+1)
				if !ok {
					return 0, false
				}
				total, ok = add(total, n+uint64(6*it.Key().Len()+8))
				if !ok {
					return 0, false
				}
			}
			return total, true
		default:
			return 0, false
		}
	}
	if limit < 4 {
		return 0, false
	}
	n, ok := walk(reflect.ValueOf(value), 0)
	return n, ok && n <= limit
}

func EncodePrivateNativeEvidenceV2(p PrivateNativeEvidenceV2, composite []byte) ([]byte, error) {
	if _, ok := PrivateNativeEncodedBoundV2(p, 1<<20); !ok {
		return nil, fmt.Errorf("%w: pre-encode bound", ErrPrivateNativeEvidenceV2)
	}
	raw, err := json.Marshal(p)
	if err != nil || len(raw) >= 1<<20 {
		return nil, ErrPrivateNativeEvidenceV2
	}
	raw = append(raw, '\n')
	if err = ReplayPrivateNativeEvidenceV2(raw, composite); err != nil {
		return nil, err
	}
	return raw, nil
}

// Offline reconstruction never reads source files or turns a DTO into custody.
func ReplayPrivateNativeEvidenceV2(raw, composite []byte) error {
	if len(raw) == 0 || len(raw) > 1<<20 || strictjson.RejectDuplicates(raw) != nil {
		return ErrPrivateNativeEvidenceV2
	}
	var p PrivateNativeEvidenceV2
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return fmt.Errorf("%w: decode: %v", ErrPrivateNativeEvidenceV2, err)
	}
	if requireEOF(d) != nil || p.Schema != PrivateNativeEvidenceSchemaV2 || len(p.Requests) != 3 || len(p.Inputs) != 2 || len(p.Parent.Requests) != 0 {
		return fmt.Errorf("%w: shape", ErrPrivateNativeEvidenceV2)
	}
	if _, ok := PrivateNativeEncodedBoundV2(p, 1<<20); !ok {
		return ErrPrivateNativeEvidenceV2
	}
	canonical, err := json.Marshal(p)
	if err != nil || !bytes.Equal(append(canonical, '\n'), raw) {
		return fmt.Errorf("%w: canonical codec", ErrPrivateNativeEvidenceV2)
	}
	parent := p.Parent
	for _, r := range p.Requests {
		expected, err := privateNativeLedgerV2(r, p.Parent.MaxMessages)
		if err != nil || !reflect.DeepEqual(expected, r.Observations) {
			return fmt.Errorf("%w: exact observation ledger", ErrPrivateNativeEvidenceV2)
		}
		spans, err := directionalraw.Elements(r.Result)
		if bytes.Equal(bytes.TrimSpace(r.Result), []byte("null")) {
			spans, err = nil, nil
		}
		if err != nil {
			return ErrPrivateNativeEvidenceV2
		}
		status := "OBSERVED_NONEMPTY"
		if len(spans) == 0 {
			status = "OBSERVED_EMPTY"
		}
		if r.Status != status {
			return ErrPrivateNativeEvidenceV2
		}
		parent.Requests = append(parent.Requests, acquisition.PrivateDirectionalObservationV2{Ordinal: r.Ordinal, Method: r.Method, RequestID: r.RequestID, Generation: r.Generation, Params: r.Params, Result: r.Result})
	}
	views, err := acquisition.BuildDirectionalViewsPrivateV2(parent)
	if err != nil {
		return errors.Join(ErrPrivateNativeEvidenceV2, err)
	}
	var joins []PrivateNativeRawJoinV2
	for i, v := range views {
		if !reflect.DeepEqual(v.OriginalOrdinals, p.OriginalOrdinals[i]) {
			return fmt.Errorf("%w: view ordinals", ErrPrivateNativeEvidenceV2)
		}
		if err := graphprovenance.ValidateNativeConstructorSourcePrivateV2(p.Sources[i]); err != nil {
			return fmt.Errorf("%w: source: %v", ErrPrivateNativeEvidenceV2, err)
		}
		actual, _ := json.Marshal(v.Result)
		source, _ := json.Marshal(p.Sources[i].Acquisition)
		if !bytes.Equal(actual, source) {
			return fmt.Errorf("%w: source acquisition", ErrPrivateNativeEvidenceV2)
		}
		native := v.Result.Graph
		native.SchemaVersion = graph.SchemaVersionV5
		native.Invocation = p.Invocation
		native.Capabilities.CallHierarchyProvider = true
		native.Canonicalize()
		graphBytes, err := json.Marshal(native)
		if err != nil {
			return ErrPrivateNativeEvidenceV2
		}
		envelope, err := graphprovenance.CaptureV5(graphBytes, parent.SessionID, parent.Generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable}, &p.Sources[i])
		if err != nil {
			return err
		}
		in := p.Inputs[i]
		sum := fmt.Sprintf("sha256:%x", sha256.Sum256(envelope))
		if !bytes.Equal(envelope, in.Bytes) || in.ByteLength != len(envelope) || in.SHA256 != sum || in.Identity != fmt.Sprintf("%s:view-%d", p.Invocation.Provenance.InvocationID, i) || in.ExactMetadata.PositionEncoding != parent.PositionEncoding || in.ExactMetadata.WorkspaceIdentity != p.Invocation.WorkspaceURI || in.ExactMetadata.RevisionCustody != "CALLER_ASSERTED" || in.ExactMetadata.AcquisitionSemantics != "ONE_ACTUAL_DIRECTIONAL_INVOCATION_TWO_VALIDATED_VIEWS" {
			return ErrPrivateNativeEvidenceV2
		}
		j, err := PrivateNativeRangeJoinV2(parent, i, native)
		if err != nil {
			return err
		}
		joins = append(joins, j...)
	}
	if !reflect.DeepEqual(joins, p.RawJoins) {
		return fmt.Errorf("%w: raw ordinal join", ErrPrivateNativeEvidenceV2)
	}
	rebuilt, err := programccompose.Compose(p.Inputs)
	if err != nil || !bytes.Equal(rebuilt.Bytes, composite) {
		return fmt.Errorf("%w: composite", ErrPrivateNativeEvidenceV2)
	}
	return nil
}

// privateNativeLedgerV2 reconstructs the complete successful emission order:
// request, every response frame (including prefixes), then prepare selection.
// The producer independently detaches its actual manager-observed ledger.
func privateNativeLedgerV2(r PrivateNativeRequestV2, maxMessages int) ([]directionalraw.Observation, error) {
	fail := func() ([]directionalraw.Observation, error) { return nil, ErrPrivateNativeEvidenceV2 }
	if !directionalraw.Allowed(r.Method) || r.RequestID == 0 || r.Generation == 0 || maxMessages < 1 || maxMessages > 32 || len(r.ResponseFrames) < 1 || len(r.ResponseFrames) > maxMessages || len(r.RequestFrame) == 0 || len(r.RequestFrame) > 65536 {
		return fail()
	}
	total := len(r.RequestFrame)
	for _, frame := range r.ResponseFrames {
		if len(frame) == 0 || len(frame) > 65536 || len(frame) > 131072-total {
			return fail()
		}
		total += len(frame)
	}
	last := r.ResponseFrames[len(r.ResponseFrames)-1]
	w := r.Wire
	if w.RequestIDPrivate != r.RequestID || w.GenerationPrivate != r.Generation || w.RequestLengthPrivate != uint64(len(r.RequestFrame)) || w.RequestSHA256Private != sha256.Sum256(r.RequestFrame) || w.ResponseLengthPrivate != uint64(len(last)) || w.ResponseSHA256Private != sha256.Sum256(last) || w.ResultLengthPrivate != uint64(len(r.Result)) || w.ResultSHA256Private != sha256.Sum256(r.Result) {
		return fail()
	}
	request, err := privateNativeFrameV2(r.RequestFrame)
	if err != nil || request.Kind() != lspwire.KindRequest || request.Method != r.Method || string(request.ID) != strconv.FormatUint(r.RequestID, 10) || !bytes.Equal(request.Params, r.Params) {
		return fail()
	}
	observation := func(component string, raw []byte) directionalraw.Observation {
		return directionalraw.Observation{Method: r.Method, RequestID: r.RequestID, Generation: r.Generation, Component: component, Length: uint64(len(raw)), SHA256: sha256.Sum256(raw)}
	}
	ledger := []directionalraw.Observation{observation("request-frame", r.RequestFrame)}
	for i, frame := range r.ResponseFrames {
		message, err := privateNativeFrameV2(frame)
		if err != nil {
			return fail()
		}
		matched := string(message.ID) == strconv.FormatUint(r.RequestID, 10) && (message.Kind() == lspwire.KindSuccessResponse || message.Kind() == lspwire.KindErrorResponse)
		if i == len(r.ResponseFrames)-1 {
			if !matched || message.Kind() != lspwire.KindSuccessResponse || !bytes.Equal(message.Result, r.Result) {
				return fail()
			}
		} else if matched {
			return fail()
		}
		ledger = append(ledger, observation("response-frame", frame))
	}
	if r.Method == "textDocument/prepareCallHierarchy" {
		spans, err := directionalraw.Elements(r.Result)
		if err != nil || len(spans) != 1 {
			return fail()
		}
		o := observation("element", r.Result[spans[0][0]:spans[0][1]])
		o.Selected = true
		ledger = append(ledger, o)
	}
	return ledger, nil
}

// The existing bounded framing parser enforces original header/body lengths.
// Duplicate JSON keys (including nested ones), unknown envelope fields, and any
// trailing frame bytes are refused; no bytes are regenerated for commitments.
func privateNativeFrameV2(raw []byte) (lspwire.Message, error) {
	if len(raw) == 0 || len(raw) > 65536 {
		return lspwire.Message{}, ErrPrivateNativeEvidenceV2
	}
	source := bytes.NewReader(raw)
	message, _, _, _, err := lspwire.ReadPrivateBoundedFrame(source, lspwire.Limits{MaxBodyBytes: 65536, MaxHeaderBytes: 65536}, int64(len(raw)), 0)
	if err != nil || source.Len() != 0 {
		return lspwire.Message{}, ErrPrivateNativeEvidenceV2
	}
	at := 0
	for at < len(raw) {
		n := bytes.IndexByte(raw[at:], '\n')
		if n < 0 {
			return lspwire.Message{}, ErrPrivateNativeEvidenceV2
		}
		line := raw[at : at+n+1]
		at += n + 1
		if len(bytes.TrimRight(line, "\r\n")) == 0 {
			break
		}
	}
	body := raw[at:]
	fields, e := directionalraw.Fields(body)
	if e != nil || strictjson.RejectDuplicates(body) != nil {
		return lspwire.Message{}, ErrPrivateNativeEvidenceV2
	}
	for k := range fields {
		switch k {
		case "jsonrpc", "id", "method", "params", "result", "error":
		default:
			return lspwire.Message{}, ErrPrivateNativeEvidenceV2
		}
	}
	return message, nil
}

func PrivateNativeRangeJoinV2(v acquisition.PrivateDirectionalParentV2, direction int, g graph.Result) ([]PrivateNativeRawJoinV2, error) {
	if len(v.Requests) != 3 || direction < 0 || direction > 1 {
		return nil, ErrPrivateNativeEvidenceV2
	}
	var prepared []lsp.CallHierarchyItem
	if json.Unmarshal(v.Requests[0].Result, &prepared) != nil || len(prepared) != 1 {
		return nil, ErrPrivateNativeEvidenceV2
	}
	convert := func(r lsp.Range) graph.Range {
		return graph.Range{Start: graph.Position{Line: r.Start.Line, Character: r.Start.Character}, End: graph.Position{Line: r.End.Line, Character: r.End.Character}}
	}
	node := func(i lsp.CallHierarchyItem) graph.Node {
		return graph.NewNode(graph.Item{Name: i.Name, Kind: i.Kind, Detail: i.Detail, URI: i.URI, Range: convert(i.Range), SelectionRange: convert(i.SelectionRange), Data: i.Data})
	}
	root := node(prepared[0])
	var joins []PrivateNativeRawJoinV2
	add := func(element int, other lsp.CallHierarchyItem, ranges []lsp.Range) error {
		caller, callee, label := node(other).ID, root.ID, "INCOMING"
		if direction == 1 {
			caller, callee, label = root.ID, node(other).ID, "OUTGOING"
		}
		for ordinal, r := range ranges {
			found := false
			for _, edge := range g.Edges {
				if edge.CallerNodeID != caller || edge.CalleeNodeID != callee {
					continue
				}
				for local, site := range edge.CallSites {
					if site == convert(r) {
						joins = append(joins, PrivateNativeRawJoinV2{direction + 1, element, ordinal, direction, local, label, edge.RelationID, site})
						found = true
						break
					}
				}
			}
			if !found {
				return ErrPrivateNativeEvidenceV2
			}
		}
		return nil
	}
	if direction == 0 {
		var calls []lsp.CallHierarchyIncomingCall
		if json.Unmarshal(v.Requests[1].Result, &calls) != nil {
			return nil, ErrPrivateNativeEvidenceV2
		}
		for i, c := range calls {
			if err := add(i, c.From, c.FromRanges); err != nil {
				return nil, err
			}
		}
	} else {
		var calls []lsp.CallHierarchyOutgoingCall
		if json.Unmarshal(v.Requests[2].Result, &calls) != nil {
			return nil, ErrPrivateNativeEvidenceV2
		}
		for i, c := range calls {
			if err := add(i, c.To, c.FromRanges); err != nil {
				return nil, err
			}
		}
	}
	return joins, nil
}
