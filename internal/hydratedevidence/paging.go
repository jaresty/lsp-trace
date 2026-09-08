package hydratedevidence

import (
	"encoding/base64"
	"encoding/json"
	"errors"
)

type Entry struct {
	Header *Bundle `json:"header,omitempty"`
	Source *Source `json:"source,omitempty"`
	Origin *Origin `json:"origin,omitempty"`
	Span   *Span   `json:"span,omitempty"`
}
type Page struct {
	Snapshot     string  `json:"snapshot"`
	Ordinal      int     `json:"ordinal"`
	TotalPages   int     `json:"total_pages"`
	TotalOrigins int     `json:"total_origins"`
	TotalSpans   int     `json:"total_spans"`
	Entries      []Entry `json:"entries"`
	Next         string  `json:"next"`
	Digest       string  `json:"digest"`
}

// Snapshot privately owns serialized immutable pages, never a mutable input alias.
// It is an in-memory view, not a durable publication store or source reference.
type Snapshot struct {
	pages  [][]byte
	digest string
}
type continuation struct {
	Snapshot string `json:"snapshot"`
	Ordinal  int    `json:"ordinal"`
}

func cursor(digest string, ordinal int) string {
	raw, _ := json.Marshal(continuation{digest, ordinal})
	return base64.RawURLEncoding.EncodeToString(raw)
}
func NewSnapshot(input Input, r Request, b Bundle) (*Snapshot, error) {
	if e := Validate(input, r, b); e != nil {
		return nil, e
	}
	return pack(b)
}
func pack(b Bundle) (*Snapshot, error) {
	entries := []Entry{}
	header := b
	header.Sources = []Source{}
	header.Origins = []Origin{}
	header.Spans = []Span{}
	entries = append(entries, Entry{Header: &header})
	for i := range b.Sources {
		entries = append(entries, Entry{Source: &b.Sources[i]})
	}
	for i := range b.Origins {
		entries = append(entries, Entry{Origin: &b.Origins[i]})
	}
	for i := range b.Spans {
		entries = append(entries, Entry{Span: &b.Spans[i]})
	}
	partitions := [][]Entry{}
	current := []Entry{}
	fits := func(es []Entry) bool {
		p := Page{Snapshot: b.Digest, Ordinal: 10000, TotalPages: 10000, TotalOrigins: b.TotalOrigins, TotalSpans: b.TotalSpans, Entries: es, Next: cursor(b.Digest, 10000), Digest: b.Digest}
		raw, _ := json.Marshal(p)
		return len(raw) <= b.Policy.MaxPageBytes
	}
	for _, entry := range entries {
		candidate := append(append([]Entry{}, current...), entry)
		if !fits(candidate) {
			if len(current) == 0 {
				return nil, errors.New("page budget: indivisible metadata entry")
			}
			partitions = append(partitions, current)
			current = []Entry{entry}
			if !fits(current) {
				return nil, errors.New("page budget: indivisible entry")
			}
		} else {
			current = candidate
		}
	}
	if len(current) > 0 {
		partitions = append(partitions, current)
	}
	if len(partitions) > b.Policy.MaxPages {
		return nil, errors.New("page count budget")
	}
	s := &Snapshot{digest: b.Digest, pages: make([][]byte, 0, len(partitions))}
	for i, es := range partitions {
		p := Page{Snapshot: b.Digest, Ordinal: i, TotalPages: len(partitions), TotalOrigins: b.TotalOrigins, TotalSpans: b.TotalSpans, Entries: es}
		if i+1 < len(partitions) {
			p.Next = cursor(b.Digest, i+1)
		}
		p.Digest = valueDigest(p)
		raw, _ := json.Marshal(p)
		if len(raw) > b.Policy.MaxPageBytes {
			return nil, errors.New("page byte budget")
		}
		s.pages = append(s.pages, raw)
	}
	return s, nil
}
func (s *Snapshot) Page(token string) (Page, error) {
	p := Page{}
	if s == nil || len(s.pages) == 0 {
		return p, errors.New("unavailable snapshot")
	}
	ordinal := 0
	if token != "" {
		if len(token) > 1024 {
			return p, errors.New("continuation size")
		}
		raw, e := base64.RawURLEncoding.DecodeString(token)
		if e != nil {
			return p, errors.New("invalid continuation")
		}
		var c continuation
		if e = decode(raw, &c); e != nil || c.Snapshot != s.digest || c.Ordinal < 1 || c.Ordinal >= len(s.pages) || cursor(c.Snapshot, c.Ordinal) != token {
			return p, errors.New("stale or invalid continuation")
		}
		ordinal = c.Ordinal
	}
	e := json.Unmarshal(s.pages[ordinal], &p)
	return p, e
}
func Reassemble(input Input, r Request, pages []Page) (Bundle, error) {
	b := Bundle{}
	if err := checkRequest(r); err != nil {
		return b, err
	}
	if len(pages) == 0 || len(pages) > r.Policy.MaxPages {
		return b, errors.New("page count mismatch")
	}
	headerSeen := false
	totalBytes := 0
	for i, p := range pages {
		raw, e := json.Marshal(p)
		if e != nil || len(raw) > r.Policy.MaxPageBytes {
			return Bundle{}, errors.New("page byte budget")
		}
		totalBytes += len(raw)
		if totalBytes > r.Policy.MaxOutputBytes+r.Policy.MaxPages*2048 {
			return Bundle{}, errors.New("page aggregate budget")
		}
		digest := p.Digest
		p.Digest = ""
		if valueDigest(p) != digest || p.Ordinal != i || p.TotalPages != len(pages) {
			return Bundle{}, errors.New("page order/digest mismatch")
		}
		if i == 0 {
			if len(p.Entries) == 0 || p.Entries[0].Header == nil {
				return Bundle{}, errors.New("missing page header")
			}
		}
		for _, entry := range p.Entries {
			n := 0
			if entry.Header != nil {
				n++
			}
			if entry.Source != nil {
				n++
			}
			if entry.Origin != nil {
				n++
			}
			if entry.Span != nil {
				n++
			}
			if n != 1 {
				return Bundle{}, errors.New("invalid page entry")
			}
			switch {
			case entry.Header != nil:
				if headerSeen || i != 0 {
					return Bundle{}, errors.New("duplicate header")
				}
				b = *entry.Header
				if len(b.Sources)+len(b.Origins)+len(b.Spans) != 0 {
					return Bundle{}, errors.New("nonempty header tables")
				}
				b.Sources = []Source{}
				b.Origins = []Origin{}
				b.Spans = []Span{}
				headerSeen = true
			case entry.Source != nil:
				b.Sources = append(b.Sources, *entry.Source)
			case entry.Origin != nil:
				b.Origins = append(b.Origins, *entry.Origin)
			case entry.Span != nil:
				b.Spans = append(b.Spans, *entry.Span)
			}
		}
		next := ""
		if i+1 < len(pages) {
			next = cursor(b.Digest, i+1)
		}
		if p.Snapshot != b.Digest || p.Next != next || p.TotalOrigins != b.TotalOrigins || p.TotalSpans != b.TotalSpans {
			return Bundle{}, errors.New("snapshot/continuation mismatch")
		}
	}
	if err := Validate(input, r, b); err != nil {
		return Bundle{}, err
	}
	expected, err := pack(b)
	if err != nil {
		return Bundle{}, err
	}
	if len(expected.pages) != len(pages) {
		return Bundle{}, errors.New("noncanonical page partition")
	}
	for i, p := range pages {
		var want Page
		json.Unmarshal(expected.pages[i], &want)
		if !same(p, want) {
			return Bundle{}, errors.New("noncanonical page")
		}
	}
	return b, nil
}
