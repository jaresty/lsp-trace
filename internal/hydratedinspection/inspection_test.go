package hydratedinspection

import (
	"bytes"
	"encoding/json"
	he "lsp-trace/internal/hydratedevidence"
	"os"
	"reflect"
	"strings"
	"testing"
)

const edge = "sha256:e16c80b01aa9de78ff896b411d93746a6bfa749c1a80bc7b1c3bd251df81fa4b"
const edge2 = "sha256:42df1c10ff307e342f79d0a3d3f28cf115a501f6274619801c612e8705bef707"

func fixture(t *testing.T) Request {
	t.Helper()
	raw, err := os.ReadFile("../hydratedevidence/testdata/focused-fr20.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 154035 || he.Digest(raw) != "sha256:8913b3d062312be15531728f801e2f677f4a65852fd5fbeaf4ef1007ae1f83cf" {
		t.Fatal("original fixture drift")
	}
	r := DefaultRequest()
	r.Input = string(raw)
	r.RelationIDs = []string{edge, edge2}
	return r
}
func inspect(t *testing.T, r Request) View {
	t.Helper()
	v, err := Inspect(r)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func jsonBytes(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPublicPrivacyAndNativeText(t *testing.T) {
	r := fixture(t)
	v := inspect(t, r)
	if len(v.Bundle.Spans) != 0 {
		t.Fatal("PUBLIC_PRIVACY FAIL")
	}
	for _, o := range v.Bundle.Origins {
		if o.Status != "PRIVACY_EXCLUDED" || o.CoordinateAuthority != he.Native {
			t.Fatal("PUBLIC_PRIVACY FAIL")
		}
	}
	r.IncludeBodies = true
	v = inspect(t, r)
	if err := ValidateFull(r, v); err != nil {
		t.Fatal(err)
	}
	core, err := he.Text(r.InputBytes(), v.Request, *v.Bundle)
	if err != nil {
		t.Fatal(err)
	}
	if len(core) != 6032 {
		t.Fatalf("PUBLIC_CORE_TEXT FAIL: got %d want 6032", len(core))
	}
	focused, err := Text(r, v)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(focused, "UNKNOWN_SOURCE") || !strings.Contains(focused, "Focus origin") {
		t.Fatal("PUBLIC_CORE_TEXT FAIL")
	}
	for _, line := range strings.Split(core, "\n") {
		if strings.HasPrefix(line, "Span ") && !strings.Contains(focused, line) {
			t.Fatal("PUBLIC_CORE_TEXT FAIL: core span changed")
		}
	}
	selected := map[string]bool{}
	for _, o := range v.Bundle.Origins {
		selected[o.Selection.SourceID] = true
	}
	for _, s := range v.Bundle.Sources {
		if !selected[s.ID] && strings.Contains(focused, "Source "+quote(s.ID)) {
			t.Fatal("PUBLIC_CORE_TEXT FAIL: unrelated bookkeeping")
		}
	}
	t.Logf("PUBLIC_CORE_TEXT PASS core=%d focused=%d catalog=%d origins=%d spans=%d", len(core), len(focused), len(v.Bundle.Sources), len(v.Bundle.Origins), len(v.Bundle.Spans))
	r.NodeIDs = []string{"unknown\nspoof"}
	v = inspect(t, r)
	text, err := Text(r, v)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Bundle.Complete || v.Manifest.Origins[0].Status != "UNKNOWN_ID" || strings.Contains(text, "unknown\nspoof") || !strings.Contains(text, `unknown\nspoof`) {
		t.Fatal("PUBLIC_MANIFEST_ONLY FAIL")
	}
	r.WholeFile = true
	v = inspect(t, r)
	for _, s := range v.Bundle.Spans {
		if len(s.Content) != 9 || s.Bytes != (he.Interval{Start: 0, End: 9}) || s.ContentHash != he.Digest(s.Content) {
			t.Fatal("PUBLIC_WHOLE_FILE FAIL")
		}
	}
	for _, o := range v.Bundle.Origins {
		if o.OriginalRange != nil || o.CoordinateAuthority != "UNAVAILABLE" {
			t.Fatal("PUBLIC_WHOLE_FILE FAIL")
		}
	}
	r.EndpointContext = true
	v = inspect(t, r)
	if len(v.Manifest.Origins[1].Sites) != 3 {
		t.Fatal("PUBLIC_ENDPOINT_OPTIN FAIL")
	}
	t.Log("PUBLIC_PRIVACY PASS; PUBLIC_MANIFEST_ONLY PASS; PUBLIC_WHOLE_FILE PASS; PUBLIC_ENDPOINT_OPTIN PASS")
}
func quote(s string) string { b, _ := json.Marshal(s); return string(b) }

func TestPublicSidecarPrivacyAuthority(t *testing.T) {
	r := fixture(t)
	native := inspect(t, r)
	src := native.Bundle.Origins[0].Selection.SourceID
	secret := []byte("SECRET-NOT-EXPORTED")
	hash := he.Digest(secret)
	side := he.Sidecar{SchemaVersion: he.SidecarVersion, ArtifactDigest: he.Digest([]byte(r.Input)), Authority: he.Caller, Qualification: he.NonAuthoritative, Sources: []he.AssertedSource{{ID: "secret", URI: "file:///definitely-absent/secret", ReceiptReference: "asserted-receipt", VersionReference: "asserted-version", Content: &secret, ContentHash: &hash, SourceEncoding: "utf-8", State: "RETAINED_BYTES"}}, Records: []he.AssertedRecord{{ID: "shared", Kind: "CALLS", SourceIDs: []string{src}, Range: &he.Range{Start: he.Position{Character: 1}, End: he.Position{Character: 2}}, Encoding: "utf-8", RelationshipReferences: []string{edge}}, {ID: "secret", Kind: "opaque", SourceIDs: []string{"secret"}, Range: &he.Range{End: he.Position{Character: len(secret)}}, Encoding: "utf-8"}, {ID: "unmapped", Kind: "opaque", SourceIDs: []string{}, Range: &he.Range{End: he.Position{Character: 1}}, Encoding: "utf-8"}}}
	raw := jsonBytes(t, side)
	r.Sidecars = []string{string(raw)}
	prefix := "sidecar:" + he.Digest(raw) + ":"
	r.SidecarRecordIDs = []string{prefix + "shared", prefix + "secret", prefix + "unmapped"}
	v := inspect(t, r)
	wire := jsonBytes(t, v)
	if bytes.Contains(wire, secret) || bytes.Contains(wire, []byte("U0VDUkVULU5PVC1FWFBPUlRFRA==")) || len(v.Bundle.Spans) != 0 {
		t.Fatal("PUBLIC_SIDECAR_PRIVACY FAIL")
	}
	text, err := Text(r, v)
	if err != nil || strings.Contains(text, string(secret)) || !strings.Contains(text, "NO_BOUND_SOURCE") {
		t.Fatal("PUBLIC_SIDECAR_PRIVACY FAIL", err)
	}
	r.IncludeBodies = true
	v = inspect(t, r)
	var found bool
	for _, o := range v.Bundle.Origins {
		if o.Selection.RecordID == prefix+"shared" {
			found = true
			if o.Record.Authority != he.Caller || o.CoordinateAuthority != he.Caller || o.Record.Qualification != he.NonAuthoritative || o.Selection.SourceID != src {
				t.Fatal("PUBLIC_SIDECAR_AUTHORITY FAIL")
			}
		}
	}
	if !found {
		t.Fatal("PUBLIC_SIDECAR_AUTHORITY FAIL: missing shared asserted record")
	}
	if err := ValidateFull(r, v); err != nil {
		t.Fatal(err)
	}
	t.Log("PUBLIC_SIDECAR_PRIVACY PASS; PUBLIC_SIDECAR_AUTHORITY PASS")
}

func TestPublicPagingAndValidation(t *testing.T) {
	r := fixture(t)
	r.IncludeBodies = true
	r.NodeIDs = []string{"unknown"}
	r.CorePolicy.MaxPageBytes = 4096
	r.Page = true
	v := inspect(t, r)
	if v.Delivery != "PAGE" || v.Bundle != nil || v.Page == nil || v.NextCursor == "" {
		t.Fatal("PUBLIC_PAGING FAIL: expected explicit first page")
	}
	views := []View{v}
	next := v.NextCursor
	for next != "" {
		q := r
		q.Cursor = next
		a := inspect(t, q)
		b := inspect(t, q)
		if !reflect.DeepEqual(a, b) {
			t.Fatal("PUBLIC_PAGING FAIL: replay changed")
		}
		views = append(views, a)
		next = a.NextCursor
	}
	got, err := Reassemble(r, views)
	if err != nil {
		t.Fatal(err)
	}
	f := r
	f.Page = false
	full := inspect(t, f)
	if !reflect.DeepEqual(got.Bundle, *full.Bundle) || !reflect.DeepEqual(got.Manifest, full.Manifest) {
		t.Fatal("PUBLIC_PAGING FAIL: reconstruction drift")
	}
	for _, change := range []string{"unknown", "artifact", "body", "whole", "endpoint", "policy"} {
		q := r
		q.Cursor = v.NextCursor
		switch change {
		case "unknown":
			q.NodeIDs = []string{"different unknown"}
		case "artifact":
			q.Input += " "
		case "body":
			q.IncludeBodies = false
		case "whole":
			q.WholeFile = true
		case "endpoint":
			q.EndpointContext = true
		case "policy":
			q.CorePolicy.MaxWork--
		}
		if _, err := Inspect(q); err == nil {
			t.Fatalf("PUBLIC_CURSOR_BINDING FAIL: %s", change)
		}
	}
	for _, bad := range [][]View{views[:len(views)-1], append(append([]View{}, views...), views[len(views)-1]), append([]View{views[1], views[0]}, views[2:]...)} {
		if _, err := Reassemble(r, bad); err == nil {
			t.Fatal("PUBLIC_PAGE_JOIN FAIL")
		}
	}
	wrong := append([]View{}, views...)
	wrong[0].Manifest.Origins = append([]he.FocusOrigin{}, wrong[0].Manifest.Origins...)
	wrong[0].Manifest.Origins[0].Status = "MAPPED"
	if _, err := Reassemble(r, wrong); err == nil {
		t.Fatal("PUBLIC_PAGE_MANIFEST FAIL")
	}
	for _, mutate := range []func(*View){func(v *View) { v.Manifest.Origins = v.Manifest.Origins[1:] }, func(v *View) { v.Bundle.Sources = v.Bundle.Sources[:1] }, func(v *View) { v.Bundle.Spans[0].Content = []byte("forged") }, func(v *View) { v.Bundle.Origins[0].CoordinateAuthority = he.Caller }} {
		bad := inspect(t, f)
		mutate(&bad)
		if err := ValidateFull(f, bad); err == nil {
			t.Fatal("PUBLIC_FULL_VALIDATION FAIL")
		}
	}
	t.Logf("PUBLIC_PAGING PASS pages=%d; PUBLIC_CURSOR_BINDING PASS; PUBLIC_PAGE_JOIN PASS; PUBLIC_PAGE_MANIFEST PASS; PUBLIC_FULL_VALIDATION PASS", len(views))
}

func TestPublicClosedPreflight(t *testing.T) {
	for _, raw := range []string{`{}`, `{"input":{}}`, `{"input":"{}","node_ids":null}`, `{"input":"{}","include_bodies":null}`, `{"input":"{}","include_bodies":1}`, `{"input":"{}","include_bodies":false,"include_bodies":true}`, `{"input":"{}","Input":"{}"}`, `{"input":"{}","position_encoding":"guess"}`, `{"input":"{}","core_policy":{"max_work":1.5}}`, `{"input":"{}","core_policy":{"max_work":9007199254740993}}`, `{"input":"{}","core_policy":{"include_bodies":true}}`, `{"input":"{}","core_policy":{"allow_caller_boundaries":true}}`, `{"input":"{}","cursor":"abc"}`, `{"input":"{}","output_selector":"artifact.json"}`, `{"input":"{}","sidecars":[{}]}`, `{"input":"{}","provider":"native"}`} {
		if _, err := Decode([]byte(raw)); err == nil {
			t.Fatalf("PUBLIC_CLOSED_PREFLIGHT FAIL: %s", raw)
		}
	}
	if _, err := Decode([]byte(`{"input":"{}","core_policy":{"max_body_bytes":0,"max_work":0,"max_spans":0,"max_origins":0}}`)); err != nil {
		t.Fatalf("PUBLIC_CLOSED_PREFLIGHT FAIL: valid zero budget: %v", err)
	}
	r := fixture(t)
	r.NodeIDs = []string{strings.Repeat("x", 1025)}
	if err := Check(r); err == nil {
		t.Fatal("PUBLIC_CLOSED_PREFLIGHT FAIL: ID byte bound")
	}
	r = fixture(t)
	r.CorePolicy.MaxOrigins = 1
	if err := Check(r); err == nil {
		t.Fatal("PUBLIC_CLOSED_PREFLIGHT FAIL: origin limit")
	}
	r = fixture(t)
	r.Input = strings.Repeat(" ", MaxArtifactBytes+1)
	if err := Check(r); err == nil {
		t.Fatal("PUBLIC_CLOSED_PREFLIGHT FAIL: combined bytes")
	}
	v := inspect(t, fixture(t))
	wire := jsonBytes(t, v)
	wire = bytes.Replace(wire, []byte(`"delivery":"FULL"`), []byte(`"delivery":"FULL","extra":true`), 1)
	if err := ValidateOutputJSON(wire); err == nil {
		t.Fatal("PUBLIC_CLOSED_OUTPUT FAIL")
	}
	t.Log("PUBLIC_CLOSED_PREFLIGHT PASS; PUBLIC_CLOSED_OUTPUT PASS")
}
