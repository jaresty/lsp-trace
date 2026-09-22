package describeworker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/managedprocess"
)

type Runner struct {
	validated ValidatedConfig
	manager   *managedprocess.Manager
}
type RunResult struct {
	Invocation InvocationRecord
	Response   ResponseRecord
}
type workerResult struct {
	Status        string  `json:"status"`
	Text          string  `json:"text"`
	Tokens        int     `json:"tokens"`
	LoadMS        float64 `json:"load_ms"`
	RunMS         float64 `json:"run_ms"`
	Cancelled     bool    `json:"cancelled"`
	DecodeStatus  int32   `json:"decode_status"`
	Error         string  `json:"error"`
	Grammar       string  `json:"grammar"`
	ContextTokens int     `json:"context_tokens"`
}

func NewRunner(c Config) (*Runner, error) {
	v, err := Preflight(c)
	if err != nil {
		return nil, err
	}
	m, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: c.Limits.StderrBytes, GracePeriod: 100 * time.Millisecond})
	if err != nil {
		return nil, failure(StagePreflight, CodePolicyMismatch)
	}
	return &Runner{validated: v, manager: m}, nil
}
func (r *Runner) Run(ctx context.Context, request describerequest.Record, attemptID string) (RunResult, error) {
	if r == nil || r.manager == nil {
		return RunResult{}, failure(StageStart, CodePolicyMismatch)
	}
	if err := describerequest.Validate([]describerequest.Record{request}); err != nil {
		return RunResult{}, failure(StagePreflight, CodePolicyMismatch)
	}
	if attemptID == "" || len(attemptID) > 256 {
		return RunResult{}, failure(StagePreflight, CodePolicyMismatch)
	}
	c := r.validated.config
	if err := verifyConfigPins(c); err != nil {
		return RunResult{}, failureWithSubcode(StagePreflight, CodeModelUnavailable, err.Error())
	}
	dir, err := os.MkdirTemp("", "lsp-trace-describe-")
	if err != nil {
		return RunResult{}, failure(StageStart, CodeBackendFailure)
	}
	defer os.RemoveAll(dir)
	if err = os.Chmod(dir, 0700); err != nil {
		return RunResult{}, failure(StageStart, CodeBackendFailure)
	}
	prompt := []byte(request.Envelope.Prompt)
	if len(prompt) > c.Limits.TempBytes {
		return RunResult{}, failure(StageRun, CodeResourceLimit)
	}
	promptPath := filepath.Join(dir, "prompt.txt")
	if err = os.WriteFile(promptPath, prompt, 0600); err != nil {
		return RunResult{}, failure(StageStart, CodeBackendFailure)
	}
	args := []string{"-f", c.SandboxProfile.Path, c.Worker.Path, "-model", c.Model.Path, "-lib", filepath.Dir(c.Library.Path), "-prompt-file", promptPath, "-timeout", (time.Duration(c.Limits.TimeoutMS) * time.Millisecond).String(), "-max-tokens", strconv.Itoa(c.Limits.MaxTokens), "-context-tokens", strconv.Itoa(c.Limits.ContextTokens)}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(c.Limits.TimeoutMS)*time.Millisecond)
	defer cancel()
	process, start := r.manager.Start(runCtx, managedprocess.Spec{Path: c.SandboxExecutable.Path, Args: args, Dir: dir, Env: []string{"PATH=/usr/bin:/bin", "HOME=" + dir, "TMPDIR=" + dir, "LANG=C", "LC_ALL=C"}})
	if start.Kind != managedprocess.StartStarted {
		return RunResult{}, failure(StageStart, CodeBackendFailure)
	}
	receipt := ResourceReceipt{Started: true, TerminalOutcomes: 1}
	var out limitedCapture
	out.limit = c.Limits.StdoutBytes
	readDone := make(chan error, 1)
	go func() { _, e := io.Copy(&out, process.Stdout()); readDone <- e }()
	deathDone := make(chan managedprocess.DeathObservation, 1)
	go func() { deathDone <- process.Wait() }()
	var death managedprocess.DeathObservation
	select {
	case death = <-deathDone:
	case <-runCtx.Done():
		receipt.Teardown = true
		td := process.Teardown(context.Background())
		death = td.Death
		receipt.Reaped = td.Death.Reap.Kind == managedprocess.ReapComplete
	}
	_ = process.Close()
	<-readDone
	receipt.StdoutBytes = len(out.bytes)
	receipt.StdoutTruncated = out.truncated
	receipt.StderrBytes = len(death.Stderr.Bytes)
	receipt.StderrTruncated = death.Stderr.Truncated
	if death.Reap.Kind == managedprocess.ReapComplete {
		receipt.Reaped = true
	}
	status := StatusSucceeded
	code := Code("")
	if ctx.Err() != nil {
		status = StatusCancelled
		code = CodeCancelled
	} else if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		status = StatusTimeout
		code = CodeTimeout
	} else if out.truncated || death.Stderr.Truncated {
		status = StatusResourceLimit
		code = CodeResourceLimit
	} else if death.Kind != managedprocess.DeathExited || death.ExitCode != 0 {
		status = StatusBackendFailure
		code = CodeBackendFailure
	}
	binding := bindingFrom(c, request, attemptID, prompt)
	if code != "" {
		inv, _ := NewInvocationRecord(binding, status, receipt, zeroResponse(binding))
		if code == CodeBackendFailure {
			return RunResult{Invocation: inv}, backendFailure(classifyBackendExit(death))
		}
		return RunResult{Invocation: inv}, failure(StageRun, code)
	}
	wr, err := parseWorkerResult(out.bytes)
	if err != nil {
		inv, _ := NewInvocationRecord(binding, StatusOutputInvalid, receipt, zeroResponse(binding))
		return RunResult{Invocation: inv}, failure(StageOutput, CodeOutputInvalid)
	}
	response, err := NewResponseRecord(binding, wr.semantic())
	if err != nil {
		return RunResult{}, failure(StageRecord, CodeOutputInvalid)
	}
	inv, err := NewInvocationRecord(binding, StatusSucceeded, receipt, response)
	if err != nil {
		return RunResult{}, failure(StageRecord, CodeBackendFailure)
	}
	return RunResult{Invocation: inv, Response: response}, nil
}
func classifyBackendExit(death managedprocess.DeathObservation) string {
	if death.Kind != managedprocess.DeathExited {
		return "SUPERVISION"
	}
	switch death.ExitCode {
	case 2:
		return "ADAPTER_PROTOCOL"
	case 3, 4, 5:
		return "MODEL_LOAD"
	case 6:
		return "PROMPT_LIMIT"
	case 7, 10:
		return "GRAMMAR"
	case 9:
		return "OUTPUT_PARSE"
	default:
		return "SUPERVISION"
	}
}

