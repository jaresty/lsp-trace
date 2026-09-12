package acquisitionops

import (
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/sessionruntime"
)

func TestPreparedDocumentIdentityIsExact(t *testing.T) {
	base := sessionruntime.DocumentResult{URI: "file:///w/a.go", LanguageID: "go", Version: 3, Supply: &sessionruntime.DocumentSupply{SessionID: "s", Generation: 7, URI: "file:///w/a.go", DocumentVersion: 3}}
	locator := acquisition.Locator{URI: base.URI, LanguageID: "go"}
	if err := validatePreparedDocument("s", 7, locator, base); err != nil {
		t.Fatalf("ASSERT_PREPARED_DOCUMENT_EXACT_IDENTITY_ACCEPTED: %v", err)
	}
	for _, tc := range []struct {
		name string
		edit func(*sessionruntime.DocumentResult, *acquisition.Locator)
	}{
		{"session", func(p *sessionruntime.DocumentResult, _ *acquisition.Locator) { p.Supply.SessionID = "other" }},
		{"generation", func(p *sessionruntime.DocumentResult, _ *acquisition.Locator) { p.Supply.Generation++ }},
		{"supply URI", func(p *sessionruntime.DocumentResult, _ *acquisition.Locator) { p.Supply.URI = "file:///w/b.go" }},
		{"request URI", func(_ *sessionruntime.DocumentResult, l *acquisition.Locator) { l.URI = "file:///w/b.go" }},
		{"language", func(_ *sessionruntime.DocumentResult, l *acquisition.Locator) { l.LanguageID = "rust" }},
		{"document version", func(p *sessionruntime.DocumentResult, _ *acquisition.Locator) { p.Supply.DocumentVersion++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			s := *base.Supply
			p.Supply = &s
			l := locator
			tc.edit(&p, &l)
			if err := validatePreparedDocument("s", 7, l, p); err == nil {
				t.Fatal("ASSERT_PREPARED_DOCUMENT_IDENTITY_MISMATCH_REJECTED")
			}
		})
	}
}
