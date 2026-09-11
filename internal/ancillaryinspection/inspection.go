package ancillaryinspection

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"lsp-trace/internal/inspection"
)

const Version = "lsp-trace.inspect-ancillary.v1"
const SchemaID = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.inspect-ancillary.v1.schema.json"
const Projection = "ProjectAllSeeds"
const MaxArtifactBytes = 1 << 20

type Policy struct {
	MaxPageBytes   int `json:"max_page_bytes,omitempty"`
	MaxPages       int `json:"max_pages,omitempty"`
	MaxOutputBytes int `json:"max_output_bytes,omitempty"`
}
type Request struct {
	Input    json.RawMessage `json:"input"`
	Selector struct {
		AllSeeds bool `json:"all_seeds"`
	} `json:"selector"`
	Ancillary  bool   `json:"ancillary,omitempty"`
	Page       bool   `json:"page,omitempty"`
	Cursor     string `json:"cursor,omitempty"`
	Generation string `json:"generation,omitempty"`
	Policy     Policy `json:"policy,omitempty"`
}
type Count struct {
	Total    int `json:"total"`
	Returned int `json:"returned"`
}
type Manifest struct {
	Seeds           Count `json:"seeds"`
	SeedMemberships Count `json:"seed_memberships"`
	Frontier        Count `json:"frontier"`
	Terminals       Count `json:"terminals"`
	Diagnostics     Count `json:"diagnostics"`
}
type Entry struct {
	Section string          `json:"section"`
	Value   json.RawMessage `json:"value"`
}
type Page struct {
	Snapshot   string  `json:"snapshot"`
	Ordinal    int     `json:"ordinal"`
	TotalPages int     `json:"total_pages"`
	Section    string  `json:"section"`
	Offset     int     `json:"offset"`
	Entries    []Entry `json:"entries"`
	Digest     string  `json:"digest"`
}
type View struct {
	SchemaVersion string                    `json:"schema_version"`
	Projection    string                    `json:"projection"`
	SourceDigest  string                    `json:"source_exact_bytes_digest"`
	Generation    string                    `json:"generation,omitempty"`
	Delivery      string                    `json:"delivery"`
	Manifest      Manifest                  `json:"manifest"`
	Full          *inspection.AllProjection `json:"full,omitempty"`
	Page          *Page                     `json:"page,omitempty"`
	NextCursor    string                    `json:"next_cursor"`
}
type cursor struct {
	Snapshot   string `json:"snapshot"`
	Projection string `json:"projection"`
	Section    string `json:"section"`
	Offset     int    `json:"offset"`
	Policy     Policy `json:"policy"`
	Generation string `json:"generation,omitempty"`
	Ordinal    int    `json:"ordinal"`
}
type Snapshot struct {
	pages      []Page
	binding    string
	projection inspection.AllProjection
	policy     Policy
	generation string
}

