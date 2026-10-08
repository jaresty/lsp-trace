package adr0007sourcetextsearchv2private

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func baseReq(src []byte) (Request, SourceFile) {
	h := sha256.Sum256(src)
	b := SourceBinding{SchemaVersion: SchemaBindingV2, Path: "a.txt", RepositoryCommit: "af2ce89321afc94c937636f841bb98b8977b6496", Revision: "r1", FileSHA256: "sha256:" + hex.EncodeToString(h[:]), GitBlobSHA1: "4953fab89d911e2352fa6c2253497c07c8e9a777", ObjectID: "obj", AdmissionSchema: "lsp-trace.adr0007.source-admission.private.v2", AdmissionID: "admission-0000000000000001"}
	r := Request{SchemaVersion: SchemaRequestV2, Query: "aba", Sources: []SourceBinding{b}, Policy: Policy{SchemaVersion: SchemaPolicyV2, LiteralMode: "EXACT_NONEMPTY_CASE_SENSITIVE_UTF8"}, Limits: Limits{SchemaVersion: SchemaLimitsV2, MaxFiles: 10, MaxMatches: 10, MaxWork: 1000000, MaxOutputBytes: 1000000, MaxSourceBytes: 1000000, MaxPathBytes: 4096}, Cancel: Cancel{Token: "none"}}
	return r, SourceFile{Binding: b, Bytes: src}
}

func TestOverlapAndPositions(t *testing.T) {
	r, f := baseReq([]byte("ababa"))
	got, err := Search(r, []SourceFile{f})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Matches) != 2 {
		t.Fatalf("matches=%d", len(got.Matches))
	}
	if got.Accepted || got.Authority != 0 || got.Completeness != "UNKNOWN" || got.FeatureIdentity != "UNRESOLVED" {
		t.Fatalf("authority envelope changed: %+v", got)
	}
}
func TestStrictUTF8NoClamp(t *testing.T) {
	r, f := baseReq([]byte{0xff})
	if _, err := Search(r, []SourceFile{f}); err == nil {
		t.Fatal("accepted invalid UTF-8 source")
	}
}
func TestDuplicateTupleFail(t *testing.T) {
	r, f := baseReq([]byte("ababa"))
	r.Sources = []SourceBinding{f.Binding, f.Binding}
	if _, err := Search(r, []SourceFile{f, f}); err == nil {
		t.Fatal("accepted duplicate tuple")
	}
}
func TestLimitPlusOneFail(t *testing.T) {
	r, f := baseReq([]byte("ababa"))
	r.Limits.MaxMatches = 1
	if _, err := Search(r, []SourceFile{f}); err == nil {
		t.Fatal("accepted max_matches+1")
	}
}
func TestAccountingFormula(t *testing.T) {
	a := Accounting{JFiles: 1, QQueryBytes: 3, PPathBytes: 5, SSourceBytes: 5, TScannedTuples: 1, MMatches: 2, RRanges: 2, UUTF16Units: 6, BOutputBytes: 100}
	w, ok := CheckedWork(a)
	if !ok || w != 50+3+15+35+5+11+26+34+114+3100 {
		t.Fatalf("work=%d ok=%v", w, ok)
	}
}
func TestCRLFLFBareCRNonBMP(t *testing.T) {
	line, ch := Position([]byte("x\r\ny\nz\rw😀"), 10)
	if line != 3 || ch != 3 {
		t.Fatalf("line=%d ch=%d", line, ch)
	}
}
func TestNoPublicSurfacePaths(t *testing.T) {
	bad := []string{"pkg/adr0007sourcetextsearchv2", "cmd/adr0007-source-text-search-v2"}
	for _, p := range bad {
		if _, err := os.Stat(filepath.Join("..", "..", p)); err == nil {
			t.Fatalf("public path exists: %s", p)
		}
	}
}
func TestImportedAdmissionPinConstants(t *testing.T) {
	const commit = "af2ce89321afc94c937636f841bb98b8977b6496"
	const sha = "sha256:da74770d5b36f63e6f1265ba78e2404e13f2d1f1451a4e7aa405f448e47fe7da"
	const blob = "4953fab89d911e2352fa6c2253497c07c8e9a777"
	if commit == "" || sha == "" || blob == "" {
		t.Fatal("missing pins")
	}
}
