//go:build darwin

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func startupOverlayTool(t *testing.T, base string, replacements map[string][]byte) string {
	t.Helper()
	mapping := map[string]string{}
	for original, raw := range replacements {
		absolute, e := filepath.Abs(original)
		if e != nil {
			t.Fatal(e)
		}
		replacement := filepath.Join(base, filepath.Base(original)+"-overlay.go")
		if e = os.WriteFile(replacement, raw, 0600); e != nil {
			t.Fatal(e)
		}
		mapping[absolute] = replacement
	}
	raw, _ := json.Marshal(struct{ Replace map[string]string }{mapping})
	path := filepath.Join(base, "startup-overlay.json")
	os.WriteFile(path, raw, 0600)
	tool := filepath.Join(base, "startup-tool")
	args := []string{"build", "-overlay", path, "-o", tool}
	if applicationFixtureRace {
		args = append(args, "-race")
	}
	args = append(args, ".")
	if raw, e := exec.Command("go", args...).CombinedOutput(); e != nil {
		t.Fatalf("overlay build: %v %s", e, raw)
	}
	return tool
}

func TestHostApplicationStartupDurabilityBoundary(t *testing.T) {
	r, path, base := hostApplicationCLIFixture(t)
	source, e := os.ReadFile("../../internal/privatehostproductv3/store.go")
	if e != nil {
		t.Fatal(e)
	}
	needle := []byte("\treturn errors.Join(d.Sync(), d.Close())")
	if bytes.Count(source, needle) != 1 {
		t.Fatal("durability seam changed")
	}
	// Hold the ACTUAL activation publisher before directory Sync/Close. Readers
	// cannot admit its still-locked/two-link contents; notification must be absent.
	insert := []byte(`
 if strings.HasPrefix(name,"activation-") {
  signal:=os.NewFile(4,"owned-activation-phase");if signal==nil {return ErrInvalid};_,er:=signal.Write([]byte{1});signal.Close();if er!=nil {return er}
  syscall.SetNonblock(5,true);release:=os.NewFile(5,"owned-publication-release");if release==nil {return ErrInvalid};defer release.Close();release.SetReadDeadline(time.Now().Add(10*time.Second));var one [1]byte;if _,er=release.Read(one[:]);er!=nil {return er}
 }
`)
	source = bytes.Replace(source, needle, append(insert, needle...), 1)
	source = bytes.Replace(source, []byte("\t\"syscall\""), []byte("\t\"syscall\"\n\t\"time\""), 1)
	replacements := map[string][]byte{"../../internal/privatehostproductv3/store.go": source}
	if os.Getenv("TEST_STARTUP_PREMATURE") == "1" {
		cli, er := os.ReadFile("private_host_application.go")
		if er != nil {
			t.Fatal(er)
		}
		start := bytes.Index(cli, []byte("\t\tif startup != nil && !startup.attempted {"))
		if start < 0 {
			t.Fatal("emit seam")
		}
		end := bytes.Index(cli[start:], []byte("\t\tfor mailbox.CheckReady()"))
		if end < 0 {
			t.Fatal("readiness seam")
		}
		end += start
		block := append([]byte(nil), cli[start:end]...)
		cli = append(append([]byte(nil), cli[:start]...), cli[end:]...)
		anchor := []byte("\t\tif e = a.WriteActivation(s.Activation, activation); e != nil {")
		if bytes.Count(cli, anchor) != 1 {
			t.Fatal("activation call seam")
		}
		replacements["private_host_application.go"] = bytes.Replace(cli, anchor, append(block, anchor...), 1)
	}
	tool := startupOverlayTool(t, base, replacements)
	var setup, diag bytes.Buffer
	if runPrivateHostApplication([]string{"setup", path}, bytes.NewReader(nil), &setup, &diag) != 0 {
		t.Fatal(diag.String())
	}
	var ticket struct{ RunID string }
	json.Unmarshal(setup.Bytes(), &ticket)
	r, e = readHostApplication(path)
	if e != nil {
		t.Fatal(e)
	}
	nr, nw, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer nr.Close()
	pr, pw, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer pr.Close()
	rr, rw, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer rw.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool, "private-host-application", "run", path, ticket.RunID, "--startup-notify-fd", "3")
	cmd.ExtraFiles = []*os.File{nw, pw, rr}
	cmd.Env = append(os.Environ(), "LSP_HOST_APPLICATION_OWNED_SERVER=1")
	var out, errout bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errout
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	nw.Close()
	pw.Close()
	rr.Close()
	pr.SetReadDeadline(time.Now().Add(30 * time.Second))
	var phase [1]byte
	if _, e = pr.Read(phase[:]); e != nil || phase[0] != 1 {
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatalf("publication barrier not reached: %v", e)
	}
	nr.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var first [1]byte
	early, earlyErr := nr.Read(first[:])
	timedOut := errors.Is(earlyErr, os.ErrDeadlineExceeded)
	nr.SetReadDeadline(time.Now().Add(10 * time.Second))
	rw.Write([]byte{1})
	rw.Close()
	raw, re := io.ReadAll(io.LimitReader(nr, 513))
	if early > 0 {
		raw = append(first[:early], raw...)
	}
	cmd.Process.Signal(os.Interrupt)
	cmd.Wait()
	if early != 0 || !timedOut || re != nil || verifyStartupFixturePacket(raw, r, ticket.RunID) != nil {
		t.Fatalf("ASSERT_STARTUP_AFTER_DURABLE_PUBLICATION A-fail early_bytes=%d timeout=%t early_error=%v final_bytes=%d read=%v diagnostic=%s", early, timedOut, earlyErr, len(raw), re, errout.String())
	}
	t.Log("ASSERT_STARTUP_AFTER_DURABLE_PUBLICATION A-pass early_bytes=0 publisher=BEFORE_DIRECTORY_SYNC packet=AFTER_RELEASE")
}

