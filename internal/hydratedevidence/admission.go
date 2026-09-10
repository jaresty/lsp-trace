package hydratedevidence

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/source"
	"lsp-trace/internal/v5sourcesnapshot"
)

type admitted struct {
	Catalog
	bodies    map[string][]byte
	sourceMap map[string]Source
	recordMap map[string]Record
}

func checkPolicy(p Policy) error {
	if p.MaxInputBytes < 1 || p.MaxInputBytes > 192<<20 || p.MaxOutputBytes < 1 || p.MaxOutputBytes > 64<<20 || p.MaxBodyBytes < 0 || p.MaxBodyBytes > 16<<20 || p.MaxOrigins < 0 || p.MaxOrigins > 10000 || p.MaxSpans < 0 || p.MaxSpans > 10000 || p.MaxWork < 0 || p.MaxWork > 512<<20 || p.MaxPageBytes < 4096 || p.MaxPageBytes > 1<<20 || p.MaxPages < 1 || p.MaxPages > 10000 {
		return errors.New("invalid bounded hydration policy")
	}
	return nil
}

// preflight bounds nesting before native validators or recursive JSON decoding.
// Opaque data is never interpreted as an anchor. Deep opaque JSON is rejected,
// not mined for source references. Source contents are base64 scalar strings.
func preflight(raw []byte) error {
	if !utf8.Valid(raw) {
		return errors.New("invalid JSON UTF-8")
	}
	type frame struct {
		object, key bool
		keys        map[string]bool
	}
	stack := []frame{}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	roots := 0
	for {
		tok, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		if len(stack) == 0 {
			roots++
		}
		if len(stack) > 0 {
			f := &stack[len(stack)-1]
			if f.object && f.key {
				if k, ok := tok.(string); ok {
					if f.keys[k] {
						return errors.New("duplicate JSON member")
					}
					f.keys[k] = true
					f.key = false
					continue
				}
			}
		}
		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{', '[':
				if len(stack) >= 64 {
					return errors.New("JSON depth limit")
				}
				stack = append(stack, frame{object: delim == '{', key: delim == '{', keys: map[string]bool{}})
				continue
			case '}', ']':
				if len(stack) == 0 {
					return errors.New("unbalanced JSON")
				}
				stack = stack[:len(stack)-1]
			}
		}
		if len(stack) > 0 && stack[len(stack)-1].object {
			stack[len(stack)-1].key = true
		}
	}
	if len(stack) != 0 || roots != 1 {
		return errors.New("expected one JSON value")
	}
	return nil
}
func decode(raw []byte, out any) error {
	if e := preflight(raw); e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(out)
}
func Inspect(input Input, p Policy) (Catalog, error) { a, e := admit(input, p); return a.Catalog, e }
func admit(input Input, p Policy) (admitted, error) {
	a := admitted{Catalog: Catalog{InputDigests: []string{}, Sources: []Source{}, Records: []Record{}}, bodies: map[string][]byte{}, sourceMap: map[string]Source{}, recordMap: map[string]Record{}}
	fail := func(e error) (admitted, error) { return a, e }
	if e := checkPolicy(p); e != nil {
		return fail(e)
	}
	used := len(input.Artifact)
	if len(input.Sidecars) > 64 {
		return fail(errors.New("sidecar count limit"))
	}
	for _, raw := range input.Sidecars {
		if len(raw) > p.MaxInputBytes-used {
			return fail(errors.New("input byte budget"))
		}
		used += len(raw)
	}
	if used > p.MaxInputBytes || len(input.Artifact) == 0 {
		return fail(errors.New("input byte budget"))
	}
	if e := preflight(input.Artifact); e != nil {
		return fail(e)
	}
	var header struct {
		SchemaVersion string `json:"schema_version"`
	}
	if e := json.Unmarshal(input.Artifact, &header); e != nil {
		return fail(e)
	}
	version := ""
	switch header.SchemaVersion {
	case graphprovenance.Version:
		version = "v1"
	case graphprovenance.VersionV2:
		version = "v2"
	case v5sourcesnapshot.Version:
		version = "source-snapshot-v1"
	default:
		return fail(fmt.Errorf("unsupported evidence family/version: %s", header.SchemaVersion))
	}
	if version == "source-snapshot-v1" {
		if _, e := v5sourcesnapshot.Validate(input.Artifact); e != nil {
			return fail(e)
		}
	} else if _, e := graphprovenance.ValidateFor(input.Artifact, graphprovenance.Family, version); e != nil {
		return fail(e)
	}
	artifactDigest := Digest(input.Artifact)
	a.InputDigests = append(a.InputDigests, artifactDigest)
	addSource := func(s Source, content []byte) error {
		if _, ok := a.sourceMap[s.ID]; ok {
			return errors.New("duplicate source identity")
		}
		if len(a.Sources) >= 100000 {
			return errors.New("source count limit")
		}
		a.sourceMap[s.ID] = s
		a.Sources = append(a.Sources, s)
		if s.State == "RETAINED_BYTES" || s.State == "TRUNCATED_INPUT" {
			a.bodies[s.ID] = append([]byte{}, content...)
		}
		return nil
	}
	addRecord := func(r Record) error {
		if _, ok := a.recordMap[r.ID]; ok {
			return errors.New("duplicate record identity")
		}
		if len(a.Records) >= 100000 {
			return errors.New("record count limit")
		}
		if r.SourceIDs == nil {
			r.SourceIDs = []string{}
		}
		if r.RelationshipReferences == nil {
			r.RelationshipReferences = []string{}
		}
		a.recordMap[r.ID] = r
		a.Records = append(a.Records, r)
		return nil
	}
	receipts := []graphprovenance.Receipt{}
	bindings := []graphprovenance.Binding{}
	anchorStatuses := map[string]string{}
	var graphBytes []byte
	var document any
	encoding := ""
	if version == "v1" {
		var e graphprovenance.Evidence
		if err := json.Unmarshal(input.Artifact, &e); err != nil {
			return fail(err)
		}
		graphBytes = e.GraphBytes
		receipts = append(receipts, e.Captures...)
		if e.Supply != nil {
			receipts = append(receipts, *e.Supply)
		}
		bindings = e.Bindings
	} else if version == "v2" {
		var e graphprovenance.EvidenceV2
		if err := json.Unmarshal(input.Artifact, &e); err != nil {
			return fail(err)
		}
		graphBytes = e.GraphBytes
		receipts = append(receipts, e.Captures...)
		for _, s := range e.Supplies {
			if s.Receipt != nil {
				receipts = append(receipts, *s.Receipt)
			}
		}
		encoding = e.Acquisition.Request.Context.PositionEncoding
		for _, b := range e.Bindings {
			anchorStatuses[b.Pointer] = b.AnchorStatus
			bindings = append(bindings, graphprovenance.Binding{Pointer: b.Pointer, URI: b.URI, Attribution: b.Attribution, ReceiptIDs: b.ReceiptIDs})
		}
	} else {
		var snapshot v5sourcesnapshot.Artifact
		if err := json.Unmarshal(input.Artifact, &snapshot); err != nil {
			return fail(err)
		}
		var v5 graphprovenance.EvidenceV5
		if err := json.Unmarshal(snapshot.GraphV5Bytes, &v5); err != nil {
			return fail(err)
		}
		var err error
		graphBytes, err = base64.StdEncoding.DecodeString(v5.GraphV5)
		if err != nil {
			return fail(err)
		}
		for _, r := range snapshot.Receipts {
			receipts = append(receipts, graphprovenance.Receipt{ID: r.ID, URI: r.URI, Classification: "SOURCE", Status: "READABLE", Content: r.Content, CanonicalReceipt: r.CanonicalReceipt})
		}
		encoding = snapshot.PositionEncoding
		for _, b := range snapshot.Bindings {
			anchorStatuses[b.Pointer] = "VALID_COORDINATES"
			bindings = append(bindings, graphprovenance.Binding{Pointer: b.Pointer, URI: b.URI, Attribution: "SOURCE", ReceiptIDs: b.ReceiptIDs})
		}
	}
	if err := preflight(graphBytes); err != nil {
		return fail(err)
	}
	if err := json.Unmarshal(graphBytes, &document); err != nil {
		return fail(err)
	}
	// V2 binding pointers can address acquisition carriers as well as graph bytes.
	var envelope any
	if err := json.Unmarshal(input.Artifact, &envelope); err != nil {
		return fail(err)
	}
	for _, r := range receipts {
		var canonical source.Receipt
		if err := json.Unmarshal(r.CanonicalReceipt, &canonical); err != nil {
			return fail(err)
		}
		s := Source{ID: "native:" + r.ID, ArtifactDigest: artifactDigest, ReceiptReference: r.ID, ReceiptDigest: Digest(r.CanonicalReceipt), VersionReference: r.ID, URI: r.URI, SourceEncoding: "utf-8", State: "MISSING", InputStatus: r.Status, Authority: Native, Qualification: "INTEGRITY_ONLY", Classification: r.Classification, AnalyzedVersion: r.AnalyzedVersion}
		if canonical.Provenance.Revision != "" {
			v := canonical.Provenance.Revision
			s.Revision = &v
		}
		if canonical.ContentIdentity != nil {
			v := canonical.ContentIdentity.Digest
			s.ContentHash = &v
		}
		if r.Status == "READABLE" {
			s.State = "RETAINED_BYTES"
		}
		if r.Status == "BYTE_LIMIT_EXCEEDED" {
			s.State = "MISSING"
		} // rejected full read, not retained truncation
		if r.Supply != nil {
			v := r.Supply.Version
			s.DocumentVersion = &v
		}
		if old, ok := a.sourceMap[s.ID]; ok {
			if !same(old, s) {
				return fail(errors.New("conflicting repeated receipt"))
			}
			continue
		}
		if err := addSource(s, r.Content); err != nil {
			return fail(err)
		}
	}
	for _, b := range bindings {
		status := anchorStatuses[b.Pointer]
		if status == "" {
			status = "UNAVAILABLE"
		}
		r := Record{SourceAttribution: b.Attribution, AnchorStatus: status, ID: "native:" + b.Pointer, ArtifactDigest: artifactDigest, Pointer: b.Pointer, Kind: "NATIVE_BINDING", Authority: Native, Qualification: "INTEGRITY_ONLY", SourceIDs: []string{}, RelationshipReferences: []string{}, Encoding: encoding}
		for _, id := range b.ReceiptIDs {
			sid := "native:" + id
			if _, ok := a.sourceMap[sid]; !ok {
				return fail(errors.New("native source receipt foreign key"))
			}
			r.SourceIDs = append(r.SourceIDs, sid)
		}
		v := pointer(document, b.Pointer)
		if strings.HasPrefix(b.Pointer, "/graph/") {
			v = pointer(document, strings.TrimPrefix(b.Pointer, "/graph"))
		}
		if strings.HasPrefix(b.Pointer, "/acquisition/") {
			v = pointer(envelope, b.Pointer)
		}
		if m, ok := v.(map[string]any); ok {
			if _, ok = m["start"]; ok {
				raw, _ := json.Marshal(m)
				var rg Range
				if json.Unmarshal(raw, &rg) == nil {
					r.Range = &rg
				}
			}
		}
		// Retain exact enclosing native IDs where the known graph carrier has them.
		parts := strings.Split(b.Pointer, "/")
		for i := len(parts) - 1; i > 0; i-- {
			q := strings.Join(parts[:i], "/")
			parent := pointer(document, q)
			if strings.HasPrefix(q, "/acquisition/") {
				parent = pointer(envelope, q)
			}
			if strings.HasPrefix(q, "/graph/") {
				parent = pointer(document, strings.TrimPrefix(q, "/graph"))
			}
			if m, ok := parent.(map[string]any); ok {
				if id, ok := m["relation_id"].(string); ok {
					r.RelationshipReferences = append(r.RelationshipReferences, id)
					r.Kind = "NATIVE_RELATION"
					break
				}
				if id, ok := m["id"].(string); ok && r.NativeID == "" {
					r.NativeID = id
					r.Kind = "NATIVE_RECORD"
				}
			}
		}
		if err := addRecord(r); err != nil {
			return fail(err)
		}
	}
	seenSide := map[string]bool{}
	for _, raw := range input.Sidecars {
		sd := Digest(raw)
		if seenSide[sd] {
			return fail(errors.New("duplicate sidecar input"))
		}
		seenSide[sd] = true
		var side Sidecar
		if err := decode(raw, &side); err != nil {
			return fail(err)
		}
		if side.SchemaVersion != SidecarVersion {
			return fail(errors.New("unsupported sidecar family/version"))
		}
		if err := structure(raw); err != nil {
			return fail(err)
		}
		if side.ArtifactDigest != artifactDigest || side.Authority != Caller || side.Qualification != NonAuthoritative {
			return fail(errors.New("sidecar digest or authority mismatch"))
		}
		if len(side.Sources)+len(side.Records) > 10000 {
			return fail(errors.New("sidecar record limit"))
		}
		a.InputDigests = append(a.InputDigests, sd)
		prefix := "sidecar:" + sd + ":"
		for _, s := range side.Sources {
			if s.ID == "" || s.ReceiptReference == "" || s.VersionReference == "" || s.URI == "" || s.SourceEncoding != "utf-8" {
				return fail(errors.New("invalid asserted source identity/encoding"))
			}
			switch s.State {
			case "RETAINED_BYTES", "TRUNCATED_INPUT":
				if s.Content == nil || s.ContentHash == nil || *s.ContentHash != Digest(*s.Content) || !utf8.Valid(*s.Content) {
					return fail(errors.New("asserted content receipt mismatch"))
				}
			case "REFERENCE_ONLY", "MISSING":
				if s.Content != nil {
					return fail(errors.New("unavailable source has content"))
				}
			default:
				return fail(errors.New("unsupported source state"))
			}
			receiptBytes, _ := json.Marshal(s)
			out := Source{ID: prefix + s.ID, ArtifactDigest: sd, ReceiptReference: s.ReceiptReference, ReceiptDigest: Digest(receiptBytes), VersionReference: s.VersionReference, URI: s.URI, Revision: s.Revision, ContentHash: s.ContentHash, SourceEncoding: s.SourceEncoding, State: s.State, InputStatus: s.State, Authority: Caller, Qualification: NonAuthoritative, Classification: "SIDECAR_ASSERTION", AnalyzedVersion: "ANALYZED_VERSION_UNVERIFIED"}
			var content []byte
			if s.Content != nil {
				content = *s.Content
			}
			if err := addSource(out, content); err != nil {
				return fail(err)
			}
		}
		for recordIndex, r := range side.Records {
			if r.ID == "" || r.Kind == "" {
				return fail(errors.New("invalid sidecar record"))
			}
			out := Record{ID: prefix + r.ID, ArtifactDigest: sd, Pointer: "/records/" + strconv.Itoa(recordIndex), Kind: r.Kind, Authority: Caller, Qualification: NonAuthoritative, SourceIDs: []string{}, RelationshipReferences: r.RelationshipReferences, Range: r.Range, Encoding: r.Encoding}
			seen := map[string]bool{}
			for _, sid := range r.SourceIDs {
				if !strings.HasPrefix(sid, "native:") {
					sid = prefix + sid
				}
				if _, ok := a.sourceMap[sid]; !ok || seen[sid] {
					return fail(errors.New("sidecar source foreign key"))
				}
				seen[sid] = true
				out.SourceIDs = append(out.SourceIDs, sid)
			}
			if err := addRecord(out); err != nil {
				return fail(err)
			}
		}
	}
	for _, s := range a.Sources {
		if err := addRecord(Record{ID: "receipt:" + s.ID, ArtifactDigest: s.ArtifactDigest, Pointer: s.ReceiptReference, Kind: "SOURCE_RECEIPT", Authority: s.Authority, Qualification: s.Qualification, SourceIDs: []string{s.ID}, RelationshipReferences: []string{}}); err != nil {
			return fail(err)
		}
	}
	sort.Slice(a.Sources, func(i, j int) bool { return a.Sources[i].ID < a.Sources[j].ID })
	sort.Slice(a.Records, func(i, j int) bool { return a.Records[i].ID < a.Records[j].ID })
	return a, nil
}
func pointer(doc any, path string) any {
	if path == "" {
		return doc
	}
	if !strings.HasPrefix(path, "/") {
		return nil
	}
	for _, part := range strings.Split(path[1:], "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch v := doc.(type) {
		case map[string]any:
			doc = v[part]
		case []any:
			i, e := strconv.Atoi(part)
			if e != nil || i < 0 || i >= len(v) {
				return nil
			}
			doc = v[i]
		default:
			return nil
		}
	}
	return doc
}
func same(a, b any) bool { x, _ := json.Marshal(a); y, _ := json.Marshal(b); return bytes.Equal(x, y) }
