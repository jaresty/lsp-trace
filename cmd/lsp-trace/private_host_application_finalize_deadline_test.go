//go:build darwin

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Only the owned overlay calls these hooks; production clocks and syscalls are
// unchanged. Phase observations, not elapsed-time assertions, establish refusal.
var deadlineFixtureStage string
var deadlineFixtureExpired bool
var deadlineFixturePhases []string
var deadlineFixtureTicket string
var deadlineFixturePipe chan struct{}
var deadlineFixturePublicationPending bool

func deadlineFixturePhase(name string) {
	if deadlineFixtureExpired {
		deadlineFixturePhases = append(deadlineFixturePhases, name)
	}
}
func deadlineFixtureSync(f *os.File) error {
	e := f.Sync()
	st, _ := f.Stat()
	publication := strings.Contains(f.Name(), "host-observation-") || strings.Contains(f.Name(), "host-refusal-")
	if st != nil && st.IsDir() {
		publication = deadlineFixturePublicationPending
	} else {
		deadlineFixturePublicationPending = publication
	}
	publicationSlow := publication && ((strings.HasSuffix(deadlineFixtureStage, "-file") && !st.IsDir()) || (strings.HasSuffix(deadlineFixtureStage, "-dir") && st.IsDir())) && strings.HasPrefix(deadlineFixtureStage, "publication-")
	if e == nil && !deadlineFixtureExpired && (publicationSlow || (deadlineFixtureStage == "slow-file" && strings.Contains(f.Name(), "host-finalize-attempt-")) || (deadlineFixtureStage == "slow-dir" && st != nil && st.IsDir())) {
		time.Sleep(160 * time.Millisecond)
		deadlineFixtureExpired = true
	}
	return e
}
func deadlineFixtureAfterRead(name string) {
	if deadlineFixtureStage == "ticket" && strings.Contains(name, "data-"+deadlineFixtureTicket) || deadlineFixtureStage == "header" && name == "header.json" || deadlineFixtureStage == "terminal" && strings.HasPrefix(name, "terminal-") || deadlineFixtureStage == "ready" && strings.HasPrefix(name, "role-") {
		time.Sleep(160 * time.Millisecond)
		deadlineFixtureExpired = true
	}
}
func deadlineFixtureDecode(e error) error {
	if deadlineFixtureStage == "decode" && !deadlineFixtureExpired {
		time.Sleep(160 * time.Millisecond)
		deadlineFixtureExpired = true
	}
	return e
}
func deadlineFixtureObservations() {
	if deadlineFixturePipe != nil {
		close(deadlineFixturePipe)
		deadlineFixturePipe = nil
	}
}

func TestHostApplicationFinalizeUnexpiredExactBytes(t *testing.T) {
	r, path, id, raw := finalizeRepairFixture(t)
	raw = bytes.Replace(raw, []byte("wrong-role"), []byte("interpreter"), 1)
	var ret [2]struct{ TerminalSHA256 string }
	if json.Unmarshal(raw, &ret) != nil {
		t.Fatal("fixture returns")
	}
	want := fmt.Sprintf("{\"Schema\":\"private-application-host-observation.v1\",\"Status\":\"SCRIPTED_CONTRACT_RETURNS_NOT_AGENTS\",\"ApplicationSHA256\":%q,\"RunID\":%q,\"TerminalSHA256\":%q,\"InventoryHead\":\"\",\"Returns\":%s,\"Authority\":0,\"Accepted\":false,\"SourceCompleteness\":\"UNKNOWN\"}\n", r.Configuration.ApprovalSHA256, id, ret[0].TerminalSHA256, raw)
	var out, errout bytes.Buffer
	if code := runPrivateHostApplication([]string{"finalize", path, id}, bytes.NewReader(raw), &out, &errout); code != 0 || out.String() != want {
		t.Fatalf("success bytes changed: %s %s", out.String(), errout.String())
	}
}