func TestHostApplicationStartupShortWrite(t *testing.T) {
	_, path, base := hostApplicationCLIFixture(t)
	source, e := os.ReadFile("private_host_application_startup.go")
	if e != nil {
		t.Fatal(e)
	}
	needle := []byte("wrote, e := unix.Write(fd, raw)")
	if bytes.Count(source, needle) != 1 {
		t.Fatal("short write seam")
	}
	source = bytes.Replace(source, needle, []byte("wrote, e := len(raw)-1, error(nil)"), 1)
	tool := startupOverlayTool(t, base, map[string][]byte{"private_host_application_startup.go": source})
	var setup, diag bytes.Buffer
	if runPrivateHostApplication([]string{"setup", path}, bytes.NewReader(nil), &setup, &diag) != 0 {
		t.Fatal(diag.String())
	}
	var ticket struct{ RunID string }
	json.Unmarshal(setup.Bytes(), &ticket)
	reader, writer, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer reader.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool, "private-host-application", "run", path, ticket.RunID, "--startup-notify-fd", "3")
	cmd.ExtraFiles = []*os.File{writer}
	cmd.Env = append(os.Environ(), "LSP_HOST_APPLICATION_OWNED_SERVER=1")
	var out, errout bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errout
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	writer.Close()
	e = cmd.Wait()
	reader.SetReadDeadline(time.Now().Add(time.Second))
	raw, re := io.ReadAll(io.LimitReader(reader, 513))
	var terminal applicationTerminal
	if e == nil || re != nil || len(raw) != 0 || json.Unmarshal(out.Bytes(), &terminal) != nil || terminal.Status != "FAILED_STARTUP_NOTIFICATION" || terminal.Calls != 0 {
		t.Fatalf("ASSERT_STARTUP_SHORT_WRITE_FAIL_CLOSED A-fail error=%v terminal=%s diagnostic=%s", e, out.Bytes(), errout.Bytes())
	}
	t.Log("ASSERT_STARTUP_SHORT_WRITE_FAIL_CLOSED A-pass")
}
