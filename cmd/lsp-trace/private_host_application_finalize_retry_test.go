//go:build darwin

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

func finalizeRepairFixture(t *testing.T, changes ...func(*hostApplicationRequest)) (hostApplicationRequest, string, string, []byte) {
	t.Helper()
	r, path, _ := hostApplicationCLIFixture(t)
	for _, change := range changes {
		change(&r)
		writeApplicationFixture(t, r, path)
	}
	var out, errout bytes.Buffer
	if runPrivateHostApplication([]string{"setup", path}, strings.NewReader(""), &out, &errout) != 0 {
		t.Fatal(errout.String())
	}
	var setup struct{ RunID string }
	if json.Unmarshal(out.Bytes(), &setup) != nil {
		t.Fatal("setup")
	}
	terminal, _ := json.Marshal(applicationTerminal{Schema: "private-host-application-terminal.v1", Status: "CONTROLLED_FIXTURE"})
	if e := applicationWriteOnce(applicationTerminalPath(r.Configuration.Store, setup.RunID), terminal); e != nil {
		t.Fatal(e)
	}
	var returns [2]product.CohortRoleReturn
	for i, role := range []string{"interpreter", "reviewer"} {
		out.Reset()
		errout.Reset()
		if runPrivateHostApplication([]string{"register", path, setup.RunID, role, role, r.Configuration.RolePromptSHA256[i]}, strings.NewReader(""), &out, &errout) != 0 {
			t.Fatal(errout.String())
		}
		returns[i] = product.CohortRoleReturn{Role: role, LaunchID: role, TerminalSHA256: pilot.Digest(terminal), Status: "RETURNED", ReturnSHA256: pilot.Digest([]byte("owned-return")), Kind: "SCRIPTED_CONTRACT"}
	}
	returns[0].Role = "wrong-role"
	raw, _ := json.Marshal(returns)
	return r, path, setup.RunID, raw
}

func TestHostApplicationFinalizeRetryRepair(t *testing.T) {
	_, path, id, raw := finalizeRepairFixture(t)
	for n := 0; n < 16; n++ {
		var out, errout bytes.Buffer
		runPrivateHostApplication([]string{"finalize", path, id}, bytes.NewReader(raw), &out, &errout)
		if !bytes.Contains(out.Bytes(), []byte("PUBLISHED")) {
			t.Fatalf("attempt %d: %s %s", n, out.String(), errout.String())
		}
	}
	var out, errout bytes.Buffer
	runPrivateHostApplication([]string{"finalize", path, id}, bytes.NewReader(raw), &out, &errout)
	if out.Len() != 0 || !strings.Contains(errout.String(), "FINALIZE_ATTEMPTS_EXHAUSTED") {
		t.Fatalf("ASSERT_FINITE_FINALIZE A-fail: output=%s error=%s", out.String(), errout.String())
	}
	t.Log("ASSERT_FINITE_FINALIZE A-pass")
}

// These functions are linked only into an owned overlay test binary. Production
// source retains direct os.File operations. Faults never traverse external errors.
var finalizeFixtureFault string
var finalizeFixtureSyncs int
var finalizeFixtureDirectoryPending bool

func finalizeFixtureTarget(f *os.File) bool { return strings.Contains(f.Name(), "host-refusal-") }
func finalizeFixtureWrite(f *os.File, p []byte) (int, error) {
	if finalizeFixtureTarget(f) && finalizeFixtureFault == "write" {
		n, _ := f.Write(p[:len(p)/2])
		return n, io.ErrShortWrite
	}
	return f.Write(p)
}
func finalizeFixtureSync(f *os.File) error {
	st, e := f.Stat()
	if e != nil {
		return e
	}
	relevant := finalizeFixtureTarget(f)
	if st.IsDir() {
		relevant = finalizeFixtureDirectoryPending
		finalizeFixtureDirectoryPending = false
	} else {
		finalizeFixtureDirectoryPending = relevant
	}
	if relevant {
		finalizeFixtureSyncs++
	}
	if relevant && (finalizeFixtureFault == "sync" && !st.IsDir() || finalizeFixtureFault == "dirsync" && st.IsDir()) {
		return errors.New("owned sync fault")
	}
	return f.Sync()
}
func finalizeFixtureClose(f *os.File) error {
	relevant := finalizeFixtureTarget(f)
	e := f.Close()
	if relevant && finalizeFixtureFault == "close" {
		return errors.New("owned close fault")
	}
	return e
}

