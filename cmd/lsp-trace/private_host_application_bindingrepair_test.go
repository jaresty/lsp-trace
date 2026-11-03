//go:build darwin

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

var bindingBarrier, bindingRelease chan struct{}
var bindingSyncFailure, bindingRemoveFailure bool
var bindingMoveRoot, bindingReplaceRoot bool
var bindingSubstitute string
var bindingRemoveCalls int

func bindingCrash(point string) {
	if os.Getenv("LSP_BINDING_CRASH") == point {
		os.Exit(23)
	}
}
func bindingBeforeWrite() { bindingCrash("stage") }
func bindingBeforeRename(dir *os.File, stage, name string) {
	bindingCrash("rename")
	if bindingSubstitute == "stage" {
		if e := os.Rename(filepath.Join(dir.Name(), stage), filepath.Join(dir.Name(), stage)+".displaced"); e != nil {
			panic(e)
		}
		if e := os.WriteFile(filepath.Join(dir.Name(), stage), []byte("FOREIGN_FINAL_BYTE_EXACT"), 0600); e != nil {
			panic(e)
		}
	}
}
func bindingDirSync(dir *os.File, path string) error {
	if strings.HasPrefix(filepath.Base(path), "terminal-") {
		if _, e := os.Lstat(path); e == nil {
			bindingCrash("sync")
			if bindingBarrier != nil {
				close(bindingBarrier)
				<-bindingRelease
			}
			if bindingSubstitute == "final" {
				if e := os.Rename(path, path+".displaced"); e != nil {
					return e
				}
				if e := os.WriteFile(path, []byte("FOREIGN_FINAL_BYTE_EXACT"), 0600); e != nil {
					return e
				}
			}
			if bindingSyncFailure {
				return errors.New("controlled namespace Sync failure")
			}
		}
	}
	return dir.Sync()
}
func bindingRemove(root *os.Root, name string) error {
	bindingRemoveCalls++
	if bindingRemoveFailure {
		return errors.New("controlled unlink failure")
	}
	return root.Remove(name)
}
func bindingRootAcquired(path string) {
	if bindingMoveRoot {
		bindingMoveRoot = false
		if e := os.Rename(path, path+"-moved"); e != nil {
			panic(e)
		}
		if bindingReplaceRoot {
			if e := os.MkdirAll(filepath.Join(path, "a"), 0700); e != nil {
				panic(e)
			}
			if e := os.WriteFile(filepath.Join(path, "a/x.go"), []byte("package owned\n"), 0600); e != nil {
				panic(e)
			}
		}
	}
}

