package graphprovenance

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/source"
	"lsp-trace/sessionruntime"
)

const VersionV2 = "lsp-trace.graph-provenance.v2"
const MaxEnvelopeBytesV2 = 192 << 20
const MaxGraphBytesV2 = 32 << 20
const MaxRecordsV2 = 100000
const MaxDepthV2 = 64
const PolicyV2 = "NATIVE_V3_CANONICAL_BYTES_TYPED_ACQUISITION_POST_CAPTURE_V2"

// EvidenceV2 retains the entire typed coordinator result. On the wire its graph
// is the same canonical native v3 object as GraphBytes, not a second identity.
// No digest includes itself. Source observations never authenticate analysis.
type EvidenceV2 struct {
	CaptureBudget          CaptureBudgetV2    `json:"capture_budget"`
	SchemaVersion          string             `json:"schema_version"`
	Policy                 string             `json:"policy"`
	GraphBytes             []byte             `json:"graph_bytes"`
	GraphDigest            string             `json:"graph_digest"`
	WorkspaceURI           string             `json:"workspace_uri"`
	AnalyzedVersion        string             `json:"analyzed_version"`
	DependencyCompleteness string             `json:"dependency_completeness"`
	Acquisition            acquisition.Result `json:"-"`
	Supplies               []SupplyReceiptV2  `json:"supplies"`
	Captures               []Receipt          `json:"captures"`
	Bindings               []BindingV2        `json:"bindings"`
}

type SupplyReceiptV2 struct {
	RequestID   string             `json:"request_id"`
	Status      string             `json:"status"`
	Observation acquisition.Supply `json:"observation"`
	Receipt     *Receipt           `json:"receipt"`
}

type evidenceV2Alias EvidenceV2

func (e EvidenceV2) MarshalJSON() ([]byte, error) {
	// Never silently replace a caller's changed typed graph with cached bytes.
	// Re-encoding is a consistency comparison, not a claim about captured bytes.
	current, err := json.Marshal(e.Acquisition.Graph)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(current, e.GraphBytes) {
		return nil, errors.New("V2 typed graph differs from captured bytes")
	}
	// Emit the captured serialization as the descriptor's redundant graph view.
	a, err := json.Marshal(struct {
		*acquisition.Result
		Graph json.RawMessage `json:"graph"`
	}{&e.Acquisition, json.RawMessage(e.GraphBytes)})
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		evidenceV2Alias
		Acquisition json.RawMessage `json:"acquisition"`
	}{evidenceV2Alias(e), a})
}