func TestHostApplicationFinalizeDurabilityRepair(t *testing.T) {
	if stage := os.Getenv("LSP_OWNED_FINALIZE_FAULT"); stage != "" {
		r, path, id, raw := finalizeRepairFixture(t)
		finalizeFixtureFault = stage
		for n := 0; n < 2; n++ {
			var out, errout bytes.Buffer
			runPrivateHostApplication([]string{"finalize", path, id}, bytes.NewReader(raw), &out, &errout)
			if !bytes.Contains(out.Bytes(), []byte("PUBLICATION_FAILED")) {
				t.Fatalf("ASSERT_DURABLE_EQUAL_RECORD A-fail: stage=%s retry=%d output=%s error=%s", stage, n, out.String(), errout.String())
			}
		}
		finalizeFixtureFault = ""
		var out, errout bytes.Buffer
		before := finalizeFixtureSyncs
		runPrivateHostApplication([]string{"finalize", path, id}, bytes.NewReader(raw), &out, &errout)
		if stage != "write" && (!bytes.Contains(out.Bytes(), []byte("PUBLISHED")) || finalizeFixtureSyncs < before+2) {
			t.Fatalf("ASSERT_DURABLE_EQUAL_RECORD A-fail: no completed resync %s", out.String())
		}
		if stage == "write" && !bytes.Contains(out.Bytes(), []byte("PUBLICATION_FAILED")) {
			t.Fatal("partial bytes overwritten")
		}
		for n := 0; n < 3; n++ {
			if _, e := os.Stat(filepath.Join(r.Configuration.Store, fmt.Sprintf("host-finalize-attempt-%02d.json", n))); e != nil {
				t.Fatal("sync failure refunded attempt")
			}
		}
		t.Log("ASSERT_DURABLE_EQUAL_RECORD A-pass")
		return
	}
	// The overlay is generated exclusively in t.TempDir; it instruments owned
	// publication IO, not syscalls, providers, or installed binaries.
	dir := t.TempDir()
	replace := map[string]string{}
	for _, name := range []string{"private_host_application_host.go", "private_host_application_finalize_publication.go"} {
		src := filepath.Join(mustFinalizeWorkingDir(t), name)
		raw, e := os.ReadFile(src)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			t.Fatal(e)
		}
		text := string(raw)
		for _, v := range []string{"f", "dir"} {
			text = strings.ReplaceAll(text, v+".Sync()", "finalizeFixtureSync("+v+")")
			text = strings.ReplaceAll(text, v+".Close()", "finalizeFixtureClose("+v+")")
		}
		text = strings.ReplaceAll(text, "f.Write(raw)", "finalizeFixtureWrite(f, raw)")
		dst := filepath.Join(dir, name)
		if e = os.WriteFile(dst, []byte(text), 0600); e != nil {
			t.Fatal(e)
		}
		replace[src] = dst
	}
	overlay, _ := json.Marshal(struct{ Replace map[string]string }{replace})
	op := filepath.Join(dir, "overlay.json")
	os.WriteFile(op, overlay, 0600)
	binary := filepath.Join(dir, "fixture.test")
	buildArgs := []string{"test", "-c", "-overlay", op, "-o", binary}
	if applicationFixtureRace {
		buildArgs = append(buildArgs, "-race")
	}
	buildArgs = append(buildArgs, ".")
	cmd := exec.Command("go", buildArgs...)
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("owned harness build: %v %s", e, b)
	}
	for _, stage := range []string{"write", "sync", "close", "dirsync"} {
		t.Run(stage, func(t *testing.T) {
			cmd := exec.Command(binary, "-test.run=^TestHostApplicationFinalizeDurabilityRepair$", "-test.v")
			cmd.Env = append(os.Environ(), "LSP_OWNED_FINALIZE_FAULT="+stage)
			b, e := cmd.CombinedOutput()
			if e != nil {
				t.Fatalf("%s", b)
			}
			t.Logf("%s", b)
		})
	}
}
func TestHostApplicationFinalizeFreshProcesses(t *testing.T) {
	r, path, id, raw := finalizeRepairFixture(t)
	tool := buildApplicationTool(t, t.TempDir())
	run := func(id string, raw []byte) (string, string) {
		cmd := exec.Command(tool, "private-host-application", "finalize", path, id)
		cmd.Stdin = bytes.NewReader(raw)
		var out, errout bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errout
		_ = cmd.Run()
		return out.String(), errout.String()
	}
	// Distinct fresh processes consume the same pool on success, repeat-success,
	// refusal, conflicting first-record retry, malformed input and stale run ID.
	var good [2]product.CohortRoleReturn
	json.Unmarshal(raw, &good)
	good[0].Role = "interpreter"
	goodRaw, _ := json.Marshal(good)
	firstPath := filepath.Join(r.Configuration.Store, "host-refusal-"+id[7:]+"-0-ROLE_ORDER.json")
	var first []byte
	for n := 0; n < 16; n++ {
		operand := raw
		runID := id
		switch n {
		case 0, 1:
			operand = goodRaw
		case 3:
			var conflict [2]product.CohortRoleReturn
			json.Unmarshal(raw, &conflict)
			conflict[0].Role = "different-wrong-role"
			operand, _ = json.Marshal(conflict)
		case 4:
			operand = []byte("invalid")
		case 5:
			runID = pilot.Digest([]byte("stale-run"))
		}
		out, errout := run(runID, operand)
		if strings.Contains(errout, "FINALIZE_ATTEMPTS_EXHAUSTED") {
			t.Fatalf("early exhaustion at %d", n)
		}
		if n == 0 || n == 1 {
			if !strings.Contains(out, "private-application-host-observation.v1") {
				t.Fatalf("successful observation lost: %s %s", out, errout)
			}
		}
		if n == 2 {
			first, _ = os.ReadFile(firstPath)
		}
	}
	out, errout := run(id, raw)
	if out != "" || !strings.Contains(errout, "FINALIZE_ATTEMPTS_EXHAUSTED") {
		t.Fatalf("ASSERT_FRESH_PROCESS_FINITE %s %s", out, errout)
	}
	after, _ := os.ReadFile(firstPath)
	if len(first) == 0 || !bytes.Equal(first, after) {
		t.Fatal("first diagnostic changed")
	}
	// A fresh run ID and altered operands cannot bypass the exhausted entry pool.
	out, errout = run(pilot.Digest([]byte("another-run-alias")), []byte("not-json"))
	if out != "" || !strings.Contains(errout, "FINALIZE_ATTEMPTS_EXHAUSTED") {
		t.Fatalf("alias replenished %s %s", out, errout)
	}
	t.Log("ASSERT_FRESH_PROCESS_FINITE A-pass")
}

