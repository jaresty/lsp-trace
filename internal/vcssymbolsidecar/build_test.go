package vcssymbolsidecar

import (
	"context"
	"testing"
)

type fakeDiffSource struct{ collection DiffCollection }

func (f fakeDiffSource) Collect(string, []string, string, string) (DiffCollection, error) {
	return f.collection, nil
}

type revisionWorkspace struct {
	revisions []string
	n         int
}

func (w *revisionWorkspace) Open(context.Context, string, string) (HistoricalWorkspace, error) {
	r := w.revisions[w.n]
	w.n++
	return HistoricalWorkspace{Root: "/tmp/exact", Revision: r, Close: func() error { return nil }}, nil
}
func TestBuildV2BindsDigestRevisionsAndClosedAttribution(t *testing.T) {
	a := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	b := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	w := &revisionWorkspace{revisions: []string{a, b}}
	s := &fakeSymbolSession{}
	got, err := BuildV2(context.Background(), []byte(`{"schema_version":"lsp-trace.transient-structural-result.v2"}`), BuildRequest{Repository: "/repo", FromRevision: "old", ToRevision: "new", Paths: []string{"a.go"}, LanguageID: "go"}, fakeDiffSource{DiffCollection{FromRevision: a, ToRevision: b, Lines: []ChangedLine{{Side: "NEW", Path: "a.go", Line: 0}}}}, w, s)
	if err != nil {
		t.Fatal(err)
	}
	if got.FromRevision != a || got.ToRevision != b || got.GraphArtifactDigest == "" || got.LineCount != 1 || got.AttributedLineCount != 1 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_V2_COMPOSITION: %+v", got)
	}
}
func TestBuildV2RejectsRevisionDisagreement(t *testing.T) {
	a := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	b := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	w := &revisionWorkspace{revisions: []string{b, b}}
	_, err := BuildV2(context.Background(), []byte("x"), BuildRequest{Repository: "/repo", FromRevision: "old", ToRevision: "new", Paths: []string{"a.go"}, LanguageID: "go"}, fakeDiffSource{DiffCollection{FromRevision: a, ToRevision: b}}, w, &fakeSymbolSession{})
	if err == nil {
		t.Fatal("ASSERT_SYMBOL_CHURN_V2_REJECTS_REVISION_DISAGREEMENT")
	}
}
