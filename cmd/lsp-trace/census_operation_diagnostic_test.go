//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/publication"
)

func TestCensusOperationConfigReasons(t *testing.T) {
	t.Setenv("LSP_TRACE_CALLER_FD", "")
	t.Setenv(censusDurableStoreEnv, "")
	base, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	private := filepath.Join(base, "private")
	if e = os.Mkdir(private, 0700); e != nil {
		t.Fatal(e)
	}
	alias := filepath.Join(base, "SECRET_MARKER-alias")
	if e = os.Symlink(private, alias); e != nil {
		t.Fatal(e)
	}
	dangling := filepath.Join(base, "SECRET_MARKER-dangling")
	if e = os.Symlink(filepath.Join(base, "absent"), dangling); e != nil {
		t.Fatal(e)
	}
	public := filepath.Join(base, "public")
	if e = os.Mkdir(public, 0755); e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(public, 0755); e != nil {
		t.Fatal(e)
	}
	regular := filepath.Join(base, "regular")
	if e = os.WriteFile(regular, []byte("x"), 0600); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name, path, workspace string
		reason                censusresult.OperationReason
	}{
		{"missing", filepath.Join(base, "SECRET_MARKER-missing"), "", censusresult.StoreRootMissing},
		{"alias", alias, "", censusresult.StoreAliasRefused},
		{"dangling", dangling, "", censusresult.StoreRootMissing},
		{"permissions", public, "", censusresult.StorePermissions},
		{"overlap", private, base, censusresult.StoreSourceOverlap},
		{"relative", "SECRET_MARKER-relative", "", censusresult.StorePathInvalid},
		{"regular", regular, "", censusresult.StorePathInvalid},
		{"io", filepath.Join(regular, "child"), "", censusresult.StoreIO},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _, err := openSelectedCensusStore(tc.workspace, tc.path)
			if root != nil {
				root.Close()
			}
			if err == nil {
				t.Fatal("unsafe config accepted")
			}
			if got := censusConfigReason(err); got != tc.reason {
				t.Fatalf("reason=%s want=%s", got, tc.reason)
			}
			var diagnostic bytes.Buffer
			writeCensusOperationFailure(&diagnostic, "start", censusConfigReason(err), "")
			d := checkOperationBytes(t, diagnostic.Bytes())
			if d.Retry || d.Action != "CORRECT_CONFIG" || d.Recovery != nil {
				t.Fatalf("unsafe recovery: %+v", d)
			}
			workspace := tc.workspace
			if workspace == "" {
				workspace, e = filepath.EvalSymlinks(t.TempDir())
				if e != nil {
					t.Fatal(e)
				}
			}
			calls := 0
			deps := productionCensusRunnerDependencies()
			deps.lookPath = func(string) (string, error) { calls++; return "", errors.New("provider must not be reached") }
			diagnostic.Reset()
			var out bytes.Buffer
			code := runCensusWithDependencies([]string{workspace, "--server", "gopls", "--publication-root", tc.path}, &out, &diagnostic, deps)
			got := checkOperationBytes(t, diagnostic.Bytes())
			if code != 1 || calls != 0 || out.Len() != 0 || got.Reason != tc.reason {
				t.Fatalf("CLI preflight calls=%d code=%d diagnostic=%+v", calls, code, got)
			}
		})
	}
}
func TestCensusOperationActualCLIConfigPreflight(t *testing.T) {
	t.Setenv("LSP_TRACE_CALLER_FD", "")
	t.Setenv(censusDurableStoreEnv, "")
	workspace, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	missing := filepath.Join(workspace, "SECRET_MARKER-missing")
	calls := 0
	deps := productionCensusRunnerDependencies()
	deps.lookPath = func(string) (string, error) { calls++; return "", errors.New("unexpected provider") }
	var out, diagnostic bytes.Buffer
	if runCensusWithDependencies([]string{workspace, "--server", "gopls", "--publication-root", missing}, &out, &diagnostic, deps) != 1 {
		t.Fatal("expected refusal")
	}
	d := checkOperationBytes(t, diagnostic.Bytes())
	if calls != 0 || out.Len() != 0 || d.Reason != censusresult.StoreRootMissing {
		t.Fatalf("preflight: calls=%d diagnostic=%+v", calls, d)
	}
	diagnostic.Reset()
	if runCensusDurable([]string{"submit", strings.Repeat("a", 64), strings.Repeat("b", 64), "interpreter", "--publication-root", missing}, strings.NewReader("SECRET_MARKER"), io.Discard, &diagnostic) != 1 {
		t.Fatal("expected refusal")
	}
	checkOperationBytes(t, diagnostic.Bytes())
	diagnostic.Reset()
	if runCensusDurable([]string{"--publication-root", missing, "task", strings.Repeat("a", 64)}, nil, io.Discard, &diagnostic) != 1 {
		t.Fatal("expected refusal")
	}
	if got := checkOperationBytes(t, diagnostic.Bytes()); got.Operation != "task" || got.Action != "CORRECT_CONFIG" {
		t.Fatalf("leading option lost operation: %+v", got)
	}
}
func TestCensusOperationPrivacyAttribution(t *testing.T) {
	p, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	root, e := publication.OpenRoot(p)
	if e != nil {
		t.Fatal(e)
	}
	if e = root.Close(); e != nil {
		t.Fatal(e)
	}
	if e = root.ValidatePrivate(); e == nil || errors.Is(e, publication.ErrRootPrivacy) || errors.Is(e, publication.ErrRootOwner) {
		t.Fatalf("closed root misclassified: %v", e)
	}
	if censusConfigReason(censusStoreIOError(e)) == censusresult.StorePermissions {
		t.Fatal("closed root classified permissions")
	}
}
func TestCensusOperationSchemaParity(t *testing.T) {
	s := operationSchema(t)
	for _, op := range []string{"start", "task", "read", "submit", "result", "unknown", "bad"} {
		for _, reason := range []censusresult.OperationReason{censusresult.StoreRootMissing, censusresult.ConfigUnclassified, censusresult.InvalidOperation, censusresult.RunUnavailable, censusresult.OperationUnconfirmed, "bad"} {
			for _, action := range []string{"CORRECT_CONFIG", "OBSERVE_CURRENT_TASK", "STOP", "bad"} {
				for _, retry := range []bool{false, true} {
					for _, recovery := range []*censusresult.OperationRecovery{nil, {RunID: strings.Repeat("a", 64), StoreContext: "SAME_SELECTION"}} {
						d := censusresult.OperationDiagnostic{SchemaVersion: censusresult.OperationDiagnosticSchemaVersion, Operation: op, Reason: reason, Action: action, Retry: retry, Recovery: recovery}
						raw, e := json.Marshal(d)
						if e != nil {
							t.Fatal(e)
						}
						v, e := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
						if e != nil {
							t.Fatal(e)
						}
						schemaOK := s.Validate(v) == nil
						_, e = censusresult.DecodeOperationDiagnostic(raw)
						if schemaOK != (e == nil) {
							t.Fatalf("schema/codec mismatch: %s", raw)
						}
					}
				}
			}
		}
	}
}
