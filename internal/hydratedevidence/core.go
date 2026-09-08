package hydratedevidence

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"unicode/utf8"
)

func Digest(raw []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) }
func valueDigest(v any) string { raw, _ := json.Marshal(v); return Digest(raw) }
func seal(b *Bundle)           { b.Digest = ""; b.Digest = valueDigest(*b) }
func checkRequest(r Request) error {
	if e := checkPolicy(r.Policy); e != nil {
		return e
	}
	if len(r.Selections) > r.Policy.MaxOrigins {
		return errors.New("origin input budget")
	}
	seen := map[string]bool{}
	for _, s := range r.Selections {
		if s.ID == "" || len(s.ID) > 1024 || seen[s.ID] {
			return errors.New("invalid/duplicate selection ID")
		}
		seen[s.ID] = true
		switch s.Mode {
		case "SPAN", "RETAINED_RANGE", "WHOLE_FILE", "BOUNDARY":
		default:
			return errors.New("unsupported selection mode")
		}
		if s.Mode != "SPAN" && (s.Range != nil || s.Bytes != nil) || s.Mode != "BOUNDARY" && s.Boundary != nil {
			return errors.New("conflicting selection selectors")
		}
	}
	raw, e := json.Marshal(r)
	if e != nil || len(raw) > 4<<20 {
		return errors.New("selection byte budget")
	}
	return nil
}
func includes(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
func selected(a admitted, r Request) ([]Origin, int) {
	origins := make([]Origin, 0, len(r.Selections))
	work := 0
	for _, s := range r.Selections {
		coordinateAuthority := Caller
		if s.Mode == "WHOLE_FILE" || s.Mode == "RETAINED_RANGE" {
			coordinateAuthority = "UNAVAILABLE"
		}
		o := Origin{CoordinateAuthority: coordinateAuthority, Selection: s, Status: "UNKNOWN_RECORD", OriginalCoordinatesStatus: "UNAVAILABLE", SpanIDs: []string{}}
		rec, ok := a.recordMap[s.RecordID]
		if !ok {
			origins = append(origins, o)
			continue
		}
		o.Record = &rec
		if s.Mode == "RETAINED_RANGE" && rec.Range != nil {
			o.CoordinateAuthority = rec.Authority
		}
		src, ok := a.sourceMap[s.SourceID]
		if !ok || !includes(rec.SourceIDs, s.SourceID) {
			o.Status = "UNKNOWN_SOURCE"
			origins = append(origins, o)
			continue
		}
		rg := s.Range
		encoding := s.Encoding
		if s.Mode == "RETAINED_RANGE" {
			rg = rec.Range
			if encoding == "" {
				encoding = rec.Encoding
			}
			if rec.Encoding != "" && encoding != rec.Encoding {
				o.Status = "INVALID_COORDINATES"
				origins = append(origins, o)
				continue
			}
		}
		if s.Mode == "BOUNDARY" && s.Boundary != nil {
			rg = &s.Boundary.Range
		}
		o.OriginalRange = rg
		if rg != nil {
			o.OriginalCoordinatesStatus = "AVAILABLE"
		}
		if !r.Policy.IncludeBodies {
			o.Status = "PRIVACY_EXCLUDED"
			origins = append(origins, o)
			continue
		}
		switch src.State {
		case "REFERENCE_ONLY":
			o.Status = "REFERENCE_ONLY"
		case "MISSING":
			o.Status = "MISSING_BYTES"
		case "TRUNCATED_INPUT":
			o.Status = "TRUNCATED_INPUT"
		default:
			o.Status = "PENDING"
		}
		if o.Status != "PENDING" {
			origins = append(origins, o)
			continue
		}
		if s.Mode == "RETAINED_RANGE" && rec.AnchorStatus == "INVALID_COORDINATES" {
			o.Status = "INVALID_COORDINATES"
			origins = append(origins, o)
			continue
		}
		if s.Mode == "BOUNDARY" {
			b := s.Boundary
			if b == nil || b.Reference == "" || b.SourceID != s.SourceID || src.ContentHash == nil || b.ContentHash != *src.ContentHash || b.Authority != Caller || b.Qualification != NonAuthoritative || !r.Policy.AllowCallerBoundaries || !includes([]string{"FUNCTION", "DECLARATION", "COMMENT", "TEMPLATE"}, b.Kind) {
				o.Status = "UNKNOWN_BOUNDARY"
				origins = append(origins, o)
				continue
			}
		}
		if s.Mode != "WHOLE_FILE" && rg == nil && s.Bytes == nil {
			o.Status = "UNKNOWN_BOUNDARY"
			origins = append(origins, o)
			continue
		}
		data := a.bodies[s.SourceID]
		cost := 3*len(data) + 1
		if cost > r.Policy.MaxWork-work {
			o.Status = "WORK_BUDGET"
			origins = append(origins, o)
			continue
		}
		work += cost
		if src.SourceEncoding != "utf-8" || !utf8.Valid(data) {
			o.Status = "INVALID_COORDINATES"
			origins = append(origins, o)
			continue
		}
		iv := Interval{0, len(data)}
		var err error
		if s.Mode != "WHOLE_FILE" {
			if encoding != "utf-8" && encoding != "utf-16" && encoding != "utf-32" {
				err = errors.New("unsupported encoding")
			} else if rg != nil {
				iv, err = ByteInterval(data, encoding, *rg)
				if err == nil && s.Bytes != nil && iv != *s.Bytes {
					err = errors.New("coordinate/byte mismatch")
				}
			} else {
				iv = *s.Bytes
				if iv.Start < 0 || iv.End < iv.Start || iv.End > len(data) || iv.Start < len(data) && !utf8.RuneStart(data[iv.Start]) || iv.End < len(data) && !utf8.RuneStart(data[iv.End]) {
					err = errors.New("invalid byte interval")
				}
			}
		}
		if err != nil {
			o.Status = "INVALID_COORDINATES"
			origins = append(origins, o)
			continue
		}
		o.Bytes = &iv
		o.Status = "PENDING"
		origins = append(origins, o)
	}
	return origins, work
}

type group struct {
	source, encoding string
	interval         Interval
	indices          []int
}

func originEncoding(o Origin) string {
	if o.Selection.Mode == "WHOLE_FILE" {
		return "UNAVAILABLE"
	}
	if o.Selection.Encoding != "" {
		return o.Selection.Encoding
	}
	if o.Record != nil {
		return o.Record.Encoding
	}
	return ""
}
func groups(origins []Origin) []group {
	order := []int{}
	for i, o := range origins {
		if o.Status == "PENDING" {
			order = append(order, i)
		}
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := origins[order[i]], origins[order[j]]
		if a.Selection.SourceID != b.Selection.SourceID {
			return a.Selection.SourceID < b.Selection.SourceID
		}
		ae, be := originEncoding(a), originEncoding(b)
		if ae != be {
			return ae < be
		}
		if a.Bytes.Start != b.Bytes.Start {
			return a.Bytes.Start < b.Bytes.Start
		}
		if a.Bytes.End != b.Bytes.End {
			return a.Bytes.End < b.Bytes.End
		}
		return a.Selection.ID < b.Selection.ID
	})
	out := []group{}
	for _, i := range order {
		o := origins[i]
		if len(out) > 0 {
			g := &out[len(out)-1]
			if g.source == o.Selection.SourceID && g.encoding == originEncoding(o) && o.Bytes.Start <= g.interval.End {
				if o.Bytes.End > g.interval.End {
					g.interval.End = o.Bytes.End
				}
				g.indices = append(g.indices, i)
				continue
			}
		}
		out = append(out, group{o.Selection.SourceID, originEncoding(o), *o.Bytes, []int{i}})
	}
	return out
}
func makeSpan(g group, origins []Origin, body []byte) Span {
	ids := make([]string, 0, len(g.indices))
	for _, i := range g.indices {
		ids = append(ids, origins[i].Selection.ID)
	}
	sort.Strings(ids)
	s := Span{SourceID: g.source, Encoding: g.encoding, Bytes: g.interval, Content: append([]byte{}, body[g.interval.Start:g.interval.End]...), OriginIDs: ids}
	s.ContentHash = Digest(s.Content)
	s.ID = valueDigest(struct {
		Source, Encoding string
		Bytes            Interval
	}{s.SourceID, s.Encoding, s.Bytes})
	return s
}

