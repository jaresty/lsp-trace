//go:build darwin

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Real private CLI coordinator loss: stop it BEFORE the role publishes exact
// invalid bytes, then kill it before it can copy/receipt those bytes. Recovery
// uses a fresh CLI process, not a fabricated host response or original Manager.
func TestHostCohortCLIProductionMailboxKill(t *testing.T) {
	source, sourcePath, base := hostProductCLIRequestFixture(t)
	source.Configuration.MaxCalls = 16
	source.Configuration.ApprovalSHA256 = product.ConfigDigest(source.Configuration)
	source.RuntimeGrantSHA256 = hostProductRequestDigestV3(source)
	raw, _ := json.Marshal(source)
	if e := os.WriteFile(sourcePath, raw, 0600); e != nil {
		t.Fatal(e)
	}
	c := product.CohortConfig{RolePromptSHA256: [2]string{pilot.Digest([]byte(product.CohortRolePrompt("interpreter"))), pilot.Digest([]byte(product.CohortRolePrompt("reviewer")))}, Requested: 20, Profile: product.CohortProfile, Protocol: product.CohortProtocol, SourceConfigSHA256: product.ConfigDigest(source.Configuration), ScopeSHA256: source.Configuration.ScopeSHA256, Store: filepath.Join(base, "reviewed-cohort"), Grant: pilot.Digest([]byte("controlled kill repair grant")), MaxCalls: 16, MaxRequestBytes: 1 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxOutputBytes: 4 << 20}
	r := hostCohortRequest{Schema: "private-existing-host-cohort-request.v1", SourceRequest: sourcePath, SourceRequestSHA256: pilot.Digest(raw), Configuration: c, BackendTimeoutMS: 180000}
	r.RuntimeGrantSHA256 = hostCohortDigest(r)
	path := filepath.Join(base, "cohort-kill-request.json")
	raw, _ = json.Marshal(r)
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	tool := filepath.Join(base, "lsp-trace-kill")
	if raw, e := exec.Command("go", "build", "-o", tool, ".").CombinedOutput(); e != nil {
		t.Fatalf("fixture build %v %s", e, raw)
	}
	if code := runPrivateHostCohort([]string{"setup", path}, bytes.NewReader(nil), io.Discard, io.Discard); code != 0 {
		t.Fatal("fixture setup", code)
	}
	m := hostCohortMailbox(r, source)
	for _, role := range []string{"interpreter", "reviewer"} {
		if e := product.MarkCohortReady(m.Directory, product.Ready{Protocol: product.CohortProtocol, Role: role, LaunchID: "scripted-kill-" + role, PromptSHA256: pilot.Digest([]byte(product.CohortRolePrompt(role)))}); e != nil {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 270*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool, "private-host-cohort", "run", path)
	cmd.Env = append(os.Environ(), "LSP_HOST_PRODUCT_OWNED_SERVER=1")
	var out, errout bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errout
	if e := cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer cmd.Process.Kill()
	wait, e := product.WaitCohort(ctx, m.Directory, "interpreter", 0)
	if e != nil {
		t.Fatal("fixture wait", e)
	}
	if e = cmd.Process.Signal(syscall.SIGSTOP); e != nil {
		t.Fatal(e)
	}
	payload := []byte(" {\"Protocol\":\"WRONG\"} \n")
	if e = product.SubmitCohort(m.Directory, "interpreter", wait.Bundle.Sequence, payload); e == nil {
		t.Fatal("invalid reply accepted")
	}
	if raw, e := os.ReadFile(filepath.Join(m.Directory, "reply-00000001.json")); e != nil || !bytes.Equal(raw, payload) {
		t.Fatal("fixture durable mailbox reply", e)
	}
	if _, e = os.Stat(filepath.Join(c.Store, "host/response-00000001.json")); !os.IsNotExist(e) {
		t.Fatal("fixture host already copied", e)
	}
	if e = cmd.Process.Kill(); e != nil {
		t.Fatal(e)
	}
	if e = cmd.Wait(); e == nil {
		t.Fatal("fixture kill absent")
	}
	recoverCmd := exec.CommandContext(ctx, tool, "private-host-cohort", "fail", path)
	var recovered, diagnostic bytes.Buffer
	recoverCmd.Stdout = &recovered
	recoverCmd.Stderr = &diagnostic
	e = recoverCmd.Run()
	if e == nil {
		t.Fatal("failure command must exit nonzero")
	}
	var report product.CohortReport
	if json.Unmarshal(recovered.Bytes(), &report) != nil || report.Calls != 1 || report.ResponseBytes != len(payload) || report.Reviewed != 0 || !report.Inflight {
		t.Fatalf("ASSERT_REPAIR_CLI_KILL_REPLY_SPENT: response=%d want=%d calls=%d err=%v stderr=%s", report.ResponseBytes, len(payload), report.Calls, e, diagnostic.String())
	}
	if _, e = product.LatestCohort(c); e != nil {
		t.Fatal("ASSERT_REPAIR_CLI_KILL_QUALIFIED", e)
	}
	t.Logf("ASSERT_REPAIR_CLI_KILL_REPLY_SPENT: PASS request=%d response=%d; actual SIGSTOP/SubmitCohort/SIGKILL/fresh CLI failure", report.RequestBytes, report.ResponseBytes)
}

func TestHostCohortWaitEventPublicationMatrix(t *testing.T) {
	for _, event := range []string{"NEEDS_HOST", "TERMINAL", "DEADLINE", "PUBLICATION_ERROR"} {
		t.Run(event, func(t *testing.T) {
			root, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(root, "mailbox")
			if e = product.PrepareMailbox(path); e != nil {
				t.Fatal(e)
			}
			role := "interpreter"
			if event == "TERMINAL" {
				if e = os.WriteFile(filepath.Join(path, "terminal.json"), []byte(`{"Protocol":"PRIVATE_V3_REVIEWED_COHORT_1","Event":"TERMINAL","Status":"FAILED","ReportSHA256":"unknown"}`), 0600); e != nil {
					t.Fatal(e)
				}
			}
			if event == "PUBLICATION_ERROR" {
				role = "foreign"
			}
			if event == "NEEDS_HOST" {
				for _, r := range []string{"interpreter", "reviewer"} {
					if e = product.MarkCohortReady(path, product.Ready{Protocol: product.CohortProtocol, Role: r, LaunchID: "matrix-" + r, PromptSHA256: pilot.Digest([]byte(product.CohortRolePrompt(r)))}); e != nil {
						t.Fatal(e)
					}
				}
				task := product.CohortTask{Protocol: product.CohortProtocol, ConfigSHA256: pilot.Digest([]byte("controlled matrix")), Grant: "controlled matrix", Role: role, Stage: "PROPOSE", Sequence: 1, Items: []product.CohortItem{}}
				raw, _ := json.Marshal(task)
				task.RequestDigest = pilot.Digest(raw)
				raw, _ = json.Marshal(task)
				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan struct{})
				go func() {
					product.CohortMailbox{Directory: path, ExchangeTimeout: time.Second}.ExchangeCohort(ctx, raw)
					close(done)
				}()
				t.Cleanup(func() { cancel(); <-done })
				waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
				defer waitCancel()
				if _, e = product.WaitCohort(waitCtx, path, role, 0); e != nil {
					t.Fatal("matrix request fixture", e)
				}
			}
			args := []string{"wait", path, role, "0", "1"}
			var out bytes.Buffer
			code := runPrivateHostCohort(args, bytes.NewReader(nil), &out, io.Discard)
			expectedCode := 0
			if event == "DEADLINE" || event == "PUBLICATION_ERROR" {
				expectedCode = 1
			}
			if code != expectedCode || !bytes.Contains(out.Bytes(), []byte(`"Event":"`+event+`"`)) || !json.Valid(out.Bytes()) || out.Bytes()[out.Len()-1] != '\n' {
				t.Fatalf("ASSERT_REPAIR_WAIT_EVENTS_STABLE: event=%s code=%d bytes=%q", event, code, out.Bytes())
			}
			var decoded product.CohortWait
			if e = json.Unmarshal(out.Bytes(), &decoded); e != nil {
				t.Fatal(e)
			}
			var expected bytes.Buffer
			encodeHostProductV3(&expected, decoded)
			if !bytes.Equal(out.Bytes(), expected.Bytes()) {
				t.Fatal("ASSERT_REPAIR_WAIT_JSON_BYTE_EXACT")
			}
			for _, writer := range []io.Writer{cohortBrokenWriter{}, cohortShortWriter{}} {
				if code = runPrivateHostCohort(args, bytes.NewReader(nil), writer, io.Discard); code == 0 {
					t.Fatal("ASSERT_REPAIR_WAIT_EVENT_WRITE_FAILURE", event)
				}
			}
		})
	}
}

