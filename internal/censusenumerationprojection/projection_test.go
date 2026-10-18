package censusenumerationprojection

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/publication"
)

func fixture(t *testing.T) Projection {
	t.Helper()
	start := Position{Line: 3, Character: 4}
	digest, _, err := SymbolIdentityDigest("a.go", start, 12, "secretName", 0)
	if err != nil {
		t.Fatal(err)
	}
	fo := 0
	so := 0
	observed := uint64(1)
	p, err := Build(Input{SessionID: "session", Generation: 2, RequestFingerprint: "sha256:" + strings.Repeat("a", 64), WorkspaceURI: "file:///workspace", Files: []File{{0, "a.go", "incomplete"}, {1, "failed.go", "unreadable"}}, Symbols: []Symbol{{0, 0, start, 12, "incomplete", digest}}, Observations: []Observation{{Dimension: "PROVIDER_COMPLETENESS", State: "KNOWN", Observed: &observed}}, ProviderFailures: []ProviderFailure{{Stage: "PREPARE_CALL_HIERARCHY", Code: "RESULT_MISMATCH", SymbolOrdinal: &so}, {Stage: "DOCUMENT_SUPPLY", Code: "REQUEST_FAILED", FileOrdinal: func() *int { v := fo + 1; return &v }()}}, RequestMaxNodes: 631, RequestTimeoutMS: 30, TimeoutMS: 60})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestCanonicalStrictIdentityPrivacyAndMutationBindings(t *testing.T) {
	p := fixture(t)
	raw, err := EncodeCanonical(p)
	if err != nil {
		t.Fatal(err)
	}
	again, err := EncodeCanonical(p)
	if err != nil || !bytes.Equal(raw, again) {
		t.Fatal("not deterministic")
	}
	if bytes.Contains(raw, []byte("secretName")) || bytes.Contains(raw, []byte("file:///workspace")) || bytes.Contains(raw, []byte("/workspace/")) {
		t.Fatalf("private source identity leaked: %s", raw)
	}
	if _, err := Decode(raw); err != nil {
		t.Fatal(err)
	}
	for name, mut := range map[string]func([]byte) []byte{"unknown": func(b []byte) []byte {
		return bytes.Replace(b, []byte(`{"schema_version"`), []byte(`{"extra":1,"schema_version"`), 1)
	}, "duplicate": func(b []byte) []byte {
		return bytes.Replace(b, []byte(`{"schema_version"`), []byte(`{"schema_version":"x","schema_version"`), 1)
	}, "trailing": func(b []byte) []byte { return append(b, []byte("{}")...) }, "identity": func(b []byte) []byte {
		return bytes.Replace(b, []byte(p.Symbols[0].IdentityDigest), []byte("sha256:"+strings.Repeat("b", 64)), 1)
	}} {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode(mut(append([]byte(nil), raw...))); err == nil {
				t.Fatal("mutation accepted")
			}
		})
	}
}
func TestBoundsWorkAndReconciliation(t *testing.T) {
	p := fixture(t)
	p.Work++
	if _, err := EncodeCanonical(p); err == nil {
		t.Fatal("work mutation accepted")
	}
	p = fixture(t)
	p.ProviderFailures[0].SymbolOrdinal = new(int)
	*p.ProviderFailures[0].SymbolOrdinal = 9
	if _, err := EncodeCanonical(p); err == nil {
		t.Fatal("dangling failure accepted")
	}
	p = fixture(t)
	p.Files[0].Ordinal = 1
	if _, err := EncodeCanonical(p); err == nil {
		t.Fatal("ordinal gap accepted")
	}
}
func TestLarge631FixtureUnderLimits(t *testing.T) {
	files := []File{{0, "many.go", "incomplete"}}
	symbols := make([]Symbol, 631)
	for i := range symbols {
		start := Position{Line: uint32(i)}
		d, _, e := SymbolIdentityDigest("many.go", start, 12, "n", i)
		if e != nil {
			t.Fatal(e)
		}
		symbols[i] = Symbol{i, 0, start, 12, "incomplete", d}
	}
	p, e := Build(Input{SessionID: "s", Generation: 1, RequestFingerprint: "sha256:" + strings.Repeat("a", 64), WorkspaceURI: "file:///workspace", Files: files, Symbols: symbols, RequestMaxNodes: 631, RequestTimeoutMS: 1, TimeoutMS: 1})
	if e != nil {
		t.Fatal(e)
	}
	raw, e := EncodeCanonical(p)
	if e != nil || len(raw) >= MaxCanonicalBytes {
		t.Fatalf("raw=%d err=%v", len(raw), e)
	}
}
func TestPublishDistinctNamespaceCustodyAndCollision(t *testing.T) {
	d := t.TempDir()
	if err := os.Chmod(d, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(d)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	p := fixture(t)
	r, err := Publish(root, p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(r.Selector, SelectorPrefix) || r.VerificationStatus != "VERIFIED" || !r.NamespaceAtomic {
		t.Fatalf("%+v", r)
	}
	raw, err := os.ReadFile(filepath.Join(d, filepath.FromSlash(r.Selector)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(raw); err != nil {
		t.Fatal(err)
	}
	if _, err := Publish(root, p); err == nil {
		t.Fatal("collision accepted")
	}
}
func TestReceiptClosedUnavailableAndStrictJSONRoundTrip(t *testing.T) {
	for _, reason := range []string{"ENCODE_FAILED", "PUBLICATION_FAILED", "CANCELED_PRE_RECONCILIATION", "CANCELED_PRE_PUBLICATION"} {
		raw, err := MarshalReceipt(Unavailable(reason))
		if err != nil || !json.Valid(raw) {
			t.Fatalf("%s %q %v", reason, raw, err)
		}
	}
	if _, err := MarshalReceipt(Unavailable("raw server prose")); err == nil {
		t.Fatal("open reason accepted")
	}
}