// Hydrate never reads a path. Resource admission failures return no partial
// bundle; admitted selections retain an origin even when their context is omitted.
func Hydrate(input Input, r Request) (Bundle, error) {
	b := Bundle{}
	if e := checkRequest(r); e != nil {
		return b, e
	}
	a, e := admit(input, r.Policy)
	if e != nil {
		return b, e
	}
	origins, work := selected(a, r)
	b = Bundle{SchemaVersion: Version, InputDigests: a.InputDigests, RequestDigest: valueDigest(r), Policy: r.Policy, Sources: a.Sources, Origins: origins, Spans: []Span{}, Complete: len(origins) > 0, TotalOrigins: len(origins), Work: work}
	bodyUsed := 0
	for _, g := range groups(origins) {
		span := makeSpan(g, origins, a.bodies[g.source])
		status := "EXPORTED"
		wire, _ := json.Marshal(span)
		// Conservative fixed transport overhead, checked again by exact page sizing.
		if len(wire)+2048 > r.Policy.MaxPageBytes {
			status = "PAGE_BUDGET"
		} else if len(b.Spans) >= r.Policy.MaxSpans {
			status = "SPAN_BUDGET"
		} else if len(span.Content) > r.Policy.MaxBodyBytes-bodyUsed {
			status = "BODY_BUDGET"
		}
		if status == "EXPORTED" {
			bodyUsed += len(span.Content)
			b.Spans = append(b.Spans, span)
		}
		for _, i := range g.indices {
			b.Origins[i].Status = status
			if status == "EXPORTED" {
				b.Origins[i].SpanIDs = []string{span.ID}
			}
		}
	}
	for _, o := range b.Origins {
		if o.Status != "EXPORTED" {
			b.Complete = false
		}
	}
	b.TotalSpans = len(b.Spans)
	seal(&b)
	raw, _ := json.Marshal(b)
	if len(raw) > r.Policy.MaxOutputBytes {
		return Bundle{}, errors.New("output byte budget: metadata and whole spans exceed cap")
	}
	return b, nil
}
