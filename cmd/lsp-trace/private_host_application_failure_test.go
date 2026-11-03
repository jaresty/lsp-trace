//go:build darwin

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/manageddiagnostic"
	product "lsp-trace/internal/privatehostproductv3"
)

const applicationFailureSecret = "CONTROLLED_SECRET_NOT_DIAGNOSTIC_CONTENT"

// Controlled Go stdio child only: no external provider, model or network.
func TestApplicationOriginalFailureProvider(t *testing.T) {
	if os.Getenv("TEST_APPLICATION_FAILURE_PROVIDER") != "1" {
		t.Skip("controlled child only")
	}
	r := lspwire.NewReader(os.Stdin, lspwire.DefaultLimits())
	w := lspwire.NewWriter(os.Stdout, lspwire.DefaultLimits())
	prepared := 0
	for {
		m, e := r.Read()
		if e != nil {
			os.Exit(2)
		}
		if m.Method == "textDocument/prepareCallHierarchy" {
			prepared++
		}
		if os.Getenv("TEST_APPLICATION_FAILURE_MODE") != "capture" || prepared > 1 {
			fmt.Fprintln(os.Stderr, applicationFailureSecret)
			os.Exit(7)
		}
		if m.Method == "exit" {
			os.Exit(0)
		}
		if m.Method == "initialized" || strings.HasPrefix(m.Method, "textDocument/did") {
			continue
		}
		var result any = []any{}
		switch m.Method {
		case "initialize":
			result = map[string]any{"capabilities": map[string]any{"positionEncoding": "utf-16", "documentSymbolProvider": true, "callHierarchyProvider": true}}
		case "textDocument/documentSymbol":
			result = json.RawMessage(`[{"name":"Owned00","kind":12,"range":{"start":{"line":2,"character":0},"end":{"line":2,"character":17}},"selectionRange":{"start":{"line":2,"character":5},"end":{"line":2,"character":12}}}]`)
		case "textDocument/prepareCallHierarchy":
			var p lsp.PrepareCallHierarchyParams
			if json.Unmarshal(m.Params, &p) != nil {
				os.Exit(2)
			}
			result = []lsp.CallHierarchyItem{{Name: "Owned00", Kind: 12, URI: p.TextDocument.URI, Range: lsp.Range{Start: lsp.Position{Line: 2}, End: lsp.Position{Line: 2, Character: 17}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: 2, Character: 5}, End: lsp.Position{Line: 2, Character: 12}}}}
		case "shutdown":
			result = nil
		}
		raw, _ := json.Marshal(result)
		if w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Result: raw}) != nil {
			os.Exit(3)
		}
	}
}