type cohortShortWriter struct{}

func (cohortShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

type cohortBrokenWriter struct{}

func (cohortBrokenWriter) Write(p []byte) (int, error) {
	return 0, errors.New("controlled stdout failure")
}
func TestHostCohortWaitPublicationFailure(t *testing.T) {
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(root, "mailbox")
	if e = product.PrepareMailbox(path); e != nil {
		t.Fatal(e)
	}
	// Exact canonical terminal, not a real application or Agent observation.
	if e = os.WriteFile(filepath.Join(path, "terminal.json"), []byte(`{"Protocol":"PRIVATE_V3_REVIEWED_COHORT_1","Event":"TERMINAL","Status":"FAILED","ReportSHA256":"unknown"}`), 0600); e != nil {
		t.Fatal(e)
	}
	args := []string{"wait", path, "interpreter", "0", "100"}
	var out bytes.Buffer
	if code := runPrivateHostCohort(args, bytes.NewReader(nil), &out, io.Discard); code != 0 {
		t.Fatal("fixture wait", code)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"Event":"TERMINAL"`)) {
		t.Fatal("fixture terminal event")
	}
	if code := runPrivateHostCohort(args, bytes.NewReader(nil), cohortBrokenWriter{}, io.Discard); code == 0 {
		t.Fatal("ASSERT_REPAIR_WAIT_STDOUT_FAILURE_EXIT")
	}
	if code := runPrivateHostCohort(args, bytes.NewReader(nil), cohortShortWriter{}, io.Discard); code == 0 {
		t.Fatal("ASSERT_REPAIR_WAIT_SHORT_STDOUT_FAILURE_EXIT")
	}
}