func TestHostApplicationFinalizeCompetitionAndTornClaims(t *testing.T) {
	r, path, id, raw := finalizeRepairFixture(t)
	tool := buildApplicationTool(t, t.TempDir())
	for n := 0; n < 4; n++ {
		if e := os.WriteFile(filepath.Join(r.Configuration.Store, fmt.Sprintf("host-finalize-attempt-%02d.json", n)), nil, 0600); e != nil {
			t.Fatal(e)
		}
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	exhausted, published, refused := 0, 0, 0
	for n := 0; n < 24; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(tool, "private-host-application", "finalize", path, id)
			cmd.Stdin = bytes.NewReader(raw)
			b, _ := cmd.CombinedOutput()
			mu.Lock()
			defer mu.Unlock()
			if bytes.Contains(b, []byte("FINALIZE_ATTEMPTS_EXHAUSTED")) {
				exhausted++
			}
			if bytes.Contains(b, []byte("\"DiagnosticStatus\":\"PUBLISHED\"")) {
				published++
			} else if !bytes.Contains(b, []byte("FINALIZE_ATTEMPTS_EXHAUSTED")) {
				refused++
				t.Logf("admitted refusal: %s", b)
				if !bytes.Contains(b, []byte("PUBLICATION_FAILED")) && !bytes.Contains(b, []byte("FINALIZE_SLICE_EXPIRED_CHARGED")) && !bytes.Contains(b, []byte("TERMINAL_UNAVAILABLE")) && !bytes.Contains(b, []byte("READY_UNAVAILABLE")) {
					t.Errorf("unexpected admitted failure: %s", b)
				}
			}
		}()
	}
	wg.Wait()
	if published+refused != 12 || exhausted != 12 || published == 0 {
		t.Fatalf("ASSERT_COMPETING_ATTEMPTS published=%d refused=%d exhausted=%d", published, refused, exhausted)
	}
	for n := 0; n < 4; n++ {
		b, e := os.ReadFile(filepath.Join(r.Configuration.Store, fmt.Sprintf("host-finalize-attempt-%02d.json", n)))
		if e != nil || len(b) != 0 {
			t.Fatal("torn slot reset")
		}
	}
	t.Log("ASSERT_COMPETING_ATTEMPTS A-pass")
}