func TestApplicationOriginalFailureDiagnostic(t *testing.T) {
	for _, kind := range []string{"startup", "readiness", "discovery", "capture", "pre-hook"} {
		t.Run(kind, func(t *testing.T) {
			r, path, base := hostApplicationCLIFixture(t)
			if kind == "startup" || kind == "pre-hook" {
				server := filepath.Join(base, "controlled-invalid-executable")
				raw := []byte("controlled invalid executable: " + applicationFailureSecret)
				mode := os.FileMode(0700)
				if kind == "pre-hook" {
					mode = 0400
				}
				if e := os.WriteFile(server, raw, mode); e != nil {
					t.Fatal(e)
				}
				for i, a := range r.Template.CensusArguments {
					if a == "--server" {
						r.Template.CensusArguments[i+1] = server
					}
				}
				r.Template.ServerSHA256 = pilot.Digest(raw)
				r.Configuration.ProducerExecutableSHA256 = pilot.Digest(raw)
			} else if kind != "discovery" {
				for i, a := range r.Template.CensusArguments {
					if a == "--server-arg" {
						r.Template.CensusArguments[i+1] = "-test.run=^TestApplicationOriginalFailureProvider$"
					}
				}
			}
			r = handoffBind(r)
			writeApplicationFixture(t, r, path)
			tool := buildApplicationTool(t, base)
			if applicationFixtureRace {
				info, e := exec.Command("go", "version", "-m", tool).CombinedOutput()
				if e != nil || !bytes.Contains(info, []byte("-race=true")) {
					t.Fatalf("ASSERT_APPLICATION_FAILURE_HELPER_RACE A-fail %v", e)
				}
				t.Log("ASSERT_APPLICATION_FAILURE_HELPER_RACE A-pass -race=true")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
			defer cancel()
			setup := exec.CommandContext(ctx, tool, "private-host-application", "setup", path)
			if b, e := setup.CombinedOutput(); e != nil {
				t.Fatalf("setup %v %s", e, b)
			}
			cmd := exec.CommandContext(ctx, tool, "private-host-application", "run", path)
			cmd.Env = append(os.Environ(), "TEST_APPLICATION_FAILURE_PROVIDER=1", "TEST_APPLICATION_FAILURE_MODE="+kind, "LSP_HOST_APPLICATION_OWNED_SERVER=1")
			if kind == "discovery" {
				cmd.Env = append(cmd.Env, "LSP_HOST_APPLICATION_FAIL_DOC=1")
			}
			var out, diag bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &diag
			runErr := cmd.Run()
			var terminal applicationTerminal
			if json.Unmarshal(out.Bytes(), &terminal) != nil || runErr == nil || terminal.Status != "FAILED_ORIGINAL_PROCESS_SPENT" || terminal.Units != 0 || terminal.Targets != 0 || terminal.Calls != 0 || terminal.InventoryHead != "" {
				t.Fatalf("spent prerequisite err=%v out=%s diag=%s", runErr, out.String(), diag.String())
			}
			a, e := product.OpenApplication(r.Configuration, time.Now())
			if e != nil {
				t.Fatal(e)
			}
			var state applicationState
			e = a.GetRecord(a.Head().CursorSHA256, &state)
			a.Close()
			if e != nil || state.Inflight == "" || state.Activation != 0 {
				t.Fatalf("spent state prerequisite %+v %v", state, e)
			}
			root := filepath.Join(r.Configuration.Store, "units", state.Inflight[7:], "auxiliary")
			failure, fe := os.ReadFile(filepath.Join(root, "acquisition-failure.json"))
			startup, se := os.ReadFile(filepath.Join(root, "original-startup.json"))
			if kind == "pre-hook" {
				if !os.IsNotExist(fe) || !os.IsNotExist(se) || !strings.Contains(diag.String(), "SafeDiagnosticStatus=UNAVAILABLE") || strings.Contains(diag.String(), applicationFailureSecret) {
					t.Fatalf("ASSERT_APPLICATION_FAILURE_UNAVAILABLE A-fail fe=%v se=%v diag=%s", fe, se, diag.String())
				}
				t.Log("ASSERT_APPLICATION_FAILURE_UNAVAILABLE A-pass SPENT nonzero zero activation")
				return
			}
			var f struct{ Phase, Class, StartupStatus, StartupSHA256 string }
			_ = json.Unmarshal(failure, &f)
			expectedPhase := "ORIGINAL_SESSION_START"
			if kind == "discovery" {
				expectedPhase = "ORIGINAL_CENSUS_DISCOVERY"
			}
			if kind == "capture" {
				expectedPhase = "ORIGINAL_CAPTURE_BATCH"
			}
			startupOK := f.StartupStatus == "UNAVAILABLE" && strings.Contains(diag.String(), "StartupStatus=UNAVAILABLE")
			if f.StartupStatus == "AVAILABLE" {
				startupOK = se == nil && manageddiagnostic.ValidateStartupDiagnostics(startup) == nil && f.StartupSHA256 == pilot.Digest(startup)
			}
			if fe != nil || len(failure) > 4096 || !startupOK || ((kind == "startup" || kind == "readiness") && f.StartupStatus != "AVAILABLE") || f.Phase != expectedPhase || !applicationFailureClass(f.Class) || ((kind == "startup" || kind == "readiness") && f.Class != "UNKNOWN") || !strings.Contains(diag.String(), "SafeDiagnosticStatus=AVAILABLE") {
				t.Fatalf("ASSERT_APPLICATION_ORIGINAL_FAILURE_DIAGNOSTIC A-fail kind=%s failure=%s fe=%v se=%v diag=%s", kind, failure, fe, se, diag.String())
			}
			if bytes.Contains(append(append(failure, startup...), diag.Bytes()...), []byte(applicationFailureSecret)) {
				t.Fatal("ASSERT_APPLICATION_FAILURE_PRIVACY A-fail")
			}
			var start struct {
				Attempt struct{ Outcome, Reason string }   `json:"attempt"`
				Records []struct{ Phase, Terminal string } `json:"records"`
			}
			if f.StartupStatus == "AVAILABLE" && json.Unmarshal(startup, &start) != nil {
				t.Fatal("startup decode")
			}
			if kind == "startup" && (start.Attempt.Outcome != "failed" || start.Attempt.Reason != "process-start-failed") {
				t.Fatalf("typed startup classification %+v", start)
			}
			if kind == "readiness" {
				if start.Attempt.Outcome != "admitted" || !strings.Contains(diag.String(), "StartupOutcome=STARTUP_ADMITTED") {
					t.Fatalf("qualified readiness classification %+v diag=%s", start, diag.String())
				}
				if len(start.Records) == 0 && (!strings.Contains(diag.String(), "StartupRecordsStatus=UNAVAILABLE") || !strings.Contains(diag.String(), "StartupPhase=UNKNOWN")) {
					t.Fatalf("omitted detail must remain unknown: %s", diag.String())
				}
			}
			// Existing manager failure-record validation can omit readiness detail.
			// This guard requires an explicit UNKNOWN, never an inferred timeout.
			// A runtime record-validation repair is outside this five-file wiring fix.
			t.Logf("ASSERT_APPLICATION_ORIGINAL_FAILURE_DIAGNOSTIC A-pass kind=%s outcome=%s reason=%s records=%+v", kind, start.Attempt.Outcome, start.Attempt.Reason, start.Records)
			t.Log("ASSERT_APPLICATION_FAILURE_PRIVACY A-pass")
		})
	}
}

func applicationFailureFixture(t *testing.T) (hostProductRequestV3, string, os.FileInfo) {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(root, 0700); e != nil {
		t.Fatal(e)
	}
	aux := filepath.Join(root, "auxiliary")
	if e = os.Mkdir(aux, 0700); e != nil {
		t.Fatal(e)
	}
	d := pilot.Digest([]byte("controlled failure binding"))
	r := hostProductRequestV3{AuxiliaryRoot: aux, Configuration: product.Config{Application: d, ScopeSHA256: d, ProducerConfigSHA256: d}}
	st, e := os.Lstat(root)
	if e != nil {
		t.Fatal(e)
	}
	return r, root, st
}
func writeApplicationFailureFixture(t *testing.T, r hostProductRequestV3, phase string, cause error) []byte {
	t.Helper()
	st, e := os.Lstat(r.AuxiliaryRoot)
	if e != nil {
		t.Fatal(e)
	}
	code := 1
	if e = publishApplicationFailure(context.Background(), r, st, newApplicationFailure(r), phase, cause, &code, 1, 1, true, false); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(r.AuxiliaryRoot, applicationFailureSelector))
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestApplicationFailureClosedReceipt(t *testing.T) {
	for _, phase := range []string{"ORIGINAL_SESSION_START", "ORIGINAL_SESSION_LEASE", "ORIGINAL_CENSUS_DISCOVERY", "ORIGINAL_DISCOVERY_BINDING", "ORIGINAL_CAPTURE_BATCH", "ORIGINAL_PAGED_OWNER", "ORIGINAL_PAGES", "ORIGINAL_CHECKPOINT_EXPORT", "ORIGINAL_TRANSFER"} {
		t.Run(phase, func(t *testing.T) {
			r, root, st := applicationFailureFixture(t)
			b := writeApplicationFailureFixture(t, r, phase, errors.New(applicationFailureSecret))
			p := loadApplicationFailure(root, st, r)
			if p.Status != "AVAILABLE" || p.Phase != phase || p.Class != "UNKNOWN" || p.StartupStatus != "UNAVAILABLE" || bytes.Contains(b, []byte(applicationFailureSecret)) {
				t.Fatalf("ASSERT_APPLICATION_FAILURE_CLOSED A-fail %+v", p)
			}
			t.Log("ASSERT_APPLICATION_FAILURE_CLOSED A-pass")
		})
	}
}
func TestApplicationFailureRejectsUnsafe(t *testing.T) {
	for _, kind := range []string{"missing", "foreign-request", "foreign-product", "foreign-scope", "foreign-producer", "unknown", "duplicate", "oversize", "wrong-mode", "symlink", "hardlink", "aux-symlink", "root-replaced", "aux-mode", "startup-foreign", "startup-duplicate", "startup-oversize"} {
		t.Run(kind, func(t *testing.T) {
			r, root, st := applicationFailureFixture(t)
			b := writeApplicationFailureFixture(t, r, "ORIGINAL_CAPTURE_BATCH", nil)
			path := filepath.Join(r.AuxiliaryRoot, applicationFailureSelector)
			var f applicationFailure
			_ = json.Unmarshal(b, &f)
			switch kind {
			case "missing":
				_ = os.Remove(path)
			case "foreign-request":
				f.RequestSHA256 = pilot.Digest([]byte("foreign"))
			case "foreign-product":
				f.ProductApplication = pilot.Digest([]byte("foreign"))
			case "foreign-scope":
				f.ScopeSHA256 = pilot.Digest([]byte("foreign"))
			case "foreign-producer":
				f.ProducerConfigSHA256 = pilot.Digest([]byte("foreign"))
			case "unknown":
				f.Phase = applicationFailureSecret
			case "duplicate":
				b = append([]byte(`{"Phase":"ORIGINAL_CAPTURE_BATCH",`), b[1:]...)
			case "oversize":
				b = bytes.Repeat([]byte("x"), 4097)
			case "wrong-mode":
				_ = os.Chmod(path, 0644)
			case "symlink":
				_ = os.Rename(path, path+".other")
				_ = os.Symlink(path+".other", path)
			case "hardlink":
				_ = os.Link(path, path+".other")
			case "aux-symlink":
				_ = os.Rename(r.AuxiliaryRoot, r.AuxiliaryRoot+".other")
				_ = os.Symlink(r.AuxiliaryRoot+".other", r.AuxiliaryRoot)
			case "root-replaced":
				_ = os.Rename(root, root+".other")
				t.Cleanup(func() { os.RemoveAll(root + ".other") })
				_ = os.Mkdir(root, 0700)
				_ = os.Mkdir(r.AuxiliaryRoot, 0700)
				_ = os.WriteFile(path, b, 0600)
			case "aux-mode":
				_ = os.Chmod(r.AuxiliaryRoot, 0755)
			case "startup-foreign", "startup-duplicate", "startup-oversize":
				f.StartupStatus = "AVAILABLE"
				f.StartupAttemptID = strings.Repeat("a", 64)
				f.StartupSHA256 = pilot.Digest([]byte("foreign"))
				raw := []byte(`{}`)
				if kind == "startup-duplicate" {
					raw = []byte(`{"attempt":{},"attempt":{}}`)
					f.StartupSHA256 = pilot.Digest(raw)
				}
				if kind == "startup-oversize" {
					raw = bytes.Repeat([]byte(" "), 65537)
					f.StartupSHA256 = pilot.Digest(raw)
				}
				_ = os.WriteFile(filepath.Join(r.AuxiliaryRoot, applicationStartupSelector), raw, 0600)
			}
			switch kind {
			case "foreign-request", "foreign-product", "foreign-scope", "foreign-producer", "unknown", "startup-foreign", "startup-duplicate", "startup-oversize":
				b, _ = json.Marshal(f)
			}
			switch kind {
			case "foreign-request", "foreign-product", "foreign-scope", "foreign-producer", "unknown", "duplicate", "oversize", "startup-foreign", "startup-duplicate", "startup-oversize":
				if e := os.WriteFile(path, b, 0600); e != nil {
					t.Fatal(e)
				}
			}
			p := loadApplicationFailure(root, st, r)
			if strings.HasPrefix(kind, "startup-") {
				if p.Status != "AVAILABLE" || p.StartupStatus != "UNAVAILABLE" || p.StartupOutcome != "UNKNOWN" {
					t.Fatalf("ASSERT_APPLICATION_FAILURE_REJECTION A-fail %+v", p)
				}
			} else if p.Status != "UNAVAILABLE" {
				t.Fatalf("ASSERT_APPLICATION_FAILURE_REJECTION A-fail kind=%s %+v", kind, p)
			}
			var out bytes.Buffer
			reportApplicationFailure(&out, root, st, r)
			if bytes.Contains(out.Bytes(), []byte(applicationFailureSecret)) {
				t.Fatal("unsafe output")
			}
			t.Log("ASSERT_APPLICATION_FAILURE_REJECTION A-pass")
		})
	}
}
func TestApplicationFailureCancellationAndFirstPublication(t *testing.T) {
	for _, expected := range []string{"ACTIVE", "CANCELLED", "DEADLINE_EXCEEDED"} {
		t.Run(expected, func(t *testing.T) {
			r, root, identity := applicationFailureFixture(t)
			st, _ := os.Lstat(r.AuxiliaryRoot)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if expected == "CANCELLED" {
				cancel()
			}
			if expected == "DEADLINE_EXCEEDED" {
				expired, done := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				defer done()
				ctx = expired
			}
			code := 1
			if e := publishApplicationFailure(ctx, r, st, newApplicationFailure(r), "ORIGINAL_SESSION_LEASE", nil, &code, 1, 1, false, false); e != nil {
				t.Fatal(e)
			}
			before, e := os.ReadFile(filepath.Join(r.AuxiliaryRoot, applicationFailureSelector))
			if e != nil {
				t.Fatal(e)
			}
			if e = publishApplicationFailure(ctx, r, st, newApplicationFailure(r), "ORIGINAL_CAPTURE_BATCH", context.Canceled, &code, 1, 1, true, false); e == nil {
				t.Fatal("ASSERT_APPLICATION_FAILURE_IMMUTABLE A-fail overwrite")
			}
			after, _ := os.ReadFile(filepath.Join(r.AuxiliaryRoot, applicationFailureSelector))
			p := loadApplicationFailure(root, identity, r)
			if !bytes.Equal(before, after) || p.ContextStatus != expected || p.Phase != "ORIGINAL_SESSION_LEASE" || p.Class != "UNKNOWN" {
				t.Fatalf("ASSERT_APPLICATION_FAILURE_IMMUTABLE A-fail %+v", p)
			}
			t.Log("ASSERT_APPLICATION_FAILURE_IMMUTABLE A-pass")
		})
	}
	r, root, _ := applicationFailureFixture(t)
	st, _ := os.Lstat(r.AuxiliaryRoot)
	_ = os.Remove(r.AuxiliaryRoot)
	if publishApplicationFailure(context.Background(), r, st, newApplicationFailure(r), "ORIGINAL_SESSION_START", nil, nil, 0, 0, false, false) == nil {
		t.Fatal("unowned publication accepted")
	}
	if _, e := os.Stat(filepath.Join(root, "auxiliary")); !os.IsNotExist(e) {
		t.Fatal("unowned root created")
	}
}
func TestApplicationFailureConcurrentCreateOnly(t *testing.T) {
	r, _, _ := applicationFailureFixture(t)
	st, _ := os.Lstat(r.AuxiliaryRoot)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code := 1
			results <- publishApplicationFailure(context.Background(), r, st, newApplicationFailure(r), "ORIGINAL_CAPTURE_BATCH", nil, &code, 1, 1, true, false)
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		}
	}
	raw, e := readApplicationFailureFile(r.AuxiliaryRoot, applicationFailureSelector, 4096)
	if success != 1 || e != nil || validateApplicationFailure(raw) != nil {
		t.Fatalf("ASSERT_APPLICATION_FAILURE_CREATEONLY A-fail success=%d err=%v", success, e)
	}
	t.Log("ASSERT_APPLICATION_FAILURE_CREATEONLY A-pass")
}

