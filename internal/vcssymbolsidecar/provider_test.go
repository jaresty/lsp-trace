package vcssymbolsidecar

import (
	"context"
	"errors"
	"testing"
)

type fakeHistoricalWorkspace struct{ opens, closes int }

func (f *fakeHistoricalWorkspace) Open(context.Context, string, string) (HistoricalWorkspace, error) {
	f.opens++
	return HistoricalWorkspace{Root: "/tmp/exact", Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Close: func() error { f.closes++; return nil }}, nil
}

type fakeSymbolSession struct{ starts, closes, calls int }

func (f *fakeSymbolSession) Start(context.Context, string) (SymbolClient, error) {
	f.starts++
	return f, nil
}
func (f *fakeSymbolSession) Close() error { f.closes++; return nil }
func (f *fakeSymbolSession) Symbols(_ context.Context, root, path, language string) ([]Symbol, error) {
	f.calls++
	if path == "bad.go" {
		return nil, errors.New("bad")
	}
	return []Symbol{{Path: path, Name: "A", Kind: 12, Range: Range{End: Position{Character: 1}}}}, nil
}
func TestAcquireRevisionUsesOneWorkspaceAndSession(t *testing.T) {
	w := &fakeHistoricalWorkspace{}
	s := &fakeSymbolSession{}
	got, err := AcquireRevision(context.Background(), RevisionRequest{Repository: "/repo", Revision: "abc", Paths: []string{"a.go", "bad.go"}, LanguageID: "go"}, w, s)
	if err != nil {
		t.Fatal(err)
	}
	if w.opens != 1 || w.closes != 1 || s.starts != 1 || s.closes != 1 || s.calls != 2 {
		t.Fatalf("ASSERT_SYMBOL_PROVIDER_ONE_SESSION_PER_REVISION: w=%+v s=%+v", w, s)
	}
	if got.Outcomes[0].Status != "COMPLETE" || got.Outcomes[1].Status != "FAILED" {
		t.Fatalf("ASSERT_SYMBOL_PROVIDER_CLOSED_FILE_OUTCOMES: %+v", got)
	}
}