func TestHostApplicationFinalizeDeadlineRepair(t *testing.T) {
	if stage := os.Getenv("LSP_OWNED_DEADLINE_STAGE"); stage != "" {
		if stage == "expiry-good" || stage == "expiry-bad" {
			r, path, id, raw := finalizeRepairFixture(t, func(r *hostApplicationRequest) {
				r.Configuration.Grant.ExpiresUnixNano = time.Now().Add(8 * time.Second).UnixNano()
			})
			if stage == "expiry-good" {
				raw = bytes.Replace(raw, []byte("wrong-role"), []byte("interpreter"), 1)
			}
			reader, writer, e := os.Pipe()
			if e != nil {
				t.Fatal(e)
			}
			defer reader.Close()
			expires := time.Unix(0, r.Configuration.Grant.ExpiresUnixNano)
			time.Sleep(time.Until(expires.Add(-80 * time.Millisecond)))
			entered := make(chan struct{})
			deadlineFixturePipe = entered
			delivered := make(chan time.Time, 1)
			go func() {
				<-entered
				time.Sleep(time.Until(expires.Add(5 * time.Millisecond)))
				writer.Write(raw)
				writer.Close()
				delivered <- time.Now()
			}()
			var out, errout bytes.Buffer
			start := time.Now()
			code := runPrivateHostApplication([]string{"finalize", path, id}, reader, &out, &errout)
			sent := <-delivered
			if !sent.After(expires) || sent.Sub(start) >= 125*time.Millisecond {
				t.Fatalf("fixture delivery outside expiry/slice: %s", sent.Sub(start))
			}
			if code == 0 || bytes.Contains(out.Bytes(), []byte("PUBLISHED")) || bytes.Contains(out.Bytes(), []byte("private-application-host-observation.v1")) {
				t.Fatalf("ASSERT_ABSOLUTE_FINALIZE_EXPIRY: out=%s err=%s", out.String(), errout.String())
			}
			matches, _ := filepath.Glob(filepath.Join(r.Configuration.Store, "host-observation-*"))
			refused, _ := filepath.Glob(filepath.Join(r.Configuration.Store, "host-refusal-*"))
			if len(matches)+len(refused) != 0 {
				t.Fatal("ASSERT_ABSOLUTE_FINALIZE_EXPIRY: record started after expiry")
			}
			out.Reset()
			errout.Reset()
			runPrivateHostApplication([]string{"finalize", path, id}, bytes.NewReader(raw), &out, &errout)
			if out.Len() != 0 || !strings.Contains(errout.String(), "FINALIZE_APPLICATION_EXPIRED_CHARGED") {
				t.Fatalf("retry renewed application expiry: %s %s", out.String(), errout.String())
			}
			for _, n := range []string{"00", "01"} {
				if _, e := os.Stat(filepath.Join(r.Configuration.Store, "host-finalize-attempt-"+n+".json")); e != nil {
					t.Fatal("expired attempts refunded")
				}
			}
			t.Log("ASSERT_ABSOLUTE_FINALIZE_EXPIRY A-pass")
			return
		}
		_, path, id, raw := finalizeRepairFixture(t)
		deadlineFixtureStage = stage
		deadlineFixtureTicket = id
		if strings.HasPrefix(stage, "publication-good") {
			raw = bytes.Replace(raw, []byte("wrong-role"), []byte("interpreter"), 1)
		}
		var out, errout bytes.Buffer
		code := runPrivateHostApplication([]string{"finalize", path, id}, bytes.NewReader(raw), &out, &errout)
		if !deadlineFixtureExpired {
			t.Fatal("fixture did not inject slow owned IO")
		}
		if code == 0 || len(deadlineFixturePhases) != 0 || bytes.Contains(out.Bytes(), []byte("\"DiagnosticStatus\":\"PUBLISHED\"")) || bytes.Contains(out.Bytes(), []byte("private-application-host-observation.v1")) {
			t.Fatalf("ASSERT_NO_POST_DEADLINE_PHASE: phases=%v out=%s err=%s", deadlineFixturePhases, out.String(), errout.String())
		}
		if strings.HasPrefix(stage, "publication-") {
			deadlineFixtureStage = ""
			deadlineFixtureExpired = false
			deadlineFixturePhases = nil
			out.Reset()
			errout.Reset()
			code = runPrivateHostApplication([]string{"finalize", path, id}, bytes.NewReader(raw), &out, &errout)
			if strings.HasPrefix(stage, "publication-good") {
				if code != 0 || !bytes.Contains(out.Bytes(), []byte("private-application-host-observation.v1")) {
					t.Fatalf("unexpired resync failed: %s %s", out.String(), errout.String())
				}
			} else if !bytes.Contains(out.Bytes(), []byte("\"DiagnosticStatus\":\"PUBLISHED\"")) {
				t.Fatalf("unexpired diagnostic resync failed: %s %s", out.String(), errout.String())
			}
		}
		t.Log("ASSERT_NO_POST_DEADLINE_PHASE A-pass")
		return
	}
	dir := t.TempDir()
	replace := map[string]string{}
	for _, name := range []string{"private_host_application.go", "private_host_application_finalize_attempt.go", "private_host_application_finalize_publication.go"} {
		src := filepath.Join(mustFinalizeWorkingDir(t), name)
		raw, e := os.ReadFile(src)
		if e != nil {
			t.Fatal(e)
		}
		s := string(raw)
		s = strings.ReplaceAll(s, "f.Sync()", "deadlineFixtureSync(f)")
		s = strings.ReplaceAll(s, "dir.Sync()", "deadlineFixtureSync(dir)")
		s = strings.Replace(s, "d := json.NewDecoder", "deadlineFixturePhase(\"decode\")\n\td := json.NewDecoder", 1)
		s = strings.Replace(s, "d.Decode(&r)", "deadlineFixtureDecode(d.Decode(&r))", 1)
		s = strings.Replace(s, "canonical, _ := json.Marshal(r)", "deadlineFixturePhase(\"canonical\")\n canonical, _ := json.Marshal(r)", 1)
		s = strings.Replace(s, "dir, e := root.Open(\".\")", "deadlineFixturePhase(\"directory-open\")\n dir, e := root.Open(\".\")", 1)
		s = strings.Replace(s, "return root.OpenFile(name,", "deadlineFixturePhase(name)\n\treturn root.OpenFile(name,", 1)
		s = strings.Replace(s, "f, e := root.OpenFile(name,", "deadlineFixturePhase(name)\n\tf, e := root.OpenFile(name,", 1)
		s = strings.Replace(s, "b, e := applicationFinalizeReadFile(f, limit)", "b, e := applicationFinalizeReadFile(f, limit)\n\tdeadlineFixtureAfterRead(name)", 1)
		s = strings.Replace(s, "b, e := applicationFinalizeReadFile(f, limit, lanes...)", "b, e := applicationFinalizeReadFile(f, limit, lanes...)\n\tdeadlineFixtureAfterRead(name)", 1)
		s = strings.Replace(s, "func (l *applicationFinalizeLane) observations(input io.Reader) ([]byte, error) {", "func (l *applicationFinalizeLane) observations(input io.Reader) ([]byte, error) {\n deadlineFixtureObservations()", 1)
		dst := filepath.Join(dir, name)
		if e = os.WriteFile(dst, []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
		replace[src] = dst
	}
	raw, _ := json.Marshal(struct{ Replace map[string]string }{replace})
	op := filepath.Join(dir, "overlay.json")
	os.WriteFile(op, raw, 0600)
	binary := filepath.Join(dir, "fixture.test")
	args := []string{"test", "-c", "-overlay", op, "-o", binary}
	if applicationFixtureRace {
		args = append(args, "-race")
	}
	args = append(args, ".")
	if b, e := exec.Command("go", args...).CombinedOutput(); e != nil {
		t.Fatalf("overlay build: %v %s", e, b)
	}
	for _, stage := range []string{"expiry-good", "expiry-bad", "slow-file", "slow-dir", "ticket", "header", "terminal", "ready", "decode", "publication-good-file", "publication-bad-file", "publication-good-dir", "publication-bad-dir"} {
		t.Run(stage, func(t *testing.T) {
			cmd := exec.Command(binary, "-test.run=^TestHostApplicationFinalizeDeadlineRepair$", "-test.v")
			cmd.Env = append(os.Environ(), "LSP_OWNED_DEADLINE_STAGE="+stage)
			b, e := cmd.CombinedOutput()
			if e != nil {
				t.Fatalf("%s", b)
			}
			t.Logf("%s", b)
		})
	}
}