func TestApplicationFailureStartupAttemptBinding(t *testing.T) {
	for _, kind := range []string{"own", "foreign-attempt", "publication-mode", "replaced-auxiliary", "duplicate-json"} {
		t.Run(kind, func(t *testing.T) {
			r, root, identity := applicationFailureFixture(t)
			auxIdentity, _ := os.Lstat(r.AuxiliaryRoot)
			attempt := strings.Repeat("a", 64)
			record := manageddiagnostic.StartupAttemptRecord{SchemaVersion: manageddiagnostic.StartupAttemptSchemaVersion, AttemptID: manageddiagnostic.StartupAttemptID(attempt), Sequence: 1, Outcome: manageddiagnostic.StartupFailed, Reason: manageddiagnostic.Fact[string]{Status: manageddiagnostic.Observed, Value: "process-start-failed"}, Stderr: manageddiagnostic.Stderr{Status: manageddiagnostic.Withheld}, ProcessExit: manageddiagnostic.ProcessExit{Status: manageddiagnostic.Unavailable}}
			if _, e := (manageddiagnostic.StartupDiagnosticSink{Root: r.AuxiliaryRoot, Selector: applicationStartupSelector}).Finalize(manageddiagnostic.StartupAttemptQuery{Status: manageddiagnostic.AttemptAvailable, Record: &record}, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable}); e != nil {
				t.Fatal(e)
			}
			f := newApplicationFailure(r)
			f.StartupAttemptID = attempt
			if kind == "foreign-attempt" {
				f.StartupAttemptID = strings.Repeat("b", 64)
			}
			if kind == "duplicate-json" {
				p := filepath.Join(r.AuxiliaryRoot, applicationStartupSelector)
				raw, _ := os.ReadFile(p)
				raw = append([]byte(`{"attempt":{},`), raw[1:]...)
				if e := os.WriteFile(p, raw, 0600); e != nil {
					t.Fatal(e)
				}
			}
			if kind == "publication-mode" {
				_ = os.Chmod(r.AuxiliaryRoot, 0500)
				t.Cleanup(func() { os.Chmod(r.AuxiliaryRoot, 0700) })
			}
			if kind == "replaced-auxiliary" {
				_ = os.Rename(r.AuxiliaryRoot, r.AuxiliaryRoot+".old")
				_ = os.Mkdir(r.AuxiliaryRoot, 0700)
			}
			code := 1
			e := publishApplicationFailure(context.Background(), r, auxIdentity, f, "ORIGINAL_SESSION_START", nil, &code, 0, 0, false, true)
			p := loadApplicationFailure(root, identity, r)
			if kind == "publication-mode" || kind == "replaced-auxiliary" {
				if e == nil || p.Status != "UNAVAILABLE" {
					t.Fatalf("ASSERT_APPLICATION_FAILURE_STARTUP_BINDING A-fail %+v err=%v", p, e)
				}
			} else {
				want := "UNAVAILABLE"
				if kind == "own" {
					want = "AVAILABLE"
				}
				if e != nil || p.Status != "AVAILABLE" || p.StartupStatus != want {
					t.Fatalf("ASSERT_APPLICATION_FAILURE_STARTUP_BINDING A-fail %+v err=%v", p, e)
				}
			}
			t.Log("ASSERT_APPLICATION_FAILURE_STARTUP_BINDING A-pass")
		})
	}
}