func DefaultPolicy() Policy {
	return Policy{MaxPageBytes: 48 << 10, MaxPages: 10000, MaxOutputBytes: 1 << 20}
}
func normalizePolicy(p Policy) (Policy, error) {
	d := DefaultPolicy()
	if p.MaxPageBytes != 0 {
		d.MaxPageBytes = p.MaxPageBytes
	}
	if p.MaxPages != 0 {
		d.MaxPages = p.MaxPages
	}
	if p.MaxOutputBytes != 0 {
		d.MaxOutputBytes = p.MaxOutputBytes
	}
	if d.MaxPageBytes < 4096 || d.MaxPageBytes > d.MaxOutputBytes || d.MaxPages < 1 || d.MaxPages > 10000 || d.MaxOutputBytes > 1<<20 {
		return p, errors.New("invalid ancillary policy")
	}
	return d, nil
}
func InputBytes(raw json.RawMessage) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errors.New("inspection input is required")
	}
	if raw[0] != '"' {
		return append([]byte(nil), raw...), nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return []byte(s), nil
}
func Check(r Request) error {
	if !r.Ancillary || !r.Selector.AllSeeds {
		return errors.New("ancillary requires all_seeds")
	}
	if r.Cursor != "" && !r.Page {
		return errors.New("cursor requires page")
	}
	if len(r.Cursor) > 2048 {
		return errors.New("cursor byte limit")
	}
	b, e := InputBytes(r.Input)
	if e != nil {
		return e
	}
	if len(b) > MaxArtifactBytes {
		return errors.New("inspection input byte limit")
	}
	_, e = normalizePolicy(r.Policy)
	return e
}
func digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}
func encode(c cursor) string { b, _ := json.Marshal(c); return base64.RawURLEncoding.EncodeToString(b) }
func decode(token string) (cursor, error) {
	var c cursor
	raw, e := base64.RawURLEncoding.DecodeString(token)
	if e != nil {
		return c, errors.New("invalid ancillary cursor")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil || encode(c) != token {
		return c, errors.New("stale or invalid ancillary cursor")
	}
	return c, nil
}
func counts(p inspection.AllProjection) Manifest {
	m := Manifest{Seeds: Count{Total: len(p.Seeds)}, Frontier: Count{Total: len(p.Records.Frontier)}, Terminals: Count{Total: len(p.Records.Terminals)}, Diagnostics: Count{Total: len(p.Records.Diagnostics)}}
	for _, s := range p.Seeds {
		m.SeedMemberships.Total += len(s.SeedMemberships)
	}
	return m
}
func entries(p inspection.AllProjection) []Entry {
	out := []Entry{}
	add := func(section string, v any) { b, _ := json.Marshal(v); out = append(out, Entry{section, b}) }
	for _, v := range p.Seeds {
		add("seeds", v)
	}
	for _, s := range p.Seeds {
		for _, v := range s.SeedMemberships {
			add("seed_memberships", v)
		}
	}
	for _, v := range p.Records.Frontier {
		add("frontier", v)
	}
	for _, v := range p.Records.Terminals {
		add("terminals", v)
	}
	for _, v := range p.Records.Diagnostics {
		add("diagnostics", v)
	}
	return out
}
func NewSnapshot(source []byte, p inspection.AllProjection, policy Policy, generation string) (*Snapshot, error) {
	pol, e := normalizePolicy(policy)
	if e != nil {
		return nil, e
	}
	binding := digest(struct {
		Source, Projection string
		Policy             Policy
		Generation         string
	}{p.ArtifactIdentity.ExactSerializedBytesDigest, Projection, pol, generation})
	all := entries(p)
	parts := [][]Entry{}
	cur := []Entry{}
	manifest := counts(p)
	fits := func(es []Entry) bool {
		page := Page{Snapshot: binding, Ordinal: 9999, TotalPages: 10000, Section: "seed_memberships", Offset: 999999, Entries: es, Digest: "sha256:" + string(make([]byte, 64))}
		next := encode(cursor{Snapshot: binding, Projection: Projection, Section: "seed_memberships", Offset: 999999, Policy: pol, Generation: generation, Ordinal: 9999})
		view := View{SchemaVersion: Version, Projection: Projection, SourceDigest: p.ArtifactIdentity.ExactSerializedBytesDigest, Generation: generation, Delivery: "PAGE", Manifest: manifest, Page: &page, NextCursor: next}
		raw, _ := json.Marshal(view)
		return len(raw) <= pol.MaxPageBytes
	}
	for _, x := range all {
		cand := append(append([]Entry{}, cur...), x)
		if !fits(cand) {
			if len(cur) == 0 {
				return nil, errors.New("ancillary indivisible entry")
			}
			parts = append(parts, cur)
			cur = []Entry{x}
			if !fits(cur) {
				return nil, errors.New("ancillary indivisible entry")
			}
		} else {
			cur = cand
		}
	}
	if len(cur) > 0 || len(parts) == 0 {
		parts = append(parts, cur)
	}
	if len(parts) > pol.MaxPages {
		return nil, errors.New("ancillary page count budget")
	}
	s := &Snapshot{binding: binding, projection: p, policy: pol, generation: generation}
	offset := map[string]int{}
	for i, es := range parts {
		section := "seeds"
		off := 0
		if len(es) > 0 {
			section = es[0].Section
			off = offset[section]
		}
		pg := Page{Snapshot: binding, Ordinal: i, TotalPages: len(parts), Section: section, Offset: off, Entries: es}
		pg.Digest = digest(pg)
		s.pages = append(s.pages, pg)
		for _, x := range es {
			offset[x.Section]++
		}
	}
	_ = source
	return s, nil
}
func (s *Snapshot) page(token string) (Page, string, error) {
	i := 0
	if token != "" {
		c, e := decode(token)
		if e != nil {
			return Page{}, "", e
		}
		if c.Snapshot != s.binding || c.Projection != Projection || c.Policy != s.policy || c.Generation != s.generation || c.Ordinal < 1 || c.Ordinal >= len(s.pages) {
			return Page{}, "", errors.New("stale or invalid ancillary cursor")
		}
		want := s.pages[c.Ordinal]
		if c.Section != want.Section || c.Offset != want.Offset {
			return Page{}, "", errors.New("skipped or noncanonical ancillary page")
		}
		i = c.Ordinal
	}
	p := s.pages[i]
	next := ""
	if i+1 < len(s.pages) {
		n := s.pages[i+1]
		next = encode(cursor{s.binding, Projection, n.Section, n.Offset, s.policy, s.generation, i + 1})
	}
	return p, next, nil
}
func Inspect(r Request) (View, error) {
	if e := Check(r); e != nil {
		return View{}, e
	}
	b, _ := InputBytes(r.Input)
	p, e := inspection.ProjectAllSeeds(b)
	if e != nil {
		return View{}, e
	}
	pol, _ := normalizePolicy(r.Policy)
	m := counts(p)
	v := View{Version, Projection, p.ArtifactIdentity.ExactSerializedBytesDigest, r.Generation, "FULL", m, &p, nil, ""}
	if r.Page {
		s, e := NewSnapshot(b, p, pol, r.Generation)
		if e != nil {
			return View{}, e
		}
		pg, next, e := s.page(r.Cursor)
		if e != nil {
			return View{}, e
		}
		for _, x := range pg.Entries {
			switch x.Section {
			case "seeds":
				m.Seeds.Returned++
			case "seed_memberships":
				m.SeedMemberships.Returned++
			case "frontier":
				m.Frontier.Returned++
			case "terminals":
				m.Terminals.Returned++
			case "diagnostics":
				m.Diagnostics.Returned++
			}
		}
		v.Full = nil
		v.Page = &pg
		v.NextCursor = next
		v.Manifest = m
		v.Delivery = "PAGE"
	} else {
		v.Manifest.Seeds.Returned = v.Manifest.Seeds.Total
		v.Manifest.SeedMemberships.Returned = v.Manifest.SeedMemberships.Total
		v.Manifest.Frontier.Returned = v.Manifest.Frontier.Total
		v.Manifest.Terminals.Returned = v.Manifest.Terminals.Total
		v.Manifest.Diagnostics.Returned = v.Manifest.Diagnostics.Total
	}
	raw, _ := json.Marshal(v)
	if len(raw) > pol.MaxOutputBytes {
		return View{}, errors.New("ancillary output policy byte limit")
	}
	return v, nil
}
func Reassemble(r Request, views []View) (inspection.AllProjection, error) {
	if e := Check(r); e != nil {
		return inspection.AllProjection{}, e
	}
	if !r.Page || r.Cursor != "" || len(views) == 0 {
		return inspection.AllProjection{}, errors.New("invalid ancillary page set")
	}
	b, _ := InputBytes(r.Input)
	want, e := inspection.ProjectAllSeeds(b)
	if e != nil {
		return want, e
	}
	pol, _ := normalizePolicy(r.Policy)
	s, e := NewSnapshot(b, want, pol, r.Generation)
	if e != nil {
		return want, e
	}
	if len(views) != len(s.pages) {
		return want, errors.New("incomplete ancillary pages")
	}
	token := ""
	for i, v := range views {
		pg, next, e := s.page(token)
		if e != nil {
			return want, e
		}
		if v.Delivery != "PAGE" || v.Page == nil || !reflect.DeepEqual(*v.Page, pg) || v.NextCursor != next || v.SourceDigest != want.ArtifactIdentity.ExactSerializedBytesDigest || v.Projection != Projection {
			return want, fmt.Errorf("noncanonical ancillary page %d", i)
		}
		token = next
	}
	if token != "" {
		return want, errors.New("incomplete ancillary pages")
	}
	return want, nil
}