func TestHostApplicationFinalizeExpiredAndForeign(t *testing.T) {
	t.Run("expired", func(t *testing.T) {
		r, path, id, raw := finalizeRepairFixture(t, func(r *hostApplicationRequest) {
			r.Configuration.Grant.ExpiresUnixNano = time.Now().Add(2 * time.Second).UnixNano()
		})
		time.Sleep(time.Until(time.Unix(0, r.Configuration.Grant.ExpiresUnixNano)) + time.Millisecond)
		var out, errout bytes.Buffer
		runPrivateHostApplication([]string{"finalize", path, id}, bytes.NewReader(raw), &out, &errout)
		if out.Len() != 0 || !strings.Contains(errout.String(), "FINALIZE_APPLICATION_EXPIRED_CHARGED") {
			t.Fatalf("expiry %s %s", out.String(), errout.String())
		}
		if _, e := os.Stat(filepath.Join(r.Configuration.Store, "host-finalize-attempt-00.json")); e != nil {
			t.Fatal("expired refusal not charged")
		}
	})
	t.Run("foreign", func(t *testing.T) {
		r, path, id, raw := finalizeRepairFixture(t)
		r.Configuration.RevisionAssertion = "foreign"
		writeApplicationFixture(t, r, path)
		var out, errout bytes.Buffer
		runPrivateHostApplication([]string{"finalize", path, id}, bytes.NewReader(raw), &out, &errout)
		if out.Len() != 0 || !strings.Contains(errout.String(), "FINALIZE_CONFIG_REFUSED_CHARGED") {
			t.Fatalf("foreign %s %s", out.String(), errout.String())
		}
		if _, e := os.Stat(filepath.Join(r.Configuration.Store, "host-finalize-attempt-00.json")); e != nil {
			t.Fatal("foreign refusal not charged")
		}
	})
}

func TestHostApplicationFinalizePinnedPublication(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "oversize", "mode"} {
		t.Run(kind, func(t *testing.T) {
			r, path, id, raw := finalizeRepairFixture(t)
			name := filepath.Join(r.Configuration.Store, "host-refusal-"+id[7:]+"-0-ROLE_ORDER.json")
			var out, errout bytes.Buffer
			runPrivateHostApplication([]string{"finalize", path, id}, bytes.NewReader(raw), &out, &errout)
			good, e := os.ReadFile(name)
			if e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "symlink":
				os.Rename(name, name+".original")
				if e := os.Symlink(name+".original", name); e != nil {
					t.Fatal(e)
				}
			case "hardlink":
				if e := os.Link(name, name+".link"); e != nil {
					t.Fatal(e)
				}
			case "oversize":
				if e := os.WriteFile(name, bytes.Repeat([]byte("x"), 4097), 0600); e != nil {
					t.Fatal(e)
				}
			case "mode":
				os.Chmod(name, 0644)
			}
			out.Reset()
			errout.Reset()
			runPrivateHostApplication([]string{"finalize", path, id}, bytes.NewReader(raw), &out, &errout)
			if !bytes.Contains(out.Bytes(), []byte("PUBLICATION_FAILED")) {
				t.Fatalf("unsafe resync accepted %s %s", out.String(), errout.String())
			}
			if kind == "symlink" || kind == "hardlink" {
				after, _ := os.ReadFile(name)
				if !bytes.Equal(after, good) {
					t.Fatal("alias bytes changed")
				}
			}
		})
	}
}

func TestHostApplicationFinalizeAttemptBeforeOperands(t *testing.T) {
	r, path, id, raw := finalizeRepairFixture(t)
	for n := 0; n < 16; n++ {
		os.WriteFile(filepath.Join(r.Configuration.Store, fmt.Sprintf("host-finalize-attempt-%02d.json", n)), nil, 0600)
	}
	// Exhausted entry must not touch even a now-missing application header.
	os.Rename(filepath.Join(r.Configuration.Store, "header.json"), filepath.Join(r.Configuration.Store, "owned-hidden-header.json"))
	var out, errout bytes.Buffer
	input := &finalizeForbiddenReader{}
	runPrivateHostApplication([]string{"finalize", path, id}, input, &out, &errout)
	if input.reads != 0 || out.Len() != 0 || !strings.Contains(errout.String(), "FINALIZE_ATTEMPTS_EXHAUSTED") {
		t.Fatalf("heavy work after exhaustion reads=%d out=%s err=%s", input.reads, out.String(), errout.String())
	}
	_ = raw
}

type finalizeForbiddenReader struct{ reads int }

func (r *finalizeForbiddenReader) Read([]byte) (int, error) { r.reads++; return 0, io.EOF }

func TestHostApplicationFinalizePrepaidPipeDeadline(t *testing.T) {
	r, path, id, _ := finalizeRepairFixture(t)
	reader, writer, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer reader.Close()
	defer writer.Close()
	var out, errout bytes.Buffer
	start := time.Now()
	runPrivateHostApplication([]string{"finalize", path, id}, reader, &out, &errout)
	if out.Len() != 0 || !strings.Contains(errout.String(), "INVALID_HOST_OBSERVATIONS") || time.Since(start) > time.Second {
		t.Fatalf("unbounded pipe wait: %s %s", out.String(), errout.String())
	}
	if _, e := os.Stat(filepath.Join(r.Configuration.Store, "host-finalize-attempt-00.json")); e != nil {
		t.Fatal("timeout not charged")
	}
}

func mustFinalizeWorkingDir(t *testing.T) string {
	t.Helper()
	p, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	return p
}
