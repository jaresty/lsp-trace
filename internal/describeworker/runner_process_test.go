package describeworker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/retainedprojectiontestfixture"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/targetpacket"
)

type runnerFixtureLookup struct{ body []byte }

func (l runnerFixtureLookup) Get(id sourceobject.Identity) (sourceobject.Object, error) {
	return sourceobject.Object{Identity: id, Bytes: append([]byte(nil), l.body...)}, nil
}

func validRunnerRequest(t *testing.T) describerequest.Record {
	t.Helper()
	request, _ := validRunnerFixture(t)
	return request
}

func validRunnerFixture(t *testing.T) (describerequest.Record, targetpacket.Packet) {
	t.Helper()
	fx := retainedprojectiontestfixture.ValidV2Artifact(t)
	n := censusprogramc.Representative{
		Status: censusprogramc.CandidateStatus, Authority: 0, SourceGraphComplete: "UNKNOWN",
		CensusID: "census", ConstituentIdentity: "constituent", ConstituentOrdinal: 0,
		SelectionState: "SELECTED", SelectedNode: fx.NodeID, ClaimCeiling: "STRUCTURAL",
		BatchID: "batch", CommunityIdentity: "community", ExecutionBundleID: "bundle",
		SeedLabel: "seed", SeedAt: "at", Members: []string{"member"}, SCCMembers: []string{"member"},
	}
	c := censusprogramc.Result{CensusID: "census", Admission: programcadmission.Result{Artifact: programcadmission.Artifact{Constituents: []programcadmission.ConstituentReference{{Identity: "constituent", GraphSHA256: fx.GraphV5Digest, GraphByteLength: int(fx.GraphV5ByteLen)}}}}, Representatives: censusprogramc.RepresentativeSelection{State: "SELECTED", Nominations: []censusprogramc.Representative{n}}}
	packets, err := targetpacket.Build(targetpacket.Request{
		Census: c, Snapshots: []targetpacket.Snapshot{{ConstituentIdentity: "constituent", Raw: fx.Raw}},
		Lookup: runnerFixtureLookup{fx.Content}, Policy: sourceprojection.Policy{PolicyID: "p", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 10, MaxObjects: 10, MaxWork: 100, EnforceLimits: true},
		ResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 1, MaxUniqueSourceBytes: uint64(len(fx.Content)), MaxLogicalSelections: 1}, MaxResponseBytes: 1 << 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	records, err := describerequest.Build(packets, 90_000)
	if err != nil || len(records) != 1 {
		t.Fatalf("request fixture: records=%d err=%v", len(records), err)
	}
	return records[0], packets.Packets[0]
}

func semanticJSON() string {
	return `{"verdict":"SUPPORTED","target_role":"role","nearest_outward_consumer":"consumer","consumer_need":"need","provided_behavior":"behavior","boundary_contribution":"boundary","limitations":["bounded"],"citations":["C1"]}`
}

func workerJSON() string {
	return `{"status":"COMPLETE","text":` + strconv.Quote(semanticJSON()) + `,"tokens":1,"load_ms":2.5,"run_ms":3.5,"cancelled":false,"decode_status":0,"error":"","grammar":"json","context_tokens":64}` + "\n"
}

func processConfig(t *testing.T, workerBody string) (Config, string) {
	t.Helper()
	root := t.TempDir()
	write := func(name, body string, mode os.FileMode) FilePin {
		p := filepath.Join(root, name)
		if err := os.WriteFile(p, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
		s := sha256.Sum256([]byte(body))
		return FilePin{Path: p, SHA256: "sha256:" + hex.EncodeToString(s[:])}
	}
	logPath := filepath.Join(root, "sandbox.log")
	sandbox := `#!/bin/sh
printf '%s\n' "$@" > ` + shellQuote(logPath) + `
[ "$1" = "-f" ] || exit 91
shift 2
exec "$@"
`
	cfg := Config{
		Worker: write("worker", workerBody, 0700), Model: write("model", "model", 0600),
		Library: write("lib", "lib", 0600), SandboxExecutable: write("sandbox", sandbox, 0700),
		SandboxProfile: write("profile", "(version 1)\n(deny network*)\n", 0600), Grammar: write("grammar", "root ::= object\n", 0600),
		RuntimeIdentity: "runtime-v1", AdapterIdentity: "adapter-v1", ModelIdentity: "model-v1",
		Limits: Limits{TimeoutMS: 1000, MaxTokens: 64, ContextTokens: 1024, StdoutBytes: 4096, StderrBytes: 1024, WorkBytes: 8192, TempBytes: 8192},
	}
	return cfg, logPath
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

func requireDarwinProcess(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("Darwin-only process-group supervisor and sandbox invocation contract")
	}
}

func failureCode(t *testing.T, err error) Code {
	t.Helper()
	var f *Failure
	if !AsFailure(err, &f) {
		t.Fatalf("not typed failure: %v", err)
	}
	return f.Code()
}

func TestRunnerEndToEndHappyPath(t *testing.T) {
	requireDarwinProcess(t)
	req := validRunnerRequest(t)
	observationPath := filepath.Join(t.TempDir(), "worker-observation")
	promptCopyPath := filepath.Join(t.TempDir(), "prompt-copy")
	worker := `#!/bin/sh
prompt=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -prompt-file) prompt="$2"; shift 2 ;;
    *) shift ;;
  esac
done
[ -n "$prompt" ] || exit 84
{
  printf 'secret=%s\n' "$DESCRIBE_SECRET_SENTINEL"
  printf 'cwd=%s\n' "$PWD"
  printf 'home=%s\n' "$HOME"
  printf 'tmpdir=%s\n' "$TMPDIR"
  printf 'cwdmode=%s\n' "$(stat -f %Lp "$PWD")"
  printf 'promptmode=%s\n' "$(stat -f %Lp "$prompt")"
  printf 'prompt=%s\n' "$prompt"
} > ` + shellQuote(observationPath) + `
cp "$prompt" ` + shellQuote(promptCopyPath) + `
printf '%s' ` + shellQuote(workerJSON()) + `
`
	cfg, logPath := processConfig(t, worker)
	t.Setenv("DESCRIBE_SECRET_SENTINEL", "must-not-leak")
	r, err := NewRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Run(context.Background(), req, "attempt-a")
	if err != nil {
		t.Fatal(err)
	}
	if got.Invocation.Status() != StatusSucceeded || got.Response.ID() == "" || got.Invocation.ID() == "" {
		t.Fatalf("terminal records: status=%s response=%q invocation=%q", got.Invocation.Status(), got.Response.ID(), got.Invocation.ID())
	}
	receipt := got.Invocation.Receipt()
	if !receipt.Started || !receipt.Reaped || receipt.TerminalOutcomes != 1 || receipt.Teardown || receipt.StdoutTruncated || receipt.StderrTruncated {
		t.Fatalf("receipt: %+v", receipt)
	}
	observation, err := os.ReadFile(observationPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(observation, []byte("must-not-leak")) || !bytes.Contains(observation, []byte("secret=\n")) || !bytes.Contains(observation, []byte("cwdmode=700\n")) || !bytes.Contains(observation, []byte("promptmode=600\n")) {
		t.Fatalf("worker observation: %q", observation)
	}
	promptCopy, err := os.ReadFile(promptCopyPath)
	if err != nil || string(promptCopy) != req.Envelope.Prompt {
		t.Fatalf("prompt content: err=%v got=%q", err, promptCopy)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-f", cfg.SandboxProfile.Path, cfg.Worker.Path, "-model", cfg.Model.Path, "-lib", filepath.Dir(cfg.Library.Path)}
	for _, part := range want {
		if !bytes.Contains(log, []byte(part+"\n")) {
			t.Fatalf("sandbox argv missing %q: %q", part, log)
		}
	}
	if !bytes.Contains(log, []byte("-timeout\n1s\n")) {
		t.Fatalf("ASSERT_TIMEOUT_DURATION_FRAMING: want -timeout 1s, got %q", log)
	}
	if bytes.Contains(log, []byte(cfg.Library.Path+"\n")) {
		t.Fatalf("ASSERT_LIBRARY_MANIFEST_NOT_PASSED_AS_DIRECTORY: %q", log)
	}
	if bytes.Contains(log, []byte("-grammar-file\n")) {
		t.Fatalf("ASSERT_RUN_V1_OMITS_GRAMMAR_FILE: %q", log)
	}
	lines := strings.Split(strings.TrimSpace(string(log)), "\n")
	for i, line := range lines {
		if line == "-prompt-file" && i+1 < len(lines) {
			if _, err := os.Stat(filepath.Dir(lines[i+1])); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("private temp not cleaned: %q err=%v", lines[i+1], err)
			}
			return
		}
	}
	t.Fatal("prompt argv missing")
}

func TestRunnerV2ProcessExecutionProducesVersionedTerminalRecords(t *testing.T) {
	requireDarwinProcess(t)
	request, packet := validRunnerFixture(t)

	t.Run("success", func(t *testing.T) {
		cfg, logPath := processConfig(t, "#!/bin/sh\nprintf '%s' "+shellQuote(string(EncodeAdapterResultV2(`{"verdict":"COMPLETE","target_role":"role","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"behavior","consumer_relative":false},"boundary_contribution":"boundary","limitations":[]}`)))+"\n")
		cfg.ResponseVersion = ResponseVersionV2
		runner, err := NewRunner(cfg)
		if err != nil {
			t.Fatal(err)
		}
		got, err := runner.RunV2(context.Background(), request, packet, "attempt-v2")
		if err != nil {
			t.Fatal(err)
		}
		if got.Invocation.Status() != StatusSucceeded || got.Invocation.Response().ID() == "" || got.Response.ID() == "" {
			t.Fatalf("ASSERT_RUN_V2_SUCCESS_RECORDS: invocation=%q response=%q status=%s", got.Invocation.ID(), got.Response.ID(), got.Invocation.Status())
		}
		stdout := EncodeAdapterResultV2(`{"verdict":"COMPLETE","target_role":"role","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"behavior","consumer_relative":false},"boundary_contribution":"boundary","limitations":[]}`)
		stdoutSum := sha256.Sum256(stdout)
		if got.Invocation.StdoutSHA256() != "sha256:"+hex.EncodeToString(stdoutSum[:]) || got.Invocation.StderrSHA256() != emptyStreamSHA256V2 {
			t.Fatalf("ASSERT_RUN_V2_EXACT_STREAM_DIGESTS: stdout=%s stderr=%s", got.Invocation.StdoutSHA256(), got.Invocation.StderrSHA256())
		}
		argv, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(string(argv)), "\n")
		wantPair := []string{"-grammar-file", cfg.Grammar.Path}
		found := false
		for i := 0; i+1 < len(lines); i++ {
			if lines[i] == wantPair[0] && lines[i+1] == wantPair[1] {
				found = true
			}
		}
		if !found {
			t.Fatalf("ASSERT_RUN_V2_EXACT_GRAMMAR_ARG: want adjacent %q, got %q", wantPair, lines)
		}
	})

	t.Run("terminal failure has no response", func(t *testing.T) {
		cfg, _ := processConfig(t, "#!/bin/sh\nexit 42\n")
		cfg.ResponseVersion = ResponseVersionV2
		runner, err := NewRunner(cfg)
		if err != nil {
			t.Fatal(err)
		}
		got, runErr := runner.RunV2(context.Background(), request, packet, "attempt-v2")
		if failureCode(t, runErr) != CodeBackendFailure || got.Invocation.Status() != StatusBackendFailure || got.Invocation.Response().ID() != "" || got.Response.ID() != "" {
			t.Fatalf("ASSERT_RUN_V2_FAILURE_RESPONSE_FREE: result=%+v err=%v", got, runErr)
		}
	})

	t.Run("grammar failure has source-safe subcode", func(t *testing.T) {
		cfg, _ := processConfig(t, "#!/bin/sh\nexit 7\n")
		cfg.ResponseVersion = ResponseVersionV2
		runner, err := NewRunner(cfg)
		if err != nil {
			t.Fatal(err)
		}
		got, runErr := runner.RunV2(context.Background(), request, packet, "attempt-v2")
		var failure *Failure
		if !AsFailure(runErr, &failure) || failure.Stage() != StageRun || failure.Code() != CodeBackendFailure || failure.Subcode() != "GRAMMAR" {
			t.Fatalf("ASSERT_GRAMMAR_FAILURE_SUBCODE: result=%+v err=%v", got, runErr)
		}
	})

	t.Run("legacy semantic schema has source-safe subcode", func(t *testing.T) {
		legacy := `{"verdict":"SUPPORTED","target_role":"CaptureSnapshots","nearest_outward_consumer":"resumeWithHooks","consumer_need":"need","provided_behavior":"behavior","boundary_contribution":"boundary","limitations":[],"citations":[]}`
		cfg, _ := processConfig(t, "#!/bin/sh\nprintf '%s' "+shellQuote(string(EncodeAdapterResultV2(legacy)))+"\n")
		cfg.ResponseVersion = ResponseVersionV2
		runner, err := NewRunner(cfg)
		if err != nil {
			t.Fatal(err)
		}
		got, runErr := runner.RunV2(context.Background(), request, packet, "attempt-v2")
		var failure *Failure
		if !AsFailure(runErr, &failure) || failure.Stage() != StageOutput || failure.Code() != CodeOutputInvalid || failure.Subcode() != "SEMANTIC_SCHEMA_MISMATCH" {
			t.Fatalf("ASSERT_V2_SEMANTIC_SCHEMA_SUBCODE: result=%+v err=%v", got, runErr)
		}
		if got.Invocation.FailureSubcode() != failure.Subcode() {
			t.Fatalf("ASSERT_RUN_V2_FAILURE_SUBCODE_CUSTODY: invocation=%q failure=%q", got.Invocation.FailureSubcode(), failure.Subcode())
		}
		if got.Invocation.Status() != StatusOutputInvalid || got.Invocation.Response().ID() != "" || got.Response.ID() != "" {
			t.Fatalf("ASSERT_V2_SCHEMA_FAILURE_RESPONSE_FREE: result=%+v", got)
		}
	})
}

func TestRunnerRetryIdentitiesAndAliases(t *testing.T) {
	requireDarwinProcess(t)
	req := validRunnerRequest(t)
	cfg, _ := processConfig(t, "#!/bin/sh\nprintf '%s' "+shellQuote(workerJSON())+"\n")
	r, err := NewRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	a, err := r.Run(context.Background(), req, "attempt-a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.Run(context.Background(), req, "attempt-b")
	if err != nil {
		t.Fatal(err)
	}
	if a.Invocation.ID() == b.Invocation.ID() || a.Response.ID() != b.Response.ID() {
		t.Fatalf("retry identities: inv %q/%q response %q/%q", a.Invocation.ID(), b.Invocation.ID(), a.Response.ID(), b.Response.ID())
	}
	semantic := a.Response.Response()
	semantic.Limitations[0] = "mutated"
	raw, _ := a.Response.Bytes()
	if bytes.Contains(raw, []byte("mutated")) {
		t.Fatal("response alias mutation changed immutable record")
	}
}

func TestRunnerLiveOutputBoundsAndStrictDecode(t *testing.T) {
	requireDarwinProcess(t)
	req := validRunnerRequest(t)
	valid := workerJSON()
	cases := map[string]string{
		"trailing": valid + "{}\n", "bom": "\357\273\277" + valid, "malformed": "{\n",
		"duplicate":      strings.Replace(valid, `{"status":`, `{"status":"COMPLETE","status":`, 1),
		"unknown outer":  strings.Replace(valid, `{"status":`, `{"unknown":0,"status":`, 1),
		"unknown nested": strings.Replace(valid, `\"citations\":`, `\"unknown\":0,\"citations\":`, 1),
	}
	for name, output := range cases {
		t.Run(name, func(t *testing.T) {
			cfg, _ := processConfig(t, "#!/bin/sh\nprintf '%s' "+shellQuote(output)+"\n")
			r, err := NewRunner(cfg)
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.Run(context.Background(), req, "attempt")
			if failureCode(t, err) != CodeOutputInvalid || got.Invocation.Status() != StatusOutputInvalid || !got.Invocation.Receipt().Reaped {
				t.Fatalf("invalid mapping: result=%+v err=%v", got, err)
			}
		})
	}
	t.Run("exact boundary", func(t *testing.T) {
		cfg, _ := processConfig(t, "#!/bin/sh\nprintf '%s' "+shellQuote(valid)+"\n")
		cfg.Limits.StdoutBytes = len(valid)
		r, err := NewRunner(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := r.Run(context.Background(), req, "attempt"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("over boundary", func(t *testing.T) {
		cfg, _ := processConfig(t, "#!/bin/sh\nprintf '%sX' "+shellQuote(valid)+"\n")
		cfg.Limits.StdoutBytes = len(valid)
		r, err := NewRunner(cfg)
		if err != nil {
			t.Fatal(err)
		}
		got, err := r.Run(context.Background(), req, "attempt")
		if failureCode(t, err) != CodeResourceLimit || got.Invocation.Status() != StatusResourceLimit || !got.Invocation.Receipt().StdoutTruncated || got.Response.ID() != "" {
			t.Fatalf("over-bound mapping: result=%+v err=%v", got, err)
		}
	})
}

func TestRunnerCancellationTimeoutReapsDescendants(t *testing.T) {
	requireDarwinProcess(t)
	for _, tc := range []struct {
		name   string
		cancel bool
		want   Code
		status TerminalStatus
	}{{"cancel", true, CodeCancelled, StatusCancelled}, {"timeout", false, CodeTimeout, StatusTimeout}} {
		t.Run(tc.name, func(t *testing.T) {
			for attempt := 0; attempt < 5; attempt++ {
				attemptDir := t.TempDir()
				pidFile := filepath.Join(attemptDir, fmt.Sprintf("pid-%d", attempt))
				readyFIFO := filepath.Join(attemptDir, fmt.Sprintf("ready-%d", attempt))
				if tc.cancel {
					if err := syscall.Mkfifo(readyFIFO, 0600); err != nil {
						t.Fatal(err)
					}
				}
				worker := "#!/bin/sh\n(sleep 30) &\necho $! > " + shellQuote(pidFile) + "\n"
				if tc.cancel {
					worker += "printf ready > " + shellQuote(readyFIFO) + "\n"
				}
				worker += "wait\n"
				cfg, logPath := processConfig(t, worker)
				if tc.cancel {
					cfg.Limits.TimeoutMS = 1500
				} else {
					cfg.Limits.TimeoutMS = 1000
				}
				r, err := NewRunner(cfg)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				if tc.cancel {
					go func() {
						ready, err := os.ReadFile(readyFIFO)
						if err != nil || string(ready) != "ready" {
							return
						}
						cancel()
					}()
				}
				got, runErr := r.Run(ctx, validRunnerRequest(t), "attempt")
				cancel()
				if failureCode(t, runErr) != tc.want || got.Invocation.Status() != tc.status || !got.Invocation.Receipt().Reaped || !got.Invocation.Receipt().Teardown {
					t.Fatalf("terminal mapping: result=%+v err=%v", got, runErr)
				}
				pidRaw, err := os.ReadFile(pidFile)
				if err != nil {
					t.Fatal(err)
				}
				pid, _ := strconv.Atoi(strings.TrimSpace(string(pidRaw)))
				if err := syscall.Kill(pid, 0); err == nil {
					t.Fatalf("descendant survived: pid=%d", pid)
				}
				log, _ := os.ReadFile(logPath)
				parts := strings.Split(strings.TrimSpace(string(log)), "\n")
				for i, p := range parts {
					if p == "-prompt-file" && i+1 < len(parts) {
						if _, err := os.Stat(filepath.Dir(parts[i+1])); !errors.Is(err, os.ErrNotExist) {
							t.Fatalf("temp survived: %v", err)
						}
					}
				}
			}
		})
	}
}

func TestRunnerStderrAndExitMapping(t *testing.T) {
	requireDarwinProcess(t)
	req := validRunnerRequest(t)
	t.Run("stderr truncated source safe", func(t *testing.T) {
		secret := req.Envelope.Prompt + "/private/path"
		cfg, _ := processConfig(t, "#!/bin/sh\nprintf '%s' "+shellQuote(strings.Repeat(secret, 3))+" >&2\nprintf '%s' "+shellQuote(workerJSON())+"\n")
		cfg.Limits.StderrBytes = 17
		r, err := NewRunner(cfg)
		if err != nil {
			t.Fatal(err)
		}
		got, runErr := r.Run(context.Background(), req, "attempt")
		if failureCode(t, runErr) != CodeResourceLimit {
			t.Fatalf("err=%v", runErr)
		}
		receipt := got.Invocation.Receipt()
		if receipt.StderrBytes != 17 || !receipt.StderrTruncated || !receipt.Reaped || strings.Contains(runErr.Error(), secret) || strings.Contains(runErr.Error(), "/private/path") {
			t.Fatalf("receipt=%+v err=%v", receipt, runErr)
		}
	})
	for _, tc := range []struct {
		name, output string
		exit         int
		want         Code
		status       TerminalStatus
		subcode      string
	}{
		{"adapter protocol", "", 2, CodeBackendFailure, StatusBackendFailure, "ADAPTER_PROTOCOL"},
		{"model load", "", 4, CodeBackendFailure, StatusBackendFailure, "MODEL_LOAD"},
		{"prompt limit", "", 6, CodeBackendFailure, StatusBackendFailure, "PROMPT_LIMIT"},
		{"grammar init", workerJSON(), 7, CodeBackendFailure, StatusBackendFailure, "GRAMMAR"},
		{"supervision", "", 42, CodeBackendFailure, StatusBackendFailure, "SUPERVISION"},
		{"zero partial json", "{", 0, CodeOutputInvalid, StatusOutputInvalid, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _ := processConfig(t, "#!/bin/sh\nprintf '%s' "+shellQuote(tc.output)+"\nexit "+strconv.Itoa(tc.exit)+"\n")
			r, err := NewRunner(cfg)
			if err != nil {
				t.Fatal(err)
			}
			got, runErr := r.Run(context.Background(), req, "attempt")
			if failureCode(t, runErr) != tc.want || got.Invocation.Status() != tc.status || !got.Invocation.Receipt().Reaped {
				t.Fatalf("result=%+v err=%v", got, runErr)
			}
			if tc.subcode != "" {
				subcoded, ok := any(runErr).(interface{ Subcode() string })
				if !ok || subcoded.Subcode() != tc.subcode {
					t.Fatalf("ASSERT_BACKEND_SUBCODE_CLASSIFIED: exit=%d want=%s", tc.exit, tc.subcode)
				}
			}
		})
	}
}

func TestRunnerRevalidatesPinsImmediatelyBeforeExecution(t *testing.T) {
	requireDarwinProcess(t)
	for _, pin := range []string{"sandbox", "profile", "worker", "model", "lib", "grammar"} {
		t.Run(pin, func(t *testing.T) {
			cfg, logPath := processConfig(t, "#!/bin/sh\nexit 0\n")
			r, err := NewRunner(cfg)
			if err != nil {
				t.Fatal(err)
			}
			var path string
			switch pin {
			case "sandbox":
				path = cfg.SandboxExecutable.Path
			case "profile":
				path = cfg.SandboxProfile.Path
			case "worker":
				path = cfg.Worker.Path
			case "model":
				path = cfg.Model.Path
			case "lib":
				path = cfg.Library.Path
			case "grammar":
				path = cfg.Grammar.Path
			}
			if err := os.WriteFile(path, []byte("post-preflight replacement"), 0700); err != nil {
				t.Fatal(err)
			}
			_, err = r.Run(context.Background(), validRunnerRequest(t), "attempt")
			if failureCode(t, err) != CodeModelUnavailable {
				t.Fatalf("ASSERT_POST_PREFLIGHT_PIN_REVALIDATED: pin=%s err=%v", pin, err)
			}
			if _, err := os.Stat(logPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("ASSERT_POST_PREFLIGHT_PIN_REVALIDATED: pin=%s unsafe execution occurred: %v", pin, err)
			}
		})
	}
}

func TestRunnerPinsFailBeforeExecution(t *testing.T) {
	requireDarwinProcess(t)
	cfg, logPath := processConfig(t, "#!/bin/sh\nexit 0\n")
	for _, field := range []string{"sandbox", "profile", "worker", "model", "lib"} {
		t.Run(field, func(t *testing.T) {
			bad := cfg
			switch field {
			case "sandbox":
				bad.SandboxExecutable.SHA256 = digestA
			case "profile":
				bad.SandboxProfile.SHA256 = digestA
			case "worker":
				bad.Worker.SHA256 = digestA
			case "model":
				bad.Model.SHA256 = digestA
			case "lib":
				bad.Library.SHA256 = digestA
			}
			if r, err := NewRunner(bad); err == nil || r != nil {
				t.Fatal("pin mismatch accepted")
			}
			if _, err := os.Stat(logPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unsafe execution occurred: %v", err)
			}
		})
	}
}