func bindingFrom(c Config, r describerequest.Record, attempt string, prompt []byte) IdentityBinding {
	sum := sha256.Sum256(prompt)
	return IdentityBinding{RequestRecordID: r.RecordID, MessageID: r.Envelope.MessageID, AttemptID: attempt, WorkerSHA256: c.Worker.SHA256, ModelSHA256: c.Model.SHA256, LibrarySHA256: c.Library.SHA256, SandboxExecutableSHA256: c.SandboxExecutable.SHA256, SandboxProfileSHA256: c.SandboxProfile.SHA256, RuntimeIdentity: c.RuntimeIdentity, AdapterIdentity: c.AdapterIdentity, ModelIdentity: c.ModelIdentity, PromptSHA256: "sha256:" + hex.EncodeToString(sum[:]), GrammarSHA256: c.Grammar.SHA256, Limits: c.Limits}
}
func zeroResponse(b IdentityBinding) ResponseRecord {
	b.AttemptID = ""
	r, _ := NewResponseRecord(b, SemanticResponse{Verdict: "UNAVAILABLE", TargetRole: "unavailable", NearestOutwardConsumer: "unavailable", ConsumerNeed: "unavailable", ProvidedBehavior: "unavailable", BoundaryContribution: "unavailable", Limitations: []string{"terminal failure"}, Citations: []string{}})
	return r
}
func parseWorkerResult(raw []byte) (workerResult, error) {
	if len(raw) < 2 || raw[len(raw)-1] != '\n' || raw[len(raw)-2] == '\n' || bytes.HasPrefix(raw, []byte{0xef, 0xbb, 0xbf}) {
		return workerResult{}, errors.New("worker framing invalid")
	}
	line := raw[:len(raw)-1]
	var w workerResult
	if err := strictCanonical(line, &w); err != nil {
		return w, err
	}
	if w.Status != "COMPLETE" && w.Status != "TIMEOUT" && w.Status != "CANCELLED" && w.Status != "MODEL_UNAVAILABLE" && w.Status != "ERROR" {
		return w, errors.New("worker status invalid")
	}
	if w.Tokens < 0 || w.LoadMS < 0 || w.RunMS < 0 || w.ContextTokens < 0 {
		return w, errors.New("worker accounting invalid")
	}
	if _, err := w.semanticStrict(); err != nil {
		return w, err
	}
	return w, nil
}
func (w workerResult) semanticStrict() (SemanticResponse, error) {
	var s SemanticResponse
	if err := strictCanonical([]byte(w.Text), &s); err != nil {
		return s, err
	}
	return s, validateSemantic(s)
}
func (w workerResult) semantic() SemanticResponse { s, _ := w.semanticStrict(); return s }

type limitedCapture struct {
	limit     int
	bytes     []byte
	truncated bool
}

func (b *limitedCapture) Write(p []byte) (int, error) {
	n := len(p)
	remaining := b.limit - len(b.bytes)
	if remaining > 0 {
		take := len(p)
		if take > remaining {
			take = remaining
		}
		b.bytes = append(b.bytes, p[:take]...)
	}
	if len(p) > remaining {
		b.truncated = true
	}
	return n, nil
}

var _ = fmt.Sprintf
var _ = json.Valid
