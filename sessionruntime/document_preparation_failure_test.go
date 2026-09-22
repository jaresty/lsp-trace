package sessionruntime

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/session"
)

func TestDocumentPreparationFailureCategories(t *testing.T) {
	for _, tc := range []struct {
		name       string
		configure  func(*testing.T, *Manager, *DocumentRequest, string)
		languageID string
		want       session.Failure
	}{
		{name: "uri", configure: func(_ *testing.T, _ *Manager, req *DocumentRequest, _ string) { req.URI = "not a file URI" }, languageID: "go", want: DocumentURIUnavailable},
		{name: "outside-workspace", configure: func(t *testing.T, _ *Manager, req *DocumentRequest, _ string) {
			outside := filepath.Join(t.TempDir(), "outside.go")
			if err := os.WriteFile(outside, []byte("package outside\n"), 0600); err != nil {
				t.Fatal(err)
			}
			req.URI = (&url.URL{Scheme: "file", Path: filepath.ToSlash(outside)}).String()
		}, languageID: "go", want: DocumentOutsideWorkspace},
		{name: "source", configure: func(t *testing.T, _ *Manager, _ *DocumentRequest, file string) {
			if err := os.Remove(file); err != nil {
				t.Fatal(err)
			}
		}, languageID: "go", want: DocumentSourceUnavailable},
		{name: "empty-language", configure: func(t *testing.T, _ *Manager, req *DocumentRequest, file string) {
			if err := os.Rename(file, filepath.Join(filepath.Dir(file), "seed")); err != nil {
				t.Fatal(err)
			}
			req.URI = (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(filepath.Dir(file), "seed"))}).String()
		}, want: LanguageIDUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, req, file, writer := supplyFixture(t, []byte("package fixture\n"))
			req.CaptureSupply = false
			req.LanguageID = tc.languageID
			tc.configure(t, m, &req, file)
			got := m.PrepareDocument(context.Background(), req)
			if got.Failure != tc.want || got.Supply != nil || writer.Len() != 0 {
				t.Fatalf("ASSERT_DOCUMENT_PREPARATION_FAILURE_CATEGORY: got=%+v want=%q writes=%d", got, tc.want, writer.Len())
			}
		})
	}
}
