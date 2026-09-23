// Package adr0011sourcecontext contains a private, offline source-context representation.
package adr0011sourcecontext

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

type Position struct{ Line, Character int }
type Range struct{ Start, End Position }
type Document struct {
	URI             string
	Digest          string
	ByteLength      int
	CustodyKind     string
	Revision        string
	SessionID       string
	Generation      int
	DocumentVersion string
	Encoding        string
	Privacy         string
}
type PointIdentity struct {
	Document Document
	Evidence Range
}
type SpanIdentity struct {
	Document         Document
	Display          Range
	DisplayReceipt   string
	ProjectionPolicy string
	Disposition      string
	Omitted          bool
	Body             string
}
type Point struct {
	ID       string
	Identity PointIdentity
}
type Span struct {
	ID       string
	Identity SpanIdentity
}
type Kind string

const (
	Calls                Kind = "CALLS"
	ReferencesSymbol     Kind = "REFERENCES_SYMBOL"
	ResolvesToDefinition Kind = "RESOLVES_TO_DEFINITION"
)

type Binding struct {
	ID            string
	Kind          Kind
	PointID       string
	SpanID        string
	Role          string
	MethodReceipt string
	TargetPointID string
	TargetSpanID  string
}
type Input struct {
	Points   []PointIdentity
	Spans    []SpanIdentity
	Bindings []Binding
}
type Counts struct{ PointCandidates, SpanCandidates, Points, Spans, Bindings, StoredBodyBytes int }
type Snapshot struct {
	Points   []Point
	Spans    []Span
	Bindings []Binding
	Counts   Counts
}

func (s Snapshot) Marshal() ([]byte, error) { return json.Marshal(s) }
func identityID(prefix string, value any) string {
	data, _ := json.Marshal(value) // These identities contain only JSON-compatible value fields.
	sum := sha256.Sum256(data)
	return prefix + hex.EncodeToString(sum[:])
}
func PointID(p PointIdentity) string { return identityID("point:", p) }
func SpanID(s SpanIdentity) string   { return identityID("span:", s) }

func validRange(r Range) bool {
	a, b := r.Start, r.End
	return a.Line >= 0 && a.Character >= 0 && b.Line >= 0 && b.Character >= 0 && (a.Line < b.Line || a.Line == b.Line && a.Character < b.Character)
}
func validDocument(d Document) bool {
	if d.URI == "" || d.Digest == "" || d.ByteLength <= 0 || d.Encoding == "" || d.Privacy == "" {
		return false
	}
	switch d.CustodyKind {
	case "LIVE":
		return d.SessionID != "" && d.Generation > 0 && d.DocumentVersion != "" && d.Revision == ""
	case "RETAINED", "REVISION":
		return d.Revision != "" && d.SessionID == "" && d.Generation == 0 && d.DocumentVersion == ""
	default:
		return false
	}
}
func validSpan(s SpanIdentity) bool {
	if !validDocument(s.Document) || !validRange(s.Display) || s.DisplayReceipt == "" || s.ProjectionPolicy == "" || s.Disposition == "" {
		return false
	}
	if s.Disposition == "WITHHELD" && s.Body != "" {
		return false
	}
	return (s.Omitted && s.Body == "") || (!s.Omitted && s.Body != "")
}

// Build interns only caller-supplied identities. It never acquires, resolves, or infers source relations.
func Build(input Input) (Snapshot, error) {
	result := Snapshot{Points: []Point{}, Spans: []Span{}, Bindings: []Binding{}, Counts: Counts{PointCandidates: len(input.Points), SpanCandidates: len(input.Spans)}}
	points := map[string]Point{}
	spans := map[string]Span{}
	for _, p := range input.Points {
		if !validDocument(p.Document) || !validRange(p.Evidence) {
			return Snapshot{}, fmt.Errorf("invalid source point")
		}
		id := PointID(p)
		points[id] = Point{ID: id, Identity: p}
	}
	for _, s := range input.Spans {
		if !validSpan(s) {
			return Snapshot{}, fmt.Errorf("invalid context span")
		}
		id := SpanID(s)
		spans[id] = Span{ID: id, Identity: s}
	}
	seen := map[string]bool{}
	for _, b := range input.Bindings {
		if b.ID == "" || seen[b.ID] {
			return Snapshot{}, fmt.Errorf("duplicate or empty binding identity %q", b.ID)
		}
		seen[b.ID] = true
		switch b.Kind {
		case Calls, ReferencesSymbol, ResolvesToDefinition:
		default:
			return Snapshot{}, fmt.Errorf("unknown occurrence kind %q", b.Kind)
		}
		p, ok := points[b.PointID]
		if !ok {
			return Snapshot{}, fmt.Errorf("missing or forged point %q", b.PointID)
		}
		s, ok := spans[b.SpanID]
		if !ok {
			return Snapshot{}, fmt.Errorf("missing or forged span %q", b.SpanID)
		}
		if p.Identity.Document != s.Identity.Document {
			return Snapshot{}, fmt.Errorf("point/span document mismatch")
		}
		if (b.TargetPointID == "") != (b.TargetSpanID == "") {
			return Snapshot{}, fmt.Errorf("incomplete target selection")
		}
		if b.TargetPointID != "" {
			tp, ok := points[b.TargetPointID]
			if !ok {
				return Snapshot{}, fmt.Errorf("missing target point")
			}
			ts, ok := spans[b.TargetSpanID]
			if !ok || tp.Identity.Document != ts.Identity.Document {
				return Snapshot{}, fmt.Errorf("missing or mismatched target span")
			}
		}
		result.Bindings = append(result.Bindings, b)
	}
	for _, p := range points {
		result.Points = append(result.Points, p)
	}
	for _, s := range spans {
		result.Spans = append(result.Spans, s)
		if !s.Identity.Omitted {
			result.Counts.StoredBodyBytes += len(s.Identity.Body)
		}
	}
	sort.Slice(result.Points, func(i, j int) bool { return result.Points[i].ID < result.Points[j].ID })
	sort.Slice(result.Spans, func(i, j int) bool { return result.Spans[i].ID < result.Spans[j].ID })
	sort.Slice(result.Bindings, func(i, j int) bool { return result.Bindings[i].ID < result.Bindings[j].ID })
	result.Counts.Points = len(result.Points)
	result.Counts.Spans = len(result.Spans)
	result.Counts.Bindings = len(result.Bindings)
	return result, nil
}
