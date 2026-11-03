//go:build darwin

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

var publicationCreated, publicationRelease chan struct{}
var publicationFault string
var publicationSyncReadErr error

func publicationFixtureDirSync(f *os.File, path string) error {
	if publicationFault == "dirsync" {
		_, publicationSyncReadErr = applicationReadBounded(path, 64<<10)
		return errors.New("owned publication directory sync fault")
	}
	return f.Sync()
}
func publicationFixtureSync(f *os.File) error {
	if strings.Contains(f.Name(), ".application-") && publicationFault == "sync" {
		return errors.New("owned publication sync fault")
	}
	return f.Sync()
}
func publicationFixtureClose(f *os.File) error {
	e := f.Close()
	if strings.Contains(f.Name(), ".application-") && publicationFault == "close" {
		return errors.New("owned publication close fault")
	}
	return e
}

func publicationFixtureBeforeWrite(f *os.File) {
	if publicationCreated != nil && strings.Contains(f.Name(), "terminal-") {
		close(publicationCreated)
		<-publicationRelease
	}
}

// The overlay only observes the actual publisher's pre-Write boundary. No
// replacement publisher or sleep/IO-pressure assumption supplies the result.
func TestHostApplicationPublicationRepair(t *testing.T) {
	if os.Getenv("LSP_OWNED_PUBLICATION_BARRIER") == "1" {
		publicationRepairControlled(t)
		return
	}
	src, e := os.ReadFile("private_host_application_host.go")
	if e != nil {
		t.Fatal(e)
	}
	needle := "n, e := f.Write(raw)"
	if strings.Count(string(src), needle) != 1 {
		t.Fatal("publisher observation seam changed")
	}
	overlaySource := strings.Replace(string(src), needle, "publicationFixtureBeforeWrite(f)\n\t"+needle, 1)
	overlaySource = strings.ReplaceAll(overlaySource, "f.Sync()", "publicationFixtureSync(f)")
	overlaySource = strings.ReplaceAll(overlaySource, "f.Close()", "publicationFixtureClose(f)")
	overlaySource = strings.ReplaceAll(overlaySource, "dir.Sync()", "publicationFixtureDirSync(dir,path)")
	dir := t.TempDir()
	replacement := filepath.Join(dir, "host.go")
	if e = os.WriteFile(replacement, []byte(overlaySource), 0600); e != nil {
		t.Fatal(e)
	}
	original, e := filepath.Abs("private_host_application_host.go")
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(map[string]any{"Replace": map[string]string{original: replacement}})
	overlay := filepath.Join(dir, "overlay.json")
	if e = os.WriteFile(overlay, raw, 0600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command("go", "test", "-race", "-overlay", overlay, ".", "-run", "^TestHostApplicationPublicationRepair$", "-count=1", "-v", "-timeout=120s")
	cmd.Env = append(os.Environ(), "LSP_OWNED_PUBLICATION_BARRIER=1")
	out, e := cmd.CombinedOutput()
	t.Log(string(out))
	if e != nil {
		t.Fatal("actual publisher barrier assertion failed", e)
	}
}

func publicationRepairControlled(t *testing.T) {
	r, path, _ := hostApplicationCLIFixture(t)
	var out, diag bytes.Buffer
	if runPrivateHostApplication([]string{"setup", path}, strings.NewReader(""), &out, &diag) != 0 {
		t.Fatal(diag.String())
	}
	var setup struct{ RunID string }
	if json.Unmarshal(out.Bytes(), &setup) != nil {
		t.Fatal("setup")
	}
	var returns [2]product.CohortRoleReturn
	terminal, _ := json.Marshal(applicationTerminal{Schema: "private-host-application-terminal.v1", Status: "CONTROLLED_PUBLICATION_é"})
	for n, role := range []string{"interpreter", "reviewer"} {
		out.Reset()
		diag.Reset()
		if runPrivateHostApplication([]string{"register", path, setup.RunID, role, role, r.Configuration.RolePromptSHA256[n]}, strings.NewReader(""), &out, &diag) != 0 {
			t.Fatal(diag.String())
		}
		returns[n] = product.CohortRoleReturn{Role: role, LaunchID: role, TerminalSHA256: pilot.Digest(terminal), Status: "RETURNED", ReturnSHA256: pilot.Digest([]byte("owned-return")), Kind: "SCRIPTED_CONTRACT"}
	}
	publicationCreated = make(chan struct{})
	publicationRelease = make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- applicationWriteOnce(applicationTerminalPath(r.Configuration.Store, setup.RunID), terminal)
	}()
	<-publicationCreated
	out.Reset()
	diag.Reset()
	beforeCode := runPrivateHostApplication([]string{"wait", path, setup.RunID, "interpreter", "0", "1"}, strings.NewReader(""), &out, &diag)
	var before applicationHostEvent
	beforeErr := json.Unmarshal(out.Bytes(), &before)
	t.Logf("PUBLICATION_PHASE before-write code=%d event=%s digest=%s empty-fingerprint=%s", beforeCode, before.Event, before.TerminalSHA256, pilot.Digest([]byte(pilot.Digest(nil))))
	close(publicationRelease)
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	publicationCreated = nil
	out.Reset()
	diag.Reset()
	code := runPrivateHostApplication([]string{"wait", path, setup.RunID, "reviewer", "0", "100"}, strings.NewReader(""), &out, &diag)
	var after applicationHostEvent
	if code != 0 || json.Unmarshal(out.Bytes(), &after) != nil || after.Event != "TERMINAL" || after.TerminalSHA256 != pilot.Digest(terminal) {
		t.Fatalf("complete wait %s %s", out.String(), diag.String())
	}
	raw, _ := json.Marshal(returns)
	out.Reset()
	diag.Reset()
	code = runPrivateHostApplication([]string{"finalize", path, setup.RunID}, bytes.NewReader(raw), &out, &diag)
	if code != 0 || !bytes.Contains(out.Bytes(), []byte("SCRIPTED_CONTRACT")) {
		t.Fatalf("complete finalize %s %s", out.String(), diag.String())
	}
	t.Logf("PUBLICATION_PHASE complete bytes=%d digest=%s fingerprint=%s payload=%s", len(terminal), pilot.Digest(terminal), pilot.Digest([]byte(pilot.Digest(terminal))), terminal)
	if beforeCode != 0 || beforeErr != nil || before.Event == "TERMINAL" {
		t.Errorf("ASSERT_PUBLICATION_COMPLETE_ONLY A-fail before=%+v code=%d err=%v", before, beforeCode, beforeErr)
	} else {
		t.Log("ASSERT_PUBLICATION_COMPLETE_ONLY A-pass")
	}
	if e := applicationWriteOnce(applicationTerminalPath(r.Configuration.Store, setup.RunID), []byte(`{"replacement":true}`)); !os.IsExist(e) {
		t.Fatal(fmt.Sprint("duplicate did not refuse: ", e))
	}
	publicationRepairSafety(t)
}