// Overlay observations intercept actual production operations, not alternate readers
// or publishers. Each subtest reaches its named semantic assertion on baseline.
func TestHostApplicationBindingRepair(t *testing.T) {
	if os.Getenv("LSP_BINDING_OVERLAY") == "1" {
		bindingRepairAssertions(t)
		return
	}
	replacements := map[string]string{}
	d := t.TempDir()
	for _, name := range []string{"private_host_application_host.go", "private_host_application_literal.go"} {
		raw, e := os.ReadFile(name)
		if e != nil {
			t.Fatal(e)
		}
		s := string(raw)
		if strings.Contains(name, "host.go") {
			needle := "if e = dir.Sync(); e != nil {"
			if strings.Count(s, needle) != 1 {
				t.Fatal("post-rename Sync seam", name)
			}
			s = strings.Replace(s, needle, "if e = bindingDirSync(dir, path); e != nil {", 1)
			s = strings.ReplaceAll(s, "root.Remove(", "bindingRemove(root, ")
			s = strings.Replace(s, "n, e := f.Write(raw)", "bindingBeforeWrite()\n\tn, e := f.Write(raw)", 1)
			s = strings.Replace(s, "if e = applicationPublishNoReplace(dir, stage, name);", "bindingBeforeRename(dir, stage, name)\n\tif e = applicationPublishNoReplace(dir, stage, name);", 1)
			s = strings.Replace(s, "st, e := guard.Stat()", "bindingCrash(\"commit\")\n\tst, e := guard.Stat()", 1)
		} else {
			needle := "check := func(p, identity string) error {"
			if strings.Count(s, needle) != 1 {
				t.Fatal("root-acquired seam")
			}
			s = strings.Replace(s, needle, "bindingRootAcquired(r.Configuration.SourceRoot)\n\t\t"+needle, 1)
		}
		original, _ := filepath.Abs(name)
		replacement := filepath.Join(d, name)
		if e = os.WriteFile(replacement, []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
		replacements[original] = replacement
	}
	raw, _ := json.Marshal(map[string]any{"Replace": replacements})
	overlay := filepath.Join(d, "overlay.json")
	if e := os.WriteFile(overlay, raw, 0600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command("go", "test", "-race", "-overlay", overlay, ".", "-run", "^TestHostApplicationBindingRepair$", "-count=1", "-v", "-timeout=180s")
	cmd.Env = append(os.Environ(), "LSP_BINDING_OVERLAY=1")
	out, e := cmd.CombinedOutput()
	t.Log(string(out))
	if e != nil {
		t.Fatal("binding assertion suite", e)
	}
}

func bindingFixture(t *testing.T) (hostApplicationRequest, string, string, []byte, []byte) {
	t.Helper()
	r, p, _ := hostApplicationCLIFixture(t)
	var out, diag bytes.Buffer
	if runPrivateHostApplication([]string{"setup", p}, strings.NewReader(""), &out, &diag) != 0 {
		t.Fatal(diag.String())
	}
	var setup struct{ RunID string }
	if json.Unmarshal(out.Bytes(), &setup) != nil || setup.RunID == "" {
		t.Fatal("setup")
	}
	terminal, _ := json.Marshal(applicationTerminal{Schema: "private-host-application-terminal.v1", Status: "CONTROLLED_BINDING_é"})
	var returns [2]product.CohortRoleReturn
	for n, role := range []string{"interpreter", "reviewer"} {
		out.Reset()
		diag.Reset()
		if runPrivateHostApplication([]string{"register", p, setup.RunID, role, role, r.Configuration.RolePromptSHA256[n]}, strings.NewReader(""), &out, &diag) != 0 {
			t.Fatal(diag.String())
		}
		returns[n] = product.CohortRoleReturn{Role: role, LaunchID: role, TerminalSHA256: pilot.Digest(terminal), Status: "RETURNED", ReturnSHA256: pilot.Digest([]byte("controlled return")), Kind: "SCRIPTED_CONTRACT"}
	}
	raw, _ := json.Marshal(returns)
	return r, p, setup.RunID, terminal, raw
}
func TestHostApplicationBindingLocatorName(t *testing.T) {
	r, p, id, terminal, returns := bindingFixture(t)
	if e := applicationWriteOnce(applicationTerminalPath(r.Configuration.Store, id), terminal); e != nil {
		t.Fatal(e)
	}
	alias := filepath.Join(filepath.Dir(p), "source.json")
	if e := os.Rename(p, alias); e != nil {
		t.Fatal(e)
	}
	code, out, diag := bindingFinalize(alias, id, returns)
	if code != 0 || !strings.Contains(out, "SCRIPTED_CONTRACT") {
		t.Fatalf("ASSERT_BINDING_NATIVE_LOCATOR_NAME A-fail %d %s %s", code, out, diag)
	}
	t.Log("ASSERT_BINDING_NATIVE_LOCATOR_NAME A-pass")
}
func TestHostApplicationBindingUnmarked(t *testing.T) {
	r, p, id, terminal, returns := bindingFixture(t)
	if e := os.WriteFile(applicationTerminalPath(r.Configuration.Store, id), terminal, 0600); e != nil {
		t.Fatal(e)
	}
	var out, diag bytes.Buffer
	code := runPrivateHostApplication([]string{"wait", p, id, "interpreter", "0", "1"}, strings.NewReader(""), &out, &diag)
	fc, fo, fd := bindingFinalize(p, id, returns)
	if strings.Contains(out.String(), `"Event":"TERMINAL"`) || fc == 0 {
		t.Fatalf("ASSERT_BINDING_UNKNOWN_UNADMITTED A-fail wait=%d %s %s finalize=%d %s %s", code, out.String(), diag.String(), fc, fo, fd)
	}
	t.Log("ASSERT_BINDING_UNKNOWN_UNADMITTED A-pass")
}
func TestHostApplicationBindingRootAlias(t *testing.T) {
	r, p, _ := hostApplicationCLIFixture(t)
	old := r.Configuration.SourceRoot
	alias := filepath.Join(filepath.Dir(old), strings.ToUpper(filepath.Base(old)))
	if alias == old {
		alias = filepath.Join(filepath.Dir(old), strings.ToLower(filepath.Base(old)))
	}
	a, e := os.Stat(old)
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.Stat(alias)
	if e != nil || alias == old || !os.SameFile(a, b) {
		t.Skip("owned filesystem has no case alias")
	}
	f := filepath.Join(old, "a/x.go")
	if e = os.MkdirAll(filepath.Dir(f), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(f, []byte("package owned\n"), 0600); e != nil {
		t.Fatal(e)
	}
	r.Configuration.SourceRoot = alias
	for n, arg := range r.Template.CensusArguments {
		if arg == old {
			r.Template.CensusArguments[n] = alias
		}
		if arg == "--include" {
			r.Template.CensusArguments[n+1] = "a/x.go"
		}
	}
	writeApplicationFixture(t, r, p)
	var out, diag bytes.Buffer
	code := runPrivateHostApplication([]string{"discover", p}, strings.NewReader(""), &out, &diag)
	var report applicationTerminal
	if json.Unmarshal(out.Bytes(), &report) != nil {
		t.Fatalf("not public discovery report %s %s", out.String(), diag.String())
	}
	if code == 0 || report.Status == "DISCOVERY_COMPLETE" || report.Denominator != "UNKNOWN" {
		t.Fatalf("ASSERT_BINDING_ROOT_ALIAS_REFUSED A-fail %d %+v", code, report)
	}
	t.Log("ASSERT_BINDING_ROOT_ALIAS_REFUSED A-pass")
}
func bindingFinalize(p, id string, returns []byte) (int, string, string) {
	var out, diag bytes.Buffer
	code := runPrivateHostApplication([]string{"finalize", p, id}, bytes.NewReader(returns), &out, &diag)
	return code, out.String(), diag.String()
}
func TestHostApplicationBindingReadChild(t *testing.T) {
	p := os.Getenv("LSP_BINDING_REQUEST")
	if p == "" {
		t.Skip("controlled child only")
	}
	id := os.Getenv("LSP_BINDING_TICKET")
	var out, diag bytes.Buffer
	code := runPrivateHostApplication([]string{"wait", p, id, "reviewer", "0", "1"}, strings.NewReader(""), &out, &diag)
	fc, fo, fd := bindingFinalize(p, id, []byte(os.Getenv("LSP_BINDING_RETURNS")))
	if strings.Contains(out.String(), `"Event":"TERMINAL"`) || fc == 0 {
		t.Fatalf("ASSERT_BINDING_RESTART_UNADMITTED A-fail wait=%d %s %s finalize=%d %s %s", code, out.String(), diag.String(), fc, fo, fd)
	}
	t.Log("ASSERT_BINDING_RESTART_UNADMITTED A-pass")
}
func TestHostApplicationBindingPublishChild(t *testing.T) {
	path := os.Getenv("LSP_BINDING_DEST")
	if path == "" {
		t.Skip("controlled child only")
	}
	if e := applicationWriteOnce(path, []byte(os.Getenv("LSP_BINDING_PAYLOAD"))); e != nil {
		t.Fatal(e)
	}
}
func bindingFreshRead(t *testing.T, p, id string, returns []byte) {
	t.Helper()
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(exe, "-test.run=^TestHostApplicationBindingReadChild$", "-test.v")
	cmd.Env = append(os.Environ(), "LSP_BINDING_REQUEST="+p, "LSP_BINDING_TICKET="+id, "LSP_BINDING_RETURNS="+string(returns))
	out, e := cmd.CombinedOutput()
	t.Log(string(out))
	if e != nil {
		t.Fatal("fresh-process admission", e)
	}
}
func bindingRepairAssertions(t *testing.T) {
	for _, point := range []string{"stage", "rename", "sync", "commit"} {
		t.Run("crash-"+point, func(t *testing.T) {
			r, p, id, terminal, returns := bindingFixture(t)
			exe, e := os.Executable()
			if e != nil {
				t.Fatal(e)
			}
			cmd := exec.Command(exe, "-test.run=^TestHostApplicationBindingPublishChild$", "-test.v")
			cmd.Env = append(os.Environ(), "LSP_BINDING_DEST="+applicationTerminalPath(r.Configuration.Store, id), "LSP_BINDING_PAYLOAD="+string(terminal), "LSP_BINDING_CRASH="+point)
			out, e := cmd.CombinedOutput()
			x, ok := e.(*exec.ExitError)
			if !ok || x.ExitCode() != 23 {
				t.Fatalf("crash seam not executed %s %v", out, e)
			}
			bindingFreshRead(t, p, id, returns)
		})
	}
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "failure"
		}
		t.Run("MED1-finalize-"+name, func(t *testing.T) {
			r, p, id, terminal, returns := bindingFixture(t)
			bindingBarrier = make(chan struct{})
			bindingRelease = make(chan struct{})
			bindingSyncFailure = fail
			done := make(chan error, 1)
			go func() { done <- applicationWriteOnce(applicationTerminalPath(r.Configuration.Store, id), terminal) }()
			<-bindingBarrier
			code, out, diag := bindingFinalize(p, id, returns)
			close(bindingRelease)
			publishErr := <-done
			bindingBarrier = nil
			bindingSyncFailure = false
			if code == 0 || strings.Contains(out, "HOST_OBSERVED") || strings.Contains(out, "SCRIPTED_CONTRACT") {
				t.Errorf("ASSERT_BINDING_FINALIZE_COMMITTED_ONLY A-fail code=%d out=%s diag=%s", code, out, diag)
			} else {
				t.Log("ASSERT_BINDING_FINALIZE_COMMITTED_ONLY A-pass")
			}
			if fail {
				if publishErr == nil {
					t.Fatal("fault not reached")
				}
			} else {
				if publishErr != nil {
					t.Fatal(publishErr)
				}
				code, out, diag = bindingFinalize(p, id, returns)
				if code != 0 || !strings.Contains(out, "SCRIPTED_CONTRACT") {
					t.Fatalf("committed finalize refused %d %s %s", code, out, diag)
				}
			}
		})
	}
	for _, substitute := range []string{"", "final", "stage"} {
		t.Run("MED2-failed-cleanup-"+substitute, func(t *testing.T) {
			r, p, id, terminal, returns := bindingFixture(t)
			dest := applicationTerminalPath(r.Configuration.Store, id)
			bindingSyncFailure = substitute != "stage"
			bindingRemoveCalls = 0
			bindingRemoveFailure = substitute == ""
			bindingSubstitute = substitute
			err := applicationWriteOnce(dest, terminal)
			bindingSyncFailure = false
			bindingRemoveFailure = false
			bindingSubstitute = ""
			if err == nil {
				t.Fatal("fault not reached")
			}
			if bindingRemoveCalls != 0 {
				t.Fatal("pathname cleanup attempted", bindingRemoveCalls)
			}
			if substitute != "" {
				raw, e := os.ReadFile(dest)
				if e != nil || string(raw) != "FOREIGN_FINAL_BYTE_EXACT" {
					t.Errorf("ASSERT_BINDING_FAILED_UNADMITTED_FOREIGN_PRESERVED A-fail foreign=%q err=%v", raw, e)
					return
				}
			}
			var out, diag bytes.Buffer
			code := runPrivateHostApplication([]string{"wait", p, id, "interpreter", "0", "1"}, strings.NewReader(""), &out, &diag)
			fc, fo, fd := bindingFinalize(p, id, returns)
			if strings.Contains(out.String(), `"Event":"TERMINAL"`) || fc == 0 {
				t.Errorf("ASSERT_BINDING_FAILED_UNADMITTED_FOREIGN_PRESERVED A-fail wait=%d %s %s finalize=%d %s %s", code, out.String(), diag.String(), fc, fo, fd)
			} else {
				t.Log("ASSERT_BINDING_FAILED_UNADMITTED_FOREIGN_PRESERVED A-pass")
			}
			bindingFreshRead(t, p, id, returns)
		})
	}
	for _, replacement := range []bool{false, true} {
		t.Run("MED3-root-moved-replacement-"+map[bool]string{false: "absent", true: "different"}[replacement], func(t *testing.T) {
			bindingReplaceRoot = replacement
			defer func() { bindingReplaceRoot = false }()
			r, p, _ := hostApplicationCLIFixture(t)
			f := filepath.Join(r.Configuration.SourceRoot, "a/x.go")
			if e := os.MkdirAll(filepath.Dir(f), 0700); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(f, []byte("package owned\n"), 0600); e != nil {
				t.Fatal(e)
			}
			for n, arg := range r.Template.CensusArguments {
				if arg == "--include" {
					r.Template.CensusArguments[n+1] = "a/x.go"
				}
			}
			writeApplicationFixture(t, r, p)
			bindingMoveRoot = true
			var out, diag bytes.Buffer
			code := runPrivateHostApplication([]string{"discover", p}, strings.NewReader(""), &out, &diag)
			var report applicationTerminal
			if json.Unmarshal(out.Bytes(), &report) != nil {
				t.Fatalf("not a discovery report %s %s", out.String(), diag.String())
			}
			if code == 0 || report.Status == "DISCOVERY_COMPLETE" || report.Denominator != "UNKNOWN" {
				t.Errorf("ASSERT_BINDING_APPROVED_ROOT_REQUIRED A-fail code=%d status=%s selected=%d denominator=%s", code, report.Status, report.Selected, report.Denominator)
			} else {
				t.Log("ASSERT_BINDING_APPROVED_ROOT_REQUIRED A-pass")
			}
		})
	}
	t.Run("MED3-completed-restart", func(t *testing.T) {
		r, p, _ := hostApplicationCLIFixture(t)
		f := filepath.Join(r.Configuration.SourceRoot, "a/x.go")
		if e := os.MkdirAll(filepath.Dir(f), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(f, []byte("package owned\n"), 0600); e != nil {
			t.Fatal(e)
		}
		for n, arg := range r.Template.CensusArguments {
			if arg == "--include" {
				r.Template.CensusArguments[n+1] = "a/x.go"
			}
		}
		r, _ = readinessDiscover(t, r, p)
		if e := os.Rename(r.Configuration.SourceRoot, r.Configuration.SourceRoot+"-moved"); e != nil {
			t.Fatal(e)
		}
		var out, diag bytes.Buffer
		code := runPrivateHostApplication([]string{"discover", p}, strings.NewReader(""), &out, &diag)
		var report applicationTerminal
		if json.Unmarshal(out.Bytes(), &report) != nil {
			t.Fatalf("restart not a public discovery report %s %s", out.String(), diag.String())
		}
		if code == 0 || report.Status == "DISCOVERY_COMPLETE" || report.Denominator != "UNKNOWN" {
			t.Fatalf("ASSERT_BINDING_APPROVED_ROOT_RESTART A-fail %d %+v", code, report)
		}
		t.Log("ASSERT_BINDING_APPROVED_ROOT_RESTART A-pass")
	})
}