func (e *EvidenceV2) UnmarshalJSON(raw []byte) error {
	if err := preflightV2(raw, MaxEnvelopeBytesV2); err != nil {
		return err
	}
	var wire struct {
		evidenceV2Alias
		Acquisition json.RawMessage `json:"acquisition"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&wire); err != nil {
		return err
	}
	*e = EvidenceV2(wire.evidenceV2Alias)
	if err := preflightGraphV2(e.GraphBytes); err != nil {
		return err
	}
	var a struct {
		*acquisition.Result
		Graph json.RawMessage `json:"graph"`
	}
	a.Result = &e.Acquisition
	d = json.NewDecoder(bytes.NewReader(wire.Acquisition))
	d.DisallowUnknownFields()
	if err := d.Decode(&a); err != nil {
		return err
	}
	left, le := canonicalV2(a.Graph)
	right, re := canonicalV2(e.GraphBytes)
	if le != nil || re != nil || !bytes.Equal(left, right) {
		return errors.New("acquisition graph differs from authoritative graph bytes")
	}
	if _, err := schema.Validate(e.GraphBytes, "v3"); err != nil {
		return err
	}
	g, err := graph.DecodeNativeV3(e.GraphBytes)
	if err != nil {
		return err
	}
	e.Acquisition.Graph = g
	compact := func(p *json.RawMessage) {
		if len(*p) == 0 {
			return
		}
		var b bytes.Buffer
		if json.Compact(&b, *p) == nil {
			*p = append(json.RawMessage(nil), b.Bytes()...)
		}
	}
	for i := range e.Acquisition.Requests {
		compact(&e.Acquisition.Requests[i].Params)
		compact(&e.Acquisition.Requests[i].Response)
	}
	for i := range e.Acquisition.Supplies {
		compact(&e.Acquisition.Supplies[i].Observation)
	}
	for i := range e.Acquisition.Targets {
		r := &e.Acquisition.Targets[i].Resolution
		if r.Prepared != nil {
			compact(&r.Prepared.Data)
		}
		if r.Identity != nil {
			compact(&r.Identity.Data)
		}
	}
	for i := range e.Supplies {
		compact(&e.Supplies[i].Observation.Observation)
		if e.Supplies[i].Receipt != nil && e.Supplies[i].Receipt.Supply != nil {
			compact(&e.Supplies[i].Receipt.Supply.Params)
		}
	}
	return nil
}

// preflightV2 is an iterative duplicate/depth scan, before recursive historical
// validators. The LSP data member is opaque valid JSON, not a decoded carrier.
// Source byte strings stay bytes; JSON-looking source text is never traversed.
func preflightV2(raw []byte, max int) error {
	if err := scanV2(raw, max, false); err != nil {
		return err
	}
	return scanV2(raw, max, true)
}

func scanV2(raw []byte, max int, duplicates bool) error {
	if len(raw) == 0 || len(raw) > max || !utf8.Valid(raw) {
		return errors.New("V2 byte/UTF-8 limit")
	}
	type frame struct {
		object bool
		key    bool
		keys   map[string]bool
	}
	stack := []frame{}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	roots := 0
	for {
		token, err := d.Token()
		if number, ok := token.(json.Number); ok {
			text := number.String()
			if len(text) > 64 {
				return errors.New("V2 numeric token limit")
			}
			if i := strings.IndexAny(text, "eE"); i >= 0 {
				exponent, numberErr := strconv.ParseInt(text[i+1:], 10, 32)
				if numberErr != nil || exponent < -1024 || exponent > 1024 {
					return errors.New("V2 numeric exponent limit")
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(stack) == 0 {
			roots++
		}
		if len(stack) > 0 {
			top := &stack[len(stack)-1]
			if top.object && top.key {
				if key, ok := token.(string); ok {
					if duplicates && top.keys[key] {
						return errors.New("V2 duplicate member")
					}
					top.keys[key] = true
					top.key = false
					if key == "graph_bytes" && len(stack) == 1 && max == MaxEnvelopeBytesV2 {
						token, err := d.Token()
						if err != nil {
							return err
						}
						encoded, ok := token.(string)
						if !ok || len(encoded) > base64.StdEncoding.EncodedLen(MaxGraphBytesV2) {
							return errors.New("V2 graph carrier byte limit/type")
						}
						graphBytes, err := base64.StdEncoding.DecodeString(encoded)
						if err != nil {
							return err
						}
						if err = scanV2(graphBytes, MaxGraphBytesV2, false); err != nil {
							return err
						}
						top.key = true
					}
					if key == "data" {
						var opaque json.RawMessage
						if err := d.Decode(&opaque); err != nil {
							return err
						}
						top.key = true
					}
					continue
				}
			} else if top.object {
				top.key = true
			}
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{', '[':
				stack = append(stack, frame{object: delim == '{', key: delim == '{', keys: map[string]bool{}})
				if len(stack) > MaxDepthV2 {
					return errors.New("V2 known-carrier depth limit")
				}
			case '}', ']':
				stack = stack[:len(stack)-1]
			}
		}
	}
	if roots != 1 || len(stack) != 0 {
		return errors.New("V2 requires one JSON value")
	}
	return nil
}

func sealV2(r Receipt) Receipt {
	r.ID = ""
	b, _ := json.Marshal(r)
	r.ID = digest(VersionV2+":receipt", b)
	return r
}
func receiptV2(uri, name, class, status string, content []byte, supply *SupplyMetadata) Receipt {
	return sealV2(receipt(uri, name, class, status, content, supply))
}
func validateReceiptV2(workspace string, r Receipt) error {
	name, status := RelativeURI(workspace, r.URI)
	if r.AnalyzedVersion != Unverified || r.Path != name || len(r.Content) > MaxFileBytes {
		return errors.New("V2 receipt scope/ceiling/limit mismatch")
	}
	if status != "" {
		if status != r.Status {
			return errors.New("V2 URI disposition mismatch")
		}
	} else {
		switch r.Status {
		case "READABLE", "UNREADABLE", "MISSING", "NONREGULAR", "CANCELLED", "BUDGET_EXCEEDED", "BYTE_LIMIT_EXCEEDED":
		default:
			return errors.New("V2 unknown capture outcome")
		}
	}
	if r.Status != "READABLE" && r.Content != nil {
		return errors.New("V2 failed receipt contains bytes")
	}
	if !reflect.DeepEqual(receiptV2(r.URI, r.Path, r.Classification, r.Status, r.Content, r.Supply), r) {
		return errors.New("V2 source receipt digest mismatch")
	}
	return nil
}

func suppliesV2(r acquisition.Result, workspace string) ([]SupplyReceiptV2, error) {
	out := []SupplyReceiptV2{}
	versions := map[string]bool{}
	// Runtime keeps one effective language per URI in this exact session
	// generation. This links only available language observations, not bytes.
	languages := map[string]string{}
	for _, rec := range r.Requests {
		if rec.Method != "source/prepareDocument" || rec.Outcome != "SUCCESS" {
			continue
		}
		var supply acquisition.Supply
		if err := json.Unmarshal(rec.Response, &supply); err != nil {
			return nil, err
		}
		var locator acquisition.Locator
		if err := json.Unmarshal(rec.Params, &locator); err != nil {
			return nil, err
		}
		if supply.URI != locator.URI {
			continue
		} // coordinator retains failure, not a supply
		// Nonempty explicit input is used verbatim by PrepareDocument. Empty
		// input may use an unavailable configured default before file inference.
		if locator.LanguageID != "" && locator.LanguageID != supply.LanguageID {
			return nil, errors.New("V2 supplied language differs from explicit locator")
		}
		if supply.LanguageID != "" {
			if previous := languages[supply.URI]; previous != "" && previous != supply.LanguageID {
				return nil, errors.New("V2 contradictory runtime language observations")
			}
			languages[supply.URI] = supply.LanguageID
		}
		row := SupplyReceiptV2{RequestID: rec.ID, Status: "NO_NOTIFICATION_OBSERVATION", Observation: supply}
		if len(supply.Observation) > 0 && !bytes.Equal(supply.Observation, []byte("null")) {
			if err := preflightV2(supply.Observation, 4*MaxFileBytes); err != nil {
				return nil, err
			}
			var s sessionruntime.DocumentSupply
			dec := json.NewDecoder(bytes.NewReader(supply.Observation))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&s); err != nil {
				return nil, err
			}
			normalized, err := json.Marshal(s)
			if err != nil {
				return nil, err
			}
			left, le := canonicalV2(normalized)
			right, re := canonicalV2(supply.Observation)
			if le != nil || re != nil || !bytes.Equal(left, right) {
				return nil, errors.New("V2 supply carrier field mismatch")
			}
			ctx := r.Request.Context
			name, status := RelativeURI(workspace, s.URI)
			if status != "" || s.URI != supply.URI || s.Classification != Supplied || s.SessionID != ctx.SessionID || s.Generation != ctx.Generation || len(s.Content) > MaxFileBytes || !utf8.Valid(s.Content) {
				return nil, errors.New("V2 supply context/scope mismatch")
			}
			key := fmt.Sprintf("%s\x00%d", s.URI, s.DocumentVersion)
			if versions[key] {
				return nil, errors.New("V2 duplicate supplied document version")
			}
			versions[key] = true
			rr := receiptV2(s.URI, name, Supplied, "READABLE", s.Content, &SupplyMetadata{SessionID: s.SessionID, Generation: s.Generation, Version: s.DocumentVersion, Method: s.Method, Params: s.Params})
			if err := validateSupplyV2(&rr); err != nil {
				return nil, err
			}
			if s.Method == "textDocument/didOpen" {
				var params struct {
					TextDocument struct {
						LanguageID string `json:"languageId"`
					} `json:"textDocument"`
				}
				if err := json.Unmarshal(s.Params, &params); err != nil {
					return nil, err
				}
				if params.TextDocument.LanguageID != supply.LanguageID {
					return nil, errors.New("V2 didOpen language differs from joined supply")
				}
			}
			row.Status = "OBSERVED_NOTIFICATION"
			row.Receipt = &rr
		}
		out = append(out, row)
	}
	return out, nil
}

func bindV2(e *EvidenceV2) error {
	receipts := map[string][]string{}
	for _, r := range e.Captures {
		receipts[r.URI] = append(receipts[r.URI], r.ID)
	}
	for _, s := range e.Supplies {
		if s.Receipt != nil {
			r := s.Receipt
			receipts[r.URI] = append(receipts[r.URI], r.ID)
		}
	}
	used := 0
	for i := range e.Bindings {
		b := &e.Bindings[i]
		b.ReceiptIDs = []string{}
		used += len(b.Pointer) + len(b.URI) + 128
		if b.Attribution == "SOURCE" {
			used += len(receipts[b.URI]) * 80
		}
		if used > MaxCensusBytes {
			return errors.New("V2 binding amplification budget")
		}
		if b.Attribution == "SOURCE" {
			b.ReceiptIDs = append(b.ReceiptIDs, receipts[b.URI]...)
			sort.Strings(b.ReceiptIDs)
		}
	}
	return nil
}

// CaptureV2 retains one authoritative native graph serialization and admits the
// coordinator before any source read. Limits reject, never truncate, the complete result. The host
// supplies the workspace, not a locator found in the evidence.
func CaptureV2(ctx context.Context, r acquisition.Result, workspace string) ([]byte, error) {
	if ctx == nil || !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		return nil, errors.New("V2 requires context and canonical host workspace")
	}
	raw, err := json.Marshal(r.Graph)
	if err != nil {
		return nil, err
	}
	if err = preflightGraphV2(raw); err != nil {
		return nil, err
	}
	if _, err = schema.Validate(raw, "v3"); err != nil {
		return nil, err
	}
	e := EvidenceV2{SchemaVersion: VersionV2, Policy: PolicyV2, GraphBytes: raw, GraphDigest: digest(VersionV2+":graph", raw), WorkspaceURI: (&url.URL{Scheme: "file", Path: workspace}).String(), AnalyzedVersion: Unverified, DependencyCompleteness: "UNKNOWN_INCOMPLETE", Acquisition: r, Captures: []Receipt{}, Supplies: []SupplyReceiptV2{}, Bindings: []BindingV2{}}
	if err = checkResultV2(e); err != nil {
		return nil, err
	}
	if e.Bindings, err = CensusV2(r); err != nil {
		return nil, err
	}
	if e.Supplies, err = suppliesV2(r, e.WorkspaceURI); err != nil {
		return nil, err
	}
	root, rootErr := os.OpenRoot(workspace)
	e.CaptureBudget.RootStatus = "UNAVAILABLE"
	if rootErr == nil {
		e.CaptureBudget.RootStatus = "OPENED"
		defer root.Close()
	}
	used, attempts := 0, 0
	for _, uri := range sourceURIsV2(e.Bindings) {
		name, status := RelativeURI(e.WorkspaceURI, uri)
		var content []byte
		if status == "" {
			switch {
			case ctx.Err() != nil:
				status = "CANCELLED"
			case attempts >= MaxFiles || used >= MaxTotalBytes:
				status = "BUDGET_EXCEEDED"
			case rootErr != nil:
				status = "UNREADABLE"
			default:
				attempts++
				limit := min(MaxFileBytes, MaxTotalBytes-used)
				content, err = source.ReadRegularInputBounded(root, name, int64(limit))
				if err != nil {
					used += limit
					status = "UNREADABLE"
					switch {
					case errors.Is(err, os.ErrNotExist):
						status = "MISSING"
					case err.Error() == "input must be a regular file":
						status = "NONREGULAR"
					case err.Error() == "input exceeds byte limit":
						status = "BYTE_LIMIT_EXCEEDED"
					}
				} else {
					used += len(content)
					status = "READABLE"
				}
			}
		}
		e.Captures = append(e.Captures, receiptV2(uri, name, PostTraversal, status, content, nil))
	}
	e.CaptureBudget.Attempts, e.CaptureBudget.ChargedBytes = attempts, used
	if err = bindV2(&e); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	if _, err = ValidateFor(encoded, Family, "v2"); err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func checkResultV2(e EvidenceV2) error {
	r := e.Acquisition
	rows := len(r.Graph.Nodes) + len(r.Graph.Edges) + len(r.Requests) + len(r.Supplies) + len(r.EdgeObservations)
	for _, t := range r.Targets {
		rows += len(t.Resolution.RequestIDs) + len(t.Connection.Path.Nodes) + len(t.Connection.Path.GroupIDs)
		for _, ids := range t.Connection.Path.OccurrenceIDs {
			rows += len(ids)
		}
		for _, d := range []acquisition.DirectionResult{t.Outgoing, t.Incoming} {
			rows += len(d.Expansions) + len(d.Layers) + len(d.FrontierIDs) + len(d.SuccessfulEmptyIDs) + len(d.StartIDs)
			for _, l := range d.Layers {
				rows += len(l.NodeIDs)
			}
		}
	}
	for _, edge := range r.Graph.Edges {
		rows += len(edge.CallSites)
	}
	for _, o := range r.EdgeObservations {
		rows += len(o.CallSites)
	}
	if rows > 1000000 {
		return errors.New("V2 metadata row budget")
	}
	if len(r.Graph.Nodes) > 10000 || len(r.Requests) > MaxRecordsV2 || len(r.Supplies) > MaxRecordsV2 || len(r.EdgeObservations) > MaxRecordsV2 || len(r.Graph.Edges) > MaxBindings || len(r.Targets) > 64 {
		return errors.New("V2 coordinator row limit")
	}
	// This scan covers normalized raw params/responses and hierarchical symbols
	// before ValidateResult invokes recursive historical decoding/replay.
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if err = preflightV2(raw, MaxEnvelopeBytesV2); err != nil {
		return err
	}
	if err = acquisition.ValidateResult(r); err != nil {
		return err
	}
	return nil
}

// ValidateV2 checks the typed envelope offline. No source or supplier is invoked.
func ValidateV2(e EvidenceV2) error {
	if len(e.Captures) > MaxBindings || len(e.Bindings) > MaxBindings || len(e.Supplies) > 64 {
		return errors.New("V2 source/census row limit")
	}
	if e.SchemaVersion != VersionV2 || e.Policy != PolicyV2 || e.AnalyzedVersion != Unverified || e.DependencyCompleteness != "UNKNOWN_INCOMPLETE" {
		return errors.New("V2 policy/ceiling mismatch")
	}
	root, err := url.Parse(e.WorkspaceURI)
	if err != nil || root.Scheme != "file" || root.Host != "" || !filepath.IsAbs(root.Path) || filepath.Clean(root.Path) != root.Path || (&url.URL{Scheme: "file", Path: root.Path}).String() != e.WorkspaceURI {
		return errors.New("V2 invalid host workspace")
	}
	if err = preflightGraphV2(e.GraphBytes); err != nil {
		return err
	}
	if _, err = schema.Validate(e.GraphBytes, "v3"); err != nil {
		return err
	}
	native, err := json.Marshal(e.Acquisition.Graph)
	if err != nil {
		return err
	}
	if !bytes.Equal(native, e.GraphBytes) || e.GraphDigest != digest(VersionV2+":graph", e.GraphBytes) {
		return errors.New("V2 graph bytes/digest mismatch")
	}
	if err = checkResultV2(e); err != nil {
		return err
	}
	wanted, err := suppliesV2(e.Acquisition, e.WorkspaceURI)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(wanted, e.Supplies) {
		return errors.New("V2 actual supply request projection mismatch")
	}
	bindings, err := CensusV2(e.Acquisition)
	if err != nil {
		return err
	}
	uris := sourceURIsV2(bindings)
	if len(uris) != len(e.Captures) {
		return errors.New("V2 missing/extra capture")
	}
	if err = validateCapturesV2(e, uris); err != nil {
		return err
	}
	rebuilt := e
	rebuilt.Bindings = bindings
	if err = bindV2(&rebuilt); err != nil {
		return err
	}
	if !reflect.DeepEqual(rebuilt.Bindings, e.Bindings) {
		return errors.New("V2 typed census mismatch")
	}
	return nil
}

func canonicalV2(raw []byte) ([]byte, error) {
	var v any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

func validateForV2(raw []byte) (string, error) {
	if err := preflightV2(raw, MaxEnvelopeBytesV2); err != nil {
		return "", err
	}
	if _, err := schema.ValidateStructure(raw, Family, "v2"); err != nil {
		return "", err
	}
	var e EvidenceV2
	if err := json.Unmarshal(raw, &e); err != nil {
		return "", err
	}
	// Canonical typed comparison rejects aliases, omitted fields and unknown
	// decoded carriers without rounding opaque JSON numbers through float64.
	expected, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	a, err := canonicalV2(raw)
	if err != nil {
		return "", err
	}
	b, err := canonicalV2(expected)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(a, b) {
		return "", errors.New("V2 noncanonical fields or missing accounting")
	}
	if err = ValidateV2(e); err != nil {
		return "", err
	}
	return VersionV2, nil
}
