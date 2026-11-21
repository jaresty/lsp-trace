//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	ai "lsp-trace/internal/applicationinventory"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/scopejob"
	"lsp-trace/sessionruntime"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type languageSupplySpy struct {
	requests []sessionruntime.DocumentRequest
}

func (*languageSupplySpy) SessionID() string  { return "spy" }
func (*languageSupplySpy) Generation() uint64 { return 1 }
func (s *languageSupplySpy) PrepareDocument(_ context.Context, r sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
	s.requests = append(s.requests, r)
	return sessionruntime.DocumentResult{URI: r.URI, LanguageID: r.LanguageID}
}

func TestSourcePolicySupplierExplicitLanguage(t *testing.T) {
	spy := &languageSupplySpy{}
	supplier := censusRuntimeDocumentSupplier{runtime: spy}
	for _, language := range []string{"typescript", "typescriptreact", "custom.lang"} {
		if e := supplier.Supply(context.Background(), censusacquisition.SourceFile{URI: "file:///fixture", LanguageID: language}); e != nil {
			t.Fatal(e)
		}
	}
	for i, want := range []string{"typescript", "typescriptreact", "custom.lang"} {
		if spy.requests[i].LanguageID != want || !spy.requests[i].CaptureSupply {
			t.Fatal(spy.requests)
		}
	}
}
func TestSourcePolicyProfileDeclarations(t *testing.T) {
	ids := []string{"typescriptreact", "typescript"}
	r, e := resolveProfileEnvironment("ts", serverProfile{LanguageIDs: &ids})
	if e != nil || r.LanguageID != "typescriptreact" || !reflect.DeepEqual(r.LanguageIDs, []string{"typescript", "typescriptreact"}) {
		t.Fatal(r, e)
	}
	ids[0] = "changed"
	if r.LanguageID != "typescriptreact" {
		t.Fatal(r)
	}
	w, _ := withProfileDiscovery(t)
	for _, body := range []string{`[profiles.ts]
command="unused"`, `[profiles.ts]
language_ids=[]`, `[profiles.ts]
language_ids=["go","go"]`, `[profiles.ts]
language_ids=["go"]
[file_associations]
""="go"`, `[profiles.ts]
language_ids=["go"]
[file_associations]
"*.go"="go"
"*.go"="typescript"`, `[profiles.ts]
language_ids=["go"]
unknown=true`} {
		writeProfileConfig(t, filepath.Join(w, ".lsp-trace.toml"), body)
		if _, _, e := freezeCensusSelection(w, "ts"); e == nil {
			t.Fatalf("accepted invalid declaration %s", body)
		}
	}
	writeProfileConfig(t, filepath.Join(w, ".lsp-trace.toml"), `[profiles.ts]
language_ids=["typescriptreact","typescript"]
env=["MISSING_PROVIDER_SECRET"]
[file_associations]
"*.go"="typescript"
"*.mts"="typescript"`)
	name, p, e := freezeCensusSelection(w, "ts")
	if e != nil || name != "ts" || p.DefaultLanguage != "typescriptreact" {
		t.Fatal(name, p, e)
	}
	if got, e := p.Resolve("x.go"); e != nil || got != "typescript" {
		t.Fatal(got, e)
	}
	if p.Version != scopejob.SelectionVersion {
		t.Fatal(p)
	}
}

func TestSourcePolicyProjectOnlyAndConfigBound(t *testing.T) {
	w, user := withProfileDiscovery(t)
	writeProfileConfig(t, user, `[profiles.ts]
language_ids=["typescript"]
[file_associations]
"*.mts"="typescript"`)
	_, p, e := freezeCensusSelection(w, "ts")
	if e != nil {
		t.Fatal(e)
	}
	if got, e := p.Resolve("a.mts"); e != nil || got != "" {
		t.Fatal("user associations leaked", got, e)
	}
	writeProfileConfig(t, filepath.Join(w, ".lsp-trace.toml"), strings.Repeat("#", 32769))
	if _, _, e = freezeCensusSelection(w, "ts"); e == nil {
		t.Fatal("unbounded config accepted")
	}
}

func TestSourcePolicyUnknownEligibilityDiagnostic(t *testing.T) {
	w, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	writeProfileConfig(t, filepath.Join(w, ".lsp-trace.toml"), "[profiles.ts]\ncommand=\"unused\"\n")
	store := filepath.Join(t.TempDir(), "must-not-exist")
	var out, diag bytes.Buffer
	if code := runCensusWithDependencies([]string{w, "--scope-job-candidate", "--profile", "ts", "--publication-root", store}, &out, &diag, censusRunnerDependencies{}); code == 0 || !strings.Contains(diag.String(), "DECLARED_ELIGIBILITY_UNKNOWN") {
		t.Fatal(code, diag.String())
	}
	if _, e = os.Stat(store); !os.IsNotExist(e) {
		t.Fatal("store effect before eligibility admission", e)
	}
}

func TestSourcePolicyCandidateTypeScript(t *testing.T) {
	w, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(store, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	writeProfileConfig(t, filepath.Join(w, ".lsp-trace.toml"), `[profiles.ts]
command="never-start"
language_ids=["typescript", "typescriptreact"]
[file_associations]
"*.mts"="typescript"
`)
	for _, p := range []string{"a.ts", "nested/b.tsx", "c.go", "custom.mts", "data.json", "skip.ts", "nested/generated_test.ts"} {
		writeCensusSource(t, w, p, "fixture")
	}
	writeCensusSource(t, w, ".gitignore", "*.ts\n*.tsx\n*.mts\n")
	var out, diag bytes.Buffer
	code := runCensusWithDependencies([]string{w, "--scope-job-candidate", "--profile", "ts", "--exclude", "skip.ts", "--publication-root", store}, &out, &diag, censusRunnerDependencies{})
	if code != 0 {
		t.Fatalf("ASSERT_TS_DECLARED_ELIGIBILITY: code=%d diagnostic=%s", code, &diag)
	}
	var got ai.InventoryJobStatus
	if err = json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Enumeration == nil || !got.Enumeration.Closed || got.Enumeration.Counts.Selected != 4 || got.Enumeration.Counts.UnsupportedLanguage != 1 || got.Enumeration.Counts.Excluded != 1 || got.Enumeration.Counts.NotSource != 3 {
		t.Fatalf("ASSERT_TS_DECLARED_ELIGIBILITY: %+v", got.Enumeration)
	}
}