func publicationRepairSafety(t *testing.T) {
	dir := t.TempDir()
	for _, fault := range []string{"sync", "close", "dirsync"} {
		dir := t.TempDir()
		publicationFault = fault
		dest := filepath.Join(dir, fault+".json")
		e := applicationWriteOnce(dest, []byte(`{"complete":true}`))
		publicationFault = ""
		if fault == "dirsync" && !os.IsNotExist(publicationSyncReadErr) {
			t.Fatal("reader saw publication before namespace sync", publicationSyncReadErr)
		}
		if e == nil {
			t.Fatal("fault was not reported", fault)
		}
		if _, e = applicationReadBounded(dest, 64<<10); e == nil {
			t.Fatal("failed publication admitted", fault)
		}
		names, e := os.ReadDir(dir)
		if e != nil || len(names) > 2 {
			t.Fatal("failed publication exceeded one intent plus one orphan", names, e)
		}
		if e = applicationWriteOnce(dest, []byte(`{"complete":true}`)); !os.IsExist(e) {
			t.Fatal("failed intent was not spent", e)
		}
		after, e := os.ReadDir(dir)
		if e != nil || len(after) != len(names) {
			t.Fatal("retry grew failed publication names", after, e)
		}
		t.Logf("ASSERT_PUBLICATION_FAILED_UNADMITTED_BOUNDED A-pass fault=%s names=%d", fault, len(names))
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	dest := filepath.Join(dir, "concurrent.json")
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; results <- applicationWriteOnce(dest, []byte(`{"complete":true}`)) }()
	}
	close(start)
	wg.Wait()
	close(results)
	successes, duplicates := 0, 0
	for e := range results {
		if e == nil {
			successes++
		} else if os.IsExist(e) {
			duplicates++
		} else {
			t.Fatal(e)
		}
	}
	if successes != 1 || duplicates != 1 {
		t.Fatal("no-replace concurrency", successes, duplicates)
	}
	t.Log("ASSERT_PUBLICATION_ATOMIC_NOREPLACE A-pass")
	// Empty and torn UTF-8/JSON files are durable-invalid, never TERMINAL.
	for _, payload := range [][]byte{nil, []byte("{\"Status\":\"partial\xc3")} {
		r, p, _ := hostApplicationCLIFixture(t)
		var out, diag bytes.Buffer
		if runPrivateHostApplication([]string{"setup", p}, strings.NewReader(""), &out, &diag) != 0 {
			t.Fatal(diag.String())
		}
		var setup struct{ RunID string }
		if json.Unmarshal(out.Bytes(), &setup) != nil {
			t.Fatal("setup")
		}
		if e := os.WriteFile(applicationTerminalPath(r.Configuration.Store, setup.RunID), payload, 0600); e != nil {
			t.Fatal(e)
		}
		out.Reset()
		diag.Reset()
		code := runPrivateHostApplication([]string{"wait", p, setup.RunID, "interpreter", "0", "1"}, strings.NewReader(""), &out, &diag)
		if code != 1 || out.Len() != 0 || !strings.Contains(diag.String(), "TERMINAL_REFUSED") {
			t.Fatalf("invalid terminal advertised bytes=%d %s %s", len(payload), out.String(), diag.String())
		}
		t.Logf("ASSERT_PUBLICATION_INVALID_NOT_TERMINAL A-pass bytes=%d digest=%s payload=%q", len(payload), pilot.Digest(payload), payload)
	}
}
